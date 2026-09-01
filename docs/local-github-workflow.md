# 本地 GitHub Fork 工作流

> 对应分支:`personal` 基线 `317e9ddd`(2026-09-01 刷新至 `77efd0936`;`personal` 线同步流程见「同步上游」节)

## 标准触发短语

向维护助手说 **「同步上游到 deploy」**(参考 `docs/local-github-workflow.md`) 即按本文档执行完整同步流程:fetch upstream → **`git rebase upstream/main deploy`** 重放魔改提交 → 逐提交按「保留魔改 + 采纳上游语义」解决冲突 → 合并后验证 → force-push origin deploy → 更新本文档头部标记。**默认同时同步 `main`**(merge upstream/main 后推送)。

可选追加：`不同步 main`（仅同步 deploy）、`不更新文档`（跳过头部标记更新）、`跳过验证`（不推荐）。

本仓库只保留两类远程：`origin` (个人 fork) 和 `upstream` (官方上游)。

## 分支策略

| 分支 | 角色 | 说明 |
|------|------|------|
| `main` | 贴近上游 | 仅保留少量本地改动（如 GHCR workflow 文件），几乎与上游同步 |
| `deploy` | 部署分支 | 承载全部魔改功能；GHCR 部署镜像由它构建 |
| `personal` | 个人开发线 | 历史前段（至 `317e9ddd`，原 deploy-model 中间态分支，已退役并入）承载：deploy 同步战略改造、模型级路由表前台化、model group 管理接口；其后叠加：模型组路由全套 + 计费/Ollama/订阅/OAuth/开放注册移除。与 `deploy` 共同祖先为 `2ffa3979`（原 deploy-re，已退役）。不构建镜像、不打 tag |
| `local/<feature>` | 本地定制功能分支 | 开发完成后合并进目标魔改分支 |

魔改功能清单见 `request-debug-customization-manifest.md`；魔改功能不在 `main` 上。`deploy` 与 `personal` 线已分叉，各自独立维护与同步上游；manifest 中 deploy 线功能登记以 personal 基线 `317e9ddd`（原 deploy-model，已退役）为基准，personal 改动单独登记在「personal 分支半重构登记」小节。

## 同步上游

### personal（rebase 原则同 deploy）

```bash
git fetch upstream
git checkout personal
git rebase upstream/main
# 冲突处理原则与下方 deploy 相同；完成后 force-with-lease 推送
git push --force-with-lease origin personal
```

- personal 线的魔改提交序列是自包含线性链，逐提交重放原则与 deploy 一致。
- personal 不触发 GHCR 构建，无需打 tag。

### main（一般无冲突，直接合并）

```bash
git fetch upstream
git checkout main
git merge upstream/main
git push origin main
```

### deploy（可能冲突，按下方原则解决）

```bash
git fetch upstream
git checkout deploy
git rebase upstream/main
# 有冲突 → 见"冲突处理原则"。逐提交解决：当前正在重放的魔改提交与上游本次
#           改动的冲突；解决后 git add + git rebase --continue 进入下一个提交
# 任何时刻可 git rebase --abort 整体回退到同步前状态
# 无冲突 → 仍建议执行"合并后验证"
git push --force-with-lease origin deploy
```

补充说明（决策与影响）：

- **为何 rebase 而非 merge**：冲突从「一大坨对冲」变为「逐魔改 commit 的小冲突」，定位靠 `git log --oneline upstream/main..deploy` 预扫；失败可整体回退（merge 只能回滚合并）。代价是重写 deploy 历史。
- **force-push 影响面**：GHCR 镜像由 `deploy-image` / `deploy-image-<sha>` git tag 触发构建，tag 不随分支重写；部署机只从 GHCR 拉镜像，不受分支历史重写影响。文档头部 commit 标记在每次同步后更新即可。
- **push 命令用 `--force-with-lease` 而非 `-f`**，防止覆盖他人/他机推送。
- 「合并后验证」三件套（root go build / relaykit build / bun build）在 rebase 完成后照常执行，位置不变。

#### 冲突处理原则

魔改与上游改动重叠时：**保留魔改功能 + 采纳上游语义**。逐处判断：

- 魔改独有代码（如渠道限流检查）必须保留；
- 上游新增/修改的逻辑（如计费调用、结构体字段注释）必须采纳；
- 两者位置相邻时按业务顺序拼接，不互相覆盖。

已知高风险文件（上游改动最易与魔改冲突）：

| 文件 | 魔改内容 | 上游易冲突点 | 解决决策记录 |
|------|----------|--------------|--------------|
| `controller/relay.go` | 渠道限流检查、同优先级重试 `ExcludeChannel`、全渠道限流 429 兜底、（新增）模型级禁用分支 | 重试/计费逻辑（如 `PrepareTieredBillingForSelectedGroup`） | `e0b9f243`：2026-08-01 同步 8 提交；重试/计费逻辑拼序采纳、限流检查保留；`cd98b0f8`：2026-08-19 同步 0 提交；`processChannelError` 既有禁用两行包进 else、前插模型级 if 分支（包 else 非纯追加，冲突时按「保留魔改 + 采纳上游语义」手动合并）；`7dccc6db4`：2026-08-31 同步 21 提交；上游把 task 提交重构成 `executeTaskSubmissionWith`（参数注入 submit），魔改 `ExcludeChannel`+`UpdateUpstreamModel` 织入新函数体，`requestId` 因上游把 `service.Start` 移出作用域需在函数头补局部声明；计费断言（settle 事件）按免费语义适配测试 |
| `controller/channel-test.go` | 模型级禁用/恢复分支、`ShouldDisableChannelWithDecision`、`processChannelError` 第 4 参 `nil` | 上游 `4add708e` 把单渠道测试重构成 `runChannelTestWorkers` worker 池，循环体整体搬家 | `235ae5a7`：2026-08-24 同步 6 提交；上游重构后的新调用点逐处补魔改行（判定换 `WithDecision`、模型级 if/else 包裹、enable 块后插模型级恢复块、`performChannelTests` 调用前插 `recoverExpiredModelBans` 且保留上游新增的 `concurrency` 参数）；`949e1e69`：2026-08-29 同步 6 提交；上游 int32 重构移除 `math.Round`，魔改随机化提交新增 `math/rand/v2` 后 `math` 成孤儿，autosquash 修正 |
| `service/quota.go` | 计费功能级移除（`PreWssConsumeQuota` 等短路、`strings` import 随之删除） | 上游 `a073f74b` int32 重构把 `math.Round` 换成 `common.QuotaRoundChecked` 并调整 import | `949e1e69`：2026-08-29 同步 6 提交；import 冲突净结果为两侧删除（`math` 被上游移除用法、`strings` 被魔改移除用法），`CalcOpenRouterCacheCreateTokens` 采纳上游 `QuotaRoundChecked` 饱和语义 |
`model/ability.go` | `getChannelQuery` 简化（MAX(priority) 顶层、删 getPriority）+ DB 路径注释 | 上游把 `GetChannel` 重构为 `filters []dto.ChannelFilter` + 全量查询 + `filterAbilitiesByConstraints` | `7dccc6db4`：2026-08-31 同步 21 提交；`GetChannel` 采纳上游 filters 形态（魔改 getChannelQuery 顶层查询会丢 filters 语义），魔改同层重滚语义由 cache 主路径 exclude 机制承载；`getChannelQuery` 简化保留（上游已删 getPriority 引用）；`db1f5dae9`：2026-09-01 补修；filters 重构时 `GetChannel` 丢失了 `channel_disabled_models` 的 `NOT EXISTS` 过滤（DB 路径模型级禁用失效），现于查询内补回（此前仅 cache 主路径 carry 禁用语义） |
| `model/channel_cache.go` | `GetRandomSatisfiedChannel` 追加 `excludeChannels` 参数 + exclude 过滤块；模型组接管路由（overrides/effectivePriority/`GetRandomSatisfiedChannelFromGroups` DB fallback） | 上游把签名重构为 `filters []dto.ChannelFilter` + `filterCandidateIDs` | `7dccc6db4`：2026-08-31 同步 21 提交；签名取上游 filters 版再追加 `excludeChannels` 尾参，调用点（service/channel_select.go 两处）拼接 filters+excludeChannels；模型组新增函数块整体保留 |
| `web/src/features/system-settings/models/routing-reliability-section.tsx` | 滑动窗口禁用 4 字段、慢流/TTFT 降级配置（`channel_slow_stream_setting`） | 上游把 schema 重构为 `createRoutingReliabilitySchema(t)` 工厂 + i18n 化 | `235ae5a7`：2026-08-24 同步 6 提交；schema 一律取上游工厂版再插入魔改字段（含 superRefine 两条采样校验）；i18n locale 冲突用语义三方合并（保留上游新增键、应用魔改键变更） |
| i18n locale（`web/src/i18n/locales/*.json`） | 魔改 UI 文案键 | 上游同区段增删键导致整块冲突 | `235ae5a7`：2026-08-24 同步 6 提交；用 `scripts/i18n_3way_merge.py` 语义合并（ours 为底 + theirs 增改覆盖），键序以 ours 为准，事后 `bun run i18n:sync` 归位 |
| `relay/common/relay_info.go` | `RequestDebugSnapshot` 字段 | `RelayInfo` 结构体字段增减、注释更新 | `7dccc6db4`：2026-08-31 同步 21 提交；上游字段增删自动合并，`RequestDebugSnapshot` 保留无冲突 |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | 请求调试快照面板 | 日志详情功能（如 stream status） | `7dccc6db4`：2026-08-31 同步 21 提交；上游新增 task_plugin 计费展示块（BillingBreakdown/DynamicPricingBreakdown/usage-facts），按计费移除语义整块弃用；魔改 request_debug 面板与上游 PluginAuthorLink 共存，import 拼接保留两侧 |
| `model/main.go` | PostgreSQL 连接强制 `PreferSimpleProtocol: true`（兼容 PgBouncer/Neon/Supabase）+ `normalizePostgresDSN` 兜底 `client_encoding=UTF8` + `standard_conforming_strings=on` | 上游新增 `ensureUserQuotaColumns`、`migratePrefillGroupUniqueness`（参数化 raw SQL）与 `migrateDBFast` 删除 | `7667fe5b3`：2026-09-01 fix；上游 `migratePrefillGroupUniqueness` 的 `db.Raw(...to_regclass(?) ...)` 参数化查询在 personal `PreferSimpleProtocol` + PG 连接编码非 UTF8 环境触发 pgx `sanitizeForSimpleQuery` 强制 `client_encoding=UTF8` 与 `standard_conforming_strings=on`（conn.go:1265-1269）→ 启动 FATAL；新增 `normalizePostgresDSN` 兜底给 DSN 追加两 runtime param，已显式携带时尊重用户值不覆盖 |

#### 冲突决策记录规则

每次 rebase/merge 解完冲突后，把「上游改了啥 / 本地怎么拼 / 结论一句话」追加到对应文件行的「解决决策记录」列（格式：`<提交短SHA>：<日期> 同步 N 提交；<本次决策一句话>`）。列内多记录用 `；` 分隔。记忆原则：同类冲突再次出现时，先查本表照抄决策，不再重新设计。

历史参考：2026-08-01 同步 8 个上游提交时，前两个文件各产生一处冲突，处理方式记录在合并提交 `e0b9f243`。

#### 同步后终态核对（防静默错合并，先于构建执行）

三方合并可能「无冲突但错」：重放提交的 hunk 被静默丢弃、或残留已被后续魔改重构淘汰的代码（2026-08-24 同步时 `ea4f0210` 即发生过，靠 build 才暴露）。因此 rebase 完成后、跑构建前，必须对**本次解过冲突的每个文件**做终态核对：

```bash
git diff origin/<branch> -- <冲突文件>
# 差异必须能逐行解释为「上游窗口内的改动」；解释不了的差异逐行查明
```

原理：解过冲突的文件若合并正确，其内容 = 旧线终态 + 上游本次改动，diff 中不应出现其它内容。整文件级 `git checkout --ours/--theirs` 前也必须先用 `git diff <commit>^ <commit> -- <file>` 核实另一侧对该文件的真实改动范围，禁止盲用。

#### 合并后验证

```bash
/usr/local/go/bin/go build ./...                                 # 根模块（需 Go >= 1.25）
cd relaykit && GOWORK=off /usr/local/go/bin/go build ./...       # relaykit 独立模块（须独立可构建）
cd web && bun run build                                          # 前端
systemd-run --user --scope -p MemoryMax=1G -- /usr/local/go/bin/go test ./controller/... ./service/... ./relay/... ./common/... ./pkg/billingexpr/...
cd web && systemd-run --user --scope -p MemoryMax=1G -- bun run test
```

> 构建只证明可编译；计费/禁用/结算路径的合并正确性由测试兜底（AGENTS.md 计费不变量有回归要求）。已知预存在失败用例需先在旧线终态复跑确认非本次回归（`git worktree add /tmp/old origin/<旧tip>` 后同命令复跑），并在同步报告中注明。

## 部署

> **强约束：只有用户明确要求触发构建/发布时才执行本流程。** 纯文档改动（`docs/`、`AGENTS.md`、manifest 登记、说明性提交）不触发构建——只 `git push origin <branch>` 即可，不推送任何 `deploy*` / `personal*` tag。判断依据：改动是否影响运行产物（Go 源码、前端源码、Dockerfile、依赖清单等）；仅文档/注释变更视为不触发。

`.github/workflows/deploy-image-ghcr.yml` 统一服务 deploy / personal 两条线，镜像 tag 由构建来源动态推导：

| 构建来源 | 推导规则 | 镜像滚动 tag | 镜像留档 tag |
|---------|---------|-------------|-------------|
| tag `deploy-image` | 去 `-image` 尾缀 | `:deploy` | `:deploy-<short_sha>` |
| tag `personal-image` | 去 `-image` 尾缀 | `:personal` | `:personal-<short_sha>` |
| dispatch `branch=deploy` | 直接用输入 | 同上 | 同上 |
| dispatch `branch=personal` | 直接用输入 | 同上 | 同上 |

### personal（主力线）

```bash
git push origin personal                                       # 推送代码（不触发构建）
git tag -f personal-image && git push -f origin personal-image # 覆盖滚动 tag → 自动构建
```

> 与 deploy 线完全对称：滚动 tag 用 `personal-image`（避免与分支同名歧义），单次构建同时产出 `:personal`（滚动）与 `:personal-<short_sha>`（留档）。重复发布直接 `git tag -f` 覆盖；回滚用已知良好的 `:personal-<short_sha>`。Prune 只对带 `personal*` tag 版本计数（保留最近 3 个），不与 `deploy` 线互相挤占。

也可通过 Actions UI 手动触发：GitHub → **Actions** → **Build branch image (GHCR)** → **Run workflow** → branch 输入 `personal`（默认值）。

### deploy（兼容历史）

```bash
git push origin deploy                                         # 推送代码（不触发构建）
git tag -f deploy-image && git push -f origin deploy-image     # 覆盖滚动 tag → 自动构建
```

> 逻辑与 personal 线完全对称；镜像 tag 前缀推导结果不变（`:deploy` / `:deploy-<short_sha>`），deploy 线历史镜像地址完全兼容。

### 通用说明

1. 推送 `deploy*` / `personal*` tag 后，GitHub Actions 自动构建，无需进网页
2. 构建产物（`<owner>` 为仓库属主小写）：
   - `ghcr.io/<owner>/new-api:<prefix>` — 滚动 tag，始终指向最新构建
   - `ghcr.io/<owner>/new-api:<prefix>-<short_sha>` — 不可变 tag，对应具体提交
   - 镜像内 `VERSION` 文件 = `<prefix>-<short_sha>`
3. 部署机只从 GHCR 拉取镜像，不从 Git 仓库构建：
   - 日常更新：拉 `:<prefix>`
   - 回滚：拉上一个已知良好的 `:<prefix>-<short_sha>`
4. 两条线 Prune 独立：deploy 构建只清理带 `deploy*` tag 的历史版本，personal 构建只清理带 `personal*` tag 的历史版本，各保留最近 3 个

### 确认构建状态（无需 GitHub token）

本机无 GitHub token / gh CLI 时，无法命令行直查 Actions 页面，但仓库是公开的，可用 GitHub 公开 API 定时轮询构建状态（未认证限流 60 次/小时，足够轮询）：

```bash
# 查询最近按 tag push 触发的构建（head_branch 即 tag 名）
curl -s "https://api.github.com/repos/<owner>/new-api/actions/runs?event=push&per_page=5" \
  | jq '.workflow_runs[] | {name, tag: .head_branch, sha: (.head_sha[0:7]), status, conclusion}'
```

- `status`: `queued` / `in_progress` / `completed`
- `conclusion`: 完成后为 `success` / `failure`；进行中为 `null`
- 按 tag 过滤：`?event=push&branch=deploy-image`（`head_branch` 即触发 tag 名）
- 轮询建议：每 30~60 秒一次，直到 `status == "completed"`；`conclusion == "success"` 即构建成功，可通知部署机拉取新镜像

## 定制功能分支合并

```bash
git checkout deploy
git merge local/<feature>
git push origin deploy              # 推送代码本身不触发构建
git tag -f deploy-image && git push -f origin deploy-image    # 覆盖滚动 tag 触发 GHCR 构建
```

personal 线同理：

```bash
git checkout personal
git merge local/<feature>
git push origin personal            # 推送代码本身不触发构建
git tag -f personal-image && git push -f origin personal-image    # 覆盖滚动 tag 触发 GHCR 构建
```

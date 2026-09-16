# 本地 GitHub Fork 工作流

> 对应分支:`personal` 基线 `317e9ddd`(2026-09-16 刷新至 `aedfa6f0f`;`personal` 线流程见「同步上游」与「选择性纳入上游」两节 —— 自 2026-09-16 起主力方式为后者)

## 标准触发短语

- **「纳入上游」**（**当前主力方式**，见「选择性纳入上游」节）：fetch upstream → 分类上游新提交 → 逐提交试投放探测冲突构成 → 只取与本线目标一致的提交 cherry-pick（`-x`）→ 三构建 + 测试验证 → 快进 personal → 登记 manifest。适合上游改动多为 bugfix/独立小功能时。
- **「同步上游」**（应急手段，见「同步上游」节）：按完整 rebase 流程处理，仅当需要上游某个强耦合大重构（如协议层重写）时才用。本流程代价已实测不可接受（130 处冲突/48 提交），非必要不触发。

两条路径可在不同轮次交替使用，互不冲突（判据见「与全量 rebase 的兼容性」）。

**同步上游**触发时：向维护助手说 **「同步上游」**(参考 `docs/local-github-workflow.md`) 即按本文档执行完整同步流程:fetch upstream → **`git rebase upstream/main personal`** 重放魔改提交 → 逐提交按「保留魔改 + 采纳上游语义」解决冲突 → 合并后验证 → force-push origin personal → 更新本文档头部标记。**默认同时同步 `main`**(merge upstream/main 后推送)。

可选追加：`不同步 main`（仅同步 personal）、`不更新文档`（跳过头部标记更新）、`跳过验证`（不推荐）。

本仓库只保留两类远程：`origin` (个人 fork) 和 `upstream` (官方上游)。

## 分支策略

| 分支 | 角色 | 说明 |
|------|------|------|
| `main` | 贴近上游 | 仅保留少量本地改动（如 GHCR workflow 文件），几乎与上游同步 |
| `personal` | 个人开发线（主力，唯一魔改线） | 历史前段（至 `317e9ddd`，原 deploy 线魔改基线，早期中间态分支 `deploy-model` 已退役并入）承载：同步战略改造、模型级路由表前台化、model group 管理接口；其后叠加：模型组路由全套 + 计费/Ollama/订阅/OAuth/开放注册移除。`deploy` 分支已于 2026-09-04 删除（留档 tag `deploy-image-*` 仅历史回滚）。GHCR 镜像由 `personal-image` / `personal-image-<short_sha>` 构建 |
| `local/<feature>` | 本地定制功能分支 | 开发完成后合并进 `personal` |

魔改功能清单见 `request-debug-customization-manifest.md`；魔改功能不在 `main` 上。manifest 中历史 deploy 线功能登记以 personal 基线 `317e9ddd`（原 deploy-model，已退役）为基准，personal 改动单独登记在「personal 分支半重构登记」小节。

## 同步上游

### personal（可能冲突，按下方原则解决）

```bash
git fetch upstream
git checkout personal
git rebase upstream/main
# 有冲突 → 见"冲突处理原则"。逐提交解决：当前正在重放的魔改提交与上游本次
#           改动的冲突；解决后 git add + git rebase --continue 进入下一个提交
# 任何时刻可 git rebase --abort 整体回退到同步前状态
# 无冲突 → 仍建议执行"合并后验证"
git push --force-with-lease origin personal
```

- personal 线的魔改提交序列是自包含线性链，逐提交重放。
- personal 已支持 GHCR 构建：推送分支本身不触发构建，需另推 `personal-image` 滚动 tag（见「部署」节）。

### main（一般无冲突，直接合并）

```bash
git fetch upstream
git checkout main
git merge upstream/main
git push origin main
```

补充说明（决策与影响）：

- **为何 rebase 而非 merge**：冲突从「一大坨对冲」变为「逐魔改 commit 的小冲突」，定位靠 `git log --oneline upstream/main..personal` 预扫；失败可整体回退（merge 只能回滚合并）。代价是重写 personal 历史。
- **force-push 影响面**：GHCR 镜像由 `personal-image` / `personal-image-<short_sha>` git tag 触发构建，tag 不随分支重写；部署机只从 GHCR 拉镜像，不受分支历史重写影响。文档头部 commit 标记在每次同步后更新即可。
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
| `controller/channel-test.go` | 模型级禁用/恢复分支、`ShouldDisableChannelWithDecision`、`processChannelError` 第 4 参 `nil` | 上游 `4add708e` 把单渠道测试重构成 `runChannelTestWorkers` worker 池，循环体整体搬家 | `235ae5a7`：2026-08-24 同步 6 提交；上游重构后的新调用点逐处补魔改行（判定换 `WithDecision`、模型级 if/else 包裹、enable 块后插模型级恢复块、`performChannelTests` 调用前插 `recoverExpiredModelBans` 且保留上游新增的 `concurrency` 参数）；`949e1e69`：2026-08-29 同步 6 提交；上游 int32 重构移除 `math.Round`，魔改随机化提交新增 `math/rand/v2` 后 `math` 成孤儿，autosquash 修正；`1c4f5e512`：2026-09-05 同步 6 提交；上游 `7c044d7c5` 把 `ApplyReasoningModelSuffix` 改签名 `(*gin.Context, *RelayInfo, ...Request)`，魔改 `normalizeTestOutboundModel` 投递 `(nil, info)`（c 仅关联 diagnostics，nil 即跳过）；同提交 `channel_probe_outbound_test.go` 采纳 `7c044d7c5` BREAKING（OpenRouter 通用 `-thinking` 已删，`qwen2.5-...-thinking` 两通道 `assert.False`，白名单改 `gpt-5-high→gpt-5` 经 `ParseOpenAIReasoningEffortFromModelSuffix` 剥离后命中 `^gpt-5$`），`go vet` 已过 |
| `service/quota.go` | 计费功能级移除（`PreWssConsumeQuota` 等短路、`strings` import 随之删除） | 上游 `a073f74b` int32 重构把 `math.Round` 换成 `common.QuotaRoundChecked` 并调整 import | `949e1e69`：2026-08-29 同步 6 提交；import 冲突净结果为两侧删除（`math` 被上游移除用法、`strings` 被魔改移除用法），`CalcOpenRouterCacheCreateTokens` 采纳上游 `QuotaRoundChecked` 饱和语义 |
`model/ability.go` | `getChannelQuery` 简化（MAX(priority) 顶层、删 getPriority）+ DB 路径注释 | 上游把 `GetChannel` 重构为 `filters []dto.ChannelFilter` + 全量查询 + `filterAbilitiesByConstraints` | `7dccc6db4`：2026-08-31 同步 21 提交；`GetChannel` 采纳上游 filters 形态（魔改 getChannelQuery 顶层查询会丢 filters 语义），魔改同层重滚语义由 cache 主路径 exclude 机制承载；`getChannelQuery` 简化保留（上游已删 getPriority 引用）；`db1f5dae9`：2026-09-01 补修；filters 重构时 `GetChannel` 丢失了 `channel_disabled_models` 的 `NOT EXISTS` 过滤（DB 路径模型级禁用失效），现于查询内补回（此前仅 cache 主路径 carry 禁用语义） |
| `model/channel_cache.go` | `GetRandomSatisfiedChannel` 追加 `excludeChannels` 参数 + exclude 过滤块；模型组接管路由（overrides/effectivePriority/`GetRandomSatisfiedChannelFromGroups` DB fallback） | 上游把签名重构为 `filters []dto.ChannelFilter` + `filterCandidateIDs` | `7dccc6db4`：2026-08-31 同步 21 提交；签名取上游 filters 版再追加 `excludeChannels` 尾参，调用点（service/channel_select.go 两处）拼接 filters+excludeChannels；模型组新增函数块整体保留 |
| `web/src/features/system-settings/models/routing-reliability-section.tsx` | 滑动窗口禁用 4 字段、慢流/TTFT 降级配置（`channel_slow_stream_setting`） | 上游把 schema 重构为 `createRoutingReliabilitySchema(t)` 工厂 + i18n 化 | `235ae5a7`：2026-08-24 同步 6 提交；schema 一律取上游工厂版再插入魔改字段（含 superRefine 两条采样校验）；i18n locale 冲突用语义三方合并（保留上游新增键、应用魔改键变更） |
| i18n locale（`web/src/i18n/locales/*.json`） | 魔改 UI 文案键 | 上游同区段增删键导致整块冲突 | `235ae5a7`：2026-08-24 同步 6 提交；用 `scripts/i18n_3way_merge.py` 语义合并（ours 为底 + theirs 增改覆盖），键序以 ours 为准，事后 `bun run i18n:sync` 归位；`397bddf2d`/`1c4f5e512`：2026-09-05 同步 6 提交；12 批 locale 整块冲突用字节保序三方语义（ours 全量为底 + theirs 增删精确重放，不全量 `json.dump` 归一化），TTFT 11 键追加/min_input_tokens 2 键/ring-buffer 6 删6 增/图标 3 键/模型级 8 键/孤儿 370 删/解绑 144 删/模型组 13 增等，增键插 `Zoom` 后，事后 `bun run i18n:sync` 归位；同轮 `i18n/keys.go` 首次冲突（上游 `NoAvailableChannelTaskPlugin` + 魔改 `ChannelRateLimited` 双保留，`gofmt` 对齐），yaml 三语言自动合并 |
| `relay/common/relay_info.go` | `RequestDebugSnapshot` 字段 | `RelayInfo` 结构体字段增减、注释更新 | `7dccc6db4`：2026-08-31 同步 21 提交；上游字段增删自动合并，`RequestDebugSnapshot` 保留无冲突；`397bddf2d`：2026-09-05 同步 6 提交；上游 `6b659fd61` 改 `reasoningEffortFromRequest` 直取 Gemini `ThinkingLevel`/`EffortFromBudget`、抽 `originModelName` 局部，自动合并无冲突，`go vet` 已过 |
| `relay/helper/model_mapped.go` | 加权模型映射（1:N，`resolveModelMappingValue`/`pickWeightedModel`，`map[string]any` + 链式循环检测） | 上游 `7c044d7c5` 把解析换 `rootcommon.Unmarshal` + 查表前 `hostreasoning.BaseModelName` 回退（`@修饰符` 剥离语义） | `397bddf2d`：2026-09-05 同步 6 提交；取加权 `map[string]any` 本体 + `common.UnmarshalJsonStr`（AGENTS JSON wrapper 约定，不留 `encoding/json`）+ `math/rand/v2`，直接键缺失时对 `baseModel` 再 resolve 一次，循环体沿加权检测语义 |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | 请求调试快照面板 | 日志详情功能（如 stream status） | `7dccc6db4`：2026-08-31 同步 21 提交；上游新增 task_plugin 计费展示块（BillingBreakdown/DynamicPricingBreakdown/usage-facts），按计费移除语义整块弃用；魔改 request_debug 面板与上游 PluginAuthorLink 共存，import 拼接保留两侧 |
| `model/main.go` | PostgreSQL 连接强制 `PreferSimpleProtocol: true`（兼容 PgBouncer/Neon/Supabase）+ `normalizePostgresDSN` 兜底 `client_encoding=UTF8` + `standard_conforming_strings=on` | 上游新增 `ensureUserQuotaColumns`、`migratePrefillGroupUniqueness`（参数化 raw SQL）与 `migrateDBFast` 删除 | `7667fe5b3`：2026-09-01 fix；上游 `migratePrefillGroupUniqueness` 的 `db.Raw(...to_regclass(?) ...)` 参数化查询在 personal `PreferSimpleProtocol` + PG 连接编码非 UTF8 环境触发 pgx `sanitizeForSimpleQuery` 强制 `client_encoding=UTF8` 与 `standard_conforming_strings=on`（conn.go:1265-1269）→ 启动 FATAL；新增 `normalizePostgresDSN` 兜底给 DSN 追加两 runtime param，已显式携带时尊重用户值不覆盖 |
每次 rebase/merge 解完冲突后，把「上游改了啥 / 本地怎么拼 / 结论一句话」追加到对应文件行的「解决决策记录」列（格式：`<提交短SHA>：<日期> 同步 N 提交；<本次决策一句话>`）。列内多记录用 `；` 分隔。拼法照抄 `scripts/sync-decision-template.md`（签名变更/BREAKING 跟随/locale 三方/JSON 归一/字段分类五类模板；同 PR 同轮次不另起行：`i18n/keys.go` 归 locale 行、probe 归源码行）。记忆原则：同类冲突再次出现时，先查本表照抄决策，不再重新设计。

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
systemd-run --user --scope -p MemoryMax=1G -- /usr/local/go/bin/go test -count=1 ./controller/... ./service/... ./relay/... ./common/... ./pkg/billingexpr/...   # 一律 -count=1，禁缓存假绿
cd web && systemd-run --user --scope -p MemoryMax=1G -- bun run test
```

> push 前门禁（未提交就推 = 丢修复）：`scripts/sync-gate.sh`（quick：status/gofmt/vet/locale/controller 测试）；全量三构建口径用 `scripts/sync-gate.sh --full`。红即停，不进 push/docs。

> 构建只证明可编译；计费/禁用/结算路径的合并正确性由测试兜底（AGENTS.md 计费不变量有回归要求）。已知预存在失败用例需先在旧线终态复跑确认非本次回归（`git worktree add /tmp/old origin/<旧tip>` 后同命令复跑），并在同步报告中注明。

## 选择性纳入上游（2026-09-16 起主力方式）

> 触发短语：**「纳入上游」**。与「同步上游」（全量 rebase）互斥选用：全量已退役为应急手段，仅在需要上游某个强耦合重构时才回退使用。

### 动机（实测数据，2026-09-16 窗口 `4fc9d1f1f..upstream/main`，48 提交 / 546 文件 / +51730-10356）

**冲突数口径**：`git merge-tree --write-tree --name-only` 输出**第一行是 tree hash，第 2–131 行是冲突路径清单（130 行，与描述行数 130 一致：36 内容 + 93 修改-删除 + 1 重命名），第 132 行空行，第 133 行起才是 `自动合并 X` / `冲突（内容）：…` 逐文件日志**——那部分是合并过程的逐文件日志、**不是**冲突清单，勿把它连同「自动合并」行一起计数。实际 `git merge --no-commit` 落到工作区为 **125 个冲突文件**（92 DU + 32 UU + 1 UA），与 merge-tree 口径的差异全是「merge-tree 报冲突但三方合并能自动解」的文件（实测 8 个）。

三种方案在本窗口的实测成本（均在 `/tmp` 一次性 worktree 内实跑）：

| 方案 | 冲突总数 | 机械可解 | 真需人工 | merge-base | 可维护性 |
|------|---------|---------|---------|-----------|---------|
| **全量 merge** | 125 文件 | **92 DU**：全部为 personal 主动删除（92/92 在基线存在），`git rm` 循环即清零 | **32 UU**：其中 11 个是 i18n locale（批量三方合并）、21 个代码；但经逐文件溯源，**28 个 UU 仅由本线主动排除的提交引起**、3 个混杂、**0 个仅由已纳入提交引起** | **前进到 upstream/main** | 一次推进基座；代价是 merge 会**静默带入 94 个新增文件（含 29 个属已删功能面）**，需事后逐个 `git rm`——本轮实测 `relay/responses_websocket.go`、`pkg/wsmanager/`、`relay/request_billing.go`、`model/passkey_option.go` 等全部无冲突落入 |
| **纯 cherry-pick** | 逐提交 19 干净 / 29 冲突 | 8 个纯已删文件（`git rm` 即解） | 真代码冲突仅 **3** 个提交；17 个为「已删文件 + 真代码」混合 | **不动**（停在 `4fc9d1f1f`） | 每次只碰一个提交、粒度可控、可精准跳过已删功能面；代价是基座不前进、上游提交以新 SHA 出现，需靠 `-x` 尾注追踪 |
| **混合（推荐）** | — | — | 仅对**选中提交**跑 cherry-pick（本轮 18 个中 17 个零冲突、1 个仅 `git rm` 4 文件） | 不动（同 cherry-pick） | 兼顾「精准取用」与「低跟踪成本」；基座不前进的问题由 `-x` 尾注检索缓解（`git log --grep="cherry picked from commit <sha>"`，勿用 `--cherry-mark` 判已纳入——手工解冲突的提交会被列为非等价） |

**为什么本窗口选 cherry-pick 而非 merge**（关键实测）：merge 的 32 个内容冲突里 **28 个源自本线主动排除的提交**（如 `9fe0457ee` Responses WebSocket → `controller/relay.go`/`relay/responses_handler.go`；`74629e29f` 插件任务流 → `details-dialog.tsx`/`task-plugins/*`；`12be9975c` 前端错误通知 → `users/api.ts` 等 5 文件；`385d2dfd1` passkey → `login_verification.go`）。这些冲突**不是「合并需要解决的分歧」，而是「已经决定不要的上游功能」**——为它们逐文件解冲突是纯浪费；且 merge 还会无冲突带入 29 个已删功能面的新增文件，事后仍需人工 `git rm`。相比之下 cherry-pick 可以**根本不碰**这些提交。若某轮上游改动以「想全要的 bugfix」为主、排除面很小，merge 更划算（一次推进基座）。

### 探针口径警告（避免高估人工成本）

逐提交探针是 abort-and-skip 策略：跳过的提交会让**同文件的后续提交连锁冲突**。实测 `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx` 出现在 **8 个提交**的冲突清单里（`505805a4c` 及其后继），说明渠道 UI 那串提交是**耦合单元**——要么整串取、要么整串不取，不能只取一半。汇报探针冲突时要按四类分解：**纯 i18n locale**（批量三方合并）/ **纯已删文件**（`git rm`）/ **真代码冲突**（需人工）/ **依赖链**（耦合单元整体决策）。本窗口 29 个冲突提交的分解：8 纯已删文件、1 纯 i18n、3 真代码、17 混合（含已删文件 + 真代码）。

### 分类判据（决定一个上游提交是否纳入）

先 `git log --oneline --no-decorate <base>..upstream/main` 逐条分类，对每个提交看两件事：

1. **改动性质**：bugfix / 功能新增 / 重构 / 仅测试 / 仅文档。重构与「带来新语义的删除」高风险，bugfix 与独立小功能低风险。
2. **冲突构成**（探测命令见下）：把冲突文件分成
   - `gone`：personal 已删除的文件（说明该提交属于已删功能，或不巧改到了这些文件）
   - `real`：真正的内容冲突（需要手工解决）

**纳入判据**：

- `gone` 命中已删功能（billing/pricing/OAuth/Passkey/订阅/wallet/redemption/ollama）→ **不纳入**，除非该提交同时携带与本线目标一致的 relay/稳定性修复。
- `real` 为空（纯 `gone`）→ 若与本线目标一致，纳入时直接 `git rm` 冲突文件即可；否则跳过。
- backend-touching 且与「上游稳定性 → 下游稳定」目标一致（relay 正确性、限流、渠道适配、DB 迁移加固、插件修复）→ **纳入**。
- 前端纯 UI 打磨（pricing 编辑器、passkey 设置页、渠道 UI 大改）→ 一般不纳入；已删功能的 UI 一律不纳入。
- 只增测试的提交：跟随其被守护的实现一并纳入，否则跳过。

### 探测命令（试投放前先量化，勿凭感觉挑）

```bash
# 1) 冲突模拟：一次看清整窗冲突构成（内容 / 修改-删除 / 重命名）
git merge-tree --write-tree --name-only personal upstream/main | tail -n +2

# 2) 逐提交试投放：OK=可干净落线，CONFLICT=需手工；并区分 gone/real
cd /tmp/cptest && git reset --hard personal
for h in $(git log --reverse --format=%H <base>..upstream/main); do
  if git cherry-pick -x --no-edit "$h" >/dev/null 2>&1; then echo "OK $h"
  else echo "CONFLICT $h"; git cherry-pick --abort; fi
done
```

实测口径（2026-09-16 窗口）：48 提交中 **19 个可干净 cherry-pick**，29 个冲突；再剔除属于已删功能的提交后，**实际纳入 18 个**（含 1 个仅需 `git rm` 4 个已删文件的冲突提交）。19 个干净提交里 2 个（`f256e40bc` 定价表达式默认值、`25ec832fa` 定价草稿转换）虽干净落线但属于已删的定价功能面，**不纳入**——「能干净落」不等于「该纳入」，判据以功能面为准。

### 执行流程

```bash
git fetch upstream
# 在一次性工作区试投放（勿直接动 personal）
git worktree add --detach /tmp/cptest personal
cd /tmp/cptest
for h in <按拓扑序排列的纳入清单>; do git cherry-pick -x --no-edit "$h"; done
# 冲突多为「修改/删除」：git rm 掉 personal 已删的文件后 git cherry-pick --continue
```

纳入清单**按上游拓扑序（最旧优先）**排列，保证上游提交之间的依赖顺序天然成立。落地：

```bash
cd /root/workspace/new-api
git merge --ff-only <试投放 tip>          # 试投放已跑完全量验证，直接快进
git push origin personal                  # 推代码；纯代码/文档改动才需要 tag 触发构建
```

### 与全量 rebase 的兼容性（两法可交替）

cherry-pick 用 `-x` 记录来源，patch-id 与上游原提交一致（本轮 18 个中 **17 个等价，1 个例外**：`c79b74b68` → `fc82ee85b` 因手工解冲突（`git rm` 4 个已删文件）致 patch-id 变更）。其余 17 个在日后全量 `git rebase upstream/main personal` 时会被默认丢弃（默认 `--no-reapply-cherry-picks`），不会重复。

**该例外的两个实际后果**（务必记住）：

1. **未来全量 rebase 会重放 `c79b74b68`**，再次撞上那 4 处 modify/delete（`user-binding-dialog.tsx` 及其测试、`web/src/routes/pricing/{index,$modelId/index}.tsx`），需手工 `git rm` 一次。这是本轮唯一需要人工介入的兼容点。
2. **`git rev-list --cherry-mark --right-only personal...upstream/main` 每轮都会把它重新列为 `+`**（非等价侧），不能拿它当「已纳入清单」用——它只反映 patch 等价性。实测该命令输出 `+c79b74b68 …`（`+` 共 31 个，`=` 为 17 个）。

**排查「某上游提交是否已纳入」应改用 `-x` 尾注检索**（不受手工解冲突影响）：

```bash
git log --oneline --grep="cherry picked from commit c79b74b68" personal   # 命中 fc82ee85b
# 批量核对整窗纳入情况（含手工解冲突的提交）
for h in $(git log --format=%h 4fc9d1f1f..upstream/main); do
  git log --oneline --grep="cherry picked from commit $h" personal | sed "s/^/$(git rev-parse --short $h) -> /"
done
```

**基座核对命令**（判断当前是哪种模式）：

```bash
git merge-base --is-ancestor upstream/main personal && echo "全量已同步" || echo "选择性纳入模式（upstream/main 非 personal 祖先）"
```

### 纳入后验证（与「合并后验证」同口径）

- 三构建：根模块 / relaykit 独立模块 / 前端 `bun run build`。
- 测试：`systemd-run --user --scope -p MemoryMax=1G -- go test -count=1 ./controller/... ./service/... ./relay/... ./common/... ./pkg/billingexpr/...` + `bun run test`。
- **一次性工作区构建前置（实测坑）**：`main.go` 有 `//go:embed web/dist` 与 `//go:embed web/dist/index.html` 两条指令，而 `web/dist` 被 `.gitignore` 忽略。**新建 worktree 后 `go build ./...` 会以 `main.go:44:12: pattern web/dist: no matching files found` 失败（退出码 1）**——主仓库 gate 能过是因为残留了上一次 `bun run build` 的产物。在 worktree 里跑后端构建前，须先 `cd web && bun run build`（或从主仓库 `cp -r web/dist <worktree>/web/`）。
  - **注意 `.gitkeep` 方案无效**：Go `embed` 忽略以 `.` 开头的文件，只放 `web/dist/.gitkeep` 会报 `cannot embed directory web/dist: contains no embeddable files`（实测）；且 `index.html` 那条指令要求该文件必须存在。因此占位必须是**非点开头的真实文件**（如 `placeholder.txt` + `index.html`），但本项目**不采用**占位方案——上游设计即要求真实前端产物，保持 embed 不动、验证前先构建即可。
- **DB 改动额外要求**：凡纳入 `model/` 迁移类提交（如 `043ff99a5`、`007d69942`），按 AGENTS.md 要求跑 SQLite + PostgreSQL 两库；PG 用 `.env` 的 `SQL_DSN` 作 `TEST_POSTGRES_DSN`：
  `set -a; source .env; set +a; TEST_POSTGRES_DSN="$SQL_DSN" go test -count=1 -run "TestMigratePrefillGroupUniquenessPostgreSQL|TestMigrationSchemaStability" -v ./model/`
  （两测试均在事务内建独立 schema/表并回滚，不触碰共享库的应用表，符合共享库清理硬约束。）
- 每个纳入提交登记一行到 manifest 的「选择性纳入上游」章节（上游 SHA → 本地 SHA、类别、验证）。

## 部署

> **强约束：只有用户明确要求触发构建/发布时才执行本流程。** 纯文档改动（`docs/`、`AGENTS.md`、manifest 登记、说明性提交）不触发构建——只 `git push origin personal` 即可，不推送任何 `personal*` tag。判断依据：改动是否影响运行产物（Go 源码、前端源码、Dockerfile、依赖清单等）；仅文档/注释变更视为不触发。**已误触发的冗余构建不取消，让其完成**（结果不影响部署，镜像 tag 语义仍正确）；后续纯文档提交勿再推 tag。

`.github/workflows/deploy-image-ghcr.yml` 仅服务 `personal` 单线，镜像 tag 由构建来源动态推导：

| 构建来源 | 推导规则 | 镜像滚动 tag | 镜像留档 tag |
|---------|---------|-------------|-------------|
| tag `personal-image` | 去 `-image` 尾缀 | `:personal` | `:personal-<short_sha>` |
| dispatch `branch=personal` | 直接用输入 | 同上 | 同上 |

```bash
git push origin personal                                       # 推送代码（不触发构建）
git tag -f personal-image && git push -f origin personal-image # 覆盖滚动 tag → 自动构建
```

> 滚动 tag 用 `personal-image`（避免与分支同名歧义），单次构建同时产出 `:personal`（滚动）与 `:personal-<short_sha>`（留档）。重复发布直接 `git tag -f` 覆盖；回滚用已知良好的 `:personal-<short_sha>`。Prune 只对带 `personal*` tag 版本计数（保留最近 3 个）。历史 `deploy` 线镜像（`:deploy` / `deploy-image-*` tag）已随分支删除停止更新，仅留档回滚。

也可通过 Actions UI 手动触发：GitHub → **Actions** → **Build branch image (GHCR)** → **Run workflow** → branch 输入 `personal`（默认值）。

### 通用说明

1. 推送 `personal*` tag 后，GitHub Actions 自动构建，无需进网页
2. 构建产物（`<owner>` 为仓库属主小写）：
   - `ghcr.io/<owner>/new-api:<prefix>` — 滚动 tag，始终指向最新构建
   - `ghcr.io/<owner>/new-api:<prefix>-<short_sha>` — 不可变 tag，对应具体提交
   - 镜像内 `VERSION` 文件 = `<prefix>-<short_sha>`
3. 部署机只从 GHCR 拉取镜像，不从 Git 仓库构建：
   - 日常更新：拉 `:<prefix>`
   - 回滚：拉上一个已知良好的 `:<prefix>-<short_sha>`
4. Prune：personal 构建只清理带 `personal*` tag 的历史版本，保留最近 3 个（历史 `deploy*` 版本不再清理、不再计数）

### 部署机拉取与验证

```bash
docker pull ghcr.io/<owner>/new-api:personal
# 重启容器后核对镜像内版本标记（应显示 personal-<short_sha>）
curl -s http://<host>:3000/api/status | jq -r .data.version
```

> 纯文档提交不构建镜像，故版本标记可能落后 git HEAD（如 HEAD 已是后续文档修正提交）——属预期，镜像与最新代码提交的功能内容一致即可。

### 确认构建状态

**已登录 `gh` 时（优先，限流 5000 次/小时）：**

```bash
# 列出最近构建（workflow 文件名 deploy-image-ghcr.yml，展示名 Build branch image (GHCR)）
gh run list --repo <owner>/new-api --workflow deploy-image-ghcr.yml --limit 5
# 或精确查询指定 tag 触发的构建（head_branch 即 tag 名）
gh api "repos/<owner>/new-api/actions/runs?event=push&branch=personal-image&per_page=5" \
  --jq '.workflow_runs[] | {name, tag: .head_branch, sha: (.head_sha[0:7]), status, conclusion, url: .html_url}'
# 查看单次运行详情
gh api repos/<owner>/new-api/actions/runs/<run_id> --jq '{name, tag: .head_branch, status, conclusion}'
```

**未登录 / 无 token 时（仓库公开，限流 60 次/小时）：**

```bash
# 查询最近按 tag push 触发的构建（head_branch 即 tag 名）
curl -s "https://api.github.com/repos/<owner>/new-api/actions/runs?event=push&per_page=5" \
  | jq '.workflow_runs[] | {name, tag: .head_branch, sha: (.head_sha[0:7]), status, conclusion}'
```

- `status`: `queued` / `in_progress` / `completed`
- `conclusion`: 完成后为 `success` / `failure`；进行中为 `null`
- 按 tag 过滤：`?event=push&branch=personal-image`（`head_branch` 即触发 tag 名）
- 轮询建议：每 30~60 秒一次，直到 `status == "completed"`；`conclusion == "success"` 即构建成功，可通知部署机拉取新镜像
- 已完成构建与 tag 指向可能落后分支 HEAD：纯文档提交不触发构建、amend 会替换哈希，`personal-image` tag 常滞后 HEAD 数个纯文档提交；判断「镜像是否最新」以 tag 所指 commit 的父链上是否含你关心的代码提交为准，勿以 HEAD/tag 重合判断
- 若已配置 `GITHUB_TOKEN`/`GH_TOKEN`：`curl -H "Authorization: Bearer $GITHUB_TOKEN" ...` 或 `gh auth login` 后走 `gh` 路径，限流更宽松

## 定制功能分支合并

```bash
git checkout personal
git merge local/<feature>
git push origin personal            # 推送代码本身不触发构建
git tag -f personal-image && git push -f origin personal-image    # 覆盖滚动 tag 触发 GHCR 构建
```

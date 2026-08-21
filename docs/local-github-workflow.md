# 本地 GitHub Fork 工作流

> 对应分支:`personal` 基线 `7e6415e7`(2026-08-21 刷新至 `6702a043`；`personal` 线同步流程见「同步上游」节)

## 标准触发短语

向维护助手说 **「同步上游到 deploy」**(参考 `docs/local-github-workflow.md`) 即按本文档执行完整同步流程:fetch upstream → **`git rebase upstream/main deploy`** 重放魔改提交 → 逐提交按「保留魔改 + 采纳上游语义」解决冲突 → 合并后验证 → force-push origin deploy → 更新本文档头部标记。**默认同时同步 `main`**(merge upstream/main 后推送)。

可选追加：`不同步 main`（仅同步 deploy）、`不更新文档`（跳过头部标记更新）、`跳过验证`（不推荐）。

本仓库只保留两类远程：`origin` (个人 fork) 和 `upstream` (官方上游)。

## 分支策略

| 分支 | 角色 | 说明 |
|------|------|------|
| `main` | 贴近上游 | 仅保留少量本地改动（如 GHCR workflow 文件），几乎与上游同步 |
| `deploy` | 部署分支 | 承载全部魔改功能；GHCR 部署镜像由它构建 |
| `personal` | 个人开发线 | 历史前段（至 `7e6415e7`，原 deploy-model 中间态分支，已退役并入）承载：deploy 同步战略改造、模型级路由表前台化、model group 管理接口；其后叠加：模型组路由全套 + 计费/Ollama/订阅/OAuth/开放注册移除。与 `deploy` 共同祖先为 `ab4d296e`（原 deploy-re，已退役）。不构建镜像、不打 tag |
| `local/<feature>` | 本地定制功能分支 | 开发完成后合并进目标魔改分支 |

魔改功能清单见 `request-debug-customization-manifest.md`；魔改功能不在 `main` 上。`deploy` 与 `personal` 线已分叉，各自独立维护与同步上游；manifest 中 deploy 线功能登记以 personal 基线 `7e6415e7`（原 deploy-model，已退役）为基准，personal 改动单独登记在「personal 分支半重构登记」小节。

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
| `controller/relay.go` | 渠道限流检查、同优先级重试 `ExcludeChannel`、全渠道限流 429 兜底、（新增）模型级禁用分支 | 重试/计费逻辑（如 `PrepareTieredBillingForSelectedGroup`） | `e0b9f243`：2026-08-01 同步 8 提交；重试/计费逻辑拼序采纳、限流检查保留；`702be7eb`：2026-08-19 同步 0 提交；`processChannelError` 既有禁用两行包进 else、前插模型级 if 分支（包 else 非纯追加，冲突时按「保留魔改 + 采纳上游语义」手动合并） |
| `relay/common/relay_info.go` | `RequestDebugSnapshot` 字段 | `RelayInfo` 结构体字段增减、注释更新 | （空，待首录） |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | 请求调试快照面板 | 日志详情功能（如 stream status） | （空，待首录） |

#### 冲突决策记录规则

每次 rebase/merge 解完冲突后，把「上游改了啥 / 本地怎么拼 / 结论一句话」追加到对应文件行的「解决决策记录」列（格式：`<提交短SHA>：<日期> 同步 N 提交；<本次决策一句话>`）。列内多记录用 `；` 分隔。记忆原则：同类冲突再次出现时，先查本表照抄决策，不再重新设计。

历史参考：2026-08-01 同步 8 个上游提交时，前两个文件各产生一处冲突，处理方式记录在合并提交 `e0b9f243`。

#### 合并后验证

```bash
/usr/local/go/bin/go build ./...                                 # 根模块（需 Go >= 1.25）
cd relaykit && GOWORK=off /usr/local/go/bin/go build ./...       # relaykit 独立模块（须独立可构建）
cd web && bun run build                                          # 前端
```

> 注意：本机 `/usr/bin/go` 是 1.19，无法解析 go.mod 的 `go 1.25.1` 指令。使用 `/usr/local/go/bin/go`（1.26），或先 `hash -r` 清除 shell 命令缓存。

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

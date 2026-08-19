# 本地 GitHub Fork 工作流

> 对应分支:`deploy` @ `822c1f85`(2026-08-19 更新)

## 标准触发短语

向维护助手说 **「同步上游到 deploy」**(参考 `docs/local-github-workflow.md`) 即按本文档执行完整同步流程:fetch upstream → merge 进 deploy → 按「保留魔改 + 采纳上游语义」解决冲突 → 合并后验证 → push origin deploy → 更新本文档头部标记。**默认同时同步 `main`**(merge upstream/main 后推送)。

可选追加：`不同步 main`（仅同步 deploy）、`不更新文档`（跳过头部标记更新）、`跳过验证`（不推荐）。

本仓库只保留两类远程：`origin` (个人 fork) 和 `upstream` (官方上游)。

## 分支策略

| 分支 | 角色 | 说明 |
|------|------|------|
| `main` | 贴近上游 | 仅保留少量本地改动（如 GHCR workflow 文件），几乎与上游同步 |
| `deploy` | 部署分支 | 承载全部魔改功能；GHCR 部署镜像由它构建 |
| `local/<feature>` | 本地定制功能分支 | 开发完成后合并进 `deploy` |

魔改功能清单见 `request-debug-customization-manifest.md`；魔改功能不在 `main` 上。

## 同步上游

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
git merge upstream/main
# 有冲突 → 见"冲突处理原则"
# 无冲突 → 仍建议执行"合并后验证"
git push origin deploy
```

#### 冲突处理原则

魔改与上游改动重叠时：**保留魔改功能 + 采纳上游语义**。逐处判断：

- 魔改独有代码（如渠道限流检查）必须保留；
- 上游新增/修改的逻辑（如计费调用、结构体字段注释）必须采纳；
- 两者位置相邻时按业务顺序拼接，不互相覆盖。

已知高风险文件（上游改动最易与魔改冲突）：

| 文件 | 魔改内容 | 上游易冲突点 |
|------|----------|--------------|
| `controller/relay.go` | 渠道限流检查、同优先级重试 `ExcludeChannel`、全渠道限流 429 兜底 | 重试/计费逻辑（如 `PrepareTieredBillingForSelectedGroup`） |
| `relay/common/relay_info.go` | `RequestDebugSnapshot` 字段 | `RelayInfo` 结构体字段增减、注释更新 |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | 请求调试快照面板 | 日志详情功能（如 stream status） |

历史参考：2026-08-01 同步 8 个上游提交时，前两个文件各产生一处冲突，处理方式记录在合并提交 `e0b9f243`。

#### 合并后验证

```bash
/usr/local/go/bin/go build ./...                                 # 根模块（需 Go >= 1.25）
cd relaykit && GOWORK=off /usr/local/go/bin/go build ./...       # relaykit 独立模块（须独立可构建）
cd web && bun run build                                          # 前端
```

> 注意：本机 `/usr/bin/go` 是 1.19，无法解析 go.mod 的 `go 1.25.1` 指令。使用 `/usr/local/go/bin/go`（1.26），或先 `hash -r` 清除 shell 命令缓存。

## 部署

> **强约束：只有用户明确要求触发构建/发布时才执行本流程。** 纯文档改动（`docs/`、`AGENTS.md`、manifest 登记、说明性提交）不触发构建——只 `git push origin deploy` 即可，不推送任何 `deploy*` tag。判断依据：改动是否影响运行产物（Go 源码、前端源码、Dockerfile、依赖清单等）；仅文档/注释变更视为不触发。

`deploy` 分支的镜像构建**不是每次 push 自动触发**。`.github/workflows/deploy-image-ghcr.yml` 支持两种触发：

- **自动（推荐）**：推送以 `deploy` 开头的 git tag（如 `deploy-image`、`deploy-image-<short_sha>`）即触发构建；
- **手动兜底**：`workflow_dispatch`，在 GitHub → **Actions** → **Build deploy image (GHCR)** → **Run workflow**（branch 输入默认 `deploy`）。

日常发布流程（**每次发布只打一个滚动 tag**，单次构建即同时产出滚动与留档镜像 tag）：

```bash
git push origin deploy                    # 推送代码（不触发构建）
git tag -f deploy-image && git push -f origin deploy-image    # 覆盖滚动 tag → 自动构建
```

> 滚动 tag 用 `deploy-image` 而非 `deploy`，避免与部署分支同名引发 git refspec 歧义。**不要额外打 `deploy-image-<short_sha>` 等留档 git tag**——workflow 单次构建已同时推送 `:deploy`（滚动）与 `:deploy-<short_sha>`（留档）两个镜像 tag（见下「构建产物」），多打 git tag 只会多触发一次重复构建。再次发布直接 `git tag -f` 覆盖滚动 tag 即可；回滚用已知良好的 `:deploy-<short_sha>` 镜像 tag。

1. 推送 `deploy*` tag 后，GitHub Actions 自动构建，无需进网页
2. 构建产物（`<owner>` 为仓库属主小写）：
   - `ghcr.io/<owner>/new-api:deploy` — 滚动 tag，始终指向最新构建
   - `ghcr.io/<owner>/new-api:deploy-<short_sha>` — 不可变 tag，对应具体提交
   - 镜像内 `VERSION` 文件 = `deploy-<short_sha>`
3. 部署机只从 GHCR 拉取镜像，不从 Git 仓库构建：
   - 日常更新：拉 `:deploy`
   - 回滚：拉上一个已知良好的 `:deploy-<short_sha>`

> 注意：tag 触发构建时，镜像 tag 的 `<short_sha>` 取自该 tag 指向的提交，与 git tag 名无关。

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

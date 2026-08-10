# 本地 GitHub Fork 工作流

> 对应分支:`deploy` @ `556804cc`(2026-08-10 更新)

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

`deploy` 分支的镜像构建**不是 push 自动触发**（`.github/workflows/deploy-image-ghcr.yml` 为 `workflow_dispatch` 手动触发）。

1. push `deploy` 后，到 GitHub → **Actions** → **Build deploy image (GHCR)** → **Run workflow**（branch 输入默认 `deploy`）
2. 构建产物（`<owner>` 为仓库属主小写）：
   - `ghcr.io/<owner>/new-api:deploy` — 滚动 tag，始终指向最新构建
   - `ghcr.io/<owner>/new-api:deploy-<short_sha>` — 不可变 tag，对应具体提交
   - 镜像内 `VERSION` 文件 = `deploy-<short_sha>`
3. 部署机只从 GHCR 拉取镜像，不从 Git 仓库构建：
   - 日常更新：拉 `:deploy`
   - 回滚：拉上一个已知良好的 `:deploy-<short_sha>`

## 定制功能分支合并

```bash
git checkout deploy
git merge local/<feature>
git push origin deploy      # 随后手动触发上方 GHCR 构建
```

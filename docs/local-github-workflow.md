# 本地 GitHub Fork 工作流

本仓库只保留两类远程：

- `origin`：个人 GitHub fork，`https://github.com/kimberxu/new-api.git`
- `upstream`：官方上游仓库，`https://github.com/QuantumNous/new-api.git`

部署机器默认不再从 Git 仓库构建镜像，只从 GHCR 拉取 GitHub Actions 发布的镜像。

## 分支

- `main`：个人 GitHub fork 的默认分支。尽量贴近官方上游，并保留手动触发 GitHub Actions 所需的 workflow 文件。`workflow_dispatch` 只有在 workflow 位于默认分支时才会出现在 Actions 页面。
- `deploy`：部署分支。GitHub Actions 默认构建该分支并发布部署镜像。
- `local/<feature>`：本地定制功能分支，例如 `local/request-debug`。

## 本地开发

从 `deploy` 开始新的本地定制：

```bash
git checkout deploy
git pull --ff-only origin deploy
git checkout -b local/request-debug
```

修改代码并完成聚焦检查后提交：

```bash
git status --short
git add <changed-files>
git commit -m "feat: add request debug logging"
git push -u origin local/request-debug
```

验证通过后合并到 `deploy`：

```bash
git checkout deploy
git pull --ff-only origin deploy
git merge --no-ff local/request-debug
git push origin deploy
```

如果某个变更也需要保留在 `main` 可见范围内，优先只挑选必要提交；不要把尚未准备进入主分支的部署定制整体合并到 `main`。

## GitHub Actions 镜像构建

在 GitHub 页面手动运行：

```text
Actions -> Build deploy image (GHCR) -> Run workflow
```

`branch` 输入框保持默认值：

```text
deploy
```

构建成功后会发布：

```text
ghcr.io/kimberxu/new-api:deploy
ghcr.io/kimberxu/new-api:deploy-<short-sha>
```

`main` 只需要保留 workflow 文件；实际部署代码在 `deploy`。

## 部署机器

部署机器只需要机器专用的 `compose.yml`、`.env`、数据目录、日志目录、Docker 和 Docker Compose，不需要写权限 Git 凭据。

`compose.yml` 使用 GHCR 镜像：

```yaml
services:
  new-api:
    image: ghcr.io/kimberxu/new-api:deploy
```

如果 GHCR 包是私有的，先用带 `read:packages` 权限的 GitHub Personal Access Token 登录：

```bash
docker login ghcr.io
```

GitHub Actions 构建完成后，在部署机器更新容器：

```bash
cd new-api
docker compose pull new-api
docker compose up -d --no-deps new-api
docker compose ps
```

不要执行 `docker compose down -v`；数据和日志目录必须保留。

## 同步上游更新

先拉取官方上游：

```bash
git fetch upstream
```

更新个人 `main`：

```bash
git checkout main
git pull --ff-only origin main
git merge upstream/main
git push origin main
```

把 `deploy` 推进到新的 `main`：

```bash
git checkout deploy
git pull --ff-only origin deploy
git merge main
```

解决冲突并完成验证后推送：

```bash
git push origin deploy
```

这个合并式流程偏保守，避免改写远程历史。需要整理补丁栈时，只在功能分支上 rebase，不要盲目 rebase 共享的 `deploy` 分支。

## 重新应用本地定制

请求调试日志的具体改造目标、文件清单、职责、不变量、冲突处理和验证命令见 [request-debug-customization-manifest.md](request-debug-customization-manifest.md)。该 manifest 是上游同步后修复本地改造的权威清单。

上游更新后如果冲突较多，不要固定 cherry-pick 某个历史 commit，也不要整文件覆盖上游新文件。推荐流程：

1. 从更新后的 `main` 创建新分支；
2. 用 manifest 的文件职责逐项检查当前 `deploy` 与 `main` 的差异；
3. 只迁移请求调试相关逻辑；
4. 运行 manifest 中的聚焦测试；
5. 合并修复后的分支到 `deploy`。

示例：

```bash
git checkout main
git pull --ff-only origin main
git checkout -b local/request-debug-refresh
git diff main..deploy -- common relay controller service model docs .github
```

## 验证清单

推送 `deploy` 前运行与改动相关的检查。请求调试日志至少包括：

- 请求调试相关聚焦测试；
- 涉及 handler 路径的 relay 测试；
- 日志格式化测试，确认非管理员日志视图会剥离 `admin_info`；
- 针对测试实例的一次手动请求，确认 `request_debug` 只出现在管理员可见日志详情中。

## 回滚

给已知正常部署打 Git 标签：

```bash
git checkout deploy
git tag deploy-YYYYMMDD-description
git push origin deploy-YYYYMMDD-description
```

容器回滚优先使用已知正常的 GHCR 固定标签：

```yaml
image: ghcr.io/kimberxu/new-api:deploy-<short-sha>
```

然后执行：

```bash
docker compose pull new-api
docker compose up -d --no-deps new-api
```

只有确认目标提交和影响范围后，才考虑调整远程 `deploy` 分支。

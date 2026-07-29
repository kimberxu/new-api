# 本地 GitHub Fork 工作流

本仓库只保留两类远程：`origin` (个人 fork) 和 `upstream` (官方上游)。

## 分支策略

- `main`: 个人 fork 默认分支，贴近上游并保留 workflow 文件
- `deploy`: 部署分支，GitHub Actions 自动构建
- `local/<feature>`: 本地定制功能分支

## 同步上游

```bash
git fetch upstream
git checkout main
git merge upstream/main
git push origin main
```

## 部署

部署机只从 GHCR 拉取 Actions 构建的镜像，不从 Git 仓库构建。

详见 `request-debug-logging-guide.md` 和 `request-debug-customization-manifest.md`。

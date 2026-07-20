# 请求调试日志本地改造清单

本文档记录 `deploy` 分支相对官方 `upstream/main` 的请求调试日志本地化改造。它用于上游更新、冲突处理和问题回滚时快速判断哪些改动必须保留。

## 改造目标

请求调试日志用于对比客户端提交的下游 JSON 请求和系统最终发送给渠道的上游 JSON 请求，帮助定位模型映射、禁用字段、参数覆盖和协议转换差异。

核心目标：

- 默认关闭，不影响正常生产请求；
- 不新增数据库表或字段；
- 仅在管理员使用日志详情中新增查看入口，不新增配置页或后端查询接口；
- 快照只写入既有日志的 `Other.admin_info.request_debug`；
- 普通用户日志视图不得看到 `admin_info`；
- 采集、脱敏、截断和快照组装集中维护，避免复制到各渠道 adaptor；
- 快照采集失败不得中断正常 relay，只记录 `request_debug_error`。

## 当前本地差异清单

用以下命令重新生成当前清单：

```bash
git diff --name-status upstream/main..deploy
```

截至当前维护版本，请求调试和部署相关本地差异包括：

```text
A .github/workflows/deploy-image-ghcr.yml
M common/constants.go
M common/init.go
A common/request_debug_config_test.go
M controller/channel-test.go
M controller/relay.go
A docs/local-github-workflow.md
A docs/request-debug-customization-manifest.md
A docs/request-debug-logging-guide.md
A docs/superpowers/specs/2026-07-19-request-debug-logging-design.md
M model/log_format_test.go
M relay/chat_completions_via_responses.go
M relay/claude_handler.go
M relay/common/relay_info.go
A relay/common/request_debug.go
A relay/common/request_debug_test.go
M relay/compatible_handler.go
M relay/gemini_handler.go
M relay/responses_handler.go
M service/log_info_generate.go
A service/request_debug_log_test.go
M service/system_task.go
M service/system_task_test.go
M web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx
A web/default/src/features/usage-logs/lib/request-debug.ts
A web/default/src/features/usage-logs/lib/request-debug.test.ts
M web/default/src/features/usage-logs/types.ts
```

如果该列表变化，先判断变化是否属于请求调试、部署工作流或文档维护；不要把无关本地改动混入该定制。

## 文件职责

### 配置入口

- `common/constants.go`：定义请求调试相关环境变量名或默认配置常量。
- `common/init.go`：初始化 `REQUEST_DEBUG_LOGGING`、`REQUEST_DEBUG_MAX_BODY_BYTES` 等配置。
- `common/request_debug_config_test.go`：覆盖配置解析、默认值和非法值回退。

维护重点：

- `REQUEST_DEBUG_LOGGING` 仅接受 `off`、`error_only`、`always`；
- 空值或非法值必须按 `off` 处理；
- body 大小限制非法时必须回退默认值。

### 快照核心逻辑

- `relay/common/request_debug.go`：集中维护请求体快照、敏感字段脱敏、正文类字段摘要、字符串截断、整体截断、SHA-256 摘要和错误兜底。
- `relay/common/relay_info.go`：在 relay 请求上下文中承载请求调试状态，使 handler 和日志服务可以传递快照。
- `relay/common/request_debug_test.go`：覆盖脱敏、截断、快照结构和错误容忍。
- `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`：负责请求调试快照的详情展示；JSON `body` 会在界面里格式化显示，复制仍保持原始字符串。
- `web/default/src/features/usage-logs/lib/request-debug.ts`：封装请求调试 `body` 的 JSON 展示格式化逻辑，供详情弹窗复用。

维护重点：

- 不要把脱敏、截断和快照组装逻辑复制到各个 provider adaptor；
- 字段名脱敏匹配应忽略大小写和首尾空格；
- 已知凭据字段必须脱敏：`authorization`、`api_key`、`apikey`、`access_token`、`refresh_token`、`key`、`token`、`password`、`secret`；
- 超长内容和正文类字段必须保存 `size`、`sha256`、`truncated` 和处理后的 `body`；
- 快照处理异常只能进入 `request_debug_error`，不能影响原请求。

### Relay 接入点

- `controller/relay.go`：在请求进入 relay 流程时捕获下游请求体，建立请求调试上下文。
- `relay/compatible_handler.go`：OpenAI Compatible JSON 路径的上游快照采集点。
- `relay/responses_handler.go`：Responses JSON 路径的上游快照采集点。
- `relay/claude_handler.go`：Claude JSON 路径的上游快照采集点。
- `relay/gemini_handler.go`：Gemini JSON 路径的上游快照采集点。
- `relay/chat_completions_via_responses.go`：Chat Completions 通过 Responses 转换路径的快照传递。
- `controller/channel-test.go`：渠道测试路径与请求调试上下文兼容。

维护重点：

- 下游快照应尽早捕获客户端原始请求体；
- 上游快照必须位于最终 JSON 变换完成之后、创建出站请求体之前；
- 模型映射、禁用字段和参数覆盖后的结果应反映在上游快照中；
- 非 JSON、multipart 或暂不支持的路径不能为了快照破坏原有请求行为。

### 日志落库与可见性

- `service/log_info_generate.go`：将 `request_debug` 附加到日志 `Other.admin_info`。
- `service/system_task.go`：复用既有 `log_cleanup` 系统任务，按环境变量自动清理旧数据库日志，默认关闭。
- `model/log_format_test.go`：确认普通用户日志格式化时会剥离 `admin_info`，管理员可见。
- `service/request_debug_log_test.go`：覆盖请求调试快照写入日志的行为。
- `service/system_task_test.go`：覆盖日志自动清理任务的默认关闭和环境变量调度。

维护重点：

- `request_debug` 必须嵌套在 `admin_info` 下；
- 非管理员查询日志时必须移除整个 `admin_info`；
- 日志写入失败路径不能因为请求调试快照导致额外业务失败。
- 自动日志清理必须默认关闭，只能删除整条旧日志，不做跨数据库 JSON 局部更新。

### 前端查看入口

- `web/default/src/features/usage-logs/types.ts`：声明 `admin_info.request_debug`、`downstream` 和 `upstream` 的前端类型。
- `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`：在管理员使用日志详情中折叠展示请求调试快照。

维护重点：

- 前端只读取既有日志 JSON，不新增后端接口或额外查询；
- 只有 `isAdmin` 为真且 `admin_info.request_debug` 存在时显示；
- 界面只如实展示当前日志保存的快照，不推断重试过程；
- 缺少 `downstream` 或 `upstream` 时只显示已有部分；
- 请求体内容继续依赖后端脱敏和截断结果，前端不要二次改写。

### 部署与文档

- `.github/workflows/deploy-image-ghcr.yml`：手动触发 GitHub Actions 构建 `deploy` 分支并推送 GHCR 镜像。
- `docs/request-debug-logging-guide.md`：使用、部署、验证和回滚指引。
- `docs/local-github-workflow.md`：GitHub fork、`origin/upstream`、`main/deploy` 和 GHCR 部署工作流。
- `docs/superpowers/specs/2026-07-19-request-debug-logging-design.md`：原始设计说明。

维护重点：

- `workflow_dispatch` 文件必须存在于 GitHub 默认分支 `main`，否则 Actions 页面不会显示；
- 实际构建分支应保持为 `deploy`；
- 部署机器只 `docker compose pull new-api`，不再本地编译。

## 不变量

上游更新或冲突修复后必须继续满足：

1. 功能默认关闭，生产默认配置为 `REQUEST_DEBUG_LOGGING=off`。
2. `error_only` 只记录错误请求快照，`always` 才记录成功和失败请求。
3. 快照只保存在 `Other.admin_info.request_debug`。
4. 普通用户日志视图不得看到 `admin_info`。
5. 下游快照反映客户端提交的请求体。
6. 上游快照反映最终发送给渠道前的 JSON 请求体。
7. 敏感字段必须脱敏。
8. 超限内容必须截断，并保留 `size`、`sha256`、`truncated`。
9. 快照采集失败不得中断 relay。
10. 自动日志清理默认关闭，由 `LOG_CLEANUP_ENABLED` 显式启用。
11. 不新增数据库 schema；前端仅提供管理员日志详情查看入口。

## 上游更新后的检查顺序

完成 `git merge upstream/main` 或把 `main` 合入 `deploy` 后，按顺序检查：

1. 先看 Git 冲突文件是否在“文件职责”列表中。
2. 如果冲突在 relay handler，确认上游是否改变请求体构造、模型映射、参数覆盖或出站请求创建点。
3. 如果冲突在 `relay/common/relay_info.go`，确认 `RelayInfo` 的请求调试字段仍能贯穿 controller、handler 和日志服务。
4. 如果冲突在 `service/log_info_generate.go`，确认 `request_debug` 仍写入 `admin_info`，并且不影响 quota、错误日志和普通日志字段。
5. 如果冲突在 `model` 日志格式化，确认非管理员视图仍剥离 `admin_info`。
6. 如果冲突在配置初始化，确认非法配置回退和默认关闭仍成立。
7. 如果冲突在前端使用日志详情，确认“请求调试快照”仍只对管理员可见，且只展示已有字段。
8. 运行聚焦测试。
9. 推送 `deploy` 后手动运行 GitHub Actions 构建镜像。

## 冲突处理策略

不要固定 cherry-pick 某个历史 commit，也不要整文件覆盖上游新文件。正确做法是按职责重新应用本地逻辑。

推荐流程：

```bash
git fetch origin
git fetch upstream
git checkout main
git pull --ff-only origin main
git merge upstream/main
git push origin main

git checkout deploy
git pull --ff-only origin deploy
git merge main
```

如果冲突很重，需要重做本地定制：

```bash
git checkout main
git pull --ff-only origin main
git checkout -b local/request-debug-refresh
```

然后参考当前 `deploy` 的差异逐项重放：

```bash
git diff main..deploy -- common relay controller service model docs .github
```

只迁移请求调试相关逻辑，不要用 `git checkout deploy -- <file>` 直接覆盖整文件，除非确认上游该文件没有新逻辑需要保留。

冲突判断原则：

- 上游新增的 provider 行为优先保留；
- 本地只补回快照采集点和日志附加点；
- 快照核心逻辑继续集中在 `relay/common/request_debug.go`；
- 配置入口继续在 `common`；
- 日志可见性继续依赖 `admin_info` 的管理员隔离。
- 前端入口继续只消费 `admin_info.request_debug`，不要新增独立日志接口。

## 验证命令

聚焦测试：

```bash
go test ./common -run InitRequestDebugConfig -count=1
go test ./relay/common -run RequestDebug -count=1
go test ./service -run RequestDebug -count=1
go test ./service -run LogCleanupHandler -count=1
go test ./model -run FormatUserLogs -count=1
cd web/default && bun run typecheck
```

相关包验证：

```bash
go test ./common ./controller ./model ./relay ./relay/common ./service \
  -run 'RequestDebug|FormatUserLogs|InitRequestDebugConfig' -count=1
```

手动验证：

1. 部署时保持 `REQUEST_DEBUG_LOGGING=off`，确认普通请求行为不变。
2. 临时切换 `REQUEST_DEBUG_LOGGING=error_only`。
3. 触发真实失败请求。
4. 管理员日志应看到 `Other.admin_info.request_debug`。
5. 管理员 Web 使用日志详情应显示“请求调试快照”折叠区。
6. 普通用户日志不应看到 `admin_info`，也不应看到“请求调试快照”。
7. 验证完成后立即恢复 `REQUEST_DEBUG_LOGGING=off` 并重建容器。

## 发布与回滚

推送部署分支：

```bash
git push origin deploy
```

手动构建镜像：

```text
Actions -> Build deploy image (GHCR) -> Run workflow
```

部署机器更新：

```bash
docker compose pull new-api
docker compose up -d --no-deps new-api
```

回滚优先使用 GHCR 固定标签：

```yaml
image: ghcr.io/kimberxu/new-api:deploy-<short-sha>
```

不要执行 `docker compose down -v`。

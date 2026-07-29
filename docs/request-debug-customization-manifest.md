# 请求调试日志 - 定制清单

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REQUEST_DEBUG_LOGGING` | `off` | `off` / `error_only` / `always` |
| `REQUEST_DEBUG_MAX_BODY_BYTES` | `32768` | 最大 body 字节数 |
| `LOG_CLEANUP_ENABLED` | `false` | 是否启用日志自动清理 |
| `LOG_CLEANUP_RETENTION_DAYS` | `30` | 日志保留天数 |
| `LOG_CLEANUP_INTERVAL_HOURS` | `24` | 清理任务执行间隔 |

## 脱敏字段

Secret keys: `authorization`, `api_key`, `apikey`, `access_token`, `refresh_token`, `key`, `token`, `password`, `secret`

## 前端展示

管理员在日志详情中可以看到 `request_debug` 面板，包含下游/上游请求体、元信息、SHA-256 校验和等。

## 文件清单

- `relay/common/request_debug.go` - 核心模块
- `common/init.go` / `common/constants.go` - 配置初始化
- `controller/relay.go` - 重试逻辑 + 错误日志集成
- `service/log_info_generate.go` - 管理员信息附加
- `service/system_task.go` - 日志清理定时任务
- `relay/*_handler.go` - 各 handler 集成点
- 前端层：`types.ts`, `request-debug.ts`, `details-dialog.tsx`

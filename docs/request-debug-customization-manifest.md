# 定制功能清单

## 请求调试日志

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REQUEST_DEBUG_LOGGING` | `off` | `off` / `error_only` / `always` |
| `REQUEST_DEBUG_MAX_BODY_BYTES` | `32768` | 最大 body 字节数 |
| `LOG_CLEANUP_ENABLED` | `false` | 是否启用日志自动清理 |
| `LOG_CLEANUP_RETENTION_DAYS` | `30` | 日志保留天数 |
| `LOG_CLEANUP_INTERVAL_HOURS` | `24` | 清理任务执行间隔 |

### 脱敏字段

Secret keys: `authorization`, `api_key`, `apikey`, `access_token`, `refresh_token`, `key`, `token`, `password`, `secret`

### 前端展示

管理员在日志详情中可以看到 `request_debug` 面板，包含下游/上游请求体、元信息、SHA-256 校验和等。

### 文件清单

- `relay/common/request_debug.go` - 核心模块
- `common/init.go` / `common/constants.go` - 配置初始化
- `controller/relay.go` - 重试逻辑 + 错误日志集成
- `service/log_info_generate.go` - 管理员信息附加
- `service/system_task.go` - 日志清理定时任务
- `relay/*_handler.go` - 各 handler 集成点
- 前端层：`types.ts`, `request-debug.ts`, `details-dialog.tsx`

---

## CustomEvent 锁移除

移除 `CustomEvent.Mutex`。同步写职责上移至调用方（流式 SSE writer），减少单次 event 渲染的锁开销。

### 文件清单

- `common/custom-event.go`

---

## 同优先级渠道重试（Same-Priority Retry）

### 功能概述

重试时新增 `ExcludeChannels` 参数，允许调用方指定排除已失败的渠道。重试时会从同优先级的可用渠道中重新选择，而非直接降级到低优先级渠道。

### 文件清单

- `service/channel_select.go` - `RetryParam` 新增 `ExcludeChannels` 和 `ExcludeChannel()`

---

## OIDC 自定义显示名称

### 功能概述

OIDC 登陆按钮可配置自定义显示名称，不设置时默认显示 "OIDC"。显示在登陆页第三方 OAuth 列表中和管理后台的 OIDC 配置区域。

### 文件清单

- `setting/system_setting/oidc.go` - `OIDCSettings` 新增 `DisplayName` 字段 + `GetEffectiveDisplayName()` 方法
- 前端层：`oauth-section.tsx`、`oauth-providers.tsx`、`types.ts`

---

## 渠道请求频率限制 (Channel Rate Limit)

### 功能概述

每渠道独立配置 RPM（每分钟请求数）、TPM（每分钟 Token 数），在分发时检查，超限返回 429。支持 Redis 和内存两种模式，全局开关 + 默认值。

### 全局配置

通过管理后台「模型设置」→ `channel_rate_limit_setting` 配置：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | `false` | 全局开关 |
| `default_rpm` | `60` | 默认每分钟请求数限制 |
| `default_tpm` | `100000` | 默认每分钟 Token 数限制 |

### 每渠道配置

在渠道编辑页面的「API 访问设置」→「Rate Limit」区域配置：

| 字段 | 类型 | 说明 |
|------|------|------|
| `rate_limit_enabled` | bool | 是否启用该渠道的限流 |
| `rate_limit_rpm` | int | 每分钟最大请求数（0 = 使用全局默认值） |
| `rate_limit_tpm` | int | 每分钟最大 Token 数（0 = 使用全局默认值） |

### 限流行为

- 窗口：固定 60 秒
- 检查时机：渠道选择后、请求转发前（`Distribute` 中间件中）
- 超限响应：HTTP 429，`{"error": {"message": "该渠道已超过速率限制,请稍后重试", ...}}`
- 回退行为：Redis 不可用时自动切到内存模式；Redis 或内存均出错时放行

### 文件清单

**后端：**
- `setting/operation_setting/channel_rate_limit_setting.go` - 全局配置定义
- `service/channel_rate_limit.go` - 核心限流逻辑
- `relaykit/dto/channel_settings.go` - `ChannelOtherSettings` 新增 `RateLimitEnabled`、`RateLimitRPM`、`RateLimitTPM`
- `middleware/distributor.go` - 分发时插入 `CheckChannelRateLimit` 检查
- `i18n/keys.go` - 新增 `MsgDistributorChannelRateLimited`
- `i18n/locales/en.yaml`、`zh-CN.yaml`、`zh-TW.yaml` - 翻译

**前端 (web/src/features/channels/):**
- `types.ts` - `ChannelOtherSettings` 接口新增字段
- `lib/channel-form.ts` - Schema、默认值、transform、buildSettingsJSON
- `components/drawers/channel-mutate-drawer.tsx` - 限流开关 + RPM/TPM 输入框 UI
# 定制功能清单（deploy 分支）

> 对应分支：`deploy` @ `91b3f9d9`（2026-08-15 更新）
> 以下功能均为 `deploy` 相对 `upstream/main` 的定制（可用 `git diff upstream/main...deploy` 核对）。
> 魔改提交：`bfa99ad6`（请求调试日志 + 日志清理 + 同优先级重试 + GHCR 构建）→ `26271295`（渠道限流 RPM/TPM）→ `e09babdf`（上下文感知限流 + float RPM）→ `102747fd`（RPM 输入 `step='any'`）→ `d0fdb047`（渠道测试请求文案定制）→ `9a20e660`（加权模型映射）→ `c6c4bcf8`（加权映射目标暴露修复）→ `83329f48`（暴露目标守卫排除 source key）→ `d8378ee7`（额度显示模式切换修复）

## 功能总览

| 功能 | 引入提交 | 上游冲突风险 |
|------|----------|--------------|
| 请求调试日志 | `bfa99ad6` | 中（`controller/relay.go`、`relay/common/relay_info.go`） |
| 日志自动清理 | `bfa99ad6` | 低 |
| 同优先级渠道重试 | `bfa99ad6` | 中（`controller/relay.go`） |
| GHCR 部署镜像构建 | `bfa99ad6` | 低 |
| GHCR 镜像自动清理 | `ae34ad6a` | 低（`.github/workflows/deploy-image-ghcr.yml`） |
| 渠道请求频率限制（RPM/TPM） | `26271295`、`e09babdf`、`102747fd` | 中（`controller/relay.go`） |
| 渠道测试请求文案定制 | `d0fdb047` | 低（`controller/channel-test.go`） |
| 加权模型映射（1 对多） | `9a20e660`、`c6c4bcf8`、`83329f48` | 中（`relay/helper/model_mapped.go`、`controller/channel_upstream_update.go`） |
| 额度显示模式切换修复 | `d8378ee7` | 低（`web/src/features/system-settings/general/pricing-section.tsx`） |
| token 大数 K/M/B 分级显示 | `ea91ebf1` | 低（`web/src/lib/currency.ts`） |

> **已上游化（非 fork 定制，无需维护）**：OIDC 自定义显示名称、`CustomEvent.Mutex` 锁移除——截至 2026-08-01 均已存在于 `upstream/main`，`deploy` 与上游文件一致。

---

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

- `relay/common/request_debug.go` - 核心模块（快照结构、脱敏、截断）
- `common/init.go` / `common/constants.go` - 配置初始化
- `controller/relay.go` - 捕获调用 + 重试逻辑 + 错误日志集成
- `service/log_info_generate.go` - 快照写入 `other.admin_info.request_debug`（仅管理员可见）
- `service/system_task.go` - 日志清理定时任务
- `relay/chat_completions_via_responses.go`、`relay/claude_handler.go`、`relay/compatible_handler.go`、`relay/gemini_handler.go`、`relay/responses_handler.go` - 各 handler 捕获集成点
- 测试：`relay/common/request_debug_test.go`、`common/request_debug_config_test.go`、`service/request_debug_log_test.go`
- 前端层：`web/src/features/usage-logs/lib/request-debug.ts`（+ `request-debug.test.ts`）、`types.ts`、`details-dialog.tsx`

---

## 日志自动清理

### 机制

`LOG_CLEANUP_ENABLED=true` 时，master 节点注册 `SystemTaskTypeLogCleanup` 定时任务（间隔 `LOG_CLEANUP_INTERVAL_HOURS` 小时），按 `LOG_CLEANUP_RETENTION_DAYS` 计算截止时间戳，分批（每批 100 条）删除 `logs` 表中超期记录，处理进度写入任务状态；一次运行全部删完为止。

### 文件清单

- `service/system_task.go` - 任务注册与执行（`runLogCleanupTask`）
- `common/init.go` / `common/constants.go` - 配置初始化

---

## GHCR 镜像自动清理

### 功能概述

`.github/workflows/deploy-image-ghcr.yml` 构建并推送镜像后，自动删除 GHCR 上的旧镜像版本，只保留最近 `KEEP=3` 个带 `deploy*` tag 的版本（含 `deploy` 滚动 tag 指向的当前版本与最近两个留档版本），其余版本全部删除。

### 背景与关键点

- 每次 buildx 构建会在 GHCR 产生 **3 个 digest version**：1 个带 tag 的主 index digest（`deploy` + `deploy-<short_sha>`）+ 2 个无 tag 的平台辅助 digest（amd64 平台 manifest + attestation manifest）。
- 清理只对**带 `deploy*` tag** 的 version 计数保留（最新 KEEP=3 个）；无 tag 的辅助 digest 不是可回滚版本，不占名额、**一律跳过不删**（见下方事故记录）。
- 历史教训（2026-08-14）：早期版本按 `created_at` 全局排序取前 3，导致无 tag 辅助 digest 挤占保留名额、误删真实留档（如 `deploy-d57f803`）。`ae34ad6a` 起改为"带 tag 版本独立编号"。
- **事故记录（2026-08-15）：删除无 tag 辅助 digest 导致 index 悬空。** 原实现"无 tag 辅助 digest 一律删除"，但这两个 digest 正是被带 tag index 引用的子 manifest（amd64 平台 manifest、attestation manifest）。删除后 index 引用悬空：curl 请求 index 返回 200（index 仍在），docker pull 解析 index 后请求子 manifest 得 404 → `manifest unknown`。症状：最新一次构建的版本可拉（辅助 digest 因 GHCR 列表最终一致性延迟逃过当次清理），**所有更早版本全部不可拉**，与镜像格式、docker 版本、网络均无关。修复：无 tag 版本一律跳过，只清理带 tag 版本；被清理 index 的孤儿辅助 manifest 仅数 KB，可忽略。
- 不使用 `gh api --paginate` + `--jq` 组合（gh CLI 逐页应用 jq 的已知问题，cli/cli#10459），先拉取完整 JSON 再本地 jq 决策。
- 清理决策写入 step summary 便于核对；删除失败仅警告不阻断构建。

### 文件清单

- `.github/workflows/deploy-image-ghcr.yml` - 构建后 `Prune old image versions` 步骤

---

## 同优先级渠道重试（Same-Priority Retry）

### 功能概述

重试时新增 `ExcludeChannels` 参数，允许调用方指定排除已失败的渠道。重试时会从同优先级的可用渠道中重新选择，而非直接降级到低优先级渠道。

### 文件清单

- `service/channel_select.go` - `RetryParam` 新增 `ExcludeChannels` 和 `ExcludeChannel()`;auto-group 循环按排除集合驱动的层内重选与切组
- `model/channel_cache.go` - 排除驱动选择:过滤 `ExcludeChannels` 后在剩余渠道中取最高优先级层,层内加权随机;该层耗尽才级联到低优先级
- `model/ability.go` - 非内存缓存(DB)回退路径始终选择最高优先级渠道(`MAX(priority)` 子查询)
- `controller/relay.go` - 请求失败后调用 `retryParam.ExcludeChannel(channel.Id)`;全渠道均被限流时返回 429

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
| `rate_limit_rpm` | float | 每分钟最大请求数（0 = 使用全局默认值；支持小数） |
| `rate_limit_tpm` | int | 每分钟最大 Token 数（0 = 使用全局默认值） |

### 限流行为

- 窗口：固定 60 秒
- 检查时机：渠道选择后、请求转发前（`Distribute` 中间件中）
- 单渠道超限：跳过该渠道并排除出重试候选，继续尝试同优先级其他渠道
- 全渠道均被限流：HTTP 429 + `Retry-After: 60`，错误码 `channel:rate_limited`，客户端可据此退避
- 回退行为：Redis 不可用时自动切到内存模式；Redis 或内存均出错时放行
- 演进：`26271295` 引入基础 RPM/TPM；`e09babdf` 改为上下文感知（仅对当前请求生效）+ RPM 支持小数；`102747fd` 前端 RPM 输入框加 `step='any'`

### 文件清单

**后端：**
- `setting/operation_setting/channel_rate_limit_setting.go` - 全局配置定义
- `service/channel_rate_limit.go` - 核心限流逻辑
- `relaykit/dto/channel_settings.go` - `ChannelOtherSettings` 新增 `RateLimitEnabled`、`RateLimitRPM`、`RateLimitTPM`
- `middleware/distributor.go` - 分发时插入 `CheckChannelRateLimit` 检查
- `controller/relay.go` - 限流渠道排除重试 + 全渠道限流 429 兜底
- `constant/context_key.go` - `ContextKeyAllChannelsRateLimited`、`ContextKeyChannelRateLimitRetryAfter`
- `model/channel_cache.go` - 渠道缓存携带限流配置
- `relaykit/types/error.go` - `ErrorCodeChannelRateLimited`
- `i18n/keys.go` - 新增 `MsgDistributorChannelRateLimited`
- `i18n/locales/en.yaml`、`zh-CN.yaml`、`zh-TW.yaml` - 翻译

**前端 (web/src/features/channels/):**
- `types.ts` - `ChannelOtherSettings` 接口新增字段
- `lib/channel-form.ts` - Schema、默认值、transform、buildSettingsJSON
- `components/drawers/channel-mutate-drawer.tsx` - 限流开关 + RPM/TPM 输入框 UI

---

## 渠道测试请求文案定制

### 功能概述

很多中转渠道把单个 `hi` 列为屏蔽词（视为无意义刷量请求），导致渠道测试/自动健康检查误报失败。本次将渠道测试（`buildTestRequest`）向上游发送的用户消息由 `hi` 改为「彩虹有几种颜色」——一个简单、少见、非敏感的中文短问句，几乎不会命中中转屏蔽词表。

覆盖全部含用户消息的测试请求格式：OpenAI chat/completions、OpenAI Responses、Responses compact、Claude、Gemini。（Embedding / Image / Rerank 测试请求不含用户消息文本，不受影响。）

### 文件清单

- `controller/channel-test.go` - `buildTestRequest` 中 7 处测试用户消息（`content` / `text` / `input`）

---

## 加权模型映射（1 对多）

### 功能概述

上游渠道可能存在多个等价模型（如 `deepseek-v4-flash`、`deepseek-ai/deepseek-v4-flash-0731`），希望下游用一个模型名按权重分发到多个上游模型。原有 `model_mapping` 仅支持 1:1（字符串 value），本次扩展为支持加权数组 value：

```json
{
  "ds-v4": [
    {"model": "deepseek-v4-flash", "weight": 5},
    {"model": "deepseek-ai/deepseek-v4-flash-0731", "weight": 3}
  ],
  "glm-5.2": "GLM-5.2"
}
```

同一映射对象中 1:1 与 1:N 可混用。`weight` 缺省（缺失或 `null`）时补全为 `1`；负权重 / 非数字权重由后端拒绝。

### 行为

- 请求到达选中渠道后，`ModelMappedHelper` 解析 mapping：value 为数组时按权重随机选一个 target，随后照常走链式映射（A → B → C）
- 计费基于 `OriginModelName`（下游模型名），选中哪个上游模型不影响价格
- 上游模型同步（`channel_upstream_update.go`）会收集加权数组中的全部 target，避免误删

### 前端

- 渠道编辑页 model mapping 编辑器的视觉模式将加权条目渲染为只读摘要（`加权映射 model×weight, ...`），编辑需切换到 JSON 模式
- 「将重定向的上游模型从 Models 移除」守卫（`channel-mutate-drawer.tsx`）会展开加权数组，识别全部 target；`83329f48` 起同时排除「既是 source key 又是 target」的模型——source key 是路由入口，从 Models 移除会使渠道不可达
- 保存时（`channel-form.ts` 的 `normalizeModelMapping`）自动为缺失 `weight` 的条目补全为 `1`

### 文件清单

- `relay/common/weighted_model.go` - `WeightedModelItem` 类型
- `relay/helper/model_mapped.go` - `resolveModelMappingValue` / `pickWeightedModel` 加权解析与随机选择
- `controller/channel_upstream_update.go` - `normalizeChannelModelMapping` 返回 `map[string][]string`，`collectMappingTargets` 提取数组 target
- 前端：`model-mapping-editor.tsx`、`channel-form.ts`、`model-mapping-validation.ts`、`channel-mutate-drawer.tsx`
- 测试：`relay/helper/model_mapped_test.go`、`controller/channel_upstream_update_test.go`
- 前端测试：`web/src/features/channels/lib/__tests__/model-mapping-validation.test.ts`（`findExposedTargetModels` 排除 source key、加权数组展开、无效输入）

---

## 额度显示模式切换修复

### 功能概述

上游 `pricing-section.tsx` 存在「鸡生蛋」逻辑：`Tokens Only` 下拉选项仅在当前已是 TOKENS 模式时渲染（`showTokensOnlyOption = displayType === 'TOKENS'`），导致站点处于 USD/CNY 等货币显示模式时，管理后台 UI **无法切换到 token 显示**——只能靠直接改库或 API 绕过。本次去掉该条件，Display Mode 下拉四项（USD / CNY / Custom Currency / Tokens Only）始终可选。

### 背景

个人用户往往不关心金额，更关注 token 用量/请求数；该修复让「系统设置 → 计费 → Currency & Display → Display Mode」可直接切换到 `Tokens Only`，全站额度以 token 数值显示（overview 卡片、日志统计等）。

### 文件清单

- `web/src/features/system-settings/general/pricing-section.tsx` - 删除 `showTokensOnlyOption` 条件，TOKENS 选项无条件渲染

---

## token 大数 K/M/B 分级显示

### 功能概述

上游 `formatNumberWithSuffix`（`web/src/lib/currency.ts`）对缩写路径只支持 `k` 后缀（÷1000 + 1 位小数）。额度显示切到 `Tokens Only` 后，百万/亿级 token 数显示为 `950003.7k` 这类难以直读的格式。本次增加 M/B 分级：

- `32,153,700` → `32.2M`
- `950,003,700` → `950M`
- `50,158,280,000` → `50.2B`

仅影响 `abbreviate` 缩写路径（overview 摘要卡片、统计徽章等）；日志页等 `abbreviate: false` 的完整数字显示不受影响；货币（USD/CNY）分支走 Intl 格式化，不经此函数。

### 文件清单

- `web/src/lib/currency.ts` - `formatNumberWithSuffix` 增加 `M`/`B` 分级

# 定制功能清单（deploy / personal 分支）

> 对应分支：`personal` 基线 `317e9ddd`（2026-08-29 刷新至 `949e1e69`；deploy 线功能部分，可用 `git diff upstream/main...317e9ddd` 核对。注意：`deploy` 分支自分叉点 `2ffa3979`（原 deploy-re，已退役）后已另行演进，分支拓扑见 `docs/local-github-workflow.md`）
> 以下功能均为魔改线相对 `upstream/main` 的定制；文末「personal 分支半重构登记」小节单独登记 `personal` 相对基线 `317e9ddd` 的改动。
> 魔改提交：`f10d688f`（上游模型自动删除开关与筛选模型）→ `ee6da30d`（请求调试日志 + 日志清理 + 同优先级重试 + GHCR 构建）→ `a5a2304f`（渠道限流 RPM）→ `6a12bc8d`（上下文感知限流 + float RPM）→ `48f9c2e2`（RPM 输入 `step='any'`）→ `fab8e37f`（渠道测试请求文案定制）→ `d840c4fb`（加权模型映射）→ `3ecd81c9`（加权映射目标暴露修复）→ `d23122a5`（暴露目标守卫排除 source key）→ `484d024c`（额度显示模式切换修复）→ `99cc5e56`（token 大数 K/M/B 分级显示）→ `827b6092`（manifest 登记 token 大数）→ `44ac09de`（三文档头部标记刷新）→ `bf00be83`（504/524 超时重试开关 + 超时自动禁用）→ `cda0a61f`（token 显示改进：删除 Token 后缀）→ `e033cc91`（流式结束原因分类与中断流语义）→ `6ff43dbc`（实时连接追踪）→ `ad37eb30`（manifest 登记实时连接追踪）→ `b1e3ff0c`（恢复上游 stream_status_test.go + 拆分分类测试）→ `3958b068`（实时连接表格优化）→ `e8078e55`（尾部随机请求 ID + 下游/上游双模型列）→ `30286246`（三文档头部标记刷新至 c759de26）→ `c0272220`（滑动窗口渠道自动禁用）→ `db70cf02`（partial_failure length 收尾 + 异常流记错误日志）→ `678cdb6c`（实时连接侧边栏入口迁至 general 组）→ `33f8aa0f`（日…

## 魔改开发约定（合并上游友好）

- **改动最小化是硬约束**：新增魔改功能时，独立逻辑优先用新增文件承载（新 service/controller/middleware/组件/API 客户端），避免改动既有文件。
- 必须改动既有文件时，限制为最小必要 diff——只做纯追加/局部插入，不修改、不删除、不重排已有代码，能不改就不改。
- 每次新增魔改前按此顺序审查：先问「这个文件能不能不动」，再问「改动能不能再小」；改动文件越少、越偏向新增文件，后续合并 `upstream/main` 冲突越少。
- 例外：`docs/` 文档与 `AGENTS.md` 登记类改动属于分支约定本身，不受最小化约束。
- **扩展点织入是改动最小化的执行形态**：新魔改必须过两道审查——①能否完全新文件承载；②必须动既有文件时，是否只插入「挂载点行」（函数调用 / 中间件 / context key / 路由行），魔改逻辑本体是否全部在新增文件中。两者都过才允许动既有文件。
- 每项魔改在总览表「扩展点形态」列登记实际形态；`内联` 形态为负债项，后续同步出现冲突时优先顺手迁移（把逻辑抽到新文件、原位置留挂载点调用）。
- 例外不变：`docs/` 与 `AGENTS.md` 登记类改动不受最小化约束。

## 功能总览

| 功能 | 引入提交 | 上游冲突风险 | 扩展点形态 | 上游实现替代 |
|------|----------|--------------|------------|--------------|
| 请求调试日志 | `ee6da30d` | 中（`controller/relay.go`、`relay/common/relay_info.go`） | `挂载点`+`独立文件`（核心在 relay/common/request_debug.go，relay.go 为挂载点） | 否 |
| 日志自动清理 | `ee6da30d` | 低 | `挂载点`+`独立文件`（service/system_task.go 任务注册挂载） | 否 |
| 同优先级渠道重试 | `ee6da30d` | 中（`controller/relay.go`） | `内联`（controller/relay.go 内）→ 待迁移 | 待观察（upstream `fix/tiered-retry-billing-followups` 主题相邻；上游合并后审查能否退役本地版） |
| 504/524 超时重试开关与自动禁用 | `bf00be83` | 中（`controller/relay.go`、`setting/operation_setting/status_code_ranges.go`、系统设置前端） | `挂载点`（relay.go 接入点 + status_code_ranges.go 独立） | 否 |
| 流式结束原因分类与中断流语义 | `e033cc91` | 中（`relay/common/stream_status.go`、`relay/channel/openai/relay-openai.go`、`service/log_info_generate.go`） | `挂载点`（stream_status.go 新文件 + relay-openai.go 接入） | 否 |
| 实时连接追踪 | `6ff43dbc`、`3958b068`、`e8078e55` | 中（`controller/relay.go`、`router/api-router.go`、`service/inflight_tracker.go`） | `独立文件`（service/inflight_tracker.go）+ `挂载点`（relay.go） | 否 |
| GHCR 部署镜像构建 | `ee6da30d` | 低 | `挂载点`+`独立文件`（workflow 文件独立） | 否 |
| GHCR 分支前缀镜像 tag | `b7419616` | 低（`.github/workflows/deploy-image-ghcr.yml`） | `内联`（workflow 步骤原地改造） | 否 |
| GHCR 镜像自动清理 | `3db875f5` | 低（`.github/workflows/deploy-image-ghcr.yml`） | `挂载点`+`独立文件`（workflow 步骤独立） | 否 |
| 渠道请求频率限制（RPM） | `a5a2304f`、`6a12bc8d`、`48f9c2e2` | 中（`controller/relay.go`） | `内联` → 待迁移 | 否 |
| 渠道测试请求文案定制 | `fab8e37f` | 低（`controller/channel-test.go`） | `内联`（channel-test.go）→ 待迁移 | 否 |
| 加权模型映射（1 对多） | `d840c4fb`、`3ecd81c9`、`d23122a5` | 中（`relay/helper/model_mapped.go`、`controller/channel_upstream_update.go`） | `内联`（relay/helper/model_mapped.go）→ 待迁移 | 否 |
| 上游模型自动删除与筛选 | `f10d688f` | 中（`controller/channel_upstream_update.go`、`relaykit/dto/channel_settings.go`、前端渠道抽屉） | `内联`（channel_upstream_update.go 既有魔改文件内扩展）→ 待迁移 | 否 |
| 额度显示模式切换修复 | `484d024c` | 低（`web/src/features/system-settings/general/pricing-section.tsx`） | `内联`（前端）→ 待迁移（低风险可不迁） | 否 |
| token 大数 K/M/B 分级显示 | `99cc5e56`、`cda0a61f` | 低（`web/src/lib/currency.ts`） | `内联`（前端）→ 待迁移（低风险可不迁） | 否 |
| 滑动窗口渠道自动禁用 | `c0272220`、`9edda449` | 中（`service/channel.go`、`controller/relay.go`、`controller/channel-test.go`、系统设置前端） | `独立文件`（service/channel_disable_window.go）+ `挂载点`（channel.go/relay.go/channel-test.go） | 否 |
| 日志 t/s 计算排除 TTFT | `33f8aa0f` | 低（`web/src/features/usage-logs/`） | `内联`（前端）→ 待迁移（低风险可不迁） | 否 |
| 渠道流速率降级（含首字延迟 TTFT 降级） | `088876eb`、`7832b0e9`、`f9198749` | 中（`model/channel_cache.go`、`model/model_group_select.go`、`service/quota.go`、`service/text_quota.go`、`web/src/features/system-settings/models/routing-reliability-section.tsx`） | `独立文件`（pkg/channel_slowstream/）+ `挂载点`（channel_cache.go/model_group_select.go/服务计费） | 否 |
| 模型级路由表前台化与模型级禁用（含 auto-ban 细化） | `cd98b0f8`、`9edda449` | 中（`controller/relay.go`、`model/channel_cache.go`、`model/ability.go`、`controller/channel-test.go`） | `独立文件`（model/channel_disabled_model.go、service/channel_model_disable.go、controller/channel_ability.go、web/src/features/channel-abilities/）+ `挂载点`（channel_cache.go、ability.go、relay.go、channel-test.go、channel.go、main.go、channel-router.go） | 否 |
| 渠道密钥查看放开安全验证 | `dbde0b31`（cherry-pick 自 deploy `570561d1`） | 低（`router/channel-router.go`） | `内联`（纯删一行中间件，无可迁移逻辑） | 否 |

> **标注口径**：「扩展点形态」按各功能详情章节文件清单判定（`独立文件`=新文件承载全部逻辑；`挂载点`=既有文件仅插入少量挂载调用；`内联`=逻辑直接改在既有文件中，为负债项，后续同步冲突时优先迁移）；「上游实现替代」以实施时 `remotes/upstream/*` 可见主题为准，发现新对应分支即改标 `待观察` 并注明分支名。

> **已上游化（非 fork 定制，无需维护）**：OIDC 自定义显示名称、`CustomEvent.Mutex` 锁移除——截至 2026-08-01 均已存在于 `upstream/main`，`deploy` 与上游文件一致。

---

## 504/524 超时重试开关与自动禁用

### 功能概述

系统设置 → 模型设置 → Routing Reliability → Request retry 增加 `Retry 504/524 timeouts` 开关。关闭时保留默认安全行为：504/524 即使包含在自动重试状态码范围内也不重试。开启后，504/524 遵循 `AutomaticRetryStatusCodes` 配置，并可切换到其它可用渠道。

### 风险与渠道处理

504/524 可能表示请求已经到达上游并开始处理。开启重试可能造成重复计费、重复生成和请求放大，因此开关默认关闭。

**自动禁用行为变更（2026-08-16）**：此前 504/524 被 `ShouldDisableByStatusCode` 硬编码为无条件强制禁用（`IsAlwaysSkipRetryStatusCode` 直接 return true），一次超时即禁用渠道，对瞬时网络抖动过于激进。现已改为遵循用户配置的 `AutomaticDisableStatusCodeRanges`：默认范围（仅 401）不包含 504/524，因此默认不再因超时禁用渠道。管理员如需 504/524 自动禁用，在系统设置「自动禁用状态码」中显式添加即可。

### 文件清单

- `common/constants.go` / `model/option.go` - 持久化与运行时加载 `AutomaticRetryTimeoutEnabled`
- `setting/operation_setting/status_code_ranges.go` - 504/524 重试门控与自动禁用判定
- `controller/relay.go` - 普通请求与任务请求重试路径接入开关
- `web/src/features/system-settings/models/routing-reliability-section.tsx` - 系统设置开关
- `web/src/i18n/locales/*.json` - 七语言文案
- `setting/operation_setting/status_code_ranges_test.go` - 默认关闭/开启行为回归测试

---


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

## 流式结束原因分类与中断流语义

### 功能概述

对流式请求的异常终止（如 HTTP/2 `RST_STREAM`、上游断流、客户端取消）建立统一的分类模型，修复此前「日志显示 error、性能指标计成功」的语义分裂：

- `StreamStatus` 新增 `Outcome`（`success` / `partial_failure` / `failed` / `cancelled`）与 `FailureDomain`（`none` / `upstream` / `downstream` / `gateway` / `protocol`）分类方法
- 消费日志 `other.stream_status` 增加 `outcome`、`failure_domain` 字段；前端详情页对 `cancelled` 显示预警色徽章，并展示故障归属
- 性能指标（`perfmetrics.RecordRelaySample`）与 `REQUEST_DEBUG_LOGGING=error_only` 快照改按真实流结果判断，不再一律按成功处理
- OpenAI chat 流处理器按分类落地终止语义：
  - 零输出上游失败（`scanner_error` / `timeout`，尚未向下游写入任何模型数据）→ 转为可重试渠道错误（502），由现有重试循环切换渠道；预扣会话兜底，最终失败全额退还
  - 已输出部分内容后上游中断（`partial_failure`）→ 补发最后一块，以 `finish_reason=length` 的终止块 + `[DONE]` 正常收尾，**不发送 error event**——SDK/终端编程工具遇 error event 会直接中断整个任务；`length` 为标准枚举，客户端标记「输出截断」并正常收尾
  - 客户端放弃（`client_gone` / `ping_fail`）→ 停止向下游输出，按已收内容结算，不归咎上游、不触发重试
- 消费日志类型按流结果修正：`partial_failure` / `failed`（非 cancelled）的请求记录为**错误类型日志**（`LogTypeError`），日志列表直接可见异常；`cancelled` 保持消耗类型（前端已用 warning 徽章标识）

### 风险与兼容

- 仅 OpenAI 共享流处理器区分处理；Claude/Gemini/图片等其它 handler 仍沿用原语义（分类与日志字段对其同样生效），`partial_failure` 时 Claude/Gemini 转换场景仍发错误事件兜底（无标准「截断」终止枚举）
- 可重试仅限「零输出 + upstream/gateway 域」（`scanner_error`、`timeout`），避免部分输出后重试造成重复生成与重复上游计费
- `stream_status.status` 保持 `ok`/`error` 兼容值，前端读取 `outcome` 区分 `cancelled`
- 消费日志类型改为 `LogTypeError` 后，该记录不计入「消耗」统计（`SumUsedQuota` 按 `type=2` 过滤），改在错误日志列表展示；计费扣减不受影响
- 与上游冲突风险：`relay/common/stream_status.go`、`relay/channel/openai/relay-openai.go`（`OaiStreamHandler` 尾部语义分派）、`service/log_info_generate.go`（`appendStreamStatus`、`consumeLogTypeForStream`）、`model/log.go`（`RecordConsumeLogParams.LogType`）

### 文件清单

- `relay/common/stream_status.go` - `StreamOutcome` / `StreamFailureDomain` 与 `Outcome()` / `FailureDomain()` 分类
- `relay/common/relay_info.go` - `StreamSucceeded()` 统一判定入口
- `relay/channel/openai/relay-openai.go` - 语义分派 + `terminateInterruptedStream`（`finish_reason=length` 收尾；Claude/Gemini 兜底 `sendStreamErrorEvent`）
- `service/log_info_generate.go` - `appendStreamStatus` 分类字段 + `consumeLogTypeForStream` 日志类型判定 + `error_only` 按流结果附加快照
- `service/text_quota.go` / `service/quota.go` - 消费日志按流结果写 `LogTypeError` + 性能指标按 `StreamSucceeded()` 记录
- `model/log.go` - `RecordConsumeLogParams.LogType` 覆盖日志类型
- 前端：`web/src/features/usage-logs/types.ts`、`details-dialog.tsx`、`web/src/i18n/locales/*.json`（`Failure Domain`）
- 测试：`relay/common/stream_outcome_test.go`（分类逻辑，新增文件）、`relay/channel/openai/relay-openai-stream_test.go`、`service/text_quota_test.go`（`TestConsumeLogTypeForStream`）

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

`.github/workflows/deploy-image-ghcr.yml` 构建并推送镜像后，自动删除 GHCR 上的旧镜像版本。同一 workflow 服务 `deploy` / `personal` 两条分支线，镜像 tag 前缀由构建来源推导（`deploy-image` tag / dispatch `deploy` → `:deploy`；`personal-image` tag / dispatch `personal` → `:personal`），清理**只对带本次构建前缀 tag** 的版本计数，各前缀家族独立保留最近 `KEEP=3` 个（含滚动 tag 指向的当前版本与最近两个留档版本），其余删除，跨前缀互不挤占。

### 背景与关键点

- 每次 buildx 构建会在 GHCR 产生 **3 个 digest version**：1 个带 tag 的主 index digest（`deploy` + `deploy-<short_sha>`）+ 2 个无 tag 的平台辅助 digest（amd64 平台 manifest + attestation manifest）。
- 清理只对**带 `deploy*` tag** 的 version 计数保留（最新 KEEP=3 个）；无 tag 的辅助 digest 不是可回滚版本，不占名额、**一律跳过不删**（见下方事故记录）。
- 历史教训（2026-08-14）：早期版本按 `created_at` 全局排序取前 3，导致无 tag 辅助 digest 挤占保留名额、误删真实留档（如 `deploy-d57f803`）。`3db875f5` 起改为"带 tag 版本独立编号"。
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

每渠道独立配置 RPM（每分钟请求数），在分发时检查，超限返回 429。支持 Redis 和内存两种模式，全局开关 + 默认值。

### 全局配置

通过管理后台「模型设置」→ `channel_rate_limit_setting` 配置：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | `false` | 全局开关 |
| `default_rpm` | `60` | 默认每分钟请求数限制 |

### 每渠道配置

在渠道编辑页面的「高级设置」→「Routing & Overrides」→「Rate Limit」区域配置（2026-08-17 起，此前位于「API 访问设置」；纯前端布局迁移，表单字段、`settings` JSON 存储结构与后端逻辑均不变）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `rate_limit_enabled` | bool | 是否启用该渠道的限流 |
| `rate_limit_rpm` | float | 每分钟最大请求数（0 = 使用全局默认值；支持小数） |

### 限流行为

- 窗口：固定 60 秒
- 检查时机：渠道选择后、请求转发前（`Distribute` 中间件中）
- 单渠道超限：跳过该渠道并排除出重试候选，继续尝试同优先级其他渠道
- 全渠道均被限流：HTTP 429 + `Retry-After: 60`，错误码 `channel:rate_limited`，客户端可据此退避
- 回退行为：Redis 不可用时自动切到内存模式；Redis 或内存均出错时放行
- 演进：`a5a2304f` 引入基础 RPM；`6a12bc8d` 改为上下文感知（仅对当前请求生效）+ RPM 支持小数；`48f9c2e2` 前端 RPM 输入框加 `step='any'`；2026-08-17 前端 UI 迁移：限流配置从「API 访问设置」移入「高级设置」→「Routing & Overrides」子节（新增 nav 跳转项、已配置状态高亮与自动展开）

### 文件清单

**后端：**
- `setting/operation_setting/channel_rate_limit_setting.go` - 全局配置定义
- `service/channel_rate_limit.go` - 核心限流逻辑
- `relaykit/dto/channel_settings.go` - `ChannelOtherSettings` 新增 `RateLimitEnabled`、`RateLimitRPM`
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
- `components/drawers/channel-mutate-drawer.tsx` - 限流开关 + RPM 输入框 UI（位于「高级设置」→「Routing & Overrides」子节，nav 可跳转、已配置自动展开）

---

## 渠道测试请求文案定制

### 功能概述

很多中转渠道把单个 `hi` 列为屏蔽词（视为无意义刷量请求），导致渠道测试/自动健康检查误报失败。历史演进：

- 初始：将测试用户消息由 `hi` 改为「彩虹有几种颜色」——一个简单、少见、非敏感的中文短问句，几乎不会命中中转屏蔽词表。
- 2026-08-19 增强：固定文案「彩虹有几种颜色」本身成为新的测活指纹（上游可按文本/请求结构匹配），改为**随机问句池 + 随机 max_tokens**：

  - `testUserMessages`：9 条简短、答案固定、无需发散思考的常识性问句（如「中国的首都是哪里」「水的化学式是什么」），每次测试随机取一条；刻意避开经典测活题（`hi`/`ping`/`test`/算术题/纯数字日历题）。
  - `testMaxTokensRange [16, 64]`：chat 类测试请求的 `max_tokens` 在该范围内随机（此前固定 16/50），打散请求长度指纹。
  - 问句池与范围均为包级变量，后续调整无需改动请求构造逻辑。

覆盖全部含用户消息的测试请求格式：OpenAI chat/completions、OpenAI Responses、Responses compact、Claude、Gemini。（Embedding / Image / Rerank 测试请求不含用户消息文本，不受影响。）

### 文件清单

- `controller/channel-test.go` - `buildTestRequest` 中 7 处测试用户消息（`content` / `text` / `input`）、`testUserMessages` 问句池、`pickTestUserMessage` / `pickTestMaxTokens` 随机选择
- `controller/channel_test_request_test.go` - 回归测试：消息来自问句池、无历史固定测活文案、随机覆盖全池、max_tokens 范围与随机性

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
- 「将重定向的上游模型从 Models 移除」守卫（`channel-mutate-drawer.tsx`）会展开加权数组，识别全部 target；`d23122a5` 起同时排除「既是 source key 又是 target」的模型——source key 是路由入口，从 Models 移除会使渠道不可达
- 保存时（`channel-form.ts` 的 `normalizeModelMapping`）自动为缺失 `weight` 的条目补全为 `1`

### 文件清单

- `relay/common/weighted_model.go` - `WeightedModelItem` 类型
- `relay/helper/model_mapped.go` - `resolveModelMappingValue` / `pickWeightedModel` 加权解析与随机选择
- `controller/channel_upstream_update.go` - `normalizeChannelModelMapping` 返回 `map[string][]string`，`collectMappingTargets` 提取数组 target
- 前端：`model-mapping-editor.tsx`、`channel-form.ts`、`model-mapping-validation.ts`、`channel-mutate-drawer.tsx`
- 测试：`relay/helper/model_mapped_test.go`、`controller/channel_upstream_update_test.go`
- 前端测试：`web/src/features/channels/lib/__tests__/model-mapping-validation.test.ts`（`findExposedTargetModels` 排除 source key、加权数组展开、无效输入）
---

## 上游模型自动删除与筛选

### 功能概述

上游模型检测（渠道「检测上游模型设置」）在原有「自动同步新增（auto-sync）」基础上补充两个独立能力：

- **自动删除（Auto Delete）**：开启 `upstream_model_update_auto_delete_enabled` 后，巡检检出「上游已不存在、本地仍在」的模型时，在定时任务（`allowAutoApply`）路径下直接将其从 `channel.Models` 移除并同步模型组；手动 detect 仅暂存待审。与 auto-sync 对称——自动新增只加不删，自动删除只删不加，两者可独立开关。
- **筛选模型（Include Filter）**：`upstream_model_update_include_filter` 为逗号分隔列表（精确名或 `regex:` 前缀正则）。非空时变更计算只取命中的模型——未命中的上游模型不会被加入，未命中的本地模型也不会因上游缺失被删除（对本地模型天然提供保护）。

### 行为与安全

- 自动删除仅在 `allowAutoApply=true`（定时巡检）时生效；手动 `detect_all` / `detect` 仅暂存。
- **空列表保护**：上游返回空模型列表视为「获取不到模型」（如上游 502 / 空响应）。此时既不执行自动删除，也不把误判的「全部本地模型可删除」写进 staging，避免一次空响应误删全部本地模型。
- 删除判定仍排除 `model_mapping` 的 source（虚拟别名从不因上游缺失删除），并复用加权映射 target 收集逻辑避免误删。

### 文件清单

- `relaykit/dto/channel_settings.go` - `ChannelOtherSettings` 新增 `UpstreamModelUpdateAutoDeleteEnabled`、`UpstreamModelUpdateIncludeFilter`
- `controller/channel_upstream_update.go` - `modelMatchesAnyFilter`（精确 + 正则命中）；`collectPendingUpstreamModelChangesFromModels` 增加 include 过滤；`collectPendingUpstreamModelChanges` 返回上游模型数；自动删除与空列表保护分支；任务汇总 `auto_removed_models`
- `controller/channel_upstream_update_test.go` - 自动删除、空列表保护、include 筛选回归测试
- 前端：`types.ts`、`lib/channel-form.ts`、`lib/channel-form-errors.ts`、`components/drawers/channel-mutate-drawer.tsx`
- `web/src/i18n/locales/*.json` - 7 语言文案

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

---

## 实时连接追踪（In-Flight Request Tracking）

### 功能概述

在日志页面新增「实时连接」分页，管理员可实时查看当前正在进行中（尚未成功或失败）的中转请求。每个条目显示请求 ID、所选渠道、模型名、请求路径、发起时间与已耗时。列表每 3 秒自动刷新，已耗时列每秒计时。

**侧边栏入口迁移（2026-08-17）**：入口从 `admin` 组移至 `general` 组，位于「使用日志」下一项，与使用日志同级。`requiredRole: ADMIN` 守卫保持仅管理员可见。

### 机制

- 请求进入 `Relay` / `RelayTask` / `RelayMidjourney` 后调用 `service.Start`，在 Redis 中以 Hash 记录请求元信息（`inflight:<request_id>`），并在 ZSET（`inflight:sorted`）按时间戳排序以支持分页
- 渠道选择/重试时通过 `service.UpdateChannel` 更新渠道 ID
- **上游模型预解析**：渠道选择后（`addUsedChannel`）、handler 执行前，调用 `helper.ModelMappedHelper(c, relayInfo, nil)` 预解析 `model_mapping` 并通过 `service.UpdateUpstreamModel` 写入 Redis。handler 内部会通过 `InitChannelMeta` + `ModelMappedHelper` 重新解析（创建全新 `ChannelMeta`），handler 执行后的 `UpdateUpstreamModel` 用最终值覆盖。这样请求从开始就显示映射后的上游模型名，而非在整个 handler 执行期间显示空/fallback 到下游模型名。对 1:1 映射完全准确；对加权映射，预解析显示一个可能的 target，handler 后用实际选中值覆盖
- 请求结束时（`defer service.Finish`）从 Redis 删除条目
- TTL 10 分钟兜底，防止进程崩溃产生残留
- Redis 不可用时全部操作降级为 no-op，不影响请求正常处理
- 管理员 API `GET /api/inflight` 返回分页列表（AdminAuth）

### 安全

- 仅管理员可访问（`AdminAuth` 中间件）
- 不记录请求体内容，仅记录路由级元信息（请求 ID、渠道、模型、路径、时间）
- Redis 不可用时静默降级，不暴露错误给下游

### 文件清单

**后端：**
- `service/inflight_tracker.go` - 核心模块（`Start`、`Finish`、`UpdateChannel`、`List`、`Count`）
- `controller/inflight.go` - Admin API `GetInflightRequests`
- `controller/relay.go` - `Relay` / `RelayTask` / `RelayMidjourney` 接入 `Start` + `Finish`；`addUsedChannel` 接入 `UpdateChannel`
- `router/api-router.go` - 路由注册 `GET /api/inflight`
- `service/inflight_tracker_test.go` - 单元测试

**前端：**
- `web/src/features/usage-logs/inflight-api.ts` - API 客户端
- `web/src/features/usage-logs/components/inflight-table.tsx` - 实时连接表格组件
- `web/src/features/usage-logs/section-registry.tsx` - 新增 `inflight` section
- `web/src/features/usage-logs/index.tsx` - 渲染分发
- `web/src/hooks/use-sidebar-data.ts` - 侧边栏导航项（位于 `general` 组「使用日志」下一项，`requiredRole: ADMIN` 守卫；不新增 `use-sidebar-config.ts` 映射：URL 不在映射表时默认可见，`requiredRole` 在 `use-sidebar-view.ts` 独立过滤）
- `web/src/i18n/locales/*.json` - 七语言文案

---

## 滑动窗口渠道自动禁用

### 功能概述

将渠道自动禁用机制从「一次命中即禁用」改为滑动窗口计数：同一渠道在时间窗口内出现同一错误达到阈值次数才执行禁用。两层策略：

- **已配置错误**（状态码在 `AutomaticDisableStatusCodeRanges` 内，或错误消息匹配 `AutomaticDisableKeywords`）：严格窗口，默认 10 分钟内 2 次即禁用。
- **未配置错误**（不在上述配置范围内，但 statusCode 在 1xx-5xx 合理范围内）：宽容窗口，默认 5 分钟内 3 次才禁用。

错误标识 key = `渠道ID + StatusCode`，不使用 errorCode（上游错误几乎都是 `bad_response_status_code`，区分度不够）或错误消息（含 request id、时间戳等，计数碎片化）。

**2026-08-22 增强（原因细化）**：`CheckAndRecordDisable` 改为返回 `(bool, string)`，第二值为触发详情（如 `3 failures in 300s window (threshold 3)`）。禁用原因不再只有笼统的 `repeated failures within window`：未配置错误原因改为 `channel disabled: status_code=X (N failures in Ws window)`；已配置错误在原错误文案后附加 `; N failures in Ws window (threshold T)`。渠道级与模型级（见「模型级路由表前台化」节）禁用原因分别带 `channel disabled:` / `model disabled:` 前缀，前端可区分粒度。

### 配置

系统设置 → 模型设置 → Routing Reliability → Auto-disable rules 区域新增 4 个配置项：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `ConfiguredDisableWindowSeconds` | `600` | 已配置错误窗口（秒） |
| `ConfiguredDisableThreshold` | `2` | 已配置错误触发禁用次数 |
| `UnconfiguredDisableWindowSeconds` | `300` | 未配置错误窗口（秒） |
| `UnconfiguredDisableThreshold` | `3` | 未配置错误触发禁用次数 |

阈值设为 1 等同于「一次即禁用」，与改造前行为一致。

### 实现细节

- Redis 模式：Lua 脚本 `LPUSH key now; LTRIM key 0 threshold-1; EXPIRE key window; LLEN key`，返回 count >= threshold 则触发禁用。key 格式 `channelDisableWindow:{channelID}:{statusCode}:{tier}`，tier 为 `configured` 或 `unconfigured`。
- 内存模式：复用 `common.InMemoryRateLimiter`（滑动窗口），传 `threshold-1` 作为 maxRequestNum（因为 `Request` 允许 maxRequestNum 次后返回 false）。threshold=1 时直接返回 true（一次即禁用）。
- Redis 不可用时自动切到内存模式；Redis 或内存均出错时 fail-open（不触发禁用）。

### 文件清单

- `service/channel_disable_window.go` - 滑动窗口核心（Redis Lua + 内存兜底，新文件）
- `service/channel.go` - `DisableDecision` 结构体 + `ShouldDisableChannelWithDecision`（原 `ShouldDisableChannel` 改为 wrapper；原因携带窗口详情）
- `controller/relay.go` - `processChannelError` 调用 `ShouldDisableChannelWithDecision`
- `controller/channel-test.go` - `shouldBanChannel` 调用 `ShouldDisableChannelWithDecision`
- `common/constants.go` / `model/option.go` - 4 个配置变量持久化
- `web/src/features/system-settings/models/routing-reliability-section.tsx` - 4 个输入框 UI
- `web/src/features/system-settings/models/index.tsx` / `section-registry.tsx` / `types.ts` - 默认值与字段透传
- `web/src/i18n/locales/*.json` - 七语言文案
- `service/channel_disable_window_test.go` - 单元测试（含触发详情文案断言）

---

## 日志 t/s 计算排除 TTFT

### 功能概述

日志列表的 t/s（每秒 token 数）列原先按 `completion_tokens / use_time`（总耗时秒）计算，把首字延迟（TTFT/prefill）计入分母，系统性低估真实生成速度。本次改为 `completion_tokens / (use_time - frt_seconds)`，其中 `frt`（First Response Time，毫秒）已存在于日志 `other.frt`；`frt` 缺失（非流式或数据缺失）时回退原行为。

### 文件清单

- `web/src/features/usage-logs/components/columns/common-logs-columns.tsx` - 桌面端日志列 t/s 计算
- `web/src/features/usage-logs/components/usage-logs-mobile-card.tsx` - 移动端日志卡片 t/s 计算

---

## 渠道流速率降级（Channel Slow-Stream Demotion）

### 功能概述

利用成功流式请求的 generation TPS（`outputTokens / generationMs * 1000`），按 `(channel_id, model)` 维度统计慢速事件，达到阈值后临时将该渠道在对应模型上的优先级拍平到 `DemotedPriority`（默认 0），定时到期自动恢复。目标：慢渠道跌出最高优先级层、少承担流量，避免直接禁用导致容量骤减。

**2026-08-17 增强（配置 UI + 排除渠道）**：默认开启（`enabled=true`），`min_tps` 默认 `8.0`；新增 `exclude_channel_ids` 排除列表，填入的渠道编号不参与采样与降级（含历史降级记录即时失效）；全部参数在「系统设置 → 模型与路由 → Routing Reliability → Slow stream demotion」可视化配置，不再需要 API 手工下发。

**2026-08-17 增强（首字延迟 TTFT 降级）**：新增第二降级源——首字延迟（`FirstResponseTime - StartTime`）超过 `max_ttft_ms`（默认 5000ms）计为慢事件，独立配置 `ttft_enabled`/`ttft_sample_size`/`ttft_threshold`。与生成速率降级互不干扰：两源各自维护计数与降级标记，任一触发即降级（`GetDemotedPriority` 合并两源取 `min`）；排除渠道、降级时长（`demote_duration_sec`）与降级优先级（`demoted_priority`）两源共用。TTFT 采样设 `min_input_tokens` 门槛（默认 0），仅对输入足够长的请求采样首字延迟，过滤短请求噪声（短输入的 frt 基本不受 prefill 影响）。

**2026-08-18 增强（固定数量窗口 / ring buffer）**：判定从「时间窗口内连续慢」改为「最近 `sample_size` 次采样中慢事件达 `threshold` 次即触发」。快慢结果都入队，快结果只挤掉最旧样本、不洗白历史慢记录（容错更高，针对持续慢的渠道）；长请求不再受影响——3-5 分钟甚至更长的请求结束后才采样，但窗口按样本数量判定而非时间，不会因采样延迟导致窗口过期。`window_seconds`/`ttft_window_seconds` 废弃不再生效（保留字段兼容旧配置）。`threshold` 默认从 `1` 调至 `3`（1 次波动即降级容错太低）。

**2026-08-18 增强（渠道页图标展示）**：渠道列表名称旁新增两个图标——降级中（`GET /api/channel/demoted` 轮询，显示降级模型与剩余恢复时长，5s 刷新）与渠道级速率限制已配置（`rate_limit_enabled` 且 rpm > 0）。图标仅展示，不影响渠道行为。
**2026-08-22 增强（DB 回退路径接入 + 查询合并 + 徽章降级原因）**：模型组 DB 回退选择器 `GetRandomSatisfiedChannelFromGroups`（MemoryCache 关闭时生效）同样按 `(channelId, model)` 应用降级拍平，语义与内存路径一致（原先该路径完全无视降级——状态照显、路由不避让）；内存路径选择改为单次选择内每渠道只查询一次降级优先级、两段循环复用，Redis 模式两源降级标记合并为一次 `MGET`（原先最多 4N 次 GET/请求）；`GET /api/channel/demoted` 的 `DemotionInfo` 新增 `sources` 字段（`tps`=生成速率过慢、`ttft`=首字延迟过慢），同一渠道同一模型两源同时降级合并为一条记录、剩余时长取较大值，渠道列表降级徽章悬停显示各模型的降级原因与剩余恢复时长。

### 判定与降级语义

- **样本**：仅成功流式请求（`IsStream && StreamSucceeded()`）；失败流式不参与（已有滑动窗口自动禁用机制），非流式无 `FirstResponseTime` 无法计算 generation TPS / TTFT，不参与
- **ring buffer 判定**：每个 `(channelId, model)` 保留最近 `sample_size` 次采样结果（快慢都记），其中慢事件数 ≥ `threshold` 即触发降级；快结果不洗白，只把最旧样本挤出窗口；`min_output_tokens` 过滤短请求噪声
- **首字延迟（TTFT）**：`frt = FirstResponseTime - StartTime`（毫秒）超过 `max_ttft_ms` 计为慢事件，独立 ring buffer（`ttft_sample_size`/`ttft_threshold`）判定；与生成速率降级独立计数、独立降级标记，任一生效即降级；`min_input_tokens` 过滤短输入请求噪声（短输入 frt 不受 prefill 影响）
- **排除渠道**：`exclude_channel_ids` 中的渠道在 `RecordSlowStream` / `RecordSlowTtft` / `GetDemotedPriority` 处均直接放行——不计数、不触发降级，且已存在的降级记录即时失效（改配置即可立即解除）
- **只降 priority，不动 weight**：降级渠道跌出原优先级层，同组更高层渠道存在时不被选中；重试链路上更高层渠道全部被排除后，降级渠道回到候选按原 weight 参与加权随机——不会永久饿死（单渠道场景仍照常选中）
- **只有一档，无阶梯**：已降级时再次慢速仅续期 `demoted_until`，不叠加、不进一步降；恢复后再次达到阈值才触发新一轮降级
- **快结果不取消降级**：快结果只入队挤掉最旧样本，不清除进行中的降级标记；降级到期由惰性检查（`GetDemotedPriority`）与后台清理（`CleanupExpiredDemotions`）恢复
- **fail-open**：Redis 出错时记录与查询均放行，不影响请求

### 配置

系统设置 → 模型与路由 → Routing Reliability → Slow stream demotion（即 `channel_slow_stream_setting`）：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | `true` | 生成速率降级开关 |
| `min_tps` | `8.0` | TPS 下限（tokens/s）；仅统计生成阶段，不含首字延迟 |
| `window_seconds` | `300` | **[废弃]** 时间窗口秒数，2026-08-18 起不再生效（ring buffer 按样本数量判定），保留兼容旧配置 |
| `sample_size` | `5` | ring buffer 容量：保留最近多少次采样结果 |
| `threshold` | `3` | 窗口内慢事件次数触发降级（须 ≤ sample_size） |
| `min_output_tokens` | `50` | 生成速率最小输出 token 数门槛 |
| `min_input_tokens` | `0` | TTFT 采样最小输入 token 数门槛（0=采样全部） |
| `demote_duration_sec` | `600` | 降级持续时间秒 |
| `demoted_priority` | `0` | 降级后优先级（拍平到此值；对原本 priority=0 的渠道无位置变化，如需更低可配负值） |
| `exclude_channel_ids` | 空 | 排除渠道编号列表（逗号分隔），不参与采样与降级（含生成速率与首字延迟两源） |
| `ttft_enabled` | `true` | 首字延迟降级开关 |
| `max_ttft_ms` | `5000` | 首字延迟上限（毫秒）；长 prompt 的 prefill 会拉高此值 |
| `ttft_window_seconds` | `300` | **[废弃]** 首字延迟时间窗口秒数，2026-08-18 起不再生效，保留兼容旧配置 |
| `ttft_sample_size` | `5` | 首字延迟 ring buffer 容量 |
| `ttft_threshold` | `3` | 窗口内首字延迟慢事件次数触发降级（须 ≤ ttft_sample_size） |

### 实现细节

- 内存模式：`sync.Map`，key = `channelId:model`，value = `demotionState{results []bool, slowCount, demotedUntil}`；采样结果入队，超出 sample_size 弹出最旧样本并同步 slowCount，slowCount ≥ threshold 触发降级
- Redis 模式：窗口 key `slowStream:{channelId}:{model}`（生成速率）与 `slowStream:ttftWindow:{channelId}:{model}`（首字延迟）复用 `LPUSH`（1=慢/0=快）+ `LTRIM`（保留 sample_size 条）+ `LRANGE` 统计慢次数 Lua；降级标记 key `slowStream:demoted:{channelId}:{model}` / `slowStream:ttftDemoted:{channelId}:{model}`（SET + EXPIRE，value = demotedUntil 时间戳）
- 采样入口 `service.RecordFromRelayInfo` 挂在 `PostAudioConsumeQuota` / `PostTextConsumeQuota` 的 `gopool.Go` 中（纯追加一行，与 `RecordRelaySample` 并列）；同一适配器内先采样 TTFT（`frt`）再采样生成 TPS，两源独立调用
- `GetDemotedPriority` 合并两源：任一降级标记未过期即降级，取 `min(originalPriority, demoted_priority)`
- 渠道选择 `GetRandomSatisfiedChannel` 单次选择内每渠道预计算一次降级优先级（`demotedPriority` map），highestPriority 计算段与 targetChannels 收集段复用；weight 与加权随机段不动；不修改缓存中的 `*Channel` 对象，降级为运行时计算。`GetDemotedPriority` Redis 分支两源标记合并为一次 `MGET`；模型组 DB 回退选择器 `GetRandomSatisfiedChannelFromGroups` 同样按 `(channelId, model)` 应用降级拍平（effectivePriority → GetDemotedPriority → 取最高层）

### 文件清单

- `pkg/channel_slowstream/tracker.go` - 双源滑动窗口（生成速率 + 首字延迟）+ 降级记录 + 查询 + 后台恢复（新文件，只依赖 `common` / `setting/operation_setting`，避免 `model → service` 与 `relay/common → operation_setting` 循环）
- `service/channel_slow_stream_record.go` - `RecordFromRelayInfo` 采样适配（过滤 + TPS/TTFT 计算，新文件）
- `setting/operation_setting/channel_slow_stream_setting.go` - 全局配置注册（新文件）
- `model/channel_cache.go` - `GetRandomSatisfiedChannel` 单次选择预计算每渠道降级优先级并两段复用（import `pkg/channel_slowstream`）
- `model/model_group_select.go` - `GetRandomSatisfiedChannelFromGroups` DB 回退选择器按 `(channelId, model)` 应用降级拍平（import `pkg/channel_slowstream`）
- `service/quota.go` / `service/text_quota.go` - 采样点各追加一行 `RecordFromRelayInfo` 调用
- `main.go` - `channelslowstream.Init()` 后台恢复 goroutine 入口
- `controller/channel-demoted.go` - `GET /api/channel/demoted` 降级状态查询（新文件）
- `router/channel-router.go` - 注册 /demoted 路由（ChannelRead 权限）
- `web/src/features/channels/components/channels-provider.tsx` - 5s 轮询降级状态注入 context
- `web/src/features/channels/components/channels-columns.tsx` - 名称列降级图标 + 速率限制图标（降级 tooltip 显示模型/降级原因/剩余时长，限流 tooltip 显示 RPM）
- `web/src/features/channels/types.ts` - `DemotedChannelInfo` 新增 `sources` 字段（tps/ttft 降级来源，tooltip 展示用）
- `web/src/features/channels/lib/channel-utils.ts` - `formatSeconds` 剩余时长格式化
- `web/src/features/system-settings/models/routing-reliability-section.tsx` - Routing Reliability 下新增 Slow stream demotion 表单块（生成速率 8 项 + 首字延迟 4 项 + 排除渠道输入）
- `web/src/features/system-settings/models/utils.ts` - 排除渠道显示/提交格式转换（`parseExcludeChannelIds` / `serializeExcludeChannelIds`）
- `web/src/features/system-settings/models/index.tsx` / `section-registry.tsx` / `types.ts` - 默认值（默认开启、min_tps 8.0、threshold 3、sample_size 5、max_ttft_ms 5000、ttft_threshold 3）与字段透传
- `web/src/i18n/locales/*.json` - 七语言文案
- 测试：`pkg/channel_slowstream/tracker_test.go`（未启用/阈值触发/ring buffer 滑动窗口——快结果压出最旧样本不洗白/长间隔仍连续计数/续期/到期恢复/清理/快结果不取消降级/排除渠道不计数不降级/排除后历史降级即时失效/首字延迟阈值触发/两源独立计数互不干扰/首字延迟排除渠道/sample_size 小于 threshold 时 sanitize 兜底/ListDemoted 两源合并为一条含 sources）、`model/channel_slow_stream_selection_test.go`（降级渠道跌出最高层 + 高层耗尽后级联 + MemoryCache 关闭时 DB 回退路径同样避让与饿死恢复）、`service/channel_slow_stream_record_test.go`（min_input_tokens 门槛生效）、`web/src/features/system-settings/models/__tests__/exclude-channel-ids.test.ts`（排除渠道格式转换）

---

## 模型级路由表前台化与模型级禁用（含 auto-ban 细化）

### 功能概述

new-api 的路由索引是 `abilities` 表（渠道×分组×模型），但管理面只有渠道级：启用/禁用、自动禁用（auto-ban）都作用于整个渠道。上游某个模型不可用（404 model not found / 400 invalid model 等）时，只能整个渠道一起禁，误伤同渠道其他可用模型。本次实现：

- **渠道模型路由表前台化**：新增独立管理页面 `/channel-abilities`（侧边栏 Admin 组「渠道路由表」+ 渠道页「路由表」入口按钮），展示渠道×模型能力行（渠道名/类型、模型、分组、优先级、权重、状态徽章），支持渠道/模型/分组/状态筛选与分页；行内可禁用/启用单模型、「测试此模型」。
- **禁用粒度细化为「渠道×模型」**：新增独立禁用表 `channel_disabled_models`（channel_id × model，source=manual|auto，reason），路由构建（内存缓存 `InitChannelCache` 与 DB 路径 `getChannelQuery` NOT EXISTS）排除被禁的 (channel, model) 对。独立表隔离于「channel.status ↔ ability.enabled」既有同步不变式——渠道重新启用不会抹掉模型级禁用。
- **auto-ban 模型级判定**：`processChannelError` 对明确模型类错误（404 消息含 model，或 400/422 命中模型关键词）只禁该渠道该模型（source=auto），不再整个渠道禁；模型维度独立滑动窗口，key = `channelID:modelName:statusCode:tier`——明确模型类错误按 configured 严格档计数，未分类兜底错误按 unconfigured 宽容档计数（2026-08-24 兜底模型级化）。
- **2026-08-22 增强（原因前缀 + 渠道列表徽章）**：模型级禁用原因加 `model disabled:` 前缀并携带窗口详情（`N failures in Ws window (threshold T)`），存入 `channel_disabled_models.reason`；新增 `GET /api/channel/disabled_models`（ChannelRead）返回全部禁用记录；渠道列表名称旁新增 Ban 徽章（30s 轮询），悬停列出各模型的来源 auto/manual、原因、剩余封禁时长或永久——此前模型级禁用在渠道页完全不可见，与渠道级自动禁用无法区分。
- **测试语义细化到模型级**：测试模型 A 通过只恢复 A（手动测试恢复任意来源，自动周期仅恢复 auto 来源）；渠道级禁用仍由任一模型测试通过整体恢复，语义不变。
- **2026-08-24 兜底模型级化**：`processChannelError` 中渠道级/模型级判定均不命中的未分类错误，原兜底进渠道级宽容窗口（反复裸 404 会拖垮整个渠道），现改为兜底进**模型级宽容窗口**（`CheckAndRecordDisableModel(..., false)`，默认 5 分钟 3 次禁该模型，封禁键同样经 `ResolveModelGroupUpstreamModel` 解析）；管理员显式配置的规则（状态码范围 / 关键词 / channel-error）仍走渠道级严格窗口，skip-retry 错误任何层级都不计数。分类逻辑抽为 `service.IsConfiguredDisableError` 供决策函数与兜底分支共用。

### 风险与渠道处理

- 渠道级恢复语义不变（`ShouldEnableChannel` + `EnableChannel` 不动）；自动测试保持每渠道单模型串行，不做全模型并发测试。
- 明确模型类错误按 configured 档计数（`isConfiguredError=true`），语义上属于「明确错误」；未分类兜底错误按 unconfigured 宽容档计数（2026-08-24 兜底模型级化，`controller/relay.go`）。
- 多 key 渠道：禁用记录按 channel_id 作用于整个渠道（所有 key），不做 key 粒度细分，与渠道级禁用粒度一致。
- 渠道更新模型列表不清除禁用记录（记录独立存在）；模型不在渠道 models 中时 ability 行不存在、路由表页不展示孤儿记录，属预期行为。
- 404 宽松判定（消息含 model 即模型级）若线上误判（上游整体 404 页面含 model 字样），可收紧为「含 model 且含 not found/not exist」，同步更新 `IsModelLevelError` 测试。

### 文件清单

**新增：**
- `model/channel_disabled_model.go` - `ChannelDisabledModel` 表模型 + CRUD（`AddChannelDisabledModels` OnConflict 幂等、`DeleteChannelDisabledModels*`、`EnableChannelModelDisabled` 按 source 清除）；`(channel_id, model)` 复合唯一索引使幂等生效
- `service/channel_model_disable.go` - `IsModelLevelError` 判定、`CheckAndRecordDisableModel` 模型维度滑动窗口（Redis Lua + 内存双模式，仿 `channel_disable_window.go`）、`DisableChannelModel` / `EnableChannelModel`（写库 + `InitChannelCache` + `NotifyRootUser`）
- `controller/channel_ability.go` - `GetChannelAbilities`（两步组装：SQL 筛 channel/model/group + 内存合并禁用记录做 status 过滤与分页）、`DisableChannelAbilities` / `EnableChannelAbilities`
- `web/src/features/channel-abilities/` - 前端页面（`lib/api.ts`、`index.tsx`、`components/channel-abilities-table.tsx`）
- `web/src/routes/_authenticated/channel-abilities/index.tsx` - 路由（ROLE.ADMIN 守卫，路径 `/channel-abilities`）

**改动（挂载点/最小插入）：**
- `model/main.go` - AutoMigrate 注册 `&ChannelDisabledModel{}`
- `model/channel.go` - 三处渠道删除清理（单删、批量删事务内、`DeleteDisabledChannel` 子查询）
- `model/channel_cache.go` - `InitChannelCache` 加载禁用记录并在 `group2model2channels` 构建循环过滤
- `model/ability.go` - `getChannelQuery` 追加 NOT EXISTS 子查询；新增 `GetChannelAbilities` / `GetChannelDisabledModelsByChannelIds`
- `controller/relay.go` - `processChannelError` 模型级分支（包进 else，task relay 传 nil 走渠道级）；模型级原因带 `model disabled:` 前缀与窗口详情
- `controller/channel-test.go` - `testResult.modelName`（命名返回值 + defer 注入）、`performChannelTests` 失败/成功分支模型级禁用与恢复、`TestChannel` 手动测试成功恢复任意来源
- `router/channel-router.go` - 注册 `/abilities`、`/abilities/disable`、`/abilities/enable`
- `service/channel.go` - `IsConfiguredDisableError` 分类器（2026-08-24 兜底模型级化抽出），`ShouldDisableChannelWithDecision` 复用
- `controller/channel-disabled-models.go` - `GET /api/channel/disabled_models` 模型级禁用记录查询（新文件）
- `web/src/hooks/use-sidebar-data.ts` - Admin 组「渠道路由表」菜单项
- `web/src/features/channels/index.tsx` - Actions 区「路由表」入口按钮
- `web/src/i18n/locales/*.json` - 七语言文案（新增 8 键）

**测试：**
- `service/channel_disable_window_test.go` - 渠道维度滑动窗口（configured/unconfigured 阈值与各维独立）+ `ShouldDisableChannelWithDecision` 决策 + `TestIsConfiguredDisableError`（状态码范围/关键词命中、未分类不命中、nil；2026-08-24 兜底模型级化新增）
- `service/channel_model_disable_test.go` - `IsModelLevelError` 表驱动（404 宽松档/400/422 关键词/非 404-400-422 不判/nil）+ 模型维度窗口（模型/状态码/渠道独立、threshold 0/1）
- `model/channel_disabled_model_test.go` - DB 路径排除、缓存路径排除、Add 幂等、按 source 清除、渠道清理
- `model/task_cas_test.go` - 测试 DB AutoMigrate 注册新表

**2026-08-22 渠道列表展示（挂载点）：**
- `web/src/features/channels/api.ts` / `types.ts` - `getChannelDisabledModels` + `ChannelDisabledModelInfo`
- `web/src/features/channels/components/channels-provider.tsx` - 30s 轮询注入 context
- `web/src/features/channels/components/channels-columns.tsx` - 名称列 Ban 徽章 + 悬停明细（来源/原因/剩余时长）


## 渠道密钥查看放开安全验证

### 功能概述

上游对查看渠道密钥（`POST /api/channel/:id/key`）强制要求安全验证（2FA / Passkey 的 `X-Security-Proof` Proof）。对本地/单管理员部署而言这一步多余。本改动移除该路由上的 `SecureVerificationRequired` 中间件：Root 管理员登录后点「Reveal key」直接返回明文密钥，不再弹安全验证对话框。

保留的防线：仍有 `AdminAuth` + `RootAuth`（仅 Root 管理员）+ `CriticalRateLimit` + `DisableCache`；前端「Reveal key」按钮也仅对 ROOT 角色渲染（`canRevealChannelKey`）。查看密钥的审计日志（`channel.key_view`）保留不变。前端 `withVerification` 流程无需改动——它先无 Proof 直连、遇 `SECURITY_PROOF_REQUIRED` 才弹验证框，后端放开后首跳即成功。

### 文件清单

- `router/channel-router.go` - 移除 `middleware.SecureVerificationRequired()`（纯删一行，保留 RootAuth/CriticalRateLimit/DisableCache）
- `controller/channel.go` - `GetChannelKey` 注释更新（说明已去掉安全验证中间件）
- `web/src/features/channels/api.ts` - `getChannelKey` 注释更新（纯注释，无逻辑改动）

---

# personal 分支半重构登记（模型组路由 + 计费/Ollama/订阅/OAuth/注册移除）

> 对应分支：`personal`（基于基线 `317e9ddd`，原 deploy-model 分支已退役，2026-08-24 更新）
> 本小节登记 `personal` 相对基线 `317e9ddd` 的半重构（`git log 317e9ddd..personal` 核对）。
> 魔改提交序列：`cfddd71b`（模型组接管路由）→ `b6925d13`（错误分级与模型级到期恢复）→ `d0f1ea52`（计费功能级移除）→ `a7e70937`（前端计费 UI 删除）→ `1ee3129d`（i18n 孤儿 key 清理）→ `6745718c`（移除 Ollama 渠道）→ `1219dfc6`（订阅后端残留清理）→ `d2c72bfe`（移除 OAuth/Passkey 登录）→ `b962fc25`（移除开放注册与 OAuth/Passkey 前端残余）→ `c99427f3`（新建模型组前端 feature）→ `c58905d9`（模型组列表工具栏）→ `5b797304`（模型组列表关键词筛选 + 排序工具栏）→ `b7419616`（GHCR 构建支持分支前缀镜像 tag）→ `9f70c191`（修复成员优先级/权重继承失效）→ `30fb8c53`（上游请求改用成员真实上游模型）→ `076db805`（移除系统设置 Billing 页残留）→ `a2473546`（模型组引用成员开放编辑）→ `66a000ae`（添加成员界面全量列表化 + 搜索）→ `8727d3ce`（勾选多选批量添加）→ `fe9cc024`（模型级禁用键解析成员上游模型 + 模型组页封禁显示与列序调整）→ `ea91c322`（成员视图透出渠道实时状态 + 页面渠道禁用徽章）→ `b1d030b6`（禁用徽章悬停显示级别/原因/时间）→ `8d34b68b`（成员测试按钮 + 测试通过即解禁）→ `711a845c`（未分类错误兜底改走模型级宽容窗口）

## 模型组路由表（一等公民）

### 功能概述

将「模型路由表」升级为一等公民的**模型组管理**：组名 = 路由模型名（下游展示名，可与上游模型无关），成员 = 渠道上的真实上游模型，按成员优先级/权重路由。支持手动建组/调参/启用禁用、组级参数覆盖。请求上游时自动把成员的真实上游模型织入该渠道的 `model_mapping`（显式配置的同名条目优先），relay 零改动。

**引用成员编辑放开（2026-08-21）**：手动组内来自引用组的展开成员此前只读；现开放优先级/权重/启停编辑。语义为**全局生效**——被编辑的就是来源组的成员记录，所有引用方与来源组同步变化（后端 `UpdateGroupItem` 本就按 itemId 更新、无组属校验，零后端改动）；前端仅移除三处 `disabled={!!item.source_group}` 并在 References 区追加影响范围提示文案（七语言）；成员删除仍仅限来源组操作。

**添加成员界面全量列表化 + 勾选批量添加（2026-08-21）**：手动组「Add Member」对话框的渠道模式下，不再「先选渠道、再选该渠道模型」级联选择，改为一次列出**全部渠道的全部（渠道, 模型）组合**（数据源为分页拉取的全量渠道 `models` 字段）：关键词框（`ComboboxInput` 惯例样式，模型名/渠道名均可搜）+ 复选框勾选列表（`max-h-64` 滚动），支持**一次勾选多个、单击 Add 批量提交**——逐项调用既有 `POST /items`，全部成功才关对话框并 toast 成功数；部分失败时保留失败项勾选、toast 列出失败明细。顺带退役只为级联服务的 `GET /api/model-groups/channels/:id/models` 接口（controller/router/前端 api 函数三处删除）。

**成员级封禁显示与模型级禁用键修正（2026-08-22）**：`processChannelError` 的模型级自动禁用此前以请求模型名（= 组名）作禁用键，而缓存排除、DB 排除与页面匹配均按 `channel_disabled_models.model` = 成员真实上游模型——手动组（组名≠成员模型，如 ox→real-model）的禁用记录永远无法命中，被封成员仍参与选路。现记录前经 `ResolveModelGroupUpstreamModel` 把禁用键解析为路由条目模型（auto 组同名成员解析为空、回落组名，行为不变）。`/model-groups` 页面首次消费后端既有 `GroupItemView.Disabled` 字段：组头显示「N banned」警告徽章；成员行模型列显示封禁徽章——auto 来源显示剩余时长（tooltip 为 reason，已过期显示「Banned」），manual 来源显示「Disabled」；成员表 Model/Channel 列互换（Model 在前）。i18n 新增 3 键七语言。

**成员视图透出渠道实时状态 + 渠道禁用徽章（2026-08-22）**：渠道被手动/自动禁用后，模型组路由三条路径（内存缓存索引、DB 兜底选路、`/v1/models` 列表）均按 `channels.status = enabled` 实时过滤、行为本就正确；但 `/model-groups` 页面此前无法看出成员所在渠道已被禁用（`GroupMemberView` 不带渠道状态），成员仍显示为正常启用，管理员易误判其仍在参与路由。现 `GroupMemberView` 新增 `channel_status int` 字段（复用 `getGroupMemberViews` 既有的逐成员 `GetChannelById` 解析，零额外查询），前端在成员行渠道列追加徽章：手动禁用红色「Manually Disabled」、自动禁用黄色「Auto Disabled」（均复用既有七语言 key，未新增文案）。

**禁用徽章悬停显示级别/原因/时间（2026-08-24）**：上一增强的徽章只有文字、无法区分「渠道级禁用」与「模型级禁用」，也看不到原因与时间。现两类徽章均改为 Tooltip 悬停详情：模型级（`disabled` 徽章）由原生 `title` 升级为 Tooltip，显示来源（Auto/Manual）、原因、禁用时间（`channel_disabled_models.created_at`，前端类型补齐该字段）与恢复倒计时/永久；渠道级（`channel_status` 2/3）徽章新增 Tooltip，显示原因与时间——后端 `GroupMemberView` 新增 `channel_status_reason string` / `channel_status_time int64` 字段，取自渠道 `other_info.status_reason/status_time`（复用既有逐成员 `GetChannelById` 解析，仅非启用状态解析，零额外查询）。i18n 新增 `Channel-level disabled` 键七语言。

**成员测试按钮 + 测试通过即解禁（2026-08-24）**：`/model-groups` 页面成员 Actions 列新增测试按钮（Zap 图标），对成员的 `(渠道, 真实上游模型)` 直接发起一次 `testChannel` 探测——新端点 `POST /api/model-groups/items/:itemId/test`（`controller/model_group_member_probe.go` 新文件承载，路由挂 `ChannelOperate` 权限对齐 `/api/channel/test`）。探测成功即清除该 `(channel, model)` 的**任意来源**模型级禁用记录（`service.EnableChannelModel(source="")`，与渠道手动测试语义一致：手动探测是权威判定）——补上被自动/手动禁用成员此前只能在渠道页间接解禁的缺口，页面内即可完成「测试 → 解禁」；失败时透出上游错误 toast、徽章保留。响应结构同 `TestChannel`（success/message/time/error_code），顺带更新渠道响应时间。前端 per-row pending 动画；被禁成员成功后 toast「Test passed, member re-enabled」，未禁成员显示「Test passed」。i18n 新增 `Test passed` / `Test passed, member re-enabled` 两键七语言，并顺带归位上次漏跑 sync 排序的 `Channel-level disabled` 键。

**未分类错误兜底模型级化（2026-08-24，`711a845c`）**：`processChannelError` 中渠道级/模型级判定均不命中的未分类错误（如裸 404 page not found），原兜底进渠道级宽容窗口——上游路径配置错误的反复 404 会把整个渠道拖进渠道级禁用，误伤同渠道其他可用模型。现兜底改为**模型级宽容窗口**（`CheckAndRecordDisableModel(..., false)`，默认 5 分钟 3 次只禁该成员，封禁键同样经 `ResolveModelGroupUpstreamModel` 解析）；管理员显式配置的规则（状态码范围/关键词/channel-error，经新分类器 `service.IsConfiguredDisableError` 判定）仍走渠道级严格窗口，skip-retry 错误任何层级不计数。

### 文件清单

**新增：**
- `model/model_group.go` - `ModelGroup`/`ModelGroupItem` 表模型 + `ParamOverride` 字段 + `SetModelGroupParamOverride`/`GetModelGroupByNameMap`/`DeleteModelGroupItemsNotIn`/`GetEnabledModelGroupsWithItems`
- `model/model_group_sync.go` - `SyncModelGroupForChannel`/`SyncAllModelGroups`（幂等）
- `model/model_group_select.go` - DB 兜底 `GetRandomSatisfiedChannelFromGroups`（selectAll=true 取全字段含 key）
- `controller/model_group.go` - 组/成员 CRUD + `SetModelGroupParamOverride` handler
- `controller/model_ban_recovery.go` - `recoverExpiredModelBans`/`decideModelBanRecovery`
- `router/model-group-router.go` - `/api/model-groups` 路由（AdminAuth + ChannelRead/Write 权限）
- `web/src/features/model-groups/` - 前端页面（组列表 + 创建/启用禁用/删除）
- `web/src/routes/_authenticated/model-groups/index.tsx` - 路由 `/model-groups`
- `model/model_group_repair.go` - 一次性数据修复 `repairModelGroupItemInheritance`：成员 Priority/Weight 曾带 gorm `default:0`，GORM 对 nil 指针省列并回填 0，把「继承渠道值」（NULL）落库成显式 0 覆盖；修复去掉 tag 并把存量 0 值重置为 NULL（options 表 flag 保证只跑一次，修复后的显式 0 覆盖不受影响）
- `model/model_group_upstream.go` - `ResolveModelGroupUpstreamModel`/`ApplyModelGroupMemberMapping`：路由名（组名）与上游模型解耦——手动组（如 ox）成员记录真实上游模型，选渠后把 `{组名: 上游模型}` 合并进该渠道 `model_mapping`，由既有 ModelMappedHelper 完成请求改写；显式渠道映射同名条目优先；内存缓存路径读 `modelGroupItemOverrides`（结构新增 model 字段），无缓存路径直接查表
- `controller/model_group_member_probe.go` - `TestModelGroupItem` handler（成员测试端点，探测成功清任意来源模型级禁用）；配套 `model.GetModelGroupItem`（纯追加于 model_group.go）

**改动（挂载点/最小插入）：**
- `model/main.go` - AutoMigrate 注册 `&ModelGroup{}`/`&ModelGroupItem{}`；`migrateDB` 挂载 `repairModelGroupItemInheritance`
- `model/ability.go` - `GetGroupEnabledModels`/`GetEnabledModels` 数据源改模型组（/v1/models）
- `model/channel_cache.go` - 路由索引数据源改模型组成员；`modelGroupItemOverrides`/`modelGroupParamOverride` 缓存（override 结构含成员上游模型）；`effectivePriority`/`effectiveWeight`
- `controller/relay.go` - `processChannelError` 渠道级判定优先于模型级；模型级分支禁用键经 `ResolveModelGroupUpstreamModel` 解析为成员真实上游模型；未分类错误兜底改走模型级宽容窗口（2026-08-24，`711a845c`）
- `middleware/distributor.go` - 组级参数覆盖逐 key 覆盖渠道级；`SetupContextForSelectedChannel` 织入成员上游模型映射
- `model/channel_disabled_model.go` - `BannedUntil` 字段
- `controller/channel-test.go` - 开头接入到期恢复
- `web/src/hooks/use-sidebar-data.ts` - Admin 组「Model Groups」菜单项
- `web/src/features/channels/index.tsx` - 路由表入口改指 `/model-groups`
- `web/src/features/model-groups/components/model-groups-page.tsx` - 组头「N banned」徽章 + 成员行封禁徽章（剩余时长/reason tooltip）+ Model/Channel 列互换
- `web/src/i18n/locales/*.json` - 新增 3 键（`Banned` / `Banned ({{time}})` / `{{count}} banned`）七语言

**新增（自动重建路由）：**
- `controller/model_group_rebuild.go` - `RebuildModelGroups` handler：遍历所有已启用渠道，对 auto_sync=true 的渠道应用 pending 上游变更（add/remove）后同步模型组，对 auto_sync=false 的渠道直接同步（不触发检测、不改动 channel.Models）；随后 `model.DeleteEmptyAutoModelGroups()` 清理成员清空的 auto 组（连带悬空引用，manual 组不触碰），删除组名记 SysLog 并以 `removed_groups` 返回；最后 `model.InitChannelCache()`；前端 `/model-groups` 页面「Rebuild Model Routing」按钮触发
- **自动组成员增删权限收归系统**：`controller/model_group.go` `AddGroupItem`/`DeleteGroupItem` 对 `source=auto` 组返回错误（成员只由渠道初始化/重建路由增删），自动组仅允许手动改优先级/权重/启停；前端隐藏自动组「Add Member」按钮并禁用成员删除按钮
- `router/model-group-router.go` - 注册 `POST /api/model-groups/rebuild`（权限 ChannelWrite）
- `web/src/features/model-groups/lib/api.ts` - `rebuildModelGroups()` 函数
- `web/src/features/model-groups/components/model-groups-page.tsx` - 页面顶部添加「Rebuild Model Routing」按钮（带旋转 loading 动画）；按钮/toast 四个文案 key 已补齐七语言 locale（eed9994c）
- `web/src/features/model-groups/components/model-groups-page.tsx` - 新增列表工具栏：关键词筛选（匹配组名或成员模型名）+ 排序（默认「手动组在前」，可选「名称」「成员数量」，支持升降序切换）

## 计费功能级移除

预扣/结算/扣费/额度限制全部短路（永不扣费、永不限额）；**DB 表与字段保留（零迁移）**。任务平台渠道（Midjourney/Suno/Kling）免费放行。abilities 表保留同步但不再参与选路。

- `service/billing.go` - `PreConsumeBilling`/`SettleBilling` 短路
- `service/tiered_settle.go` - `PrepareTieredBillingForSelectedGroup` 短路（消模型价格门槛）
- `service/quota.go` - `PostConsumeQuota`/`postConsumeQuotaWithResult`/`PreWssConsumeQuota` 短路
- `relay/mjproxy_handler.go` - 删用户额度硬检查
- 删除：`controller/topup*.go`、`redemption.go`、`subscription*.go`、`service/subscription_reset_task.go`、`service/waffo_pancake.go`、`controller/payment_webhook_availability.go` 及前端 wallet/redemption-codes/subscriptions/pricing feature
- `model/topup.go`/`redemption.go`/`subscription.go` 保留为惰性桩（migrateDB 引用 + 表保留零迁移）

## 移除 Ollama 渠道

- 删 `relay/channel/ollama/` 目录、controller Ollama 4 handlers、`/ollama/*` 路由、前端 ollama-models-dialog/ollama-utils
- **枚举值保留**：`ChannelTypeOllama`/`APITypeOllama` 与 `ChannelBaseURLs` 索引 4 保留（DB 持久化渠道 type，删除会 shift 后续值损坏存量数据），仅清空索引 4 与展示名
- `controller/model.go` init 循环跳过 `APITypeOllama`（适配器已删，防 nil 解引用）

## 移除订阅后端残留

- `controller/audit.go` 删 redemption/subscription 审计模板
- 删 `service/waffo_pancake.go`、`controller/payment_webhook_availability.go`(+test)
- `setting/payment_*.go` 配置变量保留（option 表映射，零迁移）

## 移除 OAuth/Passkey 登录

- 删 `oauth/` 目录、`controller/oauth.go`/`custom_oauth.go`/`passkey.go`/`telegram.go`/`wechat.go`、`service/passkey/`
- 删 `/oauth/*`、`/passkey/*`、`/custom-oauth-provider` 路由
- `model` 桩保留（`CustomOAuthProvider`/`UserOAuthBinding`/`PasskeyCredential` 表零迁移）
- 前端删 sign-up/oauth/passkey feature，登录页仅保留密码登录 + 2FA

## 移除开放注册

- 删 `/api/user/register` 路由与 `controller.Register`
- 前端删 sign-up/register 路由与注册入口

## 移除系统设置 Billing 页残留

`a7e70937` 删除计费 UI 时保留了系统设置「Billing & Payment」页（仅剩 Quota Settings + Check-in Rewards 两个 section）。本提交将其彻底移除：

- 删 `web/src/features/system-settings/billing/`（index + section-registry）、`/system-settings/billing/*` 路由、侧边栏「Billing & Payment」导航组、`BillingSettings` 类型与孤儿组件 `quota-settings-section.tsx` / `checkin-settings-section.tsx`（签到奖励管理入口随计费一并移除；后端 checkin 模块未动）
- 死链修复：`legacy-route` 的 payment/ratio tab 回退到 `/system-settings`；渠道测试对话框与 playground 的 model_price_error「Go to Settings」按钮（指向已删除的 model-pricing 设置页）移除，错误文案保留（后端价格校验仍在）
- i18n：本次新增孤儿 key 七语言同步各删 30 个
- 验证：rsbuild build 通过、vitest 142 全绿、typecheck 错误数与基线一致（38 = 38，均为既有问题）

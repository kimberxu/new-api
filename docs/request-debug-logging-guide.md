# 请求调试日志 - 部署指南

> 对应分支：`personal` 基线 `7e6415e7`（2026-08-24 刷新至 `e2238bc0`）
> 部署机通常以 docker 运行，环境变量在容器环境配置；修改后需重启容器生效。

## 启用调试

```bash
export REQUEST_DEBUG_LOGGING=error_only
export REQUEST_DEBUG_MAX_BODY_BYTES=65536
```

docker-compose 配置示例：

```yaml
# docker-compose.yml（示意）
services:
  new-api:
    image: ghcr.io/<owner>/new-api:deploy
    environment:
      - REQUEST_DEBUG_LOGGING=error_only
      - REQUEST_DEBUG_MAX_BODY_BYTES=65536
      - LOG_CLEANUP_ENABLED=true
      - LOG_CLEANUP_RETENTION_DAYS=30
      - LOG_CLEANUP_INTERVAL_HOURS=24
```

模式说明：
- `off`: 禁用
- `error_only`: 只在请求出错时捕获。流式请求按**实际流结果**判断：上游中断（`scanner_error` / `timeout`）、客户端取消（`client_gone`）同样视为失败并保留快照
- `always`: 每次请求都捕获

## 日志清理

```bash
export LOG_CLEANUP_ENABLED=true
export LOG_CLEANUP_RETENTION_DAYS=30
export LOG_CLEANUP_INTERVAL_HOURS=24
```

机制：master 节点注册定时任务，按保留天数计算截止时间戳，分批（每批 100 条）删除 `logs` 表中超期记录，一次运行全部删完为止。

**建议与调试日志搭配使用**：开启 `always` 或长期开启 `error_only` 会导致日志表增长，务必同时启用清理。

## 查看调试快照

管理员在日志详情页面点击 "请求调试快照" 折叠面板，查看脱敏后的请求体、SHA-256 摘要、截断状态等。

前提：
- `REQUEST_DEBUG_LOGGING` 非 `off`，且该请求命中模式（`error_only` 只对失败请求产生快照，`always` 对全部请求产生）
- 快照写入日志记录的 `other.admin_info.request_debug`，仅管理员可见

## 生产建议

- 推荐组合：`error_only` + `LOG_CLEANUP_ENABLED=true` + 保留 7~30 天
- `always` 模式全量入库，注意存储与写入开销，仅在排查期间短时开启
- 快照内容为脱敏后的请求体；脱敏字段与截断规则见 `request-debug-customization-manifest.md`

## 注意事项

- 敏感字段自动脱敏为 `[REDACTED]`
- 提示词正文被截断为 `[TRUNCATED text N bytes]`
- 长字符串被截断为 `[TRUNCATED string N bytes]`
- Body 超过 `REQUEST_DEBUG_MAX_BODY_BYTES` 时截断

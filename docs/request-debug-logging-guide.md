# 请求调试日志 - 部署指南

## 启用调试

```bash
export REQUEST_DEBUG_LOGGING=error_only
export REQUEST_DEBUG_MAX_BODY_BYTES=65536
```

模式说明：
- `off`: 禁用
- `error_only`: 只在请求出错时捕获
- `always`: 每次请求都捕获

## 日志清理

```bash
export LOG_CLEANUP_ENABLED=true
export LOG_CLEANUP_RETENTION_DAYS=30
export LOG_CLEANUP_INTERVAL_HOURS=24
```

## 查看调试快照

管理员在日志详情页面点击 "请求调试快照" 折叠面板，查看脱敏后的请求体、SHA-256 摘要、截断状态等。

## 注意事项

- 敏感字段自动脱敏为 `[REDACTED]`
- 提示词正文被截断为 `[TRUNCATED text N bytes]`
- 长字符串被截断为 `[TRUNCATED string N bytes]`
- Body 超过 `REQUEST_DEBUG_MAX_BODY_BYTES` 时截断

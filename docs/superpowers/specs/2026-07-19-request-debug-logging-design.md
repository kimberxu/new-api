# 请求调试日志 - 设计文档

## 动机

在代理多个 AI 提供商时，请求/响应调试极其困难。上游返回错误时无法判断是下游发错了还是上游拒绝了。

## 方案

在 relay 关键路径上捕获下游原始请求和上游转换后的请求，记录到日志管理员的 `admin_info.request_debug` 字段。

## 架构

```
下游请求 → relay handler → 转换 → 上游请求
    ↓                          ↓
 CaptureDownstream        CaptureUpstream
    ↓                          ↓
         → RequestDebugSnapshot ←
                  ↓
       sanitize (脱敏 + 摘要)
                  ↓
         → 存入 admin_info
                  ↓
         → 管理员日志详情 UI
```

## 安全

- 所有 secret 字段 (`api_key`, `token`, `password` 等) 在展示前脱敏
- 提示词正文只显示摘要，不泄露完整内容
- Body 大小受限，超过上限截断

# common/trace — 请求链路 ID

轻量级 TraceID 生成与 Context 传递工具。

> **注意**：此包用于自定义 TraceID 场景。大多数情况下使用 `common/otel` 的 OpenTelemetry 自动追踪即可。

## 用法

```go
import "github.com/ownforge/ownforge/pkg/trace"

// 生成新的 TraceID
traceID := trace.NewTraceID()

// 存入 Context
ctx = trace.IntoContext(ctx, traceID)

// 从 Context 提取
id := trace.FromContext(ctx)
```

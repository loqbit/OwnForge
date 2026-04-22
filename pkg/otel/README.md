# common/otel — OpenTelemetry 链路追踪

一行初始化 OpenTelemetry 分布式追踪，导出到 Jaeger。

## 用法

```go
import "github.com/ownforge/ownforge/pkg/otel"

shutdown, err := otel.InitTracer("api-gateway", "localhost:4318")
if err != nil {
    log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
}
defer shutdown(context.Background()) // flush 未发送的 Span
```

## 参数说明

| 参数 | 说明 | 示例 |
|------|------|------|
| `serviceName` | 服务名，显示在 Jaeger UI | `"api-gateway"` |
| `jaegerEndpoint` | Jaeger OTLP 接收器地址（不带协议） | `"localhost:4318"` |

## 配合使用

```go
// Gin 中间件：自动给每个 HTTP 请求打 Span
r.Use(otelgin.Middleware("api-gateway"))

// gRPC 拦截器：自动给每个 RPC 调用打 Span
grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)

// 日志关联：logger.Ctx 自动提取 TraceID
logger.Ctx(ctx, log).Info("处理完成")
```

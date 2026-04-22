# common/metrics — Prometheus 指标采集

统一的 Prometheus 指标中间件，自动记录请求计数、延时和并发数。

> **注意**：业务服务通常不需要直接注册 `/metrics` 端点，`common/probe` 已经自动处理。

## 采集的指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `gin_http_http_requests_total` | Counter | 请求总数（按 method/path/status 分组） |
| `gin_http_http_request_duration_seconds` | Histogram | 请求延时分布（P50/P95/P99） |
| `gin_http_http_requests_in_flight` | Gauge | 当前并发请求数 |
| `grpc_requests_total` | Counter | gRPC 调用计数 |
| `grpc_request_duration_seconds` | Histogram | gRPC 调用延时 |
| `grpc_requests_in_flight` | Gauge | gRPC 并发数 |

## 用法

### Gin HTTP（已被 probe 包内置调用）

```go
// probe.Register 内部会自动调用这两行，无需手动注册
r.GET("/metrics", metrics.GinMetricsHandler())
r.Use(metrics.GinMetrics())
```

### gRPC 拦截器

```go
grpc.NewServer(
    grpc.UnaryInterceptor(metrics.GRPCMetricsInterceptor()),
)
```

## 特性

- 自动跳过 `/metrics` 端点自身，避免采集请求污染业务指标
- 支持 Exemplar：每条延时指标自动关联 OTel TraceID，在 Grafana 中点击即可跳转 Jaeger

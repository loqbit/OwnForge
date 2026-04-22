# common/metrics — Prometheus Metrics Collection

Unified Prometheus metrics middleware that automatically records request counts, latency, and concurrency.

> **Note**: Business services usually do not need to register `/metrics` directly; `common/probe` already handles it automatically.

## Collected Metrics

| Metric Name | Type | Description |
|--------|------|------|
| `gin_http_http_requests_total` | Counter | Total request count grouped by method/path/status |
| `gin_http_http_request_duration_seconds` | Histogram | Request latency distribution (P50/P95/P99) |
| `gin_http_http_requests_in_flight` | Gauge | Current in-flight request count |
| `grpc_requests_total` | Counter | gRPC Call count |
| `grpc_request_duration_seconds` | Histogram | gRPC Call latency |
| `grpc_requests_in_flight` | Gauge | gRPC In-flight count |

## Usage

### Gin HTTP (already built into the probe package)

```go
// probe.Register internally calls these two lines automatically, so no manual registration is needed
r.GET("/metrics", metrics.GinMetricsHandler())
r.Use(metrics.GinMetrics())
```

### gRPC Interceptors

```go
grpc.NewServer(
    grpc.UnaryInterceptor(metrics.GRPCMetricsInterceptor()),
)
```

## Features

- Automatically skips the `/metrics` endpoint itself so scrape requests do not pollute business metrics
- Supports exemplars: each latency metric is automatically linked to an OTel TraceID, so clicking it in Grafana can jump to Jaeger

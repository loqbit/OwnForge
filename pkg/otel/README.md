# common/otel — OpenTelemetry Tracing

Initialize OpenTelemetry distributed tracing in one line and export it to Jaeger.

## Usage

```go
import "github.com/ownforge/ownforge/pkg/otel"

shutdown, err := otel.InitTracer("api-gateway", "localhost:4318")
if err != nil {
    log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
}
defer shutdown(context.Background()) // flush unsent spans
```

## ParameterDescription

| Parameter | Description | Example |
|------|------|------|
| `serviceName` | Service name shown in the Jaeger UI | `"api-gateway"` |
| `jaegerEndpoint` | Jaeger OTLP collector address without protocol | `"localhost:4318"` |

## Works With

```go
// Gin middleware: automatically create spans for each HTTP request
r.Use(otelgin.Middleware("api-gateway"))

// gRPC interceptors: automatically create spans for each RPC call
grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)

// Log correlation: `logger.Ctx` automatically extracts the TraceID
logger.Ctx(ctx, log).Info("processing complete")
```

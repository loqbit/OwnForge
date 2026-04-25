# common/logger — Structured Logging

A unified logging library based on Uber Zap with automatic OpenTelemetry TraceID integration.

## Core API

### Create a Logger

```go
import "github.com/loqbit/ownforge/pkg/logger"

log := logger.NewLogger("my-service")
defer log.Sync()
```

- Development (`APP_ENV != production`): colored Console format
- Production: JSON format, easy for Loki to parse

### Logs with TraceID

```go
// Automatically extract OTel trace_id and span_id from context
logger.Ctx(ctx, log).Info("user created successfully", zap.String("user_id", uid))
// Output: {"level":"INFO", "message":"user created successfully", "trace_id":"abc123...", "user_id":"u-001"}
```

### Gin Middleware

```go
r.Use(logger.GinLogger(log))        // Record every HTTP request (method, path, latency, TraceID)
r.Use(logger.GinRecovery(log, true)) // Capture panics and print the stack trace
```

### gRPC Interceptors

```go
grpc.NewServer(
    grpc.UnaryInterceptor(logger.GRPCUnaryServerInterceptor(log, nil)),
)
```

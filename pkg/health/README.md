# common/health — Health Check Endpoints

Provides `/healthz` (liveness) and `/readyz` (readiness) following K8s probe conventions.

> **Note**: Business services usually do not need to use this package directly. Use `common/probe` to register all operational endpoints in one line.

## Design Philosophy

- `/healthz` — Returns 200 as long as the process is alive and does **not** check the database or Redis
- `/readyz` — Returns 200 only when all dependencies are ready; otherwise 503 plus details

## Direct Use (Low-Level API)

```go
import "github.com/ownforge/ownforge/pkg/health"

checker := health.NewChecker()
checker.AddCheck("postgres", func(ctx context.Context) error {
    return db.PingContext(ctx)
})
checker.AddCheck("redis", func(ctx context.Context) error {
    return rdb.Ping(ctx).Err()
})

// Gin engine
checker.Register(r)

// Standard-library `http.ServeMux` (gRPC sidecar port)
checker.RegisterHTTP(mux)

// Non-HTTP scenarios (gRPC Health / background checks)
allHealthy, results := checker.Evaluate(ctx)
```

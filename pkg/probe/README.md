# common/probe — Unified Operational Probes

Register with one line of code `/healthz`、`/readyz`、`/metrics`，remove boilerplate。

## Two Modes

### Mode 1: Mount on a Gin Engine (HTTP Services)

```go
import "github.com/loqbit/ownforge/pkg/probe"

r := gin.New()
probe.Register(r, log,
    probe.WithCheck("postgres", func(ctx context.Context) error {
        _, err := entClient.User.Query().Exist(ctx)
        return err
    }),
    probe.WithRedis(redisClient),
)
// Automatically registers: /healthz, /readyz, /metrics + metrics middleware
```

### Mode 2: Dedicated Admin Port (gRPC / Worker Services)

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

shutdown := probe.Serve(ctx, ":9094", log,
    probe.WithRedis(redisClient),
    probe.WithGRPCHealth(grpcHealthServer, "note.NoteService"),
)
defer shutdown()
// Automatically starts a sidecar HTTP server：/healthz, /readyz, /metrics
// Automatically syncs check results to native gRPC Health
```

## Available Options

| Option | Description |
|--------|------|
| `WithCheck(name, fn)` | Custom check function |
| `WithRedis(client)` | Redis ping check (nil-safe) |
| `WithPinger(name, p)` | Any object implementing `PingContext` |
| `WithGRPCHealth(srv, services...)` | Sync to the gRPC Health service |
| `WithoutMetrics()` | Disable /metrics |

## Graceful Shutdown

```go
// gRPC When the service shuts down, mark all services as NOT_SERVING
probe.GRPCShutdown(healthServer, "user.UserService", "user.AuthService")
```

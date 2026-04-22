# common/probe — 统一运维探针

一行代码注册 `/healthz`、`/readyz`、`/metrics`，消除模板代码。

## 两种模式

### 模式 1：挂载到 Gin 引擎（HTTP 服务）

```go
import "github.com/ownforge/ownforge/pkg/probe"

r := gin.New()
probe.Register(r, log,
    probe.WithCheck("postgres", func(ctx context.Context) error {
        _, err := entClient.User.Query().Exist(ctx)
        return err
    }),
    probe.WithRedis(redisClient),
)
// 自动注册: /healthz, /readyz, /metrics + metrics 中间件
```

### 模式 2：独立管理端口（gRPC / 消费者服务）

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

shutdown := probe.Serve(ctx, ":9094", log,
    probe.WithRedis(redisClient),
    probe.WithGRPCHealth(grpcHealthServer, "note.NoteService"),
)
defer shutdown()
// 自动启动旁路 HTTP：/healthz, /readyz, /metrics
// 自动同步检查结果到 gRPC 原生 Health
```

## 可用 Options

| Option | 说明 |
|--------|------|
| `WithCheck(name, fn)` | 自定义检查函数 |
| `WithRedis(client)` | Redis ping 检查（nil 安全） |
| `WithPinger(name, p)` | 任何实现 `PingContext` 的对象 |
| `WithGRPCHealth(srv, services...)` | 同步到 gRPC Health 服务 |
| `WithoutMetrics()` | 禁用 /metrics |

## 优雅停机

```go
// gRPC 服务停机时，标记所有服务为 NOT_SERVING
probe.GRPCShutdown(healthServer, "user.UserService", "user.AuthService")
```

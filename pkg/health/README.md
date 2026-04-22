# common/health — 健康检查端点

提供 `/healthz`（存活探针）和 `/readyz`（就绪探针），遵循 K8s 探针规范。

> **注意**：业务服务通常不需要直接使用此包，请使用 `common/probe` 包来一行注册所有运维端点。

## 设计理念

- `/healthz` — 进程存活即返回 200，**不检查**数据库/Redis
- `/readyz` — 所有依赖就绪才返回 200，否则 503 + 明细

## 直接使用（低层 API）

```go
import "github.com/ownforge/ownforge/pkg/health"

checker := health.NewChecker()
checker.AddCheck("postgres", func(ctx context.Context) error {
    return db.PingContext(ctx)
})
checker.AddCheck("redis", func(ctx context.Context) error {
    return rdb.Ping(ctx).Err()
})

// Gin 引擎
checker.Register(r)

// 标准库 http.ServeMux（gRPC 旁路端口）
checker.RegisterHTTP(mux)

// 非 HTTP 场景（gRPC Health / 后台巡检）
allHealthy, results := checker.Evaluate(ctx)
```

# common/logger — 结构化日志

基于 Uber Zap 的统一日志库，自动集成 OpenTelemetry TraceID。

## 核心 API

### 创建 Logger

```go
import "github.com/ownforge/ownforge/pkg/logger"

log := logger.NewLogger("my-service")
defer log.Sync()
```

- 开发环境（`APP_ENV != production`）：彩色 Console 格式
- 生产环境：JSON 格式，方便 Loki 解析

### 带 TraceID 的日志

```go
// 自动从 context 提取 OTel trace_id 和 span_id
logger.Ctx(ctx, log).Info("创建用户成功", zap.String("user_id", uid))
// 输出: {"level":"INFO", "message":"创建用户成功", "trace_id":"abc123...", "user_id":"u-001"}
```

### Gin 中间件

```go
r.Use(logger.GinLogger(log))        // 记录每个 HTTP 请求（方法、路径、耗时、TraceID）
r.Use(logger.GinRecovery(log, true)) // 捕获 panic，打印堆栈
```

### gRPC 拦截器

```go
grpc.NewServer(
    grpc.UnaryInterceptor(logger.GRPCUnaryServerInterceptor(log, nil)),
)
```

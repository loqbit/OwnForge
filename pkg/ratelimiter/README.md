# common/ratelimiter — 多策略限流器

提供 4 种限流策略，统一 `Limiter` 接口，底层基于 Redis。

## 接口

```go
type Limiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) error
}
// 返回 nil 表示放行，返回 ErrRateLimitExceeded 表示触发限流
```

## 4 种策略

| 构造函数 | 策略 | 适用场景 |
|----------|------|----------|
| `NewFixedWindowLimiter(rdb, log)` | 固定窗口 | 登录频率限制 |
| `NewSlidingWindowLimiter(rdb, log)` | 滑动窗口 | IP 限流、用户限流 |
| `NewTokenBucketLimiter(rdb, log)` | 令牌桶 | 路由级限流（允许突发） |
| `NewBBRLimiter(buckets, window, cpuThreshold)` | BBR 自适应 | 全局过载保护（纯内存） |

## 用法示例

```go
import "github.com/ownforge/ownforge/pkg/ratelimiter"

// 创建限流器
ipLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)

// 在中间件中使用
err := ipLimiter.Allow(ctx, "rate:ip:"+clientIP, 50, time.Second)
if err != nil {
    c.JSON(429, gin.H{"msg": "请求过于频繁"})
    c.Abort()
    return
}
```

## 网关四层限流架构

```
请求 → IP限流(滑动窗口) → BBR自适应 → 路由限流(令牌桶) → 用户限流(滑动窗口) → 业务
```

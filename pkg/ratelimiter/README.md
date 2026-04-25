# common/ratelimiter — Multi-Strategy Rate Limiter

Provides four rate-limiting strategies under a unified `Limiter` interface backed by Redis.

## Interface

```go
type Limiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) error
}
// Returning nil means allow; returning ErrRateLimitExceeded means the rate limit was triggered
```

## Four Strategies

| Constructor | Strategy | Use Case |
|----------|------|----------|
| `NewFixedWindowLimiter(rdb, log)` | Fixed window | Login rate limiting |
| `NewSlidingWindowLimiter(rdb, log)` | Sliding window | IP limiting, user limiting |
| `NewTokenBucketLimiter(rdb, log)` | Token bucket | Route-level rate limiting with burst allowance |
| `NewBBRLimiter(buckets, window, cpuThreshold)` | BBR Adaptive | Global overload protection (in-memory only) |

## UsageExample

```go
import "github.com/loqbit/ownforge/pkg/ratelimiter"

// Create a rate limiter
ipLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)

// Use in middleware
err := ipLimiter.Allow(ctx, "rate:ip:"+clientIP, 50, time.Second)
if err != nil {
    c.JSON(429, gin.H{"msg": "too many requests"})
    c.Abort()
    return
}
```

## Gateway Four-Layer Rate-Limiting Architecture

```
Request -> IP rate limiting (sliding window) -> BBR adaptive limiting -> route rate limiting (token bucket) -> user rate limiting (sliding window) -> business logic
```

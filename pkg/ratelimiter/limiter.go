package ratelimiter

import (
	"context"
	"errors"
	"time"
)

// ErrRateLimitExceeded 表示某个 key 已经达到限流阈值。
var ErrRateLimitExceeded = errors.New("尝试次数过多，请稍后再试")

// Limiter 定义所有限流策略都应实现的标准接口。
// 高可用性考量：我们定义一个接口而不是直接绑死 Redis。
// 如果某天 Redis 挂了，我们可以无缝切换为基于内存的本地限流器，或者是一个“放行所有请求”的降级实现（Fail-Open）。
type Limiter interface {
	// Allow 用于检查给定 key 当前是否允许继续执行动作。
	// 返回 err == ErrRateLimitExceeded 表示被限流。
	// 参数：
	// - key: 限流的唯一标识，例如按IP限流 "login:ip:192.168.1.1"，按用户名限流 "login:user:alice"
	// - limit: 允许的最大请求数
	// - window: 时间窗口大小，比如 1分钟内允许5次
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
}

package ratelimiter

import (
	"context"
	"errors"
	"time"
)

// ErrRateLimitExceeded means a key has reached the rate limit threshold.
var ErrRateLimitExceeded = errors.New("too many attempts, please try again later")

// Limiter defines the standard interface all rate-limiting strategies should implement.
// For availability reasons, we define an interface instead of coupling directly to Redis.
// If Redis goes down one day, we can switch to an in-memory limiter or a fail-open implementation that allows all requests.
type Limiter interface {
	// Allow checks whether the given key is currently allowed to proceed.
	// If err == ErrRateLimitExceeded, the request is rate-limited.
	// parameter：
	// - key: the unique key used for rate limiting, for example "login:ip:192.168.1.1" for IP-based limits or "login:user:alice" for username-based limits
	// - limit: maximum number of allowed requests
	// - window: window size, for example 5 attempts within 1 minute
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
}

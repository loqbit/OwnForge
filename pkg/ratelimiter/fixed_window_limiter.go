package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type fixedWindowLimiter struct {
	cli    *redis.Client
	logger *zap.Logger
}

// NewFixedWindowLimiter creates a Redis-based fixed-window rate limiter.
func NewFixedWindowLimiter(cli *redis.Client, logger *zap.Logger) Limiter {
	return &fixedWindowLimiter{
		cli:    cli,
		logger: logger,
	}
}

// Allow rate-limits requests using a fixed-window counter.
func (r *fixedWindowLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// Use a Redis pipeline to execute INCR and EXPIRE atomically.
	pipe := r.cli.TxPipeline()     //start the pipeline
	incrReq := pipe.Incr(ctx, key) //increment the key by 1
	pipe.Expire(ctx, key, window)  //set the key expiration

	_, err := pipe.Exec(ctx) //execute the pipeline
	if err != nil {
		// Fail-open core logic for high availability
		// If Redis is down or network jitter causes execution to fail
		// As an auxiliary control for login protection, it should never block legitimate users just because the risk-control component is down.
		// So log an error for alerting and allow the request directly (return nil).
		r.logger.Error("rate limiter (Redis) failed; request downgraded to fail-open", zap.String("key", key), zap.Error(err))
		return nil
	}

	// check whether the current count exceeds the allowed threshold
	count := incrReq.Val()
	if count > int64(limit) {
		r.logger.Warn("anti-abuse rate limit triggered", zap.String("key", key), zap.Int64("current_count", count))
		return ErrRateLimitExceeded
	}

	return nil
}

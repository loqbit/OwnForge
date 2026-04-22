package ratelimiter

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type slidingWindowLimiter struct {
	cli    *redis.Client
	logger *zap.Logger
}

func NewSlidingWindowLimiter(cli *redis.Client, logger *zap.Logger) Limiter {
	return &slidingWindowLimiter{
		cli:    cli,
		logger: logger,
	}
}

var slidingWindowScript = redis.NewScript(`
	local key = KEYS[1]
	local window = tonumber(ARGV[1])
	local limit = tonumber(ARGV[2])
	local now = tonumber(ARGV[3])
	local req_id = ARGV[4]

	local min_score = now - window
	redis.call('ZREMRANGEBYSCORE', key, '-inf', min_score)
	local current_requests = redis.call('ZCARD', key)

	if current_requests >= limit then
		return 0
	end

	redis.call('ZADD', key, now, req_id)
	redis.call('PEXPIRE', key, window)
	return 1
`)

// Allow encapsulates rate-limiting logic for gateway middleware.
func (r *slidingWindowLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// build the unique rate-limit key
	key = fmt.Sprintf("rate_limit:%s", key)

	// get the current timestamp in milliseconds
	now := time.Now().UnixNano() / int64(time.Millisecond)

	// generate a unique request ID so the ZSet member stays unique
	reqID := fmt.Sprintf("%d:%d", now, rand.Int63())

	// 2. execute the Lua script (go-redis automatically handles EVALSHA optimization)
	result, err := slidingWindowScript.Run(ctx, r.cli, []string{key}, window.Milliseconds(), limit, now, reqID).Int64()
	if err != nil {
		// Production guidance: if Redis is temporarily unavailable, decide whether to fail open or fail closed based on business criticality.
		r.logger.Error("rate limiter (Redis) failed; request downgraded to fail-open", zap.String("key", key), zap.Error(err))
		return nil
	}

	if result == 0 {
		r.logger.Warn("anti-abuse rate limit triggered", zap.String("key", key))
		return ErrRateLimitExceeded
	}

	return nil
}

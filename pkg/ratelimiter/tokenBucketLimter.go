package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type tokenBucketLimiter struct {
	cli    *redis.Client
	logger *zap.Logger
}

func NewTokenBucketLimiter(cli *redis.Client, logger *zap.Logger) Limiter {
	return &tokenBucketLimiter{
		cli:    cli,
		logger: logger,
	}
}

var tokenBucketScript = redis.NewScript(`
	local key = KEYS[1]
	local capacity = tonumber(ARGV[1])    -- bucket capacity
	local rate = tonumber(ARGV[2])        -- tokens refilled per millisecond
	local now = tonumber(ARGV[3])
	local ttl = tonumber(ARGV[4])         -- key expiration time (milliseconds)
	-- 1. read the current bucket state
	local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
	local tokens = tonumber(bucket[1])
	local last_refill = tonumber(bucket[2])
	-- 2. if the bucket does not exist (first request), initialize it as full
	if tokens == nil then
		tokens = capacity
		last_refill = now
	end
	-- 3. compute how many tokens should be refilled (lazy refill core logic)
	local elapsed = now - last_refill
	local new_tokens = math.min(capacity, tokens + elapsed * rate)
	-- 4. try to consume one token
	if new_tokens >= 1 then
		new_tokens = new_tokens - 1
		redis.call('HSET', key, 'tokens', new_tokens, 'last_refill', now)
		redis.call('PEXPIRE', key, ttl)
		return 1
	else
		-- when no tokens remain, still update the state to avoid recomputing elapsed time next time
		redis.call('HSET', key, 'tokens', new_tokens, 'last_refill', now)
		redis.call('PEXPIRE', key, ttl)
		return 0
	end
`)

func (r *tokenBucketLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// 1. compute capacity = limit
	capacity := float64(limit)
	// 2. compute rate = float64(limit) / float64(window.Milliseconds())
	rate := capacity / float64(window.Milliseconds())
	// 3. now = current millisecond timestamp
	now := time.Now().UnixNano() / int64(time.Millisecond)
	// 4. ttl = window.Milliseconds() (expire and clean up after one full idle window)
	ttl := window.Milliseconds()
	// 5. execute the Lua script
	result, err := tokenBucketScript.Run(ctx, r.cli, []string{key}, capacity, rate, now, ttl).Int64()
	// 6. check the return value; 0 means ErrRateLimitExceeded
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

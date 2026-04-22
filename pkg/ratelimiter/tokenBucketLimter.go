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
	local capacity = tonumber(ARGV[1])    -- 桶容量
	local rate = tonumber(ARGV[2])        -- 每毫秒补充多少令牌
	local now = tonumber(ARGV[3])
	local ttl = tonumber(ARGV[4])         -- key 的过期时间(毫秒)
	-- 1. 读取桶的当前状态
	local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
	local tokens = tonumber(bucket[1])
	local last_refill = tonumber(bucket[2])
	-- 2. 如果桶不存在（首次请求），初始化为满桶
	if tokens == nil then
		tokens = capacity
		last_refill = now
	end
	-- 3. 计算应该补充多少令牌（懒补充核心）
	local elapsed = now - last_refill
	local new_tokens = math.min(capacity, tokens + elapsed * rate)
	-- 4. 尝试消耗一个令牌
	if new_tokens >= 1 then
		new_tokens = new_tokens - 1
		redis.call('HSET', key, 'tokens', new_tokens, 'last_refill', now)
		redis.call('PEXPIRE', key, ttl)
		return 1
	else
		-- 没有令牌了，但也要更新状态（防止下次重复计算 elapsed）
		redis.call('HSET', key, 'tokens', new_tokens, 'last_refill', now)
		redis.call('PEXPIRE', key, ttl)
		return 0
	end
`)

func (r *tokenBucketLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// 1. 计算 capacity = limit
	capacity := float64(limit)
	// 2. 计算 rate = float64(limit) / float64(window.Milliseconds())
	rate := capacity / float64(window.Milliseconds())
	// 3. now = 当前毫秒时间戳
	now := time.Now().UnixNano() / int64(time.Millisecond)
	// 4. ttl = window.Milliseconds()（闲置超过整个窗口就过期清理）
	ttl := window.Milliseconds()
	// 5. 执行 Lua 脚本
	result, err := tokenBucketScript.Run(ctx, r.cli, []string{key}, capacity, rate, now, ttl).Int64()
	// 6. 检查返回值，0 返回 ErrRateLimitExceeded
	if err != nil {
		// 生产环境建议：如果 Redis 暂时挂了，根据业务重要程度决定是放行还是拦截
		r.logger.Error("限流器(Redis)执行异常, 请求已被降级放行", zap.String("key", key), zap.Error(err))
		return nil
	}
	if result == 0 {
		r.logger.Warn("触发安全防刷限流", zap.String("key", key))
		return ErrRateLimitExceeded
	}
	return nil
}

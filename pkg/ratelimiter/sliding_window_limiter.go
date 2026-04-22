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

// Allow 封装限流逻辑，供网关中间件调用
func (r *slidingWindowLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// 拼接唯一的限流 Key
	key = fmt.Sprintf("rate_limit:%s", key)

	// 获取当前时间的毫秒级时间戳
	now := time.Now().UnixNano() / int64(time.Millisecond)

	// 生成唯一的请求 ID，确保 ZSet 里的 Member 不重复
	reqID := fmt.Sprintf("%d:%d", now, rand.Int63())

	// 2. 执行 Lua 脚本 (go-redis 会自动处理 EVALSHA 优化)
	result, err := slidingWindowScript.Run(ctx, r.cli, []string{key}, window.Milliseconds(), limit, now, reqID).Int64()
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

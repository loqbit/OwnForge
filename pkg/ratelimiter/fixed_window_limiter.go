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

// NewFixedWindowLimiter 创建一个基于 Redis 的固定窗口限流器。
func NewFixedWindowLimiter(cli *redis.Client, logger *zap.Logger) Limiter {
	return &fixedWindowLimiter{
		cli:    cli,
		logger: logger,
	}
}

// Allow 使用固定窗口计数器对请求进行限流。
func (r *fixedWindowLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// 使用 Redis 管道 (Pipeline) 原子性地执行 INCR 和 EXPIRE
	pipe := r.cli.TxPipeline()     //开启管道
	incrReq := pipe.Incr(ctx, key) //给key增加1
	pipe.Expire(ctx, key, window)  //给key设置过期时间

	_, err := pipe.Exec(ctx) //执行管道
	if err != nil {
		// ⚠️【高可用降级核心逻辑 / Fail-Open】
		// 如果 Redis 服务宕机或者网络波动导致执行失败
		// 作为一个登录验证的辅助手段，我们【绝不】应该因为风控组件挂了导致正常用户无法登录。
		// 所以，记录一条 Error 日志报警，然后直接放行 (return nil)！
		r.logger.Error("限流器(Redis)执行异常, 请求已被降级放行", zap.String("key", key), zap.Error(err))
		return nil
	}

	// 检查当前次数是否超过了最大允许阈值
	count := incrReq.Val()
	if count > int64(limit) {
		r.logger.Warn("触发安全防刷限流", zap.String("key", key), zap.Int64("current_count", count))
		return ErrRateLimitExceeded
	}

	return nil
}

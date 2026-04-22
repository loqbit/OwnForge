package ratelimit

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ownforge/ownforge/pkg/ratelimiter"
)

func BBRMiddleware(limiter *ratelimiter.BBRLimiter, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 检查是否过载（CPU 触发 + Little's Law）
		if limiter.ShouldReject() {
			log.Warn("BBR 自适应限流触发",
				zap.Int64("cpu", limiter.CPUUsage()),
				zap.Int64("inflight", limiter.Inflight()),
				zap.Float64("maxFlight", limiter.MaxFlight()),
			)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "服务过载，请稍后重试",
			})
			return
		}

		// 2. 放行，开始计时
		limiter.IncrInflight()
		start := time.Now()

		c.Next()

		// 3. 请求完成，记录指标（无论 CPU 高低都要记录，保持采样连续性）
		rt := time.Since(start)
		limiter.DecrInflight()
		limiter.RecordRT(rt)
	}
}

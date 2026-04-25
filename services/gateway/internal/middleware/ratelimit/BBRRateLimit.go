package ratelimit

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/loqbit/ownforge/pkg/ratelimiter"
)

func BBRMiddleware(limiter *ratelimiter.BBRLimiter, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. check whether the system is overloaded (CPU trigger + Little's Law)
		if limiter.ShouldReject() {
			log.Warn("BBR adaptive rate limit triggered",
				zap.Int64("cpu", limiter.CPUUsage()),
				zap.Int64("inflight", limiter.Inflight()),
				zap.Float64("maxFlight", limiter.MaxFlight()),
			)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "service overloaded, please try again later",
			})
			return
		}

		// 2. allow the request and start timing
		limiter.IncrInflight()
		start := time.Now()

		c.Next()

		// 3. record metrics when the request completes, regardless of CPU level, to keep sampling continuous
		rt := time.Since(start)
		limiter.DecrInflight()
		limiter.RecordRT(rt)
	}
}

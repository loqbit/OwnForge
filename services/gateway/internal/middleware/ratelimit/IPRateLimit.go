package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"go.uber.org/zap"
)

func IPrateLimit(limiter ratelimiter.Limiter, limit int, window time.Duration, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("ratelimit:ip:%s", ip)
		err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			log.Warn("gateway IP rate limit triggered", zap.String("ip", ip), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}
}

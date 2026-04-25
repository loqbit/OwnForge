package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"go.uber.org/zap"
)

func RouteRateLimit(limiter ratelimiter.Limiter, limit int, window time.Duration, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("ratelimit:route:%s", c.FullPath())
		err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			log.Warn("gateway route rate limit triggered", zap.String("key", key), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}
}

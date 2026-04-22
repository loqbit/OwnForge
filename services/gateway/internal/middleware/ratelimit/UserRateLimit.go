package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ownforge/ownforge/pkg/ratelimiter"
	"go.uber.org/zap"
)

func UserRateLimit(limiter ratelimiter.Limiter, limit int, window time.Duration, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, exists := c.Get("userID")
		// Note: this layer only has userID after JWTAuth has run
		// If there is no userID (not signed in), allow the request directly because the IP layer already provides a fallback.
		if !exists {
			c.Next()
			return
		}
		userID := userId.(int64)
		key := fmt.Sprintf("ratelimit:user:%d", userID)
		err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			log.Warn("gateway user rate limit triggered", zap.Int64("userID", userID), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}
}

package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luckysxx/common/ratelimiter"
	"go.uber.org/zap"
)

func UserRateLimit(limiter ratelimiter.Limiter, limit int, window time.Duration, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, exists := c.Get("userID")
		// 注意：这一层在 JWTAuth 后面才有 userID
		// 如果没有 userID（未登录），直接放行（已经有 IP 层兜底）
		if !exists {
			c.Next()
			return
		}
		userID := userId.(int64)
		key := fmt.Sprintf("ratelimit:user:%d", userID)
		err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			log.Warn("触发网关用户限流", zap.Int64("userID", userID), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}
}

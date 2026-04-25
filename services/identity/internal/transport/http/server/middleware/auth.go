package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GatewayAuth is a simplified auth middleware for a gateway-trust setup.
// The service no longer spends CPU validating JWTs locally and instead trusts X-User-Id from the gateway.
func GatewayAuth(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-Id")
		if userIDStr == "" {
			// Reaching this branch means someone bypassed the gateway and hit the internal service port directly.
			logger.Warn("invalid internal direct request: missing gateway identity marker", zap.String("client_ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "invalid request blocked by internal service network isolation",
			})
			return
		}

		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		// Attach the gateway-provided userID to the context for existing handlers.
		c.Set("userID", userID)

		c.Next()
	}
}

// GetUserID keeps the original access pattern by reading userID from gin.Context.
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	userID, ok := val.(int64)
	return userID, ok
}

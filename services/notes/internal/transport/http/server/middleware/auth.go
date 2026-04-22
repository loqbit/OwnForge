package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GatewayAuth trusts gateway-authenticated requests.
// The gateway has already validated the JWT and injected user_id into the X-User-Id header.
// This service only needs to read the header and does not validate the token again over gRPC.
func GatewayAuth(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-Id")
		if userIDStr == "" {
			logger.Warn("invalid internal direct request: missing gateway identity marker", zap.String("client_ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "invalid request blocked by internal service network isolation",
			})
			return
		}

		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		// Attach the gateway-provided userID to the context for downstream handlers.
		c.Set("userID", userID)

		c.Next()
	}
}

// GetUserID safely reads userID from the context.
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}

	userID, ok := val.(int64)
	return userID, ok
}

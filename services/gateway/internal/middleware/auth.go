package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/loqbit/ownforge/services/gateway/internal/auth"
)

var publicRouteWhitelist = map[string]struct{}{
	"/api/v1/config/client":              {},
	"/api/v1/users/register":             {},
	"/api/v1/users/login":                {},
	"/api/v1/users/refresh":              {},
	"/api/v1/users/sso/exchange":         {},
	"/api/v1/users/phone/code":           {},
	"/api/v1/users/phone/entry":          {},
	"/api/v1/users/phone/password-login": {},
}

// JWTAuth is the auth middleware factory.
// It injects JWTManager and Logger.
func JWTAuth(jwtManager *auth.JWTManager, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 0. Allowlist bypass: skip JWT validation for login and registration.
		if _, ok := publicRouteWhitelist[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		// 1. Read the Authorization header
		authHeader := c.GetHeader("Authorization")

		// 2. Call the underlying auth package for verification
		userID, err := auth.AuthenticateBearerToken(jwtManager, authHeader)
		if err != nil {
			// Auth failed, possibly because the token is missing, malformed, or expired.
			// Log at Debug or Warn instead of Error to avoid log flooding from malicious scans.
			logger.Debug("request blocked by auth", zap.Error(err), zap.String("client_ip", c.ClientIP()))

			// Block the request and return 401 Unauthorized
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "invalid or expired access token, please sign in again",
			})
			return
		}

		// 3. After verification, attach userID to Gin context for gateway-level use.
		c.Set("userID", userID)

		// Write the UserID back into the outgoing HTTP request headers sent to the backend.
		c.Request.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))

		// 4. If verification passes, allow the request into downstream handlers.
		c.Next()
	}
}

// GetUserID lets downstream handlers safely and cleanly read userID from context.
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}

	// assert it as int64
	userID, ok := val.(int64)
	return userID, ok
}

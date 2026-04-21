package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GatewayAuth 信任网关鉴权中间件
// 网关已完成 JWT 校验，并将 user_id 注入 X-User-Id Header
// 微服务只需读取 Header，不再重复调用 gRPC 验证 Token
func GatewayAuth(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-Id")
		if userIDStr == "" {
			logger.Warn("非法内部直连请求，缺失网关身份标识", zap.String("client_ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "非法请求，已被微服务内网隔离",
			})
			return
		}

		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		// 将网关传来的 userID 挂载到 Context，业务 Handler 通过 GetUserID 读取
		c.Set("userID", userID)

		c.Next()
	}
}

// GetUserID 从 Context 中安全获取 userID
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}

	userID, ok := val.(int64)
	return userID, ok
}

package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GatewayAuth 简化版的鉴权拦截器。拥抱网关时代！
// 微服务自身不再做耗费 CPU 的 JWT 校验，而是无脑信任网关传过来的 X-User-Id 即可！
func GatewayAuth(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-Id")
		if userIDStr == "" {
			// 如果走到这，说明有人跨过网关大门，直接攻击微服务的内网 8081 端口
			logger.Warn("非法内部直连请求，缺失网关身份标示", zap.String("client_ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "非法请求，已被微服务内网隔离",
			})
			return
		}

		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		// 将网关传来的 userID 挂载到 Context 继续让原本的业务 Handler 优雅读取
		c.Set("userID", userID)

		c.Next()
	}
}

// GetUserID 原来的代码一字不动！因为还是从 gin.Context 中拿出来的
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	userID, ok := val.(int64)
	return userID, ok
}

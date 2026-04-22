package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ownforge/ownforge/services/gateway/internal/auth"
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

// JWTAuth 鉴权中间件工厂函数
// 依赖注入 JWTManager 和 Logger
func JWTAuth(jwtManager *auth.JWTManager, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 0. 白名单放行机制：如果是登录注册，直接免去 JWT 校验
		if _, ok := publicRouteWhitelist[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		// 1. 获取 Authorization Header
		authHeader := c.GetHeader("Authorization")

		// 2. 呼叫底层的 auth 工具包进行验证
		userID, err := auth.AuthenticateBearerToken(jwtManager, authHeader)
		if err != nil {
			// 鉴权失败（可能是没带 Token、格式错误或者已过期）
			// 记一个 Debug 或 Warn 级别的日志，不要用 Error（防止被黑客恶意扫描刷爆日志）
			logger.Debug("请求鉴权拦截", zap.Error(err), zap.String("client_ip", c.ClientIP()))

			// 拦截并返回 401 Unauthorized
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "无效的访问凭证或已过期，请重新登录",
			})
			return
		}

		// 3. 查验真伪后，把 userID 挂载到 Gin 的上下文中（网关层面可用）
		c.Set("userID", userID)

		// 将 UserID 塞回真正向后端发起的 HTTP 报文请求头中！
		c.Request.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))

		// 4. 查验无误，放行进入下游 Handler
		c.Next()
	}
}

// GetUserID 专门用于让下游的 Handler 安全、优雅地从 Context 中把 userID 取出来
func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}

	// 断言为 int64
	userID, ok := val.(int64)
	return userID, ok
}

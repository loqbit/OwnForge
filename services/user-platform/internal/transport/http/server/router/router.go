package router

import (
	"github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/user-platform/internal/auth"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/handler"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// Dependencies groups the dependencies required to assemble the HTTP router.
type Dependencies struct {
	Engine      *gin.Engine
	UserHandler *handler.UserHandler
	JWTManager  *auth.JWTManager
	Logger      *zap.Logger
}

// SetupRouter wires up the HTTP routes.
func SetupRouter(deps Dependencies) {
	r := deps.Engine
	userHandler := deps.UserHandler
	log := deps.Logger

	r.Use(otelgin.Middleware("user-platform"))
	r.Use(logger.GinLogger(log))
	r.Use(logger.GinRecovery(log, true))

	// Health checks used by the Docker container.
	healthHandler := func(c *gin.Context) {
		c.String(200, "ok")
	}
	r.GET("/health", healthHandler)
	r.HEAD("/health", healthHandler)

	v1 := r.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			users.POST("/register", userHandler.Register)
			users.POST("/login", userHandler.Login)
			users.POST("/refresh", userHandler.RefreshToken)
			users.POST("/phone/code", userHandler.SendPhoneCode)
			users.POST("/phone/entry", userHandler.PhoneAuthEntry)
			users.POST("/phone/password-login", userHandler.PhonePasswordLogin)

			// Routes that require authentication, simplified to trust gateway-provided identity.
			authUsers := users.Group("")
			authUsers.Use(middleware.GatewayAuth(log))
			{
				authUsers.POST("/logout", userHandler.Logout)
				authUsers.POST("/logout-all", userHandler.LogoutAllSessions)
				authUsers.POST("/email/bind", userHandler.BindEmail)
				authUsers.POST("/password/change", userHandler.ChangePassword)
				authUsers.POST("/password/set", userHandler.SetPassword)
			}
		}
	}
}

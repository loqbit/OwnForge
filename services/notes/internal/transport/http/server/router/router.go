package router

import (
	"github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/server/handler"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// SetupRouter 配置路由。
func SetupRouter(
	r *gin.Engine,
	uploadHandler *handler.UploadHandler,
	shareHandler *handler.ShareHandler,
	groupHandler *handler.GroupHandler,
	log *zap.Logger,
) {
	r.Use(otelgin.Middleware("go-note"))
	r.Use(logger.GinLogger(log))
	r.Use(logger.GinRecovery(log, true))

	v1 := r.Group("/api/v1/notes")
	{
		authed := v1.Group("")
		authed.Use(middleware.GatewayAuth(log))
		{
			authed.POST("/uploads", uploadHandler.Upload)
			authed.POST("/uploads/presign", uploadHandler.Presign)
			authed.POST("/uploads/complete", uploadHandler.Complete)
			authed.GET("/groups/tree", groupHandler.GetTree)
		}

		public := v1.Group("/public")
		{
			public.GET("/shares/:token", shareHandler.GetPublic)
		}
	}
}

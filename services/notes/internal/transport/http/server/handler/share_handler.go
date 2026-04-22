package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	sharesvc "github.com/ownforge/ownforge/services/notes/internal/service/share"
	httperrs "github.com/ownforge/ownforge/services/notes/internal/transport/http/codec/errs"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/codec/response"
	"go.uber.org/zap"
)

// ShareHandler 处理分享相关 HTTP 请求。
type ShareHandler struct {
	svc    sharesvc.Service
	logger *zap.Logger
}

// NewShareHandler 创建 ShareHandler 实例。
func NewShareHandler(svc sharesvc.Service, logger *zap.Logger) *ShareHandler {
	return &ShareHandler{svc: svc, logger: logger}
}

func (h *ShareHandler) GetPublic(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")
	if password == "" {
		password = c.GetHeader("X-Share-Password")
	}

	result, err := h.svc.GetPublicByToken(c.Request.Context(), token, password)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("读取公开分享失败", zap.String("token", token), zap.Error(err))
		switch {
		case errors.Is(err, sharesvc.ErrPasswordRequired), errors.Is(err, sharesvc.ErrInvalidPassword):
			response.Unauthorized(c, err.Error())
		case errors.Is(err, sharesvc.ErrShareExpired):
			c.JSON(410, gin.H{"code": 410, "msg": "分享已过期", "data": nil})
		case sharedrepo.IsNotFoundError(err):
			response.NotFound(c, "分享不存在")
		default:
			response.Error(c, httperrs.ConvertToCustomError(err))
		}
		return
	}

	response.Success(c, result)
}

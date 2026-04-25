package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	sharesvc "github.com/loqbit/ownforge/services/notes/internal/service/share"
	httperrs "github.com/loqbit/ownforge/services/notes/internal/transport/http/codec/errs"
	"github.com/loqbit/ownforge/services/notes/internal/transport/http/codec/response"
	"go.uber.org/zap"
)

// ShareHandler handles share-related HTTP requests.
type ShareHandler struct {
	svc    sharesvc.Service
	logger *zap.Logger
}

// NewShareHandler creates a ShareHandler instance.
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("failed to read public share", zap.String("token", token), zap.Error(err))
		switch {
		case errors.Is(err, sharesvc.ErrPasswordRequired), errors.Is(err, sharesvc.ErrInvalidPassword):
			response.Unauthorized(c, err.Error())
		case errors.Is(err, sharesvc.ErrShareExpired):
			c.JSON(410, gin.H{"code": 410, "msg": "share has expired", "data": nil})
		case sharedrepo.IsNotFoundError(err):
			response.NotFound(c, "share not found")
		default:
			response.Error(c, httperrs.ConvertToCustomError(err))
		}
		return
	}

	response.Success(c, result)
}

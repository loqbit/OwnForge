package handler

import (
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	groupsvc "github.com/loqbit/ownforge/services/notes/internal/service/group"
	httperrs "github.com/loqbit/ownforge/services/notes/internal/transport/http/codec/errs"
	"github.com/loqbit/ownforge/services/notes/internal/transport/http/codec/response"
	"github.com/loqbit/ownforge/services/notes/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GroupHandler handles group-related HTTP requests.
type GroupHandler struct {
	svc    groupsvc.GroupService
	logger *zap.Logger
}

// NewGroupHandler creates a GroupHandler instance.
func NewGroupHandler(svc groupsvc.GroupService, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{svc: svc, logger: logger}
}

// GetTree returns the current user's full group tree.
func (h *GroupHandler) GetTree(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "not logged in")
		return
	}

	tree, err := h.svc.GetTree(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to fetch the group tree", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, tree)
}

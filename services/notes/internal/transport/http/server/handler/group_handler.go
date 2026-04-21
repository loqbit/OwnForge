package handler

import (
	commonlogger "github.com/luckysxx/common/logger"
	groupsvc "github.com/ownforge/ownforge/services/notes/internal/service/group"
	httperrs "github.com/ownforge/ownforge/services/notes/internal/transport/http/codec/errs"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/codec/response"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GroupHandler 处理分组相关的 HTTP 请求。
type GroupHandler struct {
	svc    groupsvc.GroupService
	logger *zap.Logger
}

// NewGroupHandler 创建 GroupHandler 实例。
func NewGroupHandler(svc groupsvc.GroupService, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{svc: svc, logger: logger}
}

// GetTree 获取当前用户的完整分组目录树。
func (h *GroupHandler) GetTree(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}

	tree, err := h.svc.GetTree(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("获取 group 目录树失败", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, tree)
}

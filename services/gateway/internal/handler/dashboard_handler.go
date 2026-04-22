package handler

import (
	"context"
	"time"

	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/dto"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
	notepb "github.com/luckysxx/common/proto/note"
	userpb "github.com/luckysxx/common/proto/user"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type DashboardHandler struct {
	userClient userpb.UserServiceClient
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

func NewDashboardHandler(
	userClient userpb.UserServiceClient,
	noteClient notepb.NoteServiceClient,
	log *zap.Logger,
) *DashboardHandler {
	return &DashboardHandler{
		userClient: userClient,
		noteClient: noteClient,
		log:        log,
	}
}

// GetDashboard 异构聚合演示端点
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	// 定义网关专用的超级 DTO 面板结构
	var dashResponse struct {
		Profile  *dto.GetProfileResponse    `json:"profile"`
		Snippets []*notepb.SnippetResponse  `json:"recent_snippets"`
	}

	// 第一部分：创建一个带 2 秒超时的 并发上下文，并注入网关身份信息
	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	egCtx, cancel := context.WithTimeout(grpcCtx, 2*time.Second)
	defer cancel()
	eg, egCtx := errgroup.WithContext(egCtx) // 原生标准库：errgroup 并发组

	// 第二部分：并发任务 A (走 gRPC 获取高优主数据)
	eg.Go(func() error {
		resp, err := h.userClient.GetProfile(egCtx, &userpb.GetProfileRequest{
			UserId: userID,
		})
		if err != nil {
			// Profile 是该页面的核心，若它挂了直接返回 error 阻断
			return err
		}

		dashResponse.Profile = &dto.GetProfileResponse{
			UserID:    resp.UserId,
			Nickname:  resp.Nickname,
			AvatarURL: resp.AvatarUrl,
			Bio:       resp.Bio,
			UpdatedAt: resp.UpdatedAt,
		}
		return nil
	})

	// 第三部分：并发任务 B (走 gRPC 获取笔记数据)
	eg.Go(func() error {
		resp, err := h.noteClient.ListRecentSnippets(egCtx, &notepb.ListSnippetsRequest{})
		if err != nil {
			// 核心：部分降级
			// 发生错误千万不要 return err，否则整个请求 500 崩溃
			// 只记录 Warn 日志，返回一个空数组给前台兜底
			commonlogger.Ctx(egCtx, h.log).Warn("Dashboard-获取边缘笔记链路故障，已执行降级策略", zap.Error(err))
			dashResponse.Snippets = []*notepb.SnippetResponse{}
			return nil
		}
		dashResponse.Snippets = resp.Snippets
		if dashResponse.Snippets == nil {
			dashResponse.Snippets = []*notepb.SnippetResponse{}
		}
		return nil
	})

	// 第四部分：阻塞等待所有协程全部落位
	if err := eg.Wait(); err != nil {
		// 如果接收到 err，说明某条要求保证一致性的链路（如主数据）断裂了
		commonlogger.Ctx(egCtx, h.log).Error("并发组装 Dashboard 遭遇核心系统熔断", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	// 整合完毕，统一吐出
	response.Success(c, dashResponse)
}

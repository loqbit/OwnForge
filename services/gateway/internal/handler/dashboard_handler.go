package handler

import (
	"context"
	"time"

	"github.com/loqbit/ownforge/services/gateway/internal/grpcclient"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/dto"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/response"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	notepb "github.com/loqbit/ownforge/pkg/proto/note"
	userpb "github.com/loqbit/ownforge/pkg/proto/user"
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

// GetDashboard is a heterogeneous aggregation demo endpoint.
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "unauthorized")
		return
	}
	userID := val.(int64)

	// Define the gateway-specific aggregate DTO shape.
	var dashResponse struct {
		Profile  *dto.GetProfileResponse   `json:"profile"`
		Snippets []*notepb.SnippetResponse `json:"recent_snippets"`
	}

	// Part 1: create a concurrent context with a 2-second timeout and inject gateway identity.
	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	egCtx, cancel := context.WithTimeout(grpcCtx, 2*time.Second)
	defer cancel()
	eg, egCtx := errgroup.WithContext(egCtx) // Standard-library style concurrent errgroup

	// Part 2: concurrent task A (fetch primary high-priority data over gRPC)
	eg.Go(func() error {
		resp, err := h.userClient.GetProfile(egCtx, &userpb.GetProfileRequest{
			UserId: userID,
		})
		if err != nil {
			// Profile is core to this page; if it fails, return the error directly.
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

	// Part 3: concurrent task B (fetch note data over gRPC)
	eg.Go(func() error {
		resp, err := h.noteClient.ListRecentSnippets(egCtx, &notepb.ListSnippetsRequest{})
		if err != nil {
			// Key point: partial degradation
			// Do not return err here, or the whole request will crash with a 500.
			// Only log a warning and return an empty array as a fallback for the frontend.
			commonlogger.Ctx(egCtx, h.log).Warn("Dashboard: edge-note dependency failed, degradation strategy applied", zap.Error(err))
			dashResponse.Snippets = []*notepb.SnippetResponse{}
			return nil
		}
		dashResponse.Snippets = resp.Snippets
		if dashResponse.Snippets == nil {
			dashResponse.Snippets = []*notepb.SnippetResponse{}
		}
		return nil
	})

	// Part 4: wait for all goroutines to complete
	if err := eg.Wait(); err != nil {
		// If err is returned, a consistency-critical dependency path such as primary data has failed.
		commonlogger.Ctx(egCtx, h.log).Error("Concurrent dashboard assembly hit a core-system circuit break", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	// Once everything is assembled, return the unified response
	response.Success(c, dashResponse)
}

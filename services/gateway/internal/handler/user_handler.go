package handler

import (
	"github.com/loqbit/ownforge/services/gateway/internal/config"
	"github.com/loqbit/ownforge/services/gateway/internal/grpcclient"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/dto"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/response"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	userpb "github.com/loqbit/ownforge/pkg/proto/user"

	"go.uber.org/zap"
)

// UserHandler handles user-related requests on the gateway side.
type UserHandler struct {
	userClient userpb.UserServiceClient
	ssoCookie  *ssoCookieManager
	log        *zap.Logger
}

// NewUserHandler creates a user handler.
func NewUserHandler(userClient userpb.UserServiceClient, cookieCfg config.SSOCookieConfig, log *zap.Logger) *UserHandler {
	return &UserHandler{
		userClient: userClient,
		ssoCookie:  newSSOCookieManager(cookieCfg),
		log:        log,
	}
}

// Register handles user registration.
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use the validator helper to translate validation errors into friendly messages.
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.userClient.Register(c.Request.Context(), &userpb.RegisterRequest{
		Phone:    req.Phone,
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("user registration failed", zap.Error(err))
		// This can be returned directly because the underlying service already returns domain errors.
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}
	response.Success(c, &dto.RegisterResponse{
		UserID:   user.UserId,
		Username: user.Username,
	})
}

// GetProfile fetches the current signed-in user's profile.
func (h *UserHandler) GetProfile(c *gin.Context) {
	// read identity from the gateway JWT middleware
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.GetProfile(grpcCtx, &userpb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to fetch profile", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.GetProfileResponse{
		UserID:    resp.UserId,
		Nickname:  resp.Nickname,
		AvatarURL: resp.AvatarUrl,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	})
}

// UpdateProfile updates the profile.
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.UpdateProfile(grpcCtx, &userpb.UpdateProfileRequest{
		UserId:    userID,
		Nickname:  req.Nickname,
		AvatarUrl: req.AvatarURL,
		Bio:       req.Bio,
		Birthday:  req.Birthday,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to update profile", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.UpdateProfileResponse{
		UserID:    resp.UserId,
		Nickname:  resp.Nickname,
		AvatarURL: resp.AvatarUrl,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	})
}

// ChangePassword changes the current signed-in user's password.
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.ChangePassword(grpcCtx, &userpb.ChangePasswordRequest{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to change password", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	h.ssoCookie.clear(c)
	response.Success(c, &dto.ChangePasswordResponse{
		UserID:  resp.UserId,
		Message: resp.Message,
	})
}

// LogoutAllSessions invalidates all sessions for the current user.
func (h *UserHandler) LogoutAllSessions(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.LogoutAllSessions(grpcCtx, &userpb.LogoutAllSessionsRequest{})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to log out from all devices", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	h.ssoCookie.clear(c)
	response.Success(c, &dto.LogoutAllSessionsResponse{
		UserID:  resp.UserId,
		Message: resp.Message,
	})
}

// BindEmail binds an email identity for the current signed-in user.
func (h *UserHandler) BindEmail(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.BindEmail(grpcCtx, &userpb.BindEmailRequest{
		Email: req.Email,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to bind email", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.BindEmailResponse{
		UserID:  resp.UserId,
		Email:   resp.Email,
		Message: resp.Message,
	})
}

// SetPassword sets a local password for the current signed-in user.
func (h *UserHandler) SetPassword(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.SetPassword(grpcCtx, &userpb.SetPasswordRequest{
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("failed to set password", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	h.ssoCookie.clear(c)
	response.Success(c, &dto.SetPasswordResponse{
		UserID:  resp.UserId,
		Message: resp.Message,
	})
}

func getAuthenticatedUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, false
	}

	userID, ok := val.(int64)
	return userID, ok
}

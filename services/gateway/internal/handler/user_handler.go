package handler

import (
	"github.com/ownforge/ownforge/services/gateway/internal/config"
	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/dto"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	userpb "github.com/ownforge/ownforge/pkg/proto/user"

	"go.uber.org/zap"
)

// UserHandler 负责处理网关侧的用户相关请求。
type UserHandler struct {
	userClient userpb.UserServiceClient
	ssoCookie  *ssoCookieManager
	log        *zap.Logger
}

// NewUserHandler 创建一个用户 Handler。
func NewUserHandler(userClient userpb.UserServiceClient, cookieCfg config.SSOCookieConfig, log *zap.Logger) *UserHandler {
	return &UserHandler{
		userClient: userClient,
		ssoCookie:  newSSOCookieManager(cookieCfg),
		log:        log,
	}
}

// Register 用户注册
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用 validator 翻译验证错误为友好提示
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.log).Error("用户注册失败", zap.Error(err))
		// 这里可以直接抛出，因为底层 Service 已经是 Domain Error 了
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}
	response.Success(c, &dto.RegisterResponse{
		UserID:   user.UserId,
		Username: user.Username,
	})
}

// GetProfile 获取当前登录用户的个人资料
func (h *UserHandler) GetProfile(c *gin.Context) {
	// 从网关 JWT 中间件获取身份
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.GetProfile(grpcCtx, &userpb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("获取个人资料失败", zap.Error(err))
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

// UpdateProfile 修改个人资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(grpcCtx, h.log).Error("更新个人资料失败", zap.Error(err))
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

// ChangePassword 修改当前登录用户密码。
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.ChangePassword(grpcCtx, &userpb.ChangePasswordRequest{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("修改密码失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	h.ssoCookie.clear(c)
	response.Success(c, &dto.ChangePasswordResponse{
		UserID:  resp.UserId,
		Message: resp.Message,
	})
}

// LogoutAllSessions 让当前用户的全部登录态失效。
func (h *UserHandler) LogoutAllSessions(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.LogoutAllSessions(grpcCtx, &userpb.LogoutAllSessionsRequest{})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("退出全部设备失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	h.ssoCookie.clear(c)
	response.Success(c, &dto.LogoutAllSessionsResponse{
		UserID:  resp.UserId,
		Message: resp.Message,
	})
}

// BindEmail 绑定当前登录用户的邮箱身份。
func (h *UserHandler) BindEmail(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req dto.BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.BindEmail(grpcCtx, &userpb.BindEmailRequest{
		Email: req.Email,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("绑定邮箱失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.BindEmailResponse{
		UserID:  resp.UserId,
		Email:   resp.Email,
		Message: resp.Message,
	})
}

// SetPassword 为当前登录用户设置本地密码。
func (h *UserHandler) SetPassword(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权")
		return
	}

	var req dto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.SetPassword(grpcCtx, &userpb.SetPasswordRequest{
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("设置密码失败", zap.Int64("userID", userID), zap.Error(err))
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

package handler

import (
	"github.com/ownforge/ownforge/services/gateway/internal/config"
	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/dto"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
	authpb "github.com/luckysxx/common/proto/auth"

	"go.uber.org/zap"
)

// AuthHandler 处理认证模块的 BFF 路由，通过 gRPC 调用 user-platform 的 AuthService。
type AuthHandler struct {
	authClient authpb.AuthServiceClient
	ssoCookie  *ssoCookieManager
	log        *zap.Logger
}

func NewAuthHandler(authClient authpb.AuthServiceClient, cookieCfg config.SSOCookieConfig, log *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
		ssoCookie:  newSSOCookieManager(cookieCfg),
		log:        log,
	}
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.authClient.Login(c.Request.Context(), &authpb.LoginRequest{
		Username: req.Username,
		Password: req.Password,
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("用户登录失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}
	if user.SsoToken != "" {
		h.ssoCookie.set(c, user.SsoToken)
	}
	response.Success(c, &dto.LoginResponse{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		UserID:       user.UserId,
		Username:     user.Username,
	})
}

// RefreshToken 刷新访问令牌（公共接口，无需 JWT 鉴权，但需要提供 refresh_token）
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.RefreshToken(c.Request.Context(), &authpb.RefreshTokenRequest{
		Token: req.RefreshToken,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("刷新令牌失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

// ExchangeSSO 使用浏览器中的 SSO Cookie 为目标应用换取新的双 token。
func (h *AuthHandler) ExchangeSSO(c *gin.Context) {
	var req dto.ExchangeSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	ssoToken, ok := h.ssoCookie.get(c)
	if !ok {
		response.Unauthorized(c, "SSO 登录态不存在或已失效，请重新登录")
		return
	}

	resp, err := h.authClient.ExchangeSSO(c.Request.Context(), &authpb.ExchangeSSORequest{
		SsoToken: ssoToken,
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("SSO 换票失败", zap.Error(err), zap.String("app_code", req.AppCode))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.ExchangeSSOResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserID:       resp.UserId,
		Username:     resp.Username,
	})
}

// Logout 用户退出登录（需 JWT 鉴权，从上下文获取 userID 用于日志追踪）
func (h *AuthHandler) Logout(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	// 从请求体中读取 device_id（兼容旧客户端：不提供也不报错）
	var req dto.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	_, err := h.authClient.Logout(grpcCtx, &authpb.LogoutRequest{
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("用户退出登录失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, nil)
}

// SendPhoneCode 发送手机验证码（公共接口，无需鉴权）
func (h *AuthHandler) SendPhoneCode(c *gin.Context) {
	var req dto.SendPhoneCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.SendPhoneCode(c.Request.Context(), &authpb.SendPhoneCodeRequest{
		Phone: req.Phone,
		Scene: req.Scene,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("发送手机验证码失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.SendPhoneCodeResponse{
		Action:          resp.Action,
		CooldownSeconds: resp.CooldownSeconds,
		Message:         resp.Message,
		DebugCode:       resp.DebugCode,
	})
}

// PhoneAuthEntry 手机验证码登录/注册入口（公共接口，无需鉴权）
func (h *AuthHandler) PhoneAuthEntry(c *gin.Context) {
	var req dto.PhoneAuthEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.PhoneAuthEntry(c.Request.Context(), &authpb.PhoneAuthEntryRequest{
		Phone:            req.Phone,
		VerificationCode: req.VerificationCode,
		AppCode:          req.AppCode,
		DeviceId:         req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("手机验证码认证失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	if resp.SsoToken != "" {
		h.ssoCookie.set(c, resp.SsoToken)
	}
	response.Success(c, &dto.PhoneAuthEntryResponse{
		Action:          resp.Action,
		AccessToken:     resp.AccessToken,
		RefreshToken:    resp.RefreshToken,
		UserID:          resp.UserId,
		Username:        resp.Username,
		Email:           resp.Email,
		Phone:           resp.Phone,
		ShouldBindEmail: resp.ShouldBindEmail,
		Message:         resp.Message,
	})
}

// PhonePasswordLogin 手机号+密码登录（公共接口，无需鉴权）
func (h *AuthHandler) PhonePasswordLogin(c *gin.Context) {
	var req dto.PhonePasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.PhonePasswordLogin(c.Request.Context(), &authpb.PhonePasswordLoginRequest{
		Phone:    req.Phone,
		Password: req.Password,
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("手机号密码登录失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	if resp.SsoToken != "" {
		h.ssoCookie.set(c, resp.SsoToken)
	}
	response.Success(c, &dto.PhonePasswordLoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserID:       resp.UserId,
		Username:     resp.Username,
		Phone:        resp.Phone,
		Message:      resp.Message,
	})
}

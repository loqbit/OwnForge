package handler

import (
	"github.com/loqbit/ownforge/services/gateway/internal/config"
	"github.com/loqbit/ownforge/services/gateway/internal/grpcclient"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/dto"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/response"
	"github.com/loqbit/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	authpb "github.com/loqbit/ownforge/pkg/proto/auth"

	"go.uber.org/zap"
)

// AuthHandler handles authentication BFF routes and calls identity's AuthService over gRPC.
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

// Login handles user sign-in.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.log).Error("user login failed", zap.Error(err))
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

// RefreshToken refreshes the access token. It is public and does not require JWT auth, but it does require a refresh_token.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.RefreshToken(c.Request.Context(), &authpb.RefreshTokenRequest{
		Token: req.RefreshToken,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("failed to refresh token", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

// ExchangeSSO exchanges the browser's SSO cookie for a new token pair for the target app.
func (h *AuthHandler) ExchangeSSO(c *gin.Context) {
	var req dto.ExchangeSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	ssoToken, ok := h.ssoCookie.get(c)
	if !ok {
		response.Unauthorized(c, "SSO session is missing or expired, please sign in again")
		return
	}

	resp, err := h.authClient.ExchangeSSO(c.Request.Context(), &authpb.ExchangeSSORequest{
		SsoToken: ssoToken,
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("SSO exchange failed", zap.Error(err), zap.String("app_code", req.AppCode))
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

// Logout signs out the user. It requires JWT auth and reads userID from context for log tracing.
func (h *AuthHandler) Logout(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "unauthorized")
		return
	}
	userID := val.(int64)

	// Read device_id from the request body, remaining compatible with older clients that omit it.
	var req dto.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	_, err := h.authClient.Logout(grpcCtx, &authpb.LogoutRequest{
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("user logout failed", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, nil)
}

// SendPhoneCode sends a phone verification code. It is a public endpoint.
func (h *AuthHandler) SendPhoneCode(c *gin.Context) {
	var req dto.SendPhoneCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.SendPhoneCode(c.Request.Context(), &authpb.SendPhoneCodeRequest{
		Phone: req.Phone,
		Scene: req.Scene,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("failed to send phone verification code", zap.Error(err))
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

// PhoneAuthEntry is the phone verification-code sign-in/sign-up entry point. It is public.
func (h *AuthHandler) PhoneAuthEntry(c *gin.Context) {
	var req dto.PhoneAuthEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.log).Error("phone verification-code authentication failed", zap.Error(err))
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

// PhonePasswordLogin handles sign-in with phone number and password. It is public.
func (h *AuthHandler) PhonePasswordLogin(c *gin.Context) {
	var req dto.PhonePasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.log).Error("phone-password login failed", zap.Error(err))
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

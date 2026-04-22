package handler

import (
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	authservice "github.com/ownforge/ownforge/services/user-platform/internal/service/auth"
	httpdto "github.com/ownforge/ownforge/services/user-platform/internal/transport/http/codec/dto"
	httperrs "github.com/ownforge/ownforge/services/user-platform/internal/transport/http/codec/errs"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/codec/response"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/codec/validator"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	avc    authservice.AuthService
	svc    accountservice.UserService
	logger *zap.Logger
}

// Dependencies groups dependencies required by the user HTTP handler.
type Dependencies struct {
	UserService accountservice.UserService
	AuthService authservice.AuthService
	Logger      *zap.Logger
}

func NewUserHandler(deps Dependencies) *UserHandler {
	return &UserHandler{svc: deps.UserService, avc: deps.AuthService, logger: deps.Logger}
}

// TODO: Remove the Register HTTP endpoint and call chain after the phone-based identity migration is complete.

// @Summary      User registration
// @Description  Register a user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterRequest true "Registration payload"
// @Success      200  {object}  dto.RegisterResponse
// @Router       /users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req httpdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use the validator helper to turn validation errors into user-friendly messages.
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.svc.Register(c.Request.Context(), &accountservice.RegisterCommand{
		Phone:    req.Phone,
		Email:    req.Email,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("user registration failed", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}
	response.Success(c, &httpdto.RegisterResponse{
		Phone:    user.Phone,
		Email:    user.Email,
		UserID:   user.UserID,
		Username: user.Username,
	})
}

// @Summary      User login
// @Description  Log in a user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginRequest true "Login payload"
// @Success      200  {object}  dto.LoginResponse
// @Router       /users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req httpdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use the validator helper to turn validation errors into user-friendly messages.
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.avc.Login(c.Request.Context(), &authservice.LoginCommand{
		Username: req.Username,
		Password: req.Password,
		AppCode:  req.AppCode,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("user login failed", zap.Error(err))
		// The service already returns domain errors, so they can be forwarded directly.
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}
	response.Success(c, &httpdto.LoginResponse{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		UserID:       user.UserID,
		Username:     user.Username,
	})
}

func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req httpdto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	token, err := h.avc.RefreshToken(c.Request.Context(), &authservice.RefreshTokenCommand{
		Token: req.Token,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to refresh token", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}
	response.Success(c, &httpdto.RefreshTokenResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	})
}

// @Summary      User logout
// @Description  Log out from a specific device
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.LogoutRequest true "Logout payload"
// @Success      200
// @Router       /users/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	var req httpdto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	// Read the authenticated identity after the auth middleware has run.
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized access")
		return
	}

	err := h.avc.Logout(c.Request.Context(), &authservice.LogoutCommand{
		UserID:   userID,
		AppCode:  req.AppCode,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("logout failed", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      Change password
// @Description  Change the current user's password and invalidate existing sessions
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.ChangePasswordRequest true "Change-password payload"
// @Success      200  {object}  dto.ChangePasswordResponse
// @Router       /users/password/change [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req httpdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized access")
		return
	}

	result, err := h.svc.ChangePassword(c.Request.Context(), &accountservice.ChangePasswordCommand{
		UserID:      userID,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to change password", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.ChangePasswordResponse{
		UserID:  result.UserID,
		Message: result.Message,
	})
}

// @Summary      Log out all devices
// @Description  Let the current user invalidate all active sessions
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200  {object}  dto.LogoutAllSessionsResponse
// @Router       /users/logout-all [post]
func (h *UserHandler) LogoutAllSessions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized access")
		return
	}

	result, err := h.svc.LogoutAllSessions(c.Request.Context(), &accountservice.LogoutAllSessionsCommand{
		UserID: userID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to log out from all devices", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.LogoutAllSessionsResponse{
		UserID:  result.UserID,
		Message: result.Message,
	})
}

// @Summary      Bind email
// @Description  Bind an email identity for the current user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.BindEmailRequest true "Bind-email payload"
// @Success      200  {object}  dto.BindEmailResponse
// @Router       /users/email/bind [post]
func (h *UserHandler) BindEmail(c *gin.Context) {
	var req httpdto.BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized access")
		return
	}

	result, err := h.svc.BindEmail(c.Request.Context(), &accountservice.BindEmailCommand{
		UserID: userID,
		Email:  req.Email,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to bind email", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.BindEmailResponse{
		UserID:  result.UserID,
		Email:   result.Email,
		Message: result.Message,
	})
}

// @Summary      Set password
// @Description  Let the current user set a local password for the first time
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.SetPasswordRequest true "Set-password payload"
// @Success      200  {object}  dto.SetPasswordResponse
// @Router       /users/password/set [post]
func (h *UserHandler) SetPassword(c *gin.Context) {
	var req httpdto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized access")
		return
	}

	result, err := h.svc.SetPassword(c.Request.Context(), &accountservice.SetPasswordCommand{
		UserID:      userID,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to set password", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.SetPasswordResponse{
		UserID:  result.UserID,
		Message: result.Message,
	})
}

func (h *UserHandler) SendPhoneCode(c *gin.Context) {
	var req httpdto.SendPhoneCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	result, err := h.avc.SendPhoneCode(c.Request.Context(), &authservice.SendPhoneCodeCommand{
		Phone: req.Phone,
		Scene: req.Scene,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("failed to send phone verification code", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.SendPhoneCodeResponse{
		Action:          result.Action,
		CooldownSeconds: result.CooldownSeconds,
		Message:         result.Message,
		DebugCode:       result.DebugCode,
	})
}

func (h *UserHandler) PhoneAuthEntry(c *gin.Context) {
	var req httpdto.PhoneAuthEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	result, err := h.avc.PhoneAuthEntry(c.Request.Context(), &authservice.PhoneAuthEntryCommand{
		Phone:            req.Phone,
		VerificationCode: req.VerificationCode,
		AppCode:          req.AppCode,
		DeviceID:         req.DeviceID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("phone login failed", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.PhoneAuthEntryResponse{
		Action:          result.Action,
		AccessToken:     result.AccessToken,
		RefreshToken:    result.RefreshToken,
		UserID:          result.UserID,
		Username:        result.Username,
		Email:           result.Email,
		Phone:           result.Phone,
		ShouldBindEmail: result.ShouldBindEmail,
		Message:         result.Message,
	})
}

func (h *UserHandler) PhonePasswordLogin(c *gin.Context) {
	var req httpdto.PhonePasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("parameter validation failed", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	result, err := h.avc.PhonePasswordLogin(c.Request.Context(), &authservice.PhonePasswordLoginCommand{
		Phone:    req.Phone,
		Password: req.Password,
		AppCode:  req.AppCode,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("phone-password login failed", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.PhonePasswordLoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		UserID:       result.UserID,
		Username:     result.Username,
		Phone:        result.Phone,
		Message:      result.Message,
	})
}

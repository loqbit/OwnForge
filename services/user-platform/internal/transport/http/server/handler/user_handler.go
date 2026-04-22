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

// Dependencies 描述用户 HTTP Handler 所需的依赖集合。
type Dependencies struct {
	UserService accountservice.UserService
	AuthService authservice.AuthService
	Logger      *zap.Logger
}

func NewUserHandler(deps Dependencies) *UserHandler {
	return &UserHandler{svc: deps.UserService, avc: deps.AuthService, logger: deps.Logger}
}

// TODO: 用户完成手机号体系切换后，删除 Register 相关 HTTP 接口和调用链。

// @Summary      用户注册
// @Description  用户注册接口
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterRequest true "注册信息"
// @Success      200  {object}  dto.RegisterResponse
// @Router       /users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req httpdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用 validator 翻译验证错误为友好提示
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("用户注册失败", zap.Error(err))
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

// @Summary      用户登录
// @Description  用户登录接口
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginRequest true "登录信息"
// @Success      200  {object}  dto.LoginResponse
// @Router       /users/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req httpdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用 validator 翻译验证错误为友好提示
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("用户登录失败", zap.Error(err))
		// 这里可以直接抛出，因为底层 Service 已经是 Domain Error 了
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	token, err := h.avc.RefreshToken(c.Request.Context(), &authservice.RefreshTokenCommand{
		Token: req.Token,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("刷新 Token 失败", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}
	response.Success(c, &httpdto.RefreshTokenResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	})
}

// @Summary      用户登出
// @Description  登出特定设备
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.LogoutRequest true "登出信息"
// @Success      200
// @Router       /users/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	var req httpdto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	// 经过 Auth 中间件后，安全的获取身份
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权的访问")
		return
	}

	err := h.avc.Logout(c.Request.Context(), &authservice.LogoutCommand{
		UserID:   userID,
		AppCode:  req.AppCode,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("登出失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary      修改密码
// @Description  当前登录用户修改密码，并使历史登录态全部失效
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.ChangePasswordRequest true "修改密码信息"
// @Success      200  {object}  dto.ChangePasswordResponse
// @Router       /users/password/change [post]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req httpdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权的访问")
		return
	}

	result, err := h.svc.ChangePassword(c.Request.Context(), &accountservice.ChangePasswordCommand{
		UserID:      userID,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("修改密码失败", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.ChangePasswordResponse{
		UserID:  result.UserID,
		Message: result.Message,
	})
}

// @Summary      退出全部设备
// @Description  当前登录用户主动让自己的全部登录态失效
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200  {object}  dto.LogoutAllSessionsResponse
// @Router       /users/logout-all [post]
func (h *UserHandler) LogoutAllSessions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权的访问")
		return
	}

	result, err := h.svc.LogoutAllSessions(c.Request.Context(), &accountservice.LogoutAllSessionsCommand{
		UserID: userID,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("退出全部设备失败", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.LogoutAllSessionsResponse{
		UserID:  result.UserID,
		Message: result.Message,
	})
}

// @Summary      绑定邮箱
// @Description  当前登录用户绑定邮箱身份
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.BindEmailRequest true "绑定邮箱信息"
// @Success      200  {object}  dto.BindEmailResponse
// @Router       /users/email/bind [post]
func (h *UserHandler) BindEmail(c *gin.Context) {
	var req httpdto.BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权的访问")
		return
	}

	result, err := h.svc.BindEmail(c.Request.Context(), &accountservice.BindEmailCommand{
		UserID: userID,
		Email:  req.Email,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("绑定邮箱失败", zap.Error(err))
		response.Error(c, httperrs.ConvertToCustomError(err))
		return
	}

	response.Success(c, &httpdto.BindEmailResponse{
		UserID:  result.UserID,
		Email:   result.Email,
		Message: result.Message,
	})
}

// @Summary      设置密码
// @Description  当前登录用户首次设置本地密码
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        request body dto.SetPasswordRequest true "设置密码信息"
// @Success      200  {object}  dto.SetPasswordResponse
// @Router       /users/password/set [post]
func (h *UserHandler) SetPassword(c *gin.Context) {
	var req httpdto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "未授权的访问")
		return
	}

	result, err := h.svc.SetPassword(c.Request.Context(), &accountservice.SetPasswordCommand{
		UserID:      userID,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("设置密码失败", zap.Error(err))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	result, err := h.avc.SendPhoneCode(c.Request.Context(), &authservice.SendPhoneCodeCommand{
		Phone: req.Phone,
		Scene: req.Scene,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("发送手机验证码失败", zap.Error(err))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("手机号登录失败", zap.Error(err))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
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
		commonlogger.Ctx(c.Request.Context(), h.logger).Error("手机号密码登录失败", zap.Error(err))
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

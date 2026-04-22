package authservice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ownforge/ownforge/pkg/crypto"
	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	"github.com/ownforge/ownforge/pkg/ratelimiter"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/smsauth"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	"go.uber.org/zap"
)

const (
	phoneAuthSceneLogin   = "login"
	phoneCodeBizIDTTL     = 10 * time.Minute
	phoneCodeDefaultTTL   = 60 * time.Second
	phoneCodeHourlyLimit  = 10
	phoneCodeHourlyWindow = time.Hour
)

var (
	ErrPhoneCodeInvalid         = pkgerrs.NewParamErr("验证码错误或已过期", nil)
	ErrPhoneCodeSceneInvalid    = pkgerrs.NewParamErr("暂不支持该验证码场景", nil)
	ErrPhoneInvalid             = pkgerrs.NewParamErr("手机号格式不正确", nil)
	ErrPhonePasswordNotSet      = pkgerrs.NewParamErr("请先设置密码后再使用密码登录", nil)
	ErrPhoneCodeSendTooFrequent = pkgerrs.NewParamErr("验证码发送次数过多，请稍后再试", nil)
)

// SendPhoneCode 在本地冷却与限流检查通过后发送登录验证码。
func (s *authService) SendPhoneCode(ctx context.Context, req *SendPhoneCodeCommand) (*SendPhoneCodeResult, error) {
	phone := strings.TrimSpace(req.Phone)
	scene := normalizePhoneScene(req.Scene)
	if err := validatePhoneAndScene(phone, scene); err != nil {
		return nil, err
	}

	if ttl, exists, err := s.phoneCodes.CooldownTTL(ctx, phone, scene); err != nil {
		return nil, fmt.Errorf("查询验证码冷却时间失败: %w", err)
	} else if exists {
		return &SendPhoneCodeResult{
			Action:          "rate_limited",
			CooldownSeconds: durationSeconds(ttl),
			Message:         "验证码发送过于频繁，请稍后再试",
		}, nil
	}

	limiterKey := fmt.Sprintf("rl:phone:code:%s", phone)
	if err := s.limiter.Allow(ctx, limiterKey, phoneCodeHourlyLimit, phoneCodeHourlyWindow); err != nil {
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			return nil, ErrPhoneCodeSendTooFrequent
		}
		return nil, fmt.Errorf("手机号验证码限流失败: %w", err)
	}

	sendResult, err := s.generateAndStorePhoneCode(ctx, phone, scene)
	if err != nil {
		return nil, err
	}

	return &SendPhoneCodeResult{
		Action:          "code_sent",
		CooldownSeconds: sendResult.CooldownSeconds,
		Message:         "验证码已发送",
		DebugCode:       sendResult.DebugCode,
	}, nil
}

// PhoneAuthEntry 校验手机号验证码并执行登录或无感注册一体化流程。
func (s *authService) PhoneAuthEntry(ctx context.Context, req *PhoneAuthEntryCommand) (*PhoneAuthEntryResult, error) {
	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.VerificationCode)
	appCode := strings.TrimSpace(req.AppCode)
	deviceID := strings.TrimSpace(req.DeviceID)
	if err := validatePhone(phone); err != nil {
		return nil, err
	}
	if code == "" {
		return nil, ErrPhoneCodeInvalid
	}
	if appCode == "" {
		return nil, pkgerrs.NewParamErr("app_code 不能为空", nil)
	}
	if deviceID == "" {
		return nil, pkgerrs.NewParamErr("device_id 不能为空", nil)
	}

	if err := s.consumeAndVerifyPhoneCode(ctx, phone, phoneAuthSceneLogin, code); err != nil {
		return nil, err
	}

	user, identity, action, err := s.findOrRegisterByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}

	if identity != nil {
		if err := s.identityRepo.TouchLogin(ctx, identity.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("更新登录身份最近登录时间失败: %w", err)
		}
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, appCode, identityIDPtr(identity)); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("应用授权处理失败 (uid:%d, app_code:%s): %w", user.ID, appCode, err)
	}
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("加载用户身份视图失败: %w", err)
	}

	loginResult, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, appCode, deviceID)
	if err != nil {
		return nil, err
	}
	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, appCode, deviceID, loginResult.RefreshToken, identityIDPtr(identity))
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, appCode, deviceID, loginResult.RefreshToken)
		return nil, fmt.Errorf("创建应用会话失败: %w", err)
	}
	loginResult.SSOToken = ssoToken

	shouldBindEmail := strings.TrimSpace(view.Email) == ""
	message := "登录成功"
	actionName := "logged_in"
	if action == "register" {
		message = "注册并登录成功"
		actionName = "registered_and_logged_in"
	}

	return &PhoneAuthEntryResult{
		Action:          actionName,
		AccessToken:     loginResult.AccessToken,
		RefreshToken:    loginResult.RefreshToken,
		UserID:          user.ID,
		Username:        view.Username,
		Email:           view.Email,
		Phone:           view.Phone,
		ShouldBindEmail: shouldBindEmail,
		Message:         message,
	}, nil
}

// PhonePasswordLogin 为已存在的手机号用户执行纯密码登录，不自动注册。
func (s *authService) PhonePasswordLogin(ctx context.Context, req *PhonePasswordLoginCommand) (*PhonePasswordLoginResult, error) {
	phone := strings.TrimSpace(req.Phone)
	password := req.Password
	appCode := strings.TrimSpace(req.AppCode)
	deviceID := strings.TrimSpace(req.DeviceID)
	if err := validatePhone(phone); err != nil {
		return nil, err
	}
	if strings.TrimSpace(password) == "" {
		return nil, ErrInvalidCredentials
	}
	if appCode == "" {
		return nil, pkgerrs.NewParamErr("app_code 不能为空", nil)
	}
	if deviceID == "" {
		return nil, pkgerrs.NewParamErr("device_id 不能为空", nil)
	}

	// 限制同一个手机号的高频密码尝试
	limiterKey := fmt.Sprintf("rl:login:phone:%s", phone)
	if err := s.limiter.Allow(ctx, limiterKey, 5, 15*60*1000000000); err != nil { // 15分钟(纳秒)
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			return nil, ErrTooManyLoginAttempts
		}
		return nil, fmt.Errorf("限流器验证失败: %w", err)
	}

	result, err := s.loginByPhonePassword(ctx, phone, password, appCode, deviceID)
	if err != nil {
		return nil, err
	}

	return &PhonePasswordLoginResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		SSOToken:     result.SSOToken,
		UserID:       result.UserID,
		Username:     result.Username,
		Phone:        result.Phone,
		Message:      "登录成功",
	}, nil
}

// generateAndStorePhoneCode 调用上游发码并落本地冷却与 BizID 状态。
func (s *authService) generateAndStorePhoneCode(ctx context.Context, phone string, scene string) (*smsauth.SendVerifyCodeResult, error) {
	sendResult, err := s.sender().SendVerifyCode(ctx, smsauth.SendVerifyCodeInput{
		Phone: phone,
		Scene: scene,
	})
	if err != nil {
		if errors.Is(err, smsauth.ErrSendFrequency) {
			return nil, ErrPhoneCodeSendTooFrequent
		}
		s.logger.Error("发送短信验证码失败",
			zap.Error(err),
			zap.String("phone", phone),
			zap.String("scene", scene),
		)
		return nil, pkgerrs.NewServerErr(fmt.Errorf("发送短信验证码失败: %w", err))
	}

	cooldown := phoneCodeDefaultTTL
	if sendResult.CooldownSeconds > 0 {
		cooldown = time.Duration(sendResult.CooldownSeconds) * time.Second
	} else {
		sendResult.CooldownSeconds = int(phoneCodeDefaultTTL.Seconds())
	}

	if err := s.phoneCodes.SaveCooldown(ctx, phone, scene, cooldown); err != nil {
		s.logger.Warn("保存手机号验证码冷却时间失败", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}
	if strings.TrimSpace(sendResult.BizID) != "" {
		if err := s.phoneCodes.SaveBizID(ctx, phone, scene, sendResult.BizID, phoneCodeBizIDTTL); err != nil {
			s.logger.Warn("保存手机号验证码 BizID 失败", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
		}
	}
	return sendResult, nil
}

// consumeAndVerifyPhoneCode 校验提交的验证码，并在成功后清理本地 BizID 状态。
func (s *authService) consumeAndVerifyPhoneCode(ctx context.Context, phone string, scene string, code string) error {
	_, _, err := s.phoneCodes.GetBizID(ctx, phone, scene)
	if err != nil {
		s.logger.Warn("读取手机号验证码 BizID 失败", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}

	result, err := s.sender().CheckVerifyCode(ctx, smsauth.CheckVerifyCodeInput{
		Phone: phone,
		Code:  code,
		Scene: scene,
	})
	if err != nil {
		return pkgerrs.NewServerErr(fmt.Errorf("校验短信验证码失败: %w", err))
	}
	if result == nil || !result.Passed {
		return ErrPhoneCodeInvalid
	}

	if err := s.phoneCodes.DeleteBizID(ctx, phone, scene); err != nil {
		s.logger.Warn("删除手机号验证码 BizID 失败", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}
	return nil
}

// findOrRegisterByPhone 优先按手机号身份查用户，查不到时走无感注册。
func (s *authService) findOrRegisterByPhone(ctx context.Context, phone string) (*accountrepo.User, *accountrepo.UserIdentity, string, error) {
	identity, err := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if err == nil {
		user, userErr := s.repo.GetByID(ctx, identity.UserID)
		if userErr != nil {
			return nil, nil, "", fmt.Errorf("按手机号身份回查用户失败: %w", userErr)
		}
		return user, identity, "login", nil
	}
	if !sharedrepo.IsNotFoundError(err) {
		return nil, nil, "", fmt.Errorf("按手机号身份查询用户失败: %w", err)
	}

	user, identity, err := s.registerUserByPhone(ctx, phone)
	if err != nil {
		return nil, nil, "", err
	}
	return user, identity, "register", nil
}

// registerUserByPhone 创建最小化手机号用户，并处理并发下的唯一键冲突回查。
func (s *authService) registerUserByPhone(ctx context.Context, phone string) (*accountrepo.User, *accountrepo.UserIdentity, error) {
	user, err := accountservice.RegisterUserWithProfile(ctx, accountservice.RegistrationDeps{
		TM:                  s.tm,
		UserRepo:            s.repo,
		IdentityRepo:        s.identityRepo,
		ProfileRepo:         s.profileRepo,
		Outbox:              s.outbox,
		TopicUserRegistered: s.topicUserRegistered,
	}, accountservice.RegistrationParams{
		Phone: phone,
	})
	if err == nil {
		identity, getErr := s.identityRepo.GetByProvider(ctx, "phone", phone)
		if getErr != nil {
			return nil, nil, fmt.Errorf("回查手机号身份失败: %w", getErr)
		}
		return user, identity, nil
	}
	if !sharedrepo.IsDuplicateKeyError(err) {
		return nil, nil, fmt.Errorf("手机号无感注册失败: %w", err)
	}

	identity, getErr := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if getErr != nil {
		return nil, nil, fmt.Errorf("手机号注册发生并发冲突，回查身份失败: %w", getErr)
	}
	user, userErr := s.repo.GetByID(ctx, identity.UserID)
	if userErr != nil {
		return nil, nil, fmt.Errorf("手机号注册发生并发冲突，回查用户失败: %w", userErr)
	}
	return user, identity, nil
}

// loginByPhonePassword 执行已注册手机号用户的密码登录流程。
func (s *authService) loginByPhonePassword(ctx context.Context, phone string, password string, appCode string, deviceID string) (*serviceLoginResult, error) {
	identity, err := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if err != nil {
		if sharedrepo.IsNotFoundError(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("按手机号身份查询用户失败: %w", err)
	}

	if identity.CredentialHash == nil || strings.TrimSpace(*identity.CredentialHash) == "" {
		return nil, ErrPhonePasswordNotSet
	}
	if !crypto.CheckPasswordHash(password, strings.TrimSpace(*identity.CredentialHash)) {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetByID(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("按手机号身份回查用户失败: %w", err)
	}

	if err := s.identityRepo.TouchLogin(ctx, identity.ID, time.Now()); err != nil {
		return nil, fmt.Errorf("更新登录身份最近登录时间失败: %w", err)
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, appCode, &identity.ID); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("应用授权处理失败 (uid:%d, app_code:%s): %w", user.ID, appCode, err)
	}
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("加载用户身份视图失败: %w", err)
	}

	loginResult, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, appCode, deviceID)
	if err != nil {
		return nil, err
	}
	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, appCode, deviceID, loginResult.RefreshToken, &identity.ID)
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, appCode, deviceID, loginResult.RefreshToken)
		return nil, fmt.Errorf("创建应用会话失败: %w", err)
	}
	loginResult.SSOToken = ssoToken

	return &serviceLoginResult{
		AccessToken:  loginResult.AccessToken,
		RefreshToken: loginResult.RefreshToken,
		SSOToken:     loginResult.SSOToken,
		UserID:       user.ID,
		Username:     view.Username,
		Phone:        view.Phone,
	}, nil
}

// serviceLoginResult 是手机号登录链路内部复用的标准登录结果。
type serviceLoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
	Phone        string
}

// sender 返回当前注入的短信验证码发送器。
func (s *authService) sender() smsauth.Sender {
	return s.smsAuthSender
}

// normalizePhoneScene 将外部 scene 标准化为内部使用的统一值。
func normalizePhoneScene(scene string) string {
	return strings.ToLower(strings.TrimSpace(scene))
}

// validatePhoneAndScene 校验手机号和当前支持的验证码场景。
func validatePhoneAndScene(phone string, scene string) error {
	if err := validatePhone(phone); err != nil {
		return err
	}
	if scene != phoneAuthSceneLogin {
		return ErrPhoneCodeSceneInvalid
	}
	return nil
}

// validatePhone 执行手机号认证链路使用的轻量格式校验。
func validatePhone(phone string) error {
	if phone == "" || len(phone) < 6 || len(phone) > 20 {
		return ErrPhoneInvalid
	}
	return nil
}

// durationSeconds 将时长转换为向上取整后的秒数。
func durationSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(math.Ceil(ttl.Seconds()))
}

package authservice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/loqbit/ownforge/pkg/crypto"
	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/smsauth"
	accountrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/account"
	sharedrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/shared"
	accountservice "github.com/loqbit/ownforge/services/user-platform/internal/service/account"
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
	ErrPhoneCodeInvalid         = pkgerrs.NewParamErr("invalid or expired verification code", nil)
	ErrPhoneCodeSceneInvalid    = pkgerrs.NewParamErr("this verification-code scenario is not supported yet", nil)
	ErrPhoneInvalid             = pkgerrs.NewParamErr("phone number has an invalid format", nil)
	ErrPhonePasswordNotSet      = pkgerrs.NewParamErr("set a password before using password login", nil)
	ErrPhoneCodeSendTooFrequent = pkgerrs.NewParamErr("too many verification code requests, please try again later", nil)
)

// SendPhoneCode sends a login verification code after local cooldown and rate-limit checks pass.
func (s *authService) SendPhoneCode(ctx context.Context, req *SendPhoneCodeCommand) (*SendPhoneCodeResult, error) {
	phone := strings.TrimSpace(req.Phone)
	scene := normalizePhoneScene(req.Scene)
	if err := validatePhoneAndScene(phone, scene); err != nil {
		return nil, err
	}

	if ttl, exists, err := s.phoneCodes.CooldownTTL(ctx, phone, scene); err != nil {
		return nil, fmt.Errorf("failed to query verification code cooldown: %w", err)
	} else if exists {
		return &SendPhoneCodeResult{
			Action:          "rate_limited",
			CooldownSeconds: durationSeconds(ttl),
			Message:         "verification codes are being sent too frequently, please try again later",
		}, nil
	}

	limiterKey := fmt.Sprintf("rl:phone:code:%s", phone)
	if err := s.limiter.Allow(ctx, limiterKey, phoneCodeHourlyLimit, phoneCodeHourlyWindow); err != nil {
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			return nil, ErrPhoneCodeSendTooFrequent
		}
		return nil, fmt.Errorf("failed to rate-limit phone verification codes: %w", err)
	}

	sendResult, err := s.generateAndStorePhoneCode(ctx, phone, scene)
	if err != nil {
		return nil, err
	}

	return &SendPhoneCodeResult{
		Action:          "code_sent",
		CooldownSeconds: sendResult.CooldownSeconds,
		Message:         "verification code sent",
		DebugCode:       sendResult.DebugCode,
	}, nil
}

// PhoneAuthEntry verifies the phone code and then runs the combined login or silent-signup flow.
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
		return nil, pkgerrs.NewParamErr("app_code  cannot be empty", nil)
	}
	if deviceID == "" {
		return nil, pkgerrs.NewParamErr("device_id  cannot be empty", nil)
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
			return nil, fmt.Errorf("failed to update identity last-login time: %w", err)
		}
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, appCode, identityIDPtr(identity)); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("failed to process application authorization (uid:%d, app_code:%s): %w", user.ID, appCode, err)
	}
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user identity view: %w", err)
	}

	loginResult, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, appCode, deviceID)
	if err != nil {
		return nil, err
	}
	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, appCode, deviceID, loginResult.RefreshToken, identityIDPtr(identity))
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, appCode, deviceID, loginResult.RefreshToken)
		return nil, fmt.Errorf("failed to create app session: %w", err)
	}
	loginResult.SSOToken = ssoToken

	shouldBindEmail := strings.TrimSpace(view.Email) == ""
	message := "login successful"
	actionName := "logged_in"
	if action == "register" {
		message = "registered and signed in successfully"
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

// PhonePasswordLogin performs password-only login for an existing phone user and does not auto-register.
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
		return nil, pkgerrs.NewParamErr("app_code  cannot be empty", nil)
	}
	if deviceID == "" {
		return nil, pkgerrs.NewParamErr("device_id  cannot be empty", nil)
	}

	// Limit repeated password attempts for the same phone number.
	limiterKey := fmt.Sprintf("rl:login:phone:%s", phone)
	if err := s.limiter.Allow(ctx, limiterKey, 5, 15*60*1000000000); err != nil { // 15 minutes (nanoseconds)
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			return nil, ErrTooManyLoginAttempts
		}
		return nil, fmt.Errorf("rate limiter check failed: %w", err)
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
		Message:      "login successful",
	}, nil
}

// generateAndStorePhoneCode calls the upstream sender and stores local cooldown and BizID state.
func (s *authService) generateAndStorePhoneCode(ctx context.Context, phone string, scene string) (*smsauth.SendVerifyCodeResult, error) {
	sendResult, err := s.sender().SendVerifyCode(ctx, smsauth.SendVerifyCodeInput{
		Phone: phone,
		Scene: scene,
	})
	if err != nil {
		if errors.Is(err, smsauth.ErrSendFrequency) {
			return nil, ErrPhoneCodeSendTooFrequent
		}
		s.logger.Error("failed to send SMS verification code",
			zap.Error(err),
			zap.String("phone", phone),
			zap.String("scene", scene),
		)
		return nil, pkgerrs.NewServerErr(fmt.Errorf("failed to send SMS verification code: %w", err))
	}

	cooldown := phoneCodeDefaultTTL
	if sendResult.CooldownSeconds > 0 {
		cooldown = time.Duration(sendResult.CooldownSeconds) * time.Second
	} else {
		sendResult.CooldownSeconds = int(phoneCodeDefaultTTL.Seconds())
	}

	if err := s.phoneCodes.SaveCooldown(ctx, phone, scene, cooldown); err != nil {
		s.logger.Warn("failed to save phone verification code cooldown", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}
	if strings.TrimSpace(sendResult.BizID) != "" {
		if err := s.phoneCodes.SaveBizID(ctx, phone, scene, sendResult.BizID, phoneCodeBizIDTTL); err != nil {
			s.logger.Warn("failed to save phone verification code BizID", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
		}
	}
	return sendResult, nil
}

// consumeAndVerifyPhoneCode validates the submitted code and clears the local BizID state on success.
func (s *authService) consumeAndVerifyPhoneCode(ctx context.Context, phone string, scene string, code string) error {
	_, _, err := s.phoneCodes.GetBizID(ctx, phone, scene)
	if err != nil {
		s.logger.Warn("failed to read phone verification code BizID", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}

	result, err := s.sender().CheckVerifyCode(ctx, smsauth.CheckVerifyCodeInput{
		Phone: phone,
		Code:  code,
		Scene: scene,
	})
	if err != nil {
		return pkgerrs.NewServerErr(fmt.Errorf("failed to verify SMS verification code: %w", err))
	}
	if result == nil || !result.Passed {
		return ErrPhoneCodeInvalid
	}

	if err := s.phoneCodes.DeleteBizID(ctx, phone, scene); err != nil {
		s.logger.Warn("failed to delete phone verification code BizID", zap.Error(err), zap.String("phone", phone), zap.String("scene", scene))
	}
	return nil
}

// findOrRegisterByPhone first looks up the user by phone identity and falls back to silent signup if none is found.
func (s *authService) findOrRegisterByPhone(ctx context.Context, phone string) (*accountrepo.User, *accountrepo.UserIdentity, string, error) {
	identity, err := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if err == nil {
		user, userErr := s.repo.GetByID(ctx, identity.UserID)
		if userErr != nil {
			return nil, nil, "", fmt.Errorf("failed to look up user by phone identity: %w", userErr)
		}
		return user, identity, "login", nil
	}
	if !sharedrepo.IsNotFoundError(err) {
		return nil, nil, "", fmt.Errorf("failed to query user by phone identity: %w", err)
	}

	user, identity, err := s.registerUserByPhone(ctx, phone)
	if err != nil {
		return nil, nil, "", err
	}
	return user, identity, "register", nil
}

// registerUserByPhone creates a minimal phone-based user and handles concurrent unique-key conflicts by querying again.
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
			return nil, nil, fmt.Errorf("failed to look up phone identity: %w", getErr)
		}
		return user, identity, nil
	}
	if !sharedrepo.IsDuplicateKeyError(err) {
		return nil, nil, fmt.Errorf("failed to register seamlessly by phone number: %w", err)
	}

	identity, getErr := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if getErr != nil {
		return nil, nil, fmt.Errorf("phone registration hit a concurrency conflict and identity lookup failed: %w", getErr)
	}
	user, userErr := s.repo.GetByID(ctx, identity.UserID)
	if userErr != nil {
		return nil, nil, fmt.Errorf("phone registration hit a concurrency conflict and user lookup failed: %w", userErr)
	}
	return user, identity, nil
}

// loginByPhonePassword runs the password login flow for an already registered phone user.
func (s *authService) loginByPhonePassword(ctx context.Context, phone string, password string, appCode string, deviceID string) (*serviceLoginResult, error) {
	identity, err := s.identityRepo.GetByProvider(ctx, "phone", phone)
	if err != nil {
		if sharedrepo.IsNotFoundError(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to query user by phone identity: %w", err)
	}

	if identity.CredentialHash == nil || strings.TrimSpace(*identity.CredentialHash) == "" {
		return nil, ErrPhonePasswordNotSet
	}
	if !crypto.CheckPasswordHash(password, strings.TrimSpace(*identity.CredentialHash)) {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repo.GetByID(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up user by phone identity: %w", err)
	}

	if err := s.identityRepo.TouchLogin(ctx, identity.ID, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to update identity last-login time: %w", err)
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, appCode, &identity.ID); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("failed to process application authorization (uid:%d, app_code:%s): %w", user.ID, appCode, err)
	}
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user identity view: %w", err)
	}

	loginResult, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, appCode, deviceID)
	if err != nil {
		return nil, err
	}
	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, appCode, deviceID, loginResult.RefreshToken, &identity.ID)
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, appCode, deviceID, loginResult.RefreshToken)
		return nil, fmt.Errorf("failed to create app session: %w", err)
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

// serviceLoginResult is the standard login result reused internally by the phone-login flow.
type serviceLoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
	Phone        string
}

// sender returns the currently injected SMS verification sender.
func (s *authService) sender() smsauth.Sender {
	return s.smsAuthSender
}

// normalizePhoneScene normalizes an external scene value into the internal canonical form.
func normalizePhoneScene(scene string) string {
	return strings.ToLower(strings.TrimSpace(scene))
}

// validatePhoneAndScene validates the phone number and the currently supported verification scene.
func validatePhoneAndScene(phone string, scene string) error {
	if err := validatePhone(phone); err != nil {
		return err
	}
	if scene != phoneAuthSceneLogin {
		return ErrPhoneCodeSceneInvalid
	}
	return nil
}

// validatePhone performs the lightweight phone-format validation used by the phone-auth flow.
func validatePhone(phone string) error {
	if phone == "" || len(phone) < 6 || len(phone) > 20 {
		return ErrPhoneInvalid
	}
	return nil
}

// durationSeconds converts a duration into seconds rounded up.
func durationSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(math.Ceil(ttl.Seconds()))
}

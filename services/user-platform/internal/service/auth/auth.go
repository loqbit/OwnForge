package authservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/google/uuid"
	"github.com/loqbit/ownforge/pkg/crypto"
	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	"github.com/loqbit/ownforge/pkg/ratelimiter"
	"github.com/loqbit/ownforge/services/user-platform/internal/auth"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/smsauth"
	accountrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/account"
	applicationrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/application"
	infrarepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/infra"
	sessionrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/session"
	sharedrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/shared"
	accountservice "github.com/loqbit/ownforge/services/user-platform/internal/service/account"
	"go.uber.org/zap"
)

var (
	ErrInvalidCredentials   = pkgerrs.NewParamErr("invalid username or password", nil)
	ErrTokenGeneration      = pkgerrs.NewServerErr(errors.New("failed to generate token"))
	ErrAccountAbnormal      = pkgerrs.New(pkgerrs.Forbidden, "account is abnormal or banned", nil)
	ErrAppNotFound          = pkgerrs.NewParamErr("application not found", nil)
	ErrTooManyLoginAttempts = pkgerrs.NewParamErr("too many login attempts, please try again in 15 minutes", nil)
)

type AuthService interface {
	Login(ctx context.Context, req *LoginCommand) (*LoginResult, error)
	VerifyToken(ctx context.Context, req *VerifyTokenCommand) (*VerifyTokenResult, error)
	RefreshToken(ctx context.Context, req *RefreshTokenCommand) (*RefreshTokenResult, error)
	ExchangeSSO(ctx context.Context, req *ExchangeSSOCommand) (*LoginResult, error)
	Logout(ctx context.Context, req *LogoutCommand) error
	SendPhoneCode(ctx context.Context, req *SendPhoneCodeCommand) (*SendPhoneCodeResult, error)
	PhoneAuthEntry(ctx context.Context, req *PhoneAuthEntryCommand) (*PhoneAuthEntryResult, error)
	PhonePasswordLogin(ctx context.Context, req *PhonePasswordLoginCommand) (*PhonePasswordLoginResult, error)
}

type authService struct {
	tm                  infrarepo.TransactionManager
	repo                accountrepo.UserRepository
	identityRepo        accountrepo.UserIdentityRepository
	profileRepo         accountrepo.ProfileRepository
	authzRepo           applicationrepo.UserAppAuthorizationRepository
	ssoSessionRepo      sessionrepo.SsoSessionRepository
	appSessionRepo      sessionrepo.AppSessionRepository
	session             sessionrepo.SessionRepository
	phoneCodes          sessionrepo.PhoneCodeRepository
	smsAuthSender       smsauth.Sender
	outbox              infrarepo.EventOutboxWriter
	jwtManager          *auth.JWTManager
	limiter             ratelimiter.Limiter
	logger              *zap.Logger
	requestGroup        *singleflight.Group
	appEnv              string
	topicUserRegistered string
}

// AuthDependencies groups the dependencies required by the auth service.
type AuthDependencies struct {
	TM                  infrarepo.TransactionManager
	UserRepo            accountrepo.UserRepository
	IdentityRepo        accountrepo.UserIdentityRepository
	ProfileRepo         accountrepo.ProfileRepository
	AuthorizationRepo   applicationrepo.UserAppAuthorizationRepository
	SSOSessionRepo      sessionrepo.SsoSessionRepository
	AppSessionRepo      sessionrepo.AppSessionRepository
	SessionCacheRepo    sessionrepo.SessionRepository
	PhoneCodeRepo       sessionrepo.PhoneCodeRepository
	SMSAuthSender       smsauth.Sender
	Outbox              infrarepo.EventOutboxWriter
	JWTManager          *auth.JWTManager
	Limiter             ratelimiter.Limiter
	Logger              *zap.Logger
	AppEnv              string
	TopicUserRegistered string
}

// NewAuthService creates the auth service.
func NewAuthService(deps AuthDependencies) AuthService {
	return &authService{
		tm:                  deps.TM,
		repo:                deps.UserRepo,
		identityRepo:        deps.IdentityRepo,
		profileRepo:         deps.ProfileRepo,
		authzRepo:           deps.AuthorizationRepo,
		ssoSessionRepo:      deps.SSOSessionRepo,
		appSessionRepo:      deps.AppSessionRepo,
		session:             deps.SessionCacheRepo,
		phoneCodes:          deps.PhoneCodeRepo,
		smsAuthSender:       deps.SMSAuthSender,
		outbox:              deps.Outbox,
		jwtManager:          deps.JWTManager,
		limiter:             deps.Limiter,
		logger:              deps.Logger,
		requestGroup:        &singleflight.Group{},
		appEnv:              deps.AppEnv,
		topicUserRegistered: deps.TopicUserRegistered,
	}
}

func (s *authService) Login(ctx context.Context, req *LoginCommand) (*LoginResult, error) {
	// Limit repeated login attempts for the same username.
	limiterKey := fmt.Sprintf("rl:login:user:%s", req.Username)
	if err := s.limiter.Allow(ctx, limiterKey, 5, 15*60*1000000000); err != nil { // 15 minutes (nanoseconds)
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			commonlogger.Ctx(ctx, s.logger).Warn("rate-limited account login to prevent brute-force attacks", zap.String("username", req.Username))
			return nil, ErrTooManyLoginAttempts
		}
		return nil, fmt.Errorf("rate limiter check failed: %w", err)
	}

	// 1. Resolve the login identity and load the user.
	user, identity, err := s.resolvePasswordLogin(ctx, req.Username)
	if err != nil {
		if sharedrepo.IsNotFoundError(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 2. Compare the password.
	passwordHash := ""
	if identity != nil && identity.CredentialHash != nil && strings.TrimSpace(*identity.CredentialHash) != "" {
		passwordHash = strings.TrimSpace(*identity.CredentialHash)
	}
	if passwordHash == "" || !crypto.CheckPasswordHash(req.Password, passwordHash) {
		return nil, ErrInvalidCredentials
	}

	if identity != nil {
		if err := s.identityRepo.TouchLogin(ctx, identity.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("failed to update identity last-login time: %w", err)
		}
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, req.AppCode, identityIDPtr(identity)); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("failed to process application authorization (uid:%d, app_code:%s): %w", user.ID, req.AppCode, err)
	}
	// 4. Issue the access and refresh tokens.
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user identity view: %w", err)
	}

	result, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, req.AppCode, req.DeviceID)
	if err != nil {
		return nil, err
	}

	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, req.AppCode, req.DeviceID, result.RefreshToken, identityIDPtr(identity))
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, fmt.Errorf("failed to create app session: %w", err)
	}
	result.SSOToken = ssoToken

	return result, nil
}

func (s *authService) VerifyToken(ctx context.Context, req *VerifyTokenCommand) (*VerifyTokenResult, error) {
	claims, err := s.jwtManager.VerifyToken(req.Token)
	if err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "invalid or expired access token", err)
	}
	currentVersion, err := s.repo.GetUserVersion(ctx, claims.UserID)
	if err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "invalid or expired access token", err)
	}
	if err := validateUserVersion(currentVersion, claims.UserVersion); err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "invalid or expired access token", err)
	}
	return &VerifyTokenResult{
		UserID:   claims.UserID,
		Username: claims.Username,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, req *RefreshTokenCommand) (*RefreshTokenResult, error) {
	// Check the fast Redis path first to see whether grace-period protection applies.
	if result, found := s.session.CheckGracePeriod(ctx, req.Token); found {
		return &RefreshTokenResult{
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
		}, nil
	}

	// Use singleflight to avoid duplicate cache misses on the same machine.
	v, err, _ := s.requestGroup.Do(req.Token, func() (interface{}, error) {
		lockKey := fmt.Sprintf("lock:refresh:%s", req.Token)

		locked, err := s.session.TryLock(ctx, lockKey, 5*time.Second)
		if err == nil && locked {
			defer s.session.UnLock(ctx, lockKey)

			record, sessionErr := s.appSessionRepo.GetByTokenHash(ctx, hashToken(req.Token))
			if sessionErr != nil {
				if graceRes, ok := s.session.CheckGracePeriod(ctx, req.Token); ok {
					return &RefreshTokenResult{
						AccessToken:  graceRes.AccessToken,
						RefreshToken: graceRes.RefreshToken,
					}, nil
				}
				return nil, sessionErr
			}

			if err := validateActiveAppSession(record); err != nil {
				return nil, err
			}
			user, err := s.repo.GetByID(ctx, record.UserID)
			if err != nil {
				return nil, fmt.Errorf("attempted to refresh token but user(uid:%d) does not exist: %w", record.UserID, ErrAccountAbnormal)
			}
			if err := validateUserVersion(user.UserVersion, record.UserVersion); err != nil {
				return nil, err
			}
			if err := s.validateParentSsoSession(ctx, user.UserVersion, record); err != nil {
				return nil, err
			}

			deviceID := ""
			if record.DeviceID != nil {
				deviceID = *record.DeviceID
			}

			view, err := s.loadIdentityView(ctx, user.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to load user identity view: %w", err)
			}

			result, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, record.AppCode, deviceID)
			if err != nil {
				return nil, err
			}

			if _, err := s.appSessionRepo.Rotate(ctx, sessionrepo.RotateSessionParams{
				SessionID:       record.ID,
				PreviousVersion: record.Version,
				NewTokenHash:    hashToken(result.RefreshToken),
				NextExpiresAt:   time.Now().Add(auth.RefreshTokenDuration),
				LastSeenAt:      timePtr(time.Now()),
			}); err != nil {
				s.cleanupIssuedRefreshToken(ctx, user.ID, record.AppCode, deviceID, result.RefreshToken)
				return nil, err
			}
			if record.SsoSessionID != nil {
				if err := s.ssoSessionRepo.Touch(ctx, *record.SsoSessionID, time.Now()); err != nil {
					s.cleanupIssuedRefreshToken(ctx, user.ID, record.AppCode, deviceID, result.RefreshToken)
					return nil, err
				}
			}

			res := &RefreshTokenResult{
				AccessToken:  result.AccessToken,
				RefreshToken: result.RefreshToken,
			}

			s.session.SaveGracePeriod(ctx, req.Token, sessionrepo.TokenPair{
				AccessToken:  res.AccessToken,
				RefreshToken: res.RefreshToken,
			}, 15*time.Second)
			return res, nil
		}
		// Lock already held.
		time.Sleep(200 * time.Millisecond)
		if graceRes, ok := s.session.CheckGracePeriod(ctx, req.Token); ok {
			return &RefreshTokenResult{
				AccessToken:  graceRes.AccessToken,
				RefreshToken: graceRes.RefreshToken,
			}, nil
		}
		return nil, ErrInvalidCredentials
	})
	if err != nil {
		return nil, err
	}
	return v.(*RefreshTokenResult), nil
}

// ExchangeSSO exchanges the browser-carried SSO cookie for a new token pair for the current app.
func (s *authService) ExchangeSSO(ctx context.Context, req *ExchangeSSOCommand) (*LoginResult, error) {
	if strings.TrimSpace(req.SSOToken) == "" {
		return nil, sharedrepo.ErrInvalidOrExpiredToken
	}

	ssoRecord, err := s.ssoSessionRepo.GetByTokenHash(ctx, hashToken(req.SSOToken))
	if err != nil {
		return nil, err
	}
	if err := validateActiveSsoSession(ssoRecord); err != nil {
		return nil, err
	}
	user, err := s.repo.GetByID(ctx, ssoRecord.UserID)
	if err != nil {
		return nil, fmt.Errorf("attempted to exchange SSO for an app session but user(uid:%d) does not exist: %w", ssoRecord.UserID, ErrAccountAbnormal)
	}
	if err := validateUserVersion(user.UserVersion, ssoRecord.UserVersion); err != nil {
		return nil, err
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, req.AppCode, ssoRecord.IdentityID); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("failed to process application authorization (uid:%d, app_code:%s): %w", user.ID, req.AppCode, err)
	}

	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user identity view: %w", err)
	}

	result, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, req.AppCode, req.DeviceID)
	if err != nil {
		return nil, err
	}

	if err := s.persistAppSessionFromSSO(ctx, user.ID, user.UserVersion, ssoRecord.ID, req.AppCode, req.DeviceID, result.RefreshToken, ssoRecord.IdentityID); err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, fmt.Errorf("failed to create app session: %w", err)
	}
	if err := s.ssoSessionRepo.Touch(ctx, ssoRecord.ID, time.Now()); err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, err
	}

	return result, nil
}

// issueTokens centralizes token-pair issuance and device-level Redis storage.
func (s *authService) issueTokens(ctx context.Context, userID int64, username string, userVersion int64, appCode string, deviceID string) (*LoginResult, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(userID, username, userVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token (uid:%d, cause:%v): %w", userID, err, ErrTokenGeneration)
	}

	refreshToken := uuid.New().String()

	err = s.session.SaveDeviceSession(ctx, userID, appCode, deviceID, refreshToken, auth.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to store session (uid:%d): %w", userID, err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
		Username:     username,
	}, nil
}

// resolvePasswordLogin resolves username-and-password login through the identity table.
func (s *authService) resolvePasswordLogin(ctx context.Context, login string) (*accountrepo.User, *accountrepo.UserIdentity, error) {
	login = strings.TrimSpace(login)
	provider := detectLoginProvider(login)

	identity, err := s.identityRepo.GetByProvider(ctx, provider, login)
	if err != nil {
		return nil, nil, err
	}
	user, userErr := s.repo.GetByID(ctx, identity.UserID)
	if userErr != nil {
		return nil, nil, userErr
	}
	return user, identity, nil
}

// ensureAppAuthorization maintains the user's authorization relationship for the specified app.
func (s *authService) ensureAppAuthorization(ctx context.Context, userID int64, appCode string, identityID *int) error {
	authz, err := s.authzRepo.Ensure(ctx, applicationrepo.EnsureUserAppAuthorizationParams{
		UserID:           userID,
		AppCode:          appCode,
		SourceIdentityID: identityID,
	})
	if err != nil {
		return err
	}
	if authz != nil {
		if err := s.authzRepo.TouchLogin(ctx, authz.ID, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

// persistLoginSessions creates the global SSO session and app session after issuing the refresh token.
func (s *authService) persistLoginSessions(ctx context.Context, userID int64, userVersion int64, appCode string, deviceID string, refreshToken string, identityID *int) (string, error) {
	var deviceIDPtr *string
	if strings.TrimSpace(deviceID) != "" {
		deviceIDPtr = &deviceID
	}
	ssoSeed := uuid.NewString()

	err := s.tm.WithTx(ctx, func(txCtx context.Context) error {
		ssoSession, err := s.ssoSessionRepo.Create(txCtx, sessionrepo.CreateSsoSessionParams{
			UserID:      userID,
			IdentityID:  identityID,
			TokenHash:   hashToken(ssoSeed),
			UserVersion: userVersion,
			DeviceID:    deviceIDPtr,
			ExpiresAt:   time.Now().Add(auth.RefreshTokenDuration),
		})
		if err != nil {
			return err
		}

		_, err = s.appSessionRepo.Create(txCtx, sessionrepo.CreateSessionParams{
			UserID:       userID,
			AppCode:      appCode,
			SsoSessionID: &ssoSession.ID,
			IdentityID:   identityID,
			TokenHash:    hashToken(refreshToken),
			UserVersion:  userVersion,
			DeviceID:     deviceIDPtr,
			ExpiresAt:    time.Now().Add(auth.RefreshTokenDuration),
		})
		return err
	})
	if err != nil {
		return "", err
	}
	return ssoSeed, nil
}

// persistAppSessionFromSSO creates an app session for an existing global SSO session.
func (s *authService) persistAppSessionFromSSO(ctx context.Context, userID int64, userVersion int64, ssoSessionID uuid.UUID, appCode string, deviceID string, refreshToken string, identityID *int) error {
	var deviceIDPtr *string
	if strings.TrimSpace(deviceID) != "" {
		deviceIDPtr = &deviceID
	}

	_, err := s.appSessionRepo.Create(ctx, sessionrepo.CreateSessionParams{
		UserID:       userID,
		AppCode:      appCode,
		SsoSessionID: &ssoSessionID,
		IdentityID:   identityID,
		TokenHash:    hashToken(refreshToken),
		UserVersion:  userVersion,
		DeviceID:     deviceIDPtr,
		ExpiresAt:    time.Now().Add(auth.RefreshTokenDuration),
	})
	return err
}

// validateActiveAppSession checks whether the app session is still valid for refresh.
func validateActiveAppSession(record *sessionrepo.SessionRecord) error {
	if record == nil {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	if record.Status != "active" {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	if !record.ExpiresAt.After(time.Now()) {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	return nil
}

// validateActiveSsoSession checks whether the global SSO session is still valid.
func validateActiveSsoSession(record *sessionrepo.SsoSession) error {
	if record == nil {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	if record.Status != "active" {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	if !record.ExpiresAt.After(time.Now()) {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	return nil
}

// validateUserVersion checks whether the current user version matches the session snapshot.
func validateUserVersion(current int64, snapshot int64) error {
	if current != snapshot {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	return nil
}

// detectLoginProvider infers the login identity provider from the input.
func detectLoginProvider(login string) string {
	if strings.Contains(login, "@") {
		return "email"
	}
	if looksLikePhone(login) {
		return "phone"
	}
	return "username"
}

// looksLikePhone uses a very lightweight rule set to detect phone-like input without interfering with username login.
func looksLikePhone(login string) bool {
	if len(login) < 6 || len(login) > 20 {
		return false
	}
	for _, r := range login {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// hashToken applies a one-way hash to sensitive tokens before storing them.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// identityIDPtr extracts the identity primary key for passing into authorization and session layers.
func identityIDPtr(identity *accountrepo.UserIdentity) *int {
	if identity == nil {
		return nil
	}
	return &identity.ID
}

// loadIdentityView loads user identities and aggregates the login display information.
func (s *authService) loadIdentityView(ctx context.Context, userID int64) (accountservice.IdentityView, error) {
	identities, err := s.identityRepo.ListByUserID(ctx, userID)
	if err != nil {
		return accountservice.IdentityView{}, err
	}
	return accountservice.BuildIdentityView(userID, identities), nil
}

// validateParentSsoSession checks whether the global SSO session attached to the app session is valid.
func (s *authService) validateParentSsoSession(ctx context.Context, currentUserVersion int64, record *sessionrepo.SessionRecord) error {
	if record == nil || record.SsoSessionID == nil {
		return nil
	}
	ssoRecord, err := s.ssoSessionRepo.GetByID(ctx, *record.SsoSessionID)
	if err != nil {
		return err
	}
	if err := validateActiveSsoSession(ssoRecord); err != nil {
		return err
	}
	return validateUserVersion(currentUserVersion, ssoRecord.UserVersion)
}

// cleanupIssuedRefreshToken best-effort cleans up the refresh token already written to Redis when persistence fails.
func (s *authService) cleanupIssuedRefreshToken(ctx context.Context, userID int64, appCode string, deviceID string, refreshToken string) {
	if refreshToken == "" {
		return
	}
	if err := s.session.DeleteTokenIndex(ctx, refreshToken); err != nil {
		commonlogger.Ctx(ctx, s.logger).Warn("failed to clean up refresh-token index", zap.Error(err))
	}
	if _, err := s.session.DeleteAppSession(ctx, userID, appCode, deviceID); err != nil {
		commonlogger.Ctx(ctx, s.logger).Warn("failed to clean up device session cache", zap.Error(err))
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// Logout signs out only the specified device session and does not affect the user's other devices.
func (s *authService) Logout(ctx context.Context, req *LogoutCommand) error {
	oldToken, err := s.session.DeleteAppSession(ctx, req.UserID, req.AppCode, req.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to log out device: %w", err)
	}
	if oldToken != "" {
		s.session.DeleteTokenIndex(ctx, oldToken)
		record, getErr := s.appSessionRepo.GetByTokenHash(ctx, hashToken(oldToken))
		if getErr == nil {
			if err := s.appSessionRepo.Revoke(ctx, record.ID, time.Now()); err != nil {
				return fmt.Errorf("failed to revoke app sessions: %w", err)
			}
		} else if !sharedrepo.IsNotFoundError(getErr) {
			return fmt.Errorf("failed to query app sessions: %w", getErr)
		}
	}

	commonlogger.Ctx(ctx, s.logger).Info("user logged out from app device",
		zap.Int64("user_id", req.UserID),
		zap.String("app_code", req.AppCode),
		zap.String("device_id", req.DeviceID),
	)
	return nil
}

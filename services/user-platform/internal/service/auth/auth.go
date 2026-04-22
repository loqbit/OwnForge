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
	"github.com/ownforge/ownforge/pkg/crypto"
	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/pkg/ratelimiter"
	"github.com/ownforge/ownforge/services/user-platform/internal/auth"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/smsauth"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	applicationrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/application"
	infrarepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/infra"
	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	"go.uber.org/zap"
)

var (
	ErrInvalidCredentials   = pkgerrs.NewParamErr("用户名或密码错误", nil)
	ErrTokenGeneration      = pkgerrs.NewServerErr(errors.New("生成 Token 失败"))
	ErrAccountAbnormal      = pkgerrs.New(pkgerrs.Forbidden, "账号异常或已被封禁", nil)
	ErrAppNotFound          = pkgerrs.NewParamErr("应用不存在", nil)
	ErrTooManyLoginAttempts = pkgerrs.NewParamErr("尝试登录次数过多，请15分钟后再试", nil)
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

// AuthDependencies 描述认证服务所需的依赖集合。
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

// NewAuthService 创建认证服务。
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
	// 限制同一个 Username 的高频尝试
	limiterKey := fmt.Sprintf("rl:login:user:%s", req.Username)
	if err := s.limiter.Allow(ctx, limiterKey, 5, 15*60*1000000000); err != nil { // 15分钟(纳秒)
		if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
			commonlogger.Ctx(ctx, s.logger).Warn("防止暴力破解, 账号登录被限流", zap.String("username", req.Username))
			return nil, ErrTooManyLoginAttempts
		}
		return nil, fmt.Errorf("限流器验证失败: %w", err)
	}

	// 1. 解析登录身份并获取用户
	user, identity, err := s.resolvePasswordLogin(ctx, req.Username)
	if err != nil {
		if sharedrepo.IsNotFoundError(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 2. 比较密码
	passwordHash := ""
	if identity != nil && identity.CredentialHash != nil && strings.TrimSpace(*identity.CredentialHash) != "" {
		passwordHash = strings.TrimSpace(*identity.CredentialHash)
	}
	if passwordHash == "" || !crypto.CheckPasswordHash(req.Password, passwordHash) {
		return nil, ErrInvalidCredentials
	}

	if identity != nil {
		if err := s.identityRepo.TouchLogin(ctx, identity.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("更新登录身份最近登录时间失败: %w", err)
		}
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, req.AppCode, identityIDPtr(identity)); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("应用授权处理失败 (uid:%d, app_code:%s): %w", user.ID, req.AppCode, err)
	}
	// 4. 签发双 Token
	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("加载用户身份视图失败: %w", err)
	}

	result, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, req.AppCode, req.DeviceID)
	if err != nil {
		return nil, err
	}

	ssoToken, err := s.persistLoginSessions(ctx, user.ID, user.UserVersion, req.AppCode, req.DeviceID, result.RefreshToken, identityIDPtr(identity))
	if err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, fmt.Errorf("创建应用会话失败: %w", err)
	}
	result.SSOToken = ssoToken

	return result, nil
}

func (s *authService) VerifyToken(ctx context.Context, req *VerifyTokenCommand) (*VerifyTokenResult, error) {
	claims, err := s.jwtManager.VerifyToken(req.Token)
	if err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "无效的访问凭证或已过期", err)
	}
	currentVersion, err := s.repo.GetUserVersion(ctx, claims.UserID)
	if err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "无效的访问凭证或已过期", err)
	}
	if err := validateUserVersion(currentVersion, claims.UserVersion); err != nil {
		return nil, pkgerrs.New(pkgerrs.Unauthorized, "无效的访问凭证或已过期", err)
	}
	return &VerifyTokenResult{
		UserID:   claims.UserID,
		Username: claims.Username,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, req *RefreshTokenCommand) (*RefreshTokenResult, error) {
	// 先查 Redis 极速通道（是否有 grace period 保护）
	if result, found := s.session.CheckGracePeriod(ctx, req.Token); found {
		return &RefreshTokenResult{
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
		}, nil
	}

	// 使用 Singleflight 避免同一台机器上的重复击穿
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
				return nil, fmt.Errorf("试图刷新Token但用户(uid:%d)不存在: %w", record.UserID, ErrAccountAbnormal)
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
				return nil, fmt.Errorf("加载用户身份视图失败: %w", err)
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
		// 锁被占用
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

// ExchangeSSO 使用浏览器携带的 SSO Cookie 为当前应用换取新的双 Token。
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
		return nil, fmt.Errorf("试图通过 SSO 换取应用会话但用户(uid:%d)不存在: %w", ssoRecord.UserID, ErrAccountAbnormal)
	}
	if err := validateUserVersion(user.UserVersion, ssoRecord.UserVersion); err != nil {
		return nil, err
	}

	if err := s.ensureAppAuthorization(ctx, user.ID, req.AppCode, ssoRecord.IdentityID); err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("应用授权处理失败 (uid:%d, app_code:%s): %w", user.ID, req.AppCode, err)
	}

	view, err := s.loadIdentityView(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("加载用户身份视图失败: %w", err)
	}

	result, err := s.issueTokens(ctx, user.ID, view.Username, user.UserVersion, req.AppCode, req.DeviceID)
	if err != nil {
		return nil, err
	}

	if err := s.persistAppSessionFromSSO(ctx, user.ID, user.UserVersion, ssoRecord.ID, req.AppCode, req.DeviceID, result.RefreshToken, ssoRecord.IdentityID); err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, fmt.Errorf("创建应用会话失败: %w", err)
	}
	if err := s.ssoSessionRepo.Touch(ctx, ssoRecord.ID, time.Now()); err != nil {
		s.cleanupIssuedRefreshToken(ctx, user.ID, req.AppCode, req.DeviceID, result.RefreshToken)
		return nil, err
	}

	return result, nil
}

// issueTokens 统一处理双 Token 的签发与 Redis 设备级存储。
func (s *authService) issueTokens(ctx context.Context, userID int64, username string, userVersion int64, appCode string, deviceID string) (*LoginResult, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(userID, username, userVersion)
	if err != nil {
		return nil, fmt.Errorf("生成 Access Token 失败 (uid:%d, cause:%v): %w", userID, err, ErrTokenGeneration)
	}

	refreshToken := uuid.New().String()

	err = s.session.SaveDeviceSession(ctx, userID, appCode, deviceID, refreshToken, auth.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("存储 Session 失败 (uid:%d): %w", userID, err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       userID,
		Username:     username,
	}, nil
}

// resolvePasswordLogin 按身份表解析用户名密码登录。
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

// ensureAppAuthorization 维护用户在指定应用下的授权关系。
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

// persistLoginSessions 在签发 refresh token 后补建全局登录态和应用会话。
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

// persistAppSessionFromSSO 为已存在的全局登录态补建应用会话。
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

// validateActiveAppSession 校验应用会话是否仍可用于 refresh。
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

// validateActiveSsoSession 校验全局登录态是否仍然有效。
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

// validateUserVersion 校验当前用户版本和会话快照是否一致。
func validateUserVersion(current int64, snapshot int64) error {
	if current != snapshot {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	return nil
}

// detectLoginProvider 基于输入内容推断登录身份提供方。
func detectLoginProvider(login string) string {
	if strings.Contains(login, "@") {
		return "email"
	}
	if looksLikePhone(login) {
		return "phone"
	}
	return "username"
}

// looksLikePhone 用极轻量规则识别手机号风格输入，避免干扰用户名登录。
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

// hashToken 对敏感 token 做单向哈希后再落库。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// identityIDPtr 提取身份主键，方便传给授权和会话层。
func identityIDPtr(identity *accountrepo.UserIdentity) *int {
	if identity == nil {
		return nil
	}
	return &identity.ID
}

// loadIdentityView 加载用户身份并聚合出登录展示信息。
func (s *authService) loadIdentityView(ctx context.Context, userID int64) (accountservice.IdentityView, error) {
	identities, err := s.identityRepo.ListByUserID(ctx, userID)
	if err != nil {
		return accountservice.IdentityView{}, err
	}
	return accountservice.BuildIdentityView(userID, identities), nil
}

// validateParentSsoSession 校验应用会话挂载的全局登录态是否有效。
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

// cleanupIssuedRefreshToken 在持久化失败时尽量回收已写入 Redis 的 refresh token。
func (s *authService) cleanupIssuedRefreshToken(ctx context.Context, userID int64, appCode string, deviceID string, refreshToken string) {
	if refreshToken == "" {
		return
	}
	if err := s.session.DeleteTokenIndex(ctx, refreshToken); err != nil {
		commonlogger.Ctx(ctx, s.logger).Warn("清理 refresh token 索引失败", zap.Error(err))
	}
	if _, err := s.session.DeleteAppSession(ctx, userID, appCode, deviceID); err != nil {
		commonlogger.Ctx(ctx, s.logger).Warn("清理设备会话缓存失败", zap.Error(err))
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// Logout 登出功能：只删除特定设备的 Session 信息，不影响当前用户的其他设备。
func (s *authService) Logout(ctx context.Context, req *LogoutCommand) error {
	oldToken, err := s.session.DeleteAppSession(ctx, req.UserID, req.AppCode, req.DeviceID)
	if err != nil {
		return fmt.Errorf("注销设备失败: %w", err)
	}
	if oldToken != "" {
		s.session.DeleteTokenIndex(ctx, oldToken)
		record, getErr := s.appSessionRepo.GetByTokenHash(ctx, hashToken(oldToken))
		if getErr == nil {
			if err := s.appSessionRepo.Revoke(ctx, record.ID, time.Now()); err != nil {
				return fmt.Errorf("撤销应用会话失败: %w", err)
			}
		} else if !sharedrepo.IsNotFoundError(getErr) {
			return fmt.Errorf("查询应用会话失败: %w", getErr)
		}
	}

	commonlogger.Ctx(ctx, s.logger).Info("用户退出应用设备",
		zap.Int64("user_id", req.UserID),
		zap.String("app_code", req.AppCode),
		zap.String("device_id", req.DeviceID),
	)
	return nil
}

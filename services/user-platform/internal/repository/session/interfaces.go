package sessionrepo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SsoSessionRepository 定义了全局登录态的持久化接口。
type SsoSessionRepository interface {
	Create(ctx context.Context, params CreateSsoSessionParams) (*SsoSession, error)
	GetByID(ctx context.Context, id uuid.UUID) (*SsoSession, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*SsoSession, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*SsoSession, error)
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	BumpVersion(ctx context.Context, id uuid.UUID) (int64, error)
	Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
	RevokeByUserID(ctx context.Context, userID int64, revokedAt time.Time) error
}

// AppSessionRepository 定义了应用会话的持久化接口。
type AppSessionRepository interface {
	Create(ctx context.Context, params CreateSessionParams) (*SessionRecord, error)
	GetByID(ctx context.Context, id uuid.UUID) (*SessionRecord, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*SessionRecord, error)
	ListActiveByUserAndApp(ctx context.Context, userID int64, appCode string) ([]*SessionRecord, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*SessionRecord, error)
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	Rotate(ctx context.Context, params RotateSessionParams) (*SessionRecord, error)
	Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
	RevokeByUserID(ctx context.Context, userID int64, revokedAt time.Time) error
}

// SessionRepository 屏蔽了 refresh session 缓存存储引擎的细节。
type SessionRepository interface {
	SaveDeviceSession(ctx context.Context, userID int64, appCode string, deviceID string, refreshToken string, duration time.Duration) error
	GetSessionByToken(ctx context.Context, refreshToken string) (userID int64, appCode string, deviceID string, err error)
	ValidateDeviceToken(ctx context.Context, userID int64, appCode string, deviceID string, candidateToken string) error
	DeleteTokenIndex(ctx context.Context, refreshToken string) error
	DeleteAppSession(ctx context.Context, userID int64, appCode string, deviceID string) (oldToken string, err error)
	TryLock(ctx context.Context, key string, expiration time.Duration) (bool, error)
	UnLock(ctx context.Context, key string) error
	SaveGracePeriod(ctx context.Context, oldToken string, newToken TokenPair, duration time.Duration) error
	CheckGracePeriod(ctx context.Context, oldToken string) (*TokenPair, bool)
}

// PhoneCodeRepository 定义了手机验证码的持久化接口。
type PhoneCodeRepository interface {
	SaveCode(ctx context.Context, phone string, scene string, code string, ttl time.Duration, cooldown time.Duration) error
	CooldownTTL(ctx context.Context, phone string, scene string) (time.Duration, bool, error)
	VerifyCode(ctx context.Context, phone string, scene string, code string) (bool, error)
	DeleteCode(ctx context.Context, phone string, scene string) error
	SaveCooldown(ctx context.Context, phone string, scene string, cooldown time.Duration) error
	SaveBizID(ctx context.Context, phone string, scene string, bizID string, ttl time.Duration) error
	GetBizID(ctx context.Context, phone string, scene string) (string, bool, error)
	DeleteBizID(ctx context.Context, phone string, scene string) error
}

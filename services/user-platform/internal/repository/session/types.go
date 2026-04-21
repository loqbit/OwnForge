package sessionrepo

import (
	"time"

	"github.com/google/uuid"
)

// SsoSession 是全局登录态的领域模型。
type SsoSession struct {
	ID          uuid.UUID
	UserID      int64
	IdentityID  *int
	TokenHash   string
	DeviceID    *string
	UserAgent   *string
	IP          *string
	Status      string
	Version     int64
	UserVersion int64
	ExpiresAt   time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
}

// CreateSsoSessionParams 是创建全局登录态的参数。
type CreateSsoSessionParams struct {
	UserID      int64
	IdentityID  *int
	TokenHash   string
	UserVersion int64
	DeviceID    *string
	UserAgent   *string
	IP          *string
	ExpiresAt   time.Time
}

// SessionRecord 是应用会话的领域模型。
type SessionRecord struct {
	ID      uuid.UUID
	UserID  int64
	AppID   int
	AppCode string
	// SsoSessionID 为空表示该应用会话不是从某次全局登录态派生而来，
	// 而是一次独立的应用登录。
	SsoSessionID *uuid.UUID
	IdentityID   *int
	TokenHash    string
	DeviceID     *string
	UserAgent    *string
	IP           *string
	Status       string
	Version      int64
	UserVersion  int64
	ExpiresAt    time.Time
	LastSeenAt   time.Time
	RevokedAt    *time.Time
}

// CreateSessionParams 是创建应用会话的参数。
type CreateSessionParams struct {
	UserID  int64
	AppCode string
	// SsoSessionID 可选；当应用会话由某次 SSO 登录派生时传入。
	SsoSessionID *uuid.UUID
	IdentityID   *int
	TokenHash    string
	UserVersion  int64
	DeviceID     *string
	UserAgent    *string
	IP           *string
	ExpiresAt    time.Time
}

// RotateSessionParams 是轮换应用会话令牌的参数。
type RotateSessionParams struct {
	SessionID       uuid.UUID
	PreviousVersion int64
	NewTokenHash    string
	NextExpiresAt   time.Time
	LastSeenAt      *time.Time
}

// TokenPair 是 Access Token 和 Refresh Token 的组合。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

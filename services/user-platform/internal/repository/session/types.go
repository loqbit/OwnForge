package sessionrepo

import (
	"time"

	"github.com/google/uuid"
)

// SsoSession is the domain model for a global SSO session.
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

// CreateSsoSessionParams contains parameters for creating a global SSO session.
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

// SessionRecord is the domain model for an application session.
type SessionRecord struct {
	ID      uuid.UUID
	UserID  int64
	AppID   int
	AppCode string
	// A nil SsoSessionID means the application session was not derived from a global SSO session,
	// but instead came from an independent app login.
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

// CreateSessionParams contains parameters for creating an application session.
type CreateSessionParams struct {
	UserID  int64
	AppCode string
	// SsoSessionID is optional and is set when the app session is derived from an SSO login.
	SsoSessionID *uuid.UUID
	IdentityID   *int
	TokenHash    string
	UserVersion  int64
	DeviceID     *string
	UserAgent    *string
	IP           *string
	ExpiresAt    time.Time
}

// RotateSessionParams contains parameters for rotating an application session token.
type RotateSessionParams struct {
	SessionID       uuid.UUID
	PreviousVersion int64
	NewTokenHash    string
	NextExpiresAt   time.Time
	LastSeenAt      *time.Time
}

// TokenPair is the pair of access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

package applicationrepo

import "time"

// UserAppAuthorization is the domain model for user app authorization.
type UserAppAuthorization struct {
	ID                int
	UserID            int64
	AppID             int
	SourceIdentityID  *int
	Status            string
	Scopes            []string
	ExtProfile        map[string]any
	FirstAuthorizedAt time.Time
	LastLoginAt       *time.Time
	LastActiveAt      time.Time
}

// EnsureUserAppAuthorizationParams contains parameters for ensuring user app authorization exists.
type EnsureUserAppAuthorizationParams struct {
	UserID           int64
	AppCode          string
	SourceIdentityID *int
	Scopes           []string
	ExtProfile       map[string]any
}

package applicationrepo

import "time"

// UserAppAuthorization 是用户应用授权的领域模型。
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

// EnsureUserAppAuthorizationParams 是确保用户应用授权存在的参数。
type EnsureUserAppAuthorizationParams struct {
	UserID           int64
	AppCode          string
	SourceIdentityID *int
	Scopes           []string
	ExtProfile       map[string]any
}

package accountrepo

import "time"

// User 是用户主体的领域模型。
type User struct {
	ID          int64
	Status      string
	UserVersion int64
}

// Profile 是用户资料的领域模型。
type Profile struct {
	ID        int64
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt time.Time
}

// UserIdentity 是用户登录身份的领域模型。
type UserIdentity struct {
	ID              int
	UserID          int64
	Provider        string
	ProviderUID     string
	ProviderUnionID *string
	LoginName       *string
	CredentialHash  *string
	VerifiedAt      *time.Time
	LinkedAt        time.Time
	LastLoginAt     *time.Time
	Meta            map[string]any
}

// CreateUserParams 是创建用户主体的参数。
type CreateUserParams struct{}

// CreateUserIdentityParams 是创建用户身份的参数。
type CreateUserIdentityParams struct {
	UserID          int64
	Provider        string
	ProviderUID     string
	ProviderUnionID *string
	LoginName       *string
	CredentialHash  *string
	VerifiedAt      *time.Time
	Meta            map[string]any
}

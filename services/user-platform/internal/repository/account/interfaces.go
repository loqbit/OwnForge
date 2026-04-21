package accountrepo

import (
	"context"
	"time"
)

// UserRepository 定义了用户主体的持久化接口。
type UserRepository interface {
	Create(ctx context.Context, params CreateUserParams) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetUserVersion(ctx context.Context, id int64) (int64, error)
	BumpUserVersion(ctx context.Context, id int64) (int64, error)
}

// ProfileRepository 定义了用户资料的持久化接口。
type ProfileRepository interface {
	CreateEmpty(ctx context.Context, userID int64) (*Profile, error)
	GetByUserID(ctx context.Context, userID int64) (*Profile, error)
	EnsureByUserID(ctx context.Context, userID int64) (*Profile, error)
	Update(ctx context.Context, userID int64, nickname, avatarURL, bio, birthday string) (*Profile, error)
}

// UserIdentityRepository 定义了用户身份的持久化接口。
type UserIdentityRepository interface {
	Create(ctx context.Context, params CreateUserIdentityParams) (*UserIdentity, error)
	GetByID(ctx context.Context, id int) (*UserIdentity, error)
	GetByProvider(ctx context.Context, provider string, providerUID string) (*UserIdentity, error)
	ListByUserID(ctx context.Context, userID int64) ([]*UserIdentity, error)
	TouchLogin(ctx context.Context, id int, at time.Time) error
	UpdatePasswordCredentialsByUserID(ctx context.Context, userID int64, credentialHash string) error
}

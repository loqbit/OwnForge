package applicationrepo

import (
	"context"
	"time"
)

// UserAppAuthorizationRepository 定义了用户应用授权的持久化接口。
type UserAppAuthorizationRepository interface {
	Ensure(ctx context.Context, params EnsureUserAppAuthorizationParams) (*UserAppAuthorization, error)
	GetByUserAndApp(ctx context.Context, userID int64, appCode string) (*UserAppAuthorization, error)
	ListByUserID(ctx context.Context, userID int64) ([]*UserAppAuthorization, error)
	TouchLogin(ctx context.Context, id int, at time.Time) error
	UpdateStatus(ctx context.Context, id int, status string) error
}

package applicationstore

import (
	"context"
	"time"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/app"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/userappauthorization"
	applicationrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/application"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

type UserAppAuthorizationStore struct {
	client *ent.Client
}

// NewUserAppAuthorizationStore 创建用户应用授权仓储的 Ent 实现。
func NewUserAppAuthorizationStore(client *ent.Client) applicationrepo.UserAppAuthorizationRepository {
	return &UserAppAuthorizationStore{client: client}
}

// Ensure 确保用户对指定应用的授权记录存在，不存在时自动创建。
func (s *UserAppAuthorizationStore) Ensure(ctx context.Context, params applicationrepo.EnsureUserAppAuthorizationParams) (*applicationrepo.UserAppAuthorization, error) {
	existing, err := s.GetByUserAndApp(ctx, params.UserID, params.AppCode)
	if err == nil {
		return existing, nil
	}
	if !sharedrepo.IsNotFoundError(err) {
		return nil, err
	}

	c := shared.EntClientFromCtx(ctx, s.client)
	appNode, err := shared.FindAppByCode(ctx, c, params.AppCode)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := c.UserAppAuthorization.Create().
		SetUserID(params.UserID).
		SetAppID(appNode.ID)
	if params.SourceIdentityID != nil {
		builder.SetSourceIdentityID(*params.SourceIdentityID)
	}
	if params.Scopes != nil {
		builder.SetScopes(params.Scopes)
	}
	if params.ExtProfile != nil {
		builder.SetExtProfile(params.ExtProfile)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		parsedErr := shared.ParseEntError(err)
		if sharedrepo.IsDuplicateKeyError(parsedErr) {
			return s.GetByUserAndApp(ctx, params.UserID, params.AppCode)
		}
		return nil, parsedErr
	}

	return s.getByID(ctx, created.ID)
}

// GetByUserAndApp 按用户和应用编码查询授权记录。
func (s *UserAppAuthorizationStore) GetByUserAndApp(ctx context.Context, userID int64, appCode string) (*applicationrepo.UserAppAuthorization, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.UserAppAuthorization.Query().
		Where(
			userappauthorization.HasUserWith(user.IDEQ(userID)),
			userappauthorization.HasAppWith(app.AppCodeEQ(appCode)),
		).
		WithUser().
		WithApp().
		WithSourceIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUserAppAuthorization(entity), nil
}

// ListByUserID 查询某个用户的全部应用授权记录。
func (s *UserAppAuthorizationStore) ListByUserID(ctx context.Context, userID int64) ([]*applicationrepo.UserAppAuthorization, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entities, err := c.UserAppAuthorization.Query().
		Where(userappauthorization.HasUserWith(user.IDEQ(userID))).
		WithUser().
		WithApp().
		WithSourceIdentity().
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	result := make([]*applicationrepo.UserAppAuthorization, 0, len(entities))
	for _, entity := range entities {
		result = append(result, shared.MapUserAppAuthorization(entity))
	}
	return result, nil
}

// TouchLogin 更新授权记录最近登录时间和活跃时间。
func (s *UserAppAuthorizationStore) TouchLogin(ctx context.Context, id int, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.UserAppAuthorization.UpdateOneID(id).
		SetLastLoginAt(at).
		SetLastActiveAt(at).
		Exec(ctx))
}

// UpdateStatus 更新授权记录的状态。
func (s *UserAppAuthorizationStore) UpdateStatus(ctx context.Context, id int, status string) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.UserAppAuthorization.UpdateOneID(id).
		SetStatus(userappauthorization.Status(status)).
		Exec(ctx))
}

// getByID 按主键查询授权记录。
func (s *UserAppAuthorizationStore) getByID(ctx context.Context, id int) (*applicationrepo.UserAppAuthorization, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.UserAppAuthorization.Query().
		Where(userappauthorization.IDEQ(id)).
		WithUser().
		WithApp().
		WithSourceIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUserAppAuthorization(entity), nil
}

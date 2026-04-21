package accountstore

import (
	"context"
	"time"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/useridentity"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

type UserIdentityStore struct {
	client *ent.Client
}

// NewUserIdentityStore 创建用户身份仓储的 Ent 实现。
func NewUserIdentityStore(client *ent.Client) accountrepo.UserIdentityRepository {
	return &UserIdentityStore{client: client}
}

// Create 创建一条新的用户身份记录。
func (s *UserIdentityStore) Create(ctx context.Context, params accountrepo.CreateUserIdentityParams) (*accountrepo.UserIdentity, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	builder := c.UserIdentity.Create().
		SetUserID(params.UserID).
		SetProvider(useridentity.Provider(params.Provider)).
		SetProviderUID(params.ProviderUID)

	if params.ProviderUnionID != nil {
		builder.SetProviderUnionID(*params.ProviderUnionID)
	}
	if params.LoginName != nil {
		builder.SetLoginName(*params.LoginName)
	}
	if params.CredentialHash != nil {
		builder.SetCredentialHash(*params.CredentialHash)
	}
	if params.VerifiedAt != nil {
		builder.SetVerifiedAt(*params.VerifiedAt)
	}
	if params.Meta != nil {
		builder.SetMeta(params.Meta)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return s.GetByID(ctx, created.ID)
}

// GetByID 按主键查询用户身份。
func (s *UserIdentityStore) GetByID(ctx context.Context, id int) (*accountrepo.UserIdentity, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.UserIdentity.Query().
		Where(useridentity.IDEQ(id)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUserIdentity(entity), nil
}

// GetByProvider 按身份提供方和提供方唯一标识查询用户身份。
func (s *UserIdentityStore) GetByProvider(ctx context.Context, provider string, providerUID string) (*accountrepo.UserIdentity, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.UserIdentity.Query().
		Where(
			useridentity.ProviderEQ(useridentity.Provider(provider)),
			useridentity.ProviderUIDEQ(providerUID),
		).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUserIdentity(entity), nil
}

// ListByUserID 查询某个用户绑定的全部身份。
func (s *UserIdentityStore) ListByUserID(ctx context.Context, userID int64) ([]*accountrepo.UserIdentity, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entities, err := c.UserIdentity.Query().
		Where(useridentity.HasUserWith(user.IDEQ(userID))).
		WithUser().
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	result := make([]*accountrepo.UserIdentity, 0, len(entities))
	for _, entity := range entities {
		result = append(result, shared.MapUserIdentity(entity))
	}
	return result, nil
}

// TouchLogin 更新指定身份最近一次登录时间。
func (s *UserIdentityStore) TouchLogin(ctx context.Context, id int, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.UserIdentity.UpdateOneID(id).
		SetLastLoginAt(at).
		Exec(ctx))
}

// UpdatePasswordCredentialsByUserID 批量更新用户本地密码型身份的密码哈希。
func (s *UserIdentityStore) UpdatePasswordCredentialsByUserID(ctx context.Context, userID int64, credentialHash string) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	_, err := c.UserIdentity.Update().
		Where(
			useridentity.HasUserWith(user.IDEQ(userID)),
			useridentity.ProviderIn(
				useridentity.ProviderPhone,
				useridentity.ProviderEmail,
				useridentity.ProviderUsername,
			),
		).
		SetCredentialHash(credentialHash).
		Save(ctx)
	return shared.ParseEntError(err)
}

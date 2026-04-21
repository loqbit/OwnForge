package accountstore

import (
	"context"
	"fmt"

	"github.com/luckysxx/common/rpc"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

// UserStore 是 UserRepository 的 Ent 实现。
type UserStore struct {
	client *ent.Client
}

// NewUserStore 创建一个 UserRepository 实例，直接持有 ent.Client。
func NewUserStore(client *ent.Client) accountrepo.UserRepository {
	return &UserStore{client: client}
}

// Create 创建一条用户主体记录。
func (s *UserStore) Create(ctx context.Context, params accountrepo.CreateUserParams) (*accountrepo.User, error) {
	newID, err := rpc.GenerateID(ctx)
	if err != nil {
		return nil, fmt.Errorf("生成 Snowflake ID 失败: %w", err)
	}
	_ = params

	c := shared.EntClientFromCtx(ctx, s.client)

	builder := c.User.Create().
		SetID(newID)

	u, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUser(u), nil
}

// GetByID 按主键查询处于激活状态的用户。
func (s *UserStore) GetByID(ctx context.Context, id int64) (*accountrepo.User, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	u, err := c.User.
		Query().
		Where(user.IDEQ(id)).
		Where(user.StatusEQ(user.StatusActive)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapUser(u), nil
}

// GetUserVersion 返回用户当前的全局版本号。
func (s *UserStore) GetUserVersion(ctx context.Context, id int64) (int64, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	return u.UserVersion, nil
}

// BumpUserVersion 将用户的全局版本号递增 1，并返回最新值。
func (s *UserStore) BumpUserVersion(ctx context.Context, id int64) (int64, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	u, err := c.User.
		UpdateOneID(id).
		AddUserVersion(1).
		Save(ctx)
	if err != nil {
		return 0, shared.ParseEntError(err)
	}
	return u.UserVersion, nil
}

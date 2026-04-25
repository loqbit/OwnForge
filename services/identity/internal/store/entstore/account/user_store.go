package accountstore

import (
	"context"
	"fmt"

	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/ent/user"
	platformidgen "github.com/loqbit/ownforge/services/identity/internal/platform/idgen"
	accountrepo "github.com/loqbit/ownforge/services/identity/internal/repository/account"
	"github.com/loqbit/ownforge/services/identity/internal/store/entstore/shared"
)

// UserStore is the Ent-backed implementation of UserRepository.
type UserStore struct {
	client *ent.Client
	idgen  platformidgen.Client
}

// NewUserStore creates a UserRepository backed directly by ent.Client.
func NewUserStore(client *ent.Client, idgenClient platformidgen.Client) accountrepo.UserRepository {
	return &UserStore{client: client, idgen: idgenClient}
}

// Create inserts a user record.
func (s *UserStore) Create(ctx context.Context, params accountrepo.CreateUserParams) (*accountrepo.User, error) {
	newID, err := s.idgen.NextID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Snowflake ID: %w", err)
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

// GetByID looks up an active user by primary key.
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

// GetUserVersion returns the user's current global version.
func (s *UserStore) GetUserVersion(ctx context.Context, id int64) (int64, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	return u.UserVersion, nil
}

// BumpUserVersion increments the user's global version by 1 and returns the new value.
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

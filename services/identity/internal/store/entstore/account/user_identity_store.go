package accountstore

import (
	"context"
	"time"

	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/ent/user"
	"github.com/loqbit/ownforge/services/identity/internal/ent/useridentity"
	accountrepo "github.com/loqbit/ownforge/services/identity/internal/repository/account"
	"github.com/loqbit/ownforge/services/identity/internal/store/entstore/shared"
)

type UserIdentityStore struct {
	client *ent.Client
}

// NewUserIdentityStore creates the Ent-backed user identity repository.
func NewUserIdentityStore(client *ent.Client) accountrepo.UserIdentityRepository {
	return &UserIdentityStore{client: client}
}

// Create inserts a new user identity record.
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

// GetByID looks up a user identity by primary key.
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

// GetByProvider looks up a user identity by provider and provider-specific identifier.
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

// ListByUserID returns all identities linked to a user.
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

// TouchLogin updates the last-login time for the specified identity.
func (s *UserIdentityStore) TouchLogin(ctx context.Context, id int, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.UserIdentity.UpdateOneID(id).
		SetLastLoginAt(at).
		Exec(ctx))
}

// UpdatePasswordCredentialsByUserID updates password hashes for the user's local password identities in bulk.
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

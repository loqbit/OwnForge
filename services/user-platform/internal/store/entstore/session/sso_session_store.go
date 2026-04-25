package sessionstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent/ssosession"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent/user"
	sessionrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/session"
	"github.com/loqbit/ownforge/services/user-platform/internal/store/entstore/shared"
)

type SsoSessionStore struct {
	client *ent.Client
}

// NewSsoSessionStore creates the Ent-backed global SSO session repository.
func NewSsoSessionStore(client *ent.Client) sessionrepo.SsoSessionRepository {
	return &SsoSessionStore{client: client}
}

// Create inserts a new global SSO session record.
func (s *SsoSessionStore) Create(ctx context.Context, params sessionrepo.CreateSsoSessionParams) (*sessionrepo.SsoSession, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	builder := c.SsoSession.Create().
		SetUserID(params.UserID).
		SetSSOTokenHash(params.TokenHash).
		SetUserVersion(params.UserVersion).
		SetExpiresAt(params.ExpiresAt).
		SetNillableIdentityID(params.IdentityID).
		SetNillableDeviceID(params.DeviceID).
		SetNillableUserAgent(params.UserAgent).
		SetNillableIP(params.IP)

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return s.GetByID(ctx, created.ID)
}

// GetByID looks up a global SSO session by primary key.
func (s *SsoSessionStore) GetByID(ctx context.Context, id uuid.UUID) (*sessionrepo.SsoSession, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.SsoSession.Query().
		Where(ssosession.IDEQ(id)).
		WithUser().
		WithIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapSsoSession(entity), nil
}

// GetByTokenHash looks up a global SSO session by token hash.
func (s *SsoSessionStore) GetByTokenHash(ctx context.Context, tokenHash string) (*sessionrepo.SsoSession, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.SsoSession.Query().
		Where(ssosession.SSOTokenHashEQ(tokenHash)).
		WithUser().
		WithIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapSsoSession(entity), nil
}

// ListActiveByUserID returns all currently active global SSO sessions for a user.
func (s *SsoSessionStore) ListActiveByUserID(ctx context.Context, userID int64) ([]*sessionrepo.SsoSession, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entities, err := c.SsoSession.Query().
		Where(
			ssosession.HasUserWith(user.IDEQ(userID)),
			ssosession.StatusEQ(ssosession.StatusActive),
		).
		WithUser().
		WithIdentity().
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	result := make([]*sessionrepo.SsoSession, 0, len(entities))
	for _, entity := range entities {
		result = append(result, shared.MapSsoSession(entity))
	}
	return result, nil
}

// Touch updates the last-active timestamp for a global SSO session.
func (s *SsoSessionStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.SsoSession.UpdateOneID(id).
		SetLastSeenAt(at).
		Exec(ctx))
}

// BumpVersion increments the global SSO session version and returns the latest value.
func (s *SsoSessionStore) BumpVersion(ctx context.Context, id uuid.UUID) (int64, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.SsoSession.UpdateOneID(id).
		AddSSOVersion(1).
		Save(ctx)
	if err != nil {
		return 0, shared.ParseEntError(err)
	}
	return entity.SSOVersion, nil
}

// Revoke revokes the specified global SSO session.
func (s *SsoSessionStore) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.SsoSession.UpdateOneID(id).
		SetStatus(ssosession.StatusRevoked).
		SetRevokedAt(revokedAt).
		Exec(ctx))
}

// RevokeByUserID revokes all active global SSO sessions for a user.
func (s *SsoSessionStore) RevokeByUserID(ctx context.Context, userID int64, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	_, err := c.SsoSession.Update().
		Where(
			ssosession.HasUserWith(user.IDEQ(userID)),
			ssosession.StatusEQ(ssosession.StatusActive),
		).
		SetStatus(ssosession.StatusRevoked).
		SetRevokedAt(revokedAt).
		Save(ctx)
	return shared.ParseEntError(err)
}

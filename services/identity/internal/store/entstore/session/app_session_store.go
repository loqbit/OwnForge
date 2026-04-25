package sessionstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/ent/app"
	"github.com/loqbit/ownforge/services/identity/internal/ent/session"
	"github.com/loqbit/ownforge/services/identity/internal/ent/user"
	sessionrepo "github.com/loqbit/ownforge/services/identity/internal/repository/session"
	sharedrepo "github.com/loqbit/ownforge/services/identity/internal/repository/shared"
	"github.com/loqbit/ownforge/services/identity/internal/store/entstore/shared"
)

type AppSessionStore struct {
	client *ent.Client
}

// NewAppSessionStore creates the Ent-backed application session repository.
func NewAppSessionStore(client *ent.Client) sessionrepo.AppSessionRepository {
	return &AppSessionStore{client: client}
}

// Create inserts a new application session record.
func (s *AppSessionStore) Create(ctx context.Context, params sessionrepo.CreateSessionParams) (*sessionrepo.SessionRecord, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	appNode, err := shared.FindAppByCode(ctx, c, params.AppCode)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := c.Session.Create().
		SetUserID(params.UserID).
		SetAppID(appNode.ID).
		SetSessionTokenHash(params.TokenHash).
		SetUserVersion(params.UserVersion).
		SetExpiresAt(params.ExpiresAt).
		SetNillableSSOSessionID(params.SsoSessionID).
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

// GetByID looks up an application session by primary key.
func (s *AppSessionStore) GetByID(ctx context.Context, id uuid.UUID) (*sessionrepo.SessionRecord, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.Session.Query().
		Where(session.IDEQ(id)).
		WithUser().
		WithApp().
		WithSSOSession().
		WithIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapSession(entity), nil
}

// GetByTokenHash looks up an application session by token hash.
func (s *AppSessionStore) GetByTokenHash(ctx context.Context, tokenHash string) (*sessionrepo.SessionRecord, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entity, err := c.Session.Query().
		Where(session.SessionTokenHashEQ(tokenHash)).
		WithUser().
		WithApp().
		WithSSOSession().
		WithIdentity().
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapSession(entity), nil
}

// ListActiveByUserAndApp returns all active sessions for a user in the specified app.
func (s *AppSessionStore) ListActiveByUserAndApp(ctx context.Context, userID int64, appCode string) ([]*sessionrepo.SessionRecord, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entities, err := c.Session.Query().
		Where(
			session.HasUserWith(user.IDEQ(userID)),
			session.HasAppWith(app.AppCodeEQ(appCode)),
			session.StatusEQ(session.StatusActive),
		).
		WithUser().
		WithApp().
		WithSSOSession().
		WithIdentity().
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	result := make([]*sessionrepo.SessionRecord, 0, len(entities))
	for _, entity := range entities {
		result = append(result, shared.MapSession(entity))
	}
	return result, nil
}

// ListActiveByUserID returns all active application sessions for a user.
func (s *AppSessionStore) ListActiveByUserID(ctx context.Context, userID int64) ([]*sessionrepo.SessionRecord, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	entities, err := c.Session.Query().
		Where(
			session.HasUserWith(user.IDEQ(userID)),
			session.StatusEQ(session.StatusActive),
		).
		WithUser().
		WithApp().
		WithSSOSession().
		WithIdentity().
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	result := make([]*sessionrepo.SessionRecord, 0, len(entities))
	for _, entity := range entities {
		result = append(result, shared.MapSession(entity))
	}
	return result, nil
}

// Touch updates the last-active timestamp for an application session.
func (s *AppSessionStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.Session.UpdateOneID(id).
		SetLastSeenAt(at).
		Exec(ctx))
}

// Rotate rotates the token hash for an application session and increments its version.
func (s *AppSessionStore) Rotate(ctx context.Context, params sessionrepo.RotateSessionParams) (*sessionrepo.SessionRecord, error) {
	current, err := s.GetByID(ctx, params.SessionID)
	if err != nil {
		return nil, err
	}
	if current.Version != params.PreviousVersion {
		return nil, sharedrepo.ErrInvalidOrExpiredToken
	}

	c := shared.EntClientFromCtx(ctx, s.client)
	builder := c.Session.UpdateOneID(params.SessionID).
		SetSessionTokenHash(params.NewTokenHash).
		SetExpiresAt(params.NextExpiresAt).
		AddVersion(1)
	if params.LastSeenAt != nil {
		builder.SetLastSeenAt(*params.LastSeenAt)
	}

	if _, err := builder.Save(ctx); err != nil {
		return nil, shared.ParseEntError(err)
	}
	return s.GetByID(ctx, params.SessionID)
}

// Revoke revokes the specified application session.
func (s *AppSessionStore) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.Session.UpdateOneID(id).
		SetStatus(session.StatusRevoked).
		SetRevokedAt(revokedAt).
		Exec(ctx))
}

// RevokeByUserID revokes all active application sessions for a user.
func (s *AppSessionStore) RevokeByUserID(ctx context.Context, userID int64, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	_, err := c.Session.Update().
		Where(
			session.HasUserWith(user.IDEQ(userID)),
			session.StatusEQ(session.StatusActive),
		).
		SetStatus(session.StatusRevoked).
		SetRevokedAt(revokedAt).
		Save(ctx)
	return shared.ParseEntError(err)
}

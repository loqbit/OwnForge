package sessionstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/app"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/session"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

type AppSessionStore struct {
	client *ent.Client
}

// NewAppSessionStore 创建应用会话仓储的 Ent 实现。
func NewAppSessionStore(client *ent.Client) sessionrepo.AppSessionRepository {
	return &AppSessionStore{client: client}
}

// Create 创建一条新的应用会话记录。
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

// GetByID 按主键查询应用会话。
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

// GetByTokenHash 按令牌哈希查询应用会话。
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

// ListActiveByUserAndApp 查询某个用户在指定应用下的全部活跃会话。
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

// ListActiveByUserID 查询某个用户名下全部活跃的应用会话。
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

// Touch 更新应用会话最近一次活跃时间。
func (s *AppSessionStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.Session.UpdateOneID(id).
		SetLastSeenAt(at).
		Exec(ctx))
}

// Rotate 轮换应用会话的令牌哈希并递增版本号。
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

// Revoke 撤销指定应用会话。
func (s *AppSessionStore) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.Session.UpdateOneID(id).
		SetStatus(session.StatusRevoked).
		SetRevokedAt(revokedAt).
		Exec(ctx))
}

// RevokeByUserID 撤销某个用户名下全部活跃的应用会话。
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

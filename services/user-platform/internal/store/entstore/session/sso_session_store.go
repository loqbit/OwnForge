package sessionstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/ssosession"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

type SsoSessionStore struct {
	client *ent.Client
}

// NewSsoSessionStore 创建全局登录态仓储的 Ent 实现。
func NewSsoSessionStore(client *ent.Client) sessionrepo.SsoSessionRepository {
	return &SsoSessionStore{client: client}
}

// Create 创建一条新的全局登录态记录。
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

// GetByID 按主键查询全局登录态。
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

// GetByTokenHash 按令牌哈希查询全局登录态。
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

// ListActiveByUserID 查询某个用户当前全部活跃的全局登录态。
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

// Touch 更新全局登录态最近一次活跃时间。
func (s *SsoSessionStore) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.SsoSession.UpdateOneID(id).
		SetLastSeenAt(at).
		Exec(ctx))
}

// BumpVersion 递增全局登录态版本号并返回最新值。
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

// Revoke 撤销指定全局登录态。
func (s *SsoSessionStore) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	return shared.ParseEntError(c.SsoSession.UpdateOneID(id).
		SetStatus(ssosession.StatusRevoked).
		SetRevokedAt(revokedAt).
		Exec(ctx))
}

// RevokeByUserID 撤销某个用户名下全部活跃的全局登录态。
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

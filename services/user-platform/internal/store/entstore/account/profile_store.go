package accountstore

import (
	"context"
	"errors"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/profile"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/user"
	platformidgen "github.com/ownforge/ownforge/services/user-platform/internal/platform/idgen"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	"github.com/ownforge/ownforge/services/user-platform/internal/store/entstore/shared"
)

// ProfileStore 是 ProfileRepository 的 Ent 实现。
type ProfileStore struct {
	client *ent.Client
	idgen  platformidgen.Client
}

// NewProfileStore 创建一个 ProfileRepository 实例，直接持有 ent.Client。
func NewProfileStore(client *ent.Client, idgenClient platformidgen.Client) accountrepo.ProfileRepository {
	return &ProfileStore{client: client, idgen: idgenClient}
}

// CreateEmpty 为新用户创建一条空资料记录。
func (s *ProfileStore) CreateEmpty(ctx context.Context, userID int64) (*accountrepo.Profile, error) {
	newID, err := s.idgen.NextID(ctx)
	if err != nil {
		return nil, err
	}

	c := shared.EntClientFromCtx(ctx, s.client)

	p, err := c.Profile.Create().
		SetID(newID).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapProfile(p), nil
}

// GetByUserID 按用户 ID 查询对应的资料记录。
func (s *ProfileStore) GetByUserID(ctx context.Context, userID int64) (*accountrepo.Profile, error) {
	c := shared.EntClientFromCtx(ctx, s.client)

	p, err := c.Profile.Query().
		Where(profile.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapProfile(p), nil
}

// EnsureByUserID 确保指定用户一定拥有一条资料记录。
func (s *ProfileStore) EnsureByUserID(ctx context.Context, userID int64) (*accountrepo.Profile, error) {
	p, err := s.GetByUserID(ctx, userID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, sharedrepo.ErrNoRows) {
		return nil, err
	}

	p, err = s.CreateEmpty(ctx, userID)
	if err == nil {
		return p, nil
	}

	// 如果并发请求已经补齐了资料，再查一次即可。
	if sharedrepo.IsDuplicateKeyError(err) {
		return s.GetByUserID(ctx, userID)
	}
	return nil, err
}

// Update 按用户 ID 更新一条资料记录。
func (s *ProfileStore) Update(ctx context.Context, userID int64, nickname, avatarURL, bio, birthday string) (*accountrepo.Profile, error) {
	// 先根据 userID 查出 profile 实体
	p, err := s.EnsureByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 使用 profile ID 直接更新
	c := shared.EntClientFromCtx(ctx, s.client)

	updated, err := c.Profile.UpdateOneID(p.ID).
		SetNickname(nickname).
		SetAvatarURL(avatarURL).
		SetBio(bio).
		SetBirthday(birthday).
		Save(ctx)

	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return shared.MapProfile(updated), nil
}

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

// ProfileStore is the Ent-backed implementation of ProfileRepository.
type ProfileStore struct {
	client *ent.Client
	idgen  platformidgen.Client
}

// NewProfileStore creates a ProfileRepository backed directly by ent.Client.
func NewProfileStore(client *ent.Client, idgenClient platformidgen.Client) accountrepo.ProfileRepository {
	return &ProfileStore{client: client, idgen: idgenClient}
}

// CreateEmpty creates an empty profile record for a new user.
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

// GetByUserID looks up the profile record for a user ID.
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

// EnsureByUserID makes sure the specified user has a profile record.
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

	// If a concurrent request has already created the profile, just query it again.
	if sharedrepo.IsDuplicateKeyError(err) {
		return s.GetByUserID(ctx, userID)
	}
	return nil, err
}

// Update updates a profile record by user ID.
func (s *ProfileStore) Update(ctx context.Context, userID int64, nickname, avatarURL, bio, birthday string) (*accountrepo.Profile, error) {
	// Look up the profile entity by userID first.
	p, err := s.EnsureByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Update directly by profile ID.
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

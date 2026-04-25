package accountservice

import (
	"context"

	accountrepo "github.com/loqbit/ownforge/services/identity/internal/repository/account"
	"go.uber.org/zap"
)

// ProfileService defines the domain-service interface for user profile data.
type ProfileService interface {
	GetProfile(ctx context.Context, query *GetProfileQuery) (*GetProfileResult, error)
	UpdateProfile(ctx context.Context, cmd *UpdateProfileCommand) (*UpdateProfileResult, error)
}

// profileService is the default implementation of ProfileService.
type profileService struct {
	profileRepo accountrepo.ProfileRepository
	logger      *zap.Logger
}

// ProfileDependencies groups the dependencies required by the profile service.
type ProfileDependencies struct {
	ProfileRepo accountrepo.ProfileRepository
	Logger      *zap.Logger
}

// NewProfileService creates a profile service instance.
func NewProfileService(deps ProfileDependencies) ProfileService {
	return &profileService{
		profileRepo: deps.ProfileRepo,
		logger:      deps.Logger,
	}
}

// GetProfile returns profile data for the specified user.
func (s *profileService) GetProfile(ctx context.Context, query *GetProfileQuery) (*GetProfileResult, error) {
	p, err := s.profileRepo.EnsureByUserID(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	return &GetProfileResult{
		UserID:    query.UserID,
		Nickname:  p.Nickname,
		AvatarURL: p.AvatarURL,
		Bio:       p.Bio,
		Birthday:  p.Birthday,
		UpdatedAt: p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateProfile updates profile data for the specified user.
func (s *profileService) UpdateProfile(ctx context.Context, cmd *UpdateProfileCommand) (*UpdateProfileResult, error) {
	updated, err := s.profileRepo.Update(ctx, cmd.UserID, cmd.Nickname, cmd.AvatarURL, cmd.Bio, cmd.Birthday)
	if err != nil {
		return nil, err
	}

	return &UpdateProfileResult{
		UserID:    cmd.UserID,
		Nickname:  updated.Nickname,
		AvatarURL: updated.AvatarURL,
		Bio:       updated.Bio,
		Birthday:  updated.Birthday,
		UpdatedAt: updated.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

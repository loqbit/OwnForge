package accountservice

import (
	"context"

	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	"go.uber.org/zap"
)

// ProfileService 定义了用户资料领域服务的接口。
type ProfileService interface {
	GetProfile(ctx context.Context, query *GetProfileQuery) (*GetProfileResult, error)
	UpdateProfile(ctx context.Context, cmd *UpdateProfileCommand) (*UpdateProfileResult, error)
}

// profileService 是 ProfileService 的默认实现。
type profileService struct {
	profileRepo accountrepo.ProfileRepository
	logger      *zap.Logger
}

// ProfileDependencies 描述资料服务所需的依赖集合。
type ProfileDependencies struct {
	ProfileRepo accountrepo.ProfileRepository
	Logger      *zap.Logger
}

// NewProfileService 创建一个资料服务实例。
func NewProfileService(deps ProfileDependencies) ProfileService {
	return &profileService{
		profileRepo: deps.ProfileRepo,
		logger:      deps.Logger,
	}
}

// GetProfile 查询指定用户的资料信息。
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

// UpdateProfile 更新指定用户的资料信息。
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

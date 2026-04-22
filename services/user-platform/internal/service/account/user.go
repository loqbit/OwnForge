package accountservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ownforge/ownforge/pkg/crypto"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	infrarepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/infra"
	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"

	"go.uber.org/zap"
)

type UserService interface {
	Register(ctx context.Context, req *RegisterCommand) (*RegisterResult, error)
	ChangePassword(ctx context.Context, req *ChangePasswordCommand) (*ChangePasswordResult, error)
	LogoutAllSessions(ctx context.Context, req *LogoutAllSessionsCommand) (*LogoutAllSessionsResult, error)
	BindEmail(ctx context.Context, req *BindEmailCommand) (*BindEmailResult, error)
	SetPassword(ctx context.Context, req *SetPasswordCommand) (*SetPasswordResult, error)
	// TODO: 后续补充账号管理接口：身份列表、全局登录态列表、应用会话列表、单设备下线等查询与管理能力。
}

type userService struct {
	tm                  infrarepo.TransactionManager
	userRepo            accountrepo.UserRepository
	identityRepo        accountrepo.UserIdentityRepository
	profileRepo         accountrepo.ProfileRepository
	ssoSessionRepo      sessionrepo.SsoSessionRepository
	appSessionRepo      sessionrepo.AppSessionRepository
	outbox              infrarepo.EventOutboxWriter
	logger              *zap.Logger
	topicUserRegistered string
}

// UserDependencies 描述用户服务所需的依赖集合。
type UserDependencies struct {
	TM                  infrarepo.TransactionManager
	UserRepo            accountrepo.UserRepository
	IdentityRepo        accountrepo.UserIdentityRepository
	ProfileRepo         accountrepo.ProfileRepository
	SSOSessionRepo      sessionrepo.SsoSessionRepository
	AppSessionRepo      sessionrepo.AppSessionRepository
	Outbox              infrarepo.EventOutboxWriter
	Logger              *zap.Logger
	TopicUserRegistered string
}

// NewUserService 创建用户服务。
func NewUserService(deps UserDependencies) UserService {
	return &userService{
		tm:                  deps.TM,
		userRepo:            deps.UserRepo,
		identityRepo:        deps.IdentityRepo,
		profileRepo:         deps.ProfileRepo,
		ssoSessionRepo:      deps.SSOSessionRepo,
		appSessionRepo:      deps.AppSessionRepo,
		outbox:              deps.Outbox,
		logger:              deps.Logger,
		topicUserRegistered: deps.TopicUserRegistered,
	}
}

func (s *userService) Register(ctx context.Context, req *RegisterCommand) (*RegisterResult, error) {
	// 加密密码
	hashedPwd, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	user, err := RegisterUserWithProfile(ctx, RegistrationDeps{
		TM:                  s.tm,
		UserRepo:            s.userRepo,
		IdentityRepo:        s.identityRepo,
		ProfileRepo:         s.profileRepo,
		Outbox:              s.outbox,
		TopicUserRegistered: s.topicUserRegistered,
	}, RegistrationParams{
		Phone:        req.Phone,
		Email:        optionalTrimmedString(req.Email),
		Username:     optionalTrimmedString(req.Username),
		PasswordHash: &hashedPwd,
	})
	if err != nil {
		return nil, err
	}

	return &RegisterResult{
		Phone:    req.Phone,
		Email:    req.Email,
		UserID:   user.ID,
		Username: firstNonEmpty(strings.TrimSpace(req.Username), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Email)),
	}, nil
}

// ChangePassword 修改当前用户密码，并使历史登录态全部失效。
func (s *userService) ChangePassword(ctx context.Context, req *ChangePasswordCommand) (*ChangePasswordResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("用户不存在")
	}
	if strings.TrimSpace(req.OldPassword) == "" {
		return nil, errors.New("旧密码不能为空")
	}
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		return nil, errors.New("新密码长度不能少于 8 位")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("查询用户身份失败: %w", err)
	}

	oldHash := ""
	for _, identity := range identities {
		if identity == nil || identity.CredentialHash == nil {
			continue
		}
		hash := strings.TrimSpace(*identity.CredentialHash)
		if hash != "" {
			oldHash = hash
			break
		}
	}
	if oldHash == "" {
		return nil, errors.New("当前账号未设置密码")
	}
	if !crypto.CheckPasswordHash(req.OldPassword, oldHash) {
		return nil, errors.New("旧密码不正确")
	}

	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("新密码加密失败: %w", err)
	}

	if err := s.tm.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.identityRepo.UpdatePasswordCredentialsByUserID(txCtx, req.UserID, newHash); err != nil {
			return fmt.Errorf("更新密码失败: %w", err)
		}
		if err := s.revokeAllSessions(txCtx, req.UserID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &ChangePasswordResult{
		UserID:  req.UserID,
		Message: "密码修改成功，请重新登录",
	}, nil
}

// LogoutAllSessions 让当前用户的全部登录态立即失效。
func (s *userService) LogoutAllSessions(ctx context.Context, req *LogoutAllSessionsCommand) (*LogoutAllSessionsResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("用户不存在")
	}

	if err := s.tm.WithTx(ctx, func(txCtx context.Context) error {
		return s.revokeAllSessions(txCtx, req.UserID)
	}); err != nil {
		return nil, err
	}

	return &LogoutAllSessionsResult{
		UserID:  req.UserID,
		Message: "已退出全部设备，请重新登录",
	}, nil
}

// BindEmail 为当前用户绑定邮箱身份。
func (s *userService) BindEmail(ctx context.Context, req *BindEmailCommand) (*BindEmailResult, error) {
	// TODO: 当前先走停服演进期的直接绑定；后续补邮箱验证码校验与所有权验证流程。
	if req.UserID == 0 {
		return nil, errors.New("用户不存在")
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, errors.New("邮箱不能为空")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("查询用户身份失败: %w", err)
	}
	for _, identity := range identities {
		if identity == nil || identity.Provider != "email" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(identity.ProviderUID), email) {
			return &BindEmailResult{
				UserID:  req.UserID,
				Email:   email,
				Message: "邮箱已绑定",
			}, nil
		}
		return nil, errors.New("当前账号已绑定其他邮箱")
	}

	now := time.Now()
	loginName := email
	if _, err := s.identityRepo.Create(ctx, accountrepo.CreateUserIdentityParams{
		UserID:      req.UserID,
		Provider:    "email",
		ProviderUID: email,
		LoginName:   &loginName,
		VerifiedAt:  &now,
	}); err != nil {
		return nil, fmt.Errorf("绑定邮箱失败: %w", err)
	}

	return &BindEmailResult{
		UserID:  req.UserID,
		Email:   email,
		Message: "邮箱绑定成功",
	}, nil
}

// SetPassword 为当前用户首次设置本地密码。
func (s *userService) SetPassword(ctx context.Context, req *SetPasswordCommand) (*SetPasswordResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("用户不存在")
	}
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		return nil, errors.New("新密码长度不能少于 8 位")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("查询用户身份失败: %w", err)
	}

	hasLocalIdentity := false
	for _, identity := range identities {
		if identity == nil {
			continue
		}
		switch identity.Provider {
		case "phone", "email", "username":
			hasLocalIdentity = true
			if identity.CredentialHash != nil && strings.TrimSpace(*identity.CredentialHash) != "" {
				return nil, errors.New("当前账号已设置密码，请使用修改密码")
			}
		}
	}
	if !hasLocalIdentity {
		return nil, errors.New("当前账号不支持设置本地密码")
	}

	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("新密码加密失败: %w", err)
	}

	if err := s.identityRepo.UpdatePasswordCredentialsByUserID(ctx, req.UserID, newHash); err != nil {
		return nil, fmt.Errorf("设置密码失败: %w", err)
	}

	return &SetPasswordResult{
		UserID:  req.UserID,
		Message: "密码设置成功",
	}, nil
}

// revokeAllSessions 提升用户全局版本并撤销该用户全部登录态。
func (s *userService) revokeAllSessions(ctx context.Context, userID int64) error {
	if _, err := s.userRepo.BumpUserVersion(ctx, userID); err != nil {
		return fmt.Errorf("更新用户全局版本失败: %w", err)
	}
	revokedAt := time.Now()
	if err := s.ssoSessionRepo.RevokeByUserID(ctx, userID, revokedAt); err != nil {
		return fmt.Errorf("撤销全局登录态失败: %w", err)
	}
	if err := s.appSessionRepo.RevokeByUserID(ctx, userID, revokedAt); err != nil {
		return fmt.Errorf("撤销应用会话失败: %w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

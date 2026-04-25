package accountservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/loqbit/ownforge/pkg/crypto"
	accountrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/account"
	infrarepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/infra"
	sessionrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/session"

	"go.uber.org/zap"
)

type UserService interface {
	Register(ctx context.Context, req *RegisterCommand) (*RegisterResult, error)
	ChangePassword(ctx context.Context, req *ChangePasswordCommand) (*ChangePasswordResult, error)
	LogoutAllSessions(ctx context.Context, req *LogoutAllSessionsCommand) (*LogoutAllSessionsResult, error)
	BindEmail(ctx context.Context, req *BindEmailCommand) (*BindEmailResult, error)
	SetPassword(ctx context.Context, req *SetPasswordCommand) (*SetPasswordResult, error)
	// TODO: Add account-management APIs later, including identity lists, global SSO session lists, app session lists, and per-device sign-out management.
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

// UserDependencies groups the dependencies required by the user service.
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

// NewUserService creates the user service.
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
	// Hash the password.
	hashedPwd, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
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

// ChangePassword updates the current user's password and invalidates all previous sessions.
func (s *userService) ChangePassword(ctx context.Context, req *ChangePasswordCommand) (*ChangePasswordResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("user not found")
	}
	if strings.TrimSpace(req.OldPassword) == "" {
		return nil, errors.New("old password cannot be empty")
	}
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		return nil, errors.New("new password must be at least 8 characters long")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user identity: %w", err)
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
		return nil, errors.New("the current account has no password set")
	}
	if !crypto.CheckPasswordHash(req.OldPassword, oldHash) {
		return nil, errors.New("incorrect old password")
	}

	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.tm.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.identityRepo.UpdatePasswordCredentialsByUserID(txCtx, req.UserID, newHash); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
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
		Message: "password changed successfully, please sign in again",
	}, nil
}

// LogoutAllSessions immediately invalidates every active session for the current user.
func (s *userService) LogoutAllSessions(ctx context.Context, req *LogoutAllSessionsCommand) (*LogoutAllSessionsResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("user not found")
	}

	if err := s.tm.WithTx(ctx, func(txCtx context.Context) error {
		return s.revokeAllSessions(txCtx, req.UserID)
	}); err != nil {
		return nil, err
	}

	return &LogoutAllSessionsResult{
		UserID:  req.UserID,
		Message: "all devices have been logged out, please sign in again",
	}, nil
}

// BindEmail links an email identity to the current user.
func (s *userService) BindEmail(ctx context.Context, req *BindEmailCommand) (*BindEmailResult, error) {
	// TODO: Use direct binding during the transition period for now; add email-code verification and ownership checks later.
	if req.UserID == 0 {
		return nil, errors.New("user not found")
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user identity: %w", err)
	}
	for _, identity := range identities {
		if identity == nil || identity.Provider != "email" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(identity.ProviderUID), email) {
			return &BindEmailResult{
				UserID:  req.UserID,
				Email:   email,
				Message: "email already bound",
			}, nil
		}
		return nil, errors.New("the current account is already bound to another email")
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
		return nil, fmt.Errorf("failed to bind email: %w", err)
	}

	return &BindEmailResult{
		UserID:  req.UserID,
		Email:   email,
		Message: "email bound successfully",
	}, nil
}

// SetPassword lets the current user set a local password for the first time.
func (s *userService) SetPassword(ctx context.Context, req *SetPasswordCommand) (*SetPasswordResult, error) {
	if req.UserID == 0 {
		return nil, errors.New("user not found")
	}
	if len(strings.TrimSpace(req.NewPassword)) < 8 {
		return nil, errors.New("new password must be at least 8 characters long")
	}

	identities, err := s.identityRepo.ListByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user identity: %w", err)
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
				return nil, errors.New("the current account already has a password, use change password instead")
			}
		}
	}
	if !hasLocalIdentity {
		return nil, errors.New("the current account does not support setting a local password")
	}

	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.identityRepo.UpdatePasswordCredentialsByUserID(ctx, req.UserID, newHash); err != nil {
		return nil, fmt.Errorf("failed to set password: %w", err)
	}

	return &SetPasswordResult{
		UserID:  req.UserID,
		Message: "password set successfully",
	}, nil
}

// revokeAllSessions bumps the user's global version and revokes all sessions for that user.
func (s *userService) revokeAllSessions(ctx context.Context, userID int64) error {
	if _, err := s.userRepo.BumpUserVersion(ctx, userID); err != nil {
		return fmt.Errorf("failed to update user global version: %w", err)
	}
	revokedAt := time.Now()
	if err := s.ssoSessionRepo.RevokeByUserID(ctx, userID, revokedAt); err != nil {
		return fmt.Errorf("failed to revoke global login state: %w", err)
	}
	if err := s.appSessionRepo.RevokeByUserID(ctx, userID, revokedAt); err != nil {
		return fmt.Errorf("failed to revoke app sessions: %w", err)
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

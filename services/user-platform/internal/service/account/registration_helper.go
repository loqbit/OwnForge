package accountservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mqevents "github.com/ownforge/ownforge/pkg/mq/events"
	mqtopics "github.com/ownforge/ownforge/pkg/mq/topics"
	"github.com/ownforge/ownforge/pkg/trace"
	accountrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/account"
	infrarepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/infra"
)

// RegistrationDeps groups dependencies used during registration.
type RegistrationDeps struct {
	TM                  infrarepo.TransactionManager
	UserRepo            accountrepo.UserRepository
	IdentityRepo        accountrepo.UserIdentityRepository
	ProfileRepo         accountrepo.ProfileRepository
	Outbox              infrarepo.EventOutboxWriter
	TopicUserRegistered string
}

// RegistrationParams contains registration parameters.
type RegistrationParams struct {
	Phone        string
	Email        *string
	Username     *string
	PasswordHash *string
}

// RegisterUserWithProfile registers a user and creates the user profile.
func RegisterUserWithProfile(ctx context.Context, deps RegistrationDeps, params RegistrationParams) (*accountrepo.User, error) {
	var created *accountrepo.User

	err := deps.TM.WithTx(ctx, func(txCtx context.Context) error {
		user, err := deps.UserRepo.Create(txCtx, accountrepo.CreateUserParams{})
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		if _, err := deps.ProfileRepo.CreateEmpty(txCtx, user.ID); err != nil {
			return fmt.Errorf("failed to create user profile: %w", err)
		}

		if err := createLocalIdentities(txCtx, deps.IdentityRepo, user.ID, params); err != nil {
			return fmt.Errorf("failed to create user identity: %w", err)
		}

		outboxHeaders := map[string]string{}
		if traceID := trace.FromContext(txCtx); traceID != "" {
			outboxHeaders[trace.HeaderTraceID] = traceID
		}

		payload, err := json.Marshal(mqevents.UserRegistered{
			Version:   mqevents.UserRegisteredVersion,
			EventType: mqtopics.UserRegistered,
			UserID:    user.ID,
			Email:     DerefString(params.Email),
			Username:  normalizedUsernameFromRegistrationParams(user.ID, params),
			Timestamp: time.Now().Unix(),
		})
		if err != nil {
			return fmt.Errorf("failed to marshal user registration event: %w", err)
		}

		var headerBytes json.RawMessage
		if len(outboxHeaders) > 0 {
			headerBytes, err = json.Marshal(outboxHeaders)
			if err != nil {
				return fmt.Errorf("failed to marshal event headers: %w", err)
			}
		}

		record := &infrarepo.OutboxRecord{
			ID:            uuid.NewString(),
			AggregateType: "user",
			AggregateID:   fmt.Sprintf("%d", user.ID),
			EventType:     deps.TopicUserRegistered,
			Payload:       payload,
			Headers:       headerBytes,
			CreatedAt:     time.Now(),
		}

		if err := deps.Outbox.Append(txCtx, record); err != nil {
			return fmt.Errorf("failed to create user event: %w", err)
		}

		created = user
		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func optionalTrimmedString(s string) *string {
	value := strings.TrimSpace(s)
	if value == "" {
		return nil
	}
	return &value
}

// IdentityView describes displayable login information aggregated from identity records.
type IdentityView struct {
	Username string
	Email    string
	Phone    string
}

// DerefString dereferences a nullable string pointer and falls back to an empty string.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// BuildIdentityView extracts username, email, and phone information from a user's identities.
func BuildIdentityView(userID int64, identities []*accountrepo.UserIdentity) IdentityView {
	view := IdentityView{}
	for _, identity := range identities {
		if identity == nil {
			continue
		}

		value := strings.TrimSpace(identity.ProviderUID)
		switch identity.Provider {
		case "username":
			if view.Username == "" {
				view.Username = value
			}
		case "email":
			if view.Email == "" {
				view.Email = value
			}
		case "phone":
			if view.Phone == "" {
				view.Phone = value
			}
		}
	}

	if view.Username == "" {
		switch {
		case view.Phone != "":
			view.Username = view.Phone
		case view.Email != "":
			view.Username = view.Email
		default:
			view.Username = "user-" + strconv.FormatInt(userID, 10)
		}
	}
	return view
}

func normalizedUsernameFromRegistrationParams(userID int64, params RegistrationParams) string {
	if params.Username != nil && strings.TrimSpace(*params.Username) != "" {
		return strings.TrimSpace(*params.Username)
	}
	if phone := strings.TrimSpace(params.Phone); phone != "" {
		return phone
	}
	if params.Email != nil && strings.TrimSpace(*params.Email) != "" {
		return strings.TrimSpace(*params.Email)
	}
	return "user-" + strconv.FormatInt(userID, 10)
}

// createLocalIdentities creates the local login identities needed for a locally registered user.
func createLocalIdentities(ctx context.Context, repo accountrepo.UserIdentityRepository, userID int64, params RegistrationParams) error {
	if repo == nil {
		return nil
	}

	phone := strings.TrimSpace(params.Phone)
	if phone != "" {
		loginName := phone
		if _, err := repo.Create(ctx, accountrepo.CreateUserIdentityParams{
			UserID:         userID,
			Provider:       "phone",
			ProviderUID:    phone,
			LoginName:      &loginName,
			CredentialHash: params.PasswordHash,
		}); err != nil {
			return err
		}
	}

	if params.Email != nil && strings.TrimSpace(*params.Email) != "" {
		email := strings.TrimSpace(*params.Email)
		if _, err := repo.Create(ctx, accountrepo.CreateUserIdentityParams{
			UserID:         userID,
			Provider:       "email",
			ProviderUID:    email,
			LoginName:      &email,
			CredentialHash: params.PasswordHash,
		}); err != nil {
			return err
		}
	}

	if params.Username != nil && strings.TrimSpace(*params.Username) != "" {
		username := strings.TrimSpace(*params.Username)
		if _, err := repo.Create(ctx, accountrepo.CreateUserIdentityParams{
			UserID:         userID,
			Provider:       "username",
			ProviderUID:    username,
			LoginName:      &username,
			CredentialHash: params.PasswordHash,
		}); err != nil {
			return err
		}
	}

	return nil
}

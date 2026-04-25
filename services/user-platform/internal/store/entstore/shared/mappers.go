package shared

import (
	"github.com/google/uuid"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	accountrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/account"
	applicationrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/application"
	sessionrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/session"
)

// MapUser maps an Ent User into the repository-layer user model.
func MapUser(user *ent.User) *accountrepo.User {
	if user == nil {
		return nil
	}
	return &accountrepo.User{
		ID:          user.ID,
		Status:      user.Status.String(),
		UserVersion: user.UserVersion,
	}
}

// MapProfile maps an Ent Profile into the repository-layer profile model.
func MapProfile(profile *ent.Profile) *accountrepo.Profile {
	if profile == nil {
		return nil
	}
	return &accountrepo.Profile{
		ID:        profile.ID,
		Nickname:  profile.Nickname,
		AvatarURL: profile.AvatarURL,
		Bio:       profile.Bio,
		Birthday:  profile.Birthday,
		UpdatedAt: profile.UpdatedAt,
	}
}

// MapUserIdentity maps an Ent UserIdentity into the repository-layer identity model.
func MapUserIdentity(identity *ent.UserIdentity) *accountrepo.UserIdentity {
	if identity == nil {
		return nil
	}

	var userID int64
	if identity.Edges.User != nil {
		userID = identity.Edges.User.ID
	}

	credentialHash := optionalString(identity.CredentialHash)
	return &accountrepo.UserIdentity{
		ID:              identity.ID,
		UserID:          userID,
		Provider:        identity.Provider.String(),
		ProviderUID:     identity.ProviderUID,
		ProviderUnionID: identity.ProviderUnionID,
		LoginName:       identity.LoginName,
		CredentialHash:  credentialHash,
		VerifiedAt:      identity.VerifiedAt,
		LinkedAt:        identity.LinkedAt,
		LastLoginAt:     identity.LastLoginAt,
		Meta:            identity.Meta,
	}
}

// MapUserAppAuthorization maps an Ent UserAppAuthorization into the repository-layer authorization model.
func MapUserAppAuthorization(authz *ent.UserAppAuthorization) *applicationrepo.UserAppAuthorization {
	if authz == nil {
		return nil
	}

	var appID int
	if authz.Edges.App != nil {
		appID = authz.Edges.App.ID
	}

	var userID int64
	if authz.Edges.User != nil {
		userID = authz.Edges.User.ID
	}

	var sourceIdentityID *int
	if authz.Edges.SourceIdentity != nil {
		sourceIdentityID = &authz.Edges.SourceIdentity.ID
	}

	return &applicationrepo.UserAppAuthorization{
		ID:                authz.ID,
		UserID:            userID,
		AppID:             appID,
		SourceIdentityID:  sourceIdentityID,
		Status:            authz.Status.String(),
		Scopes:            authz.Scopes,
		ExtProfile:        authz.ExtProfile,
		FirstAuthorizedAt: authz.FirstAuthorizedAt,
		LastLoginAt:       authz.LastLoginAt,
		LastActiveAt:      authz.LastActiveAt,
	}
}

// MapSsoSession maps an Ent SsoSession into the repository-layer global SSO session model.
func MapSsoSession(session *ent.SsoSession) *sessionrepo.SsoSession {
	if session == nil {
		return nil
	}

	var userID int64
	if session.Edges.User != nil {
		userID = session.Edges.User.ID
	}

	var identityID *int
	if session.Edges.Identity != nil {
		identityID = &session.Edges.Identity.ID
	}

	return &sessionrepo.SsoSession{
		ID:          session.ID,
		UserID:      userID,
		IdentityID:  identityID,
		TokenHash:   session.SSOTokenHash,
		DeviceID:    session.DeviceID,
		UserAgent:   session.UserAgent,
		IP:          session.IP,
		Status:      session.Status.String(),
		Version:     session.SSOVersion,
		UserVersion: session.UserVersion,
		ExpiresAt:   session.ExpiresAt,
		LastSeenAt:  session.LastSeenAt,
		RevokedAt:   session.RevokedAt,
	}
}

// MapSession maps an Ent Session into the repository-layer app session model.
func MapSession(session *ent.Session) *sessionrepo.SessionRecord {
	if session == nil {
		return nil
	}

	var userID int64
	if session.Edges.User != nil {
		userID = session.Edges.User.ID
	}

	var appID int
	var appCode string
	if session.Edges.App != nil {
		appID = session.Edges.App.ID
		appCode = session.Edges.App.AppCode
	}

	var ssoSessionID *uuid.UUID
	if session.Edges.SSOSession != nil {
		ssoSessionID = &session.Edges.SSOSession.ID
	}

	var identityID *int
	if session.Edges.Identity != nil {
		identityID = &session.Edges.Identity.ID
	}

	return &sessionrepo.SessionRecord{
		ID:           session.ID,
		UserID:       userID,
		AppID:        appID,
		AppCode:      appCode,
		SsoSessionID: ssoSessionID,
		IdentityID:   identityID,
		TokenHash:    session.SessionTokenHash,
		DeviceID:     session.DeviceID,
		UserAgent:    session.UserAgent,
		IP:           session.IP,
		Status:       session.Status.String(),
		Version:      session.Version,
		UserVersion:  session.UserVersion,
		ExpiresAt:    session.ExpiresAt,
		LastSeenAt:   session.LastSeenAt,
		RevokedAt:    session.RevokedAt,
	}
}

// optionalString normalizes empty strings to nil so the repository layer can express an unset value.
func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

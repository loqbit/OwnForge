package accountrepo

import "time"

// User is the domain model for the user aggregate.
type User struct {
	ID          int64
	Status      string
	UserVersion int64
}

// Profile is the domain model for user profile data.
type Profile struct {
	ID        int64
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt time.Time
}

// UserIdentity is the domain model for a user login identity.
type UserIdentity struct {
	ID              int
	UserID          int64
	Provider        string
	ProviderUID     string
	ProviderUnionID *string
	LoginName       *string
	CredentialHash  *string
	VerifiedAt      *time.Time
	LinkedAt        time.Time
	LastLoginAt     *time.Time
	Meta            map[string]any
}

// CreateUserParams contains parameters for creating a user aggregate.
type CreateUserParams struct{}

// CreateUserIdentityParams contains parameters for creating a user identity.
type CreateUserIdentityParams struct {
	UserID          int64
	Provider        string
	ProviderUID     string
	ProviderUnionID *string
	LoginName       *string
	CredentialHash  *string
	VerifiedAt      *time.Time
	Meta            map[string]any
}

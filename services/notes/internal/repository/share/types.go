package sharerepo

import "time"

// Share is the repository-layer domain model for share.
type Share struct {
	ID           int64
	Token        string
	Kind         string
	SnippetID    int64
	OwnerID      int64
	PasswordHash string
	ExpiresAt    *time.Time
	ViewCount    int
	ForkCount    int
	CreatedAt    time.Time
}

package sharerepo

import "time"

// Share 是 share 的仓储层领域模型。
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

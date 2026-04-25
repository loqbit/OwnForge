package contract

import snippetcontract "github.com/loqbit/ownforge/services/notes/internal/service/snippet/contract"

// CreateShareCommand is the input for creating a share.
type CreateShareCommand struct {
	SnippetID int64
	Kind      string
	Password  string
	ExpiresAt string
}

// ListSharesQuery contains filters for listing shares.
type ListSharesQuery struct {
	Kind string
}

// ShareResult is the share shape returned by the service layer.
type ShareResult struct {
	ID          int64
	Token       string
	Kind        string
	SnippetID   int64
	OwnerID     int64
	HasPassword bool
	ExpiresAt   string
	ViewCount   int
	ForkCount   int
	CreatedAt   string
}

// PublicShareResult is returned for public share access.
type PublicShareResult struct {
	Share   *ShareResult
	Snippet *snippetcontract.SnippetResult
}

// ShareSource carries source information when importing a copy.
type ShareSource struct {
	Share   *ShareResult
	Snippet *snippetcontract.SnippetResult
}

package snippetrepo

import "time"

// Snippet is the repository-layer domain model for snippet.
type Snippet struct {
	ID         int64
	OwnerID    int64
	Type       string
	Title      string
	Content    string
	FileURL    string
	FileSize   int64
	MimeType   string
	Language   string
	GroupID    *int64
	SortOrder  int     // Sort weight within the owning group.
	TagIDs     []int64 // List of associated tag IDs.
	IsFavorite bool
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

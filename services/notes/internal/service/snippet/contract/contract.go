package contract

// CreateSnippetCommand is the service-layer input for creating a snippet.
type CreateSnippetCommand struct {
	Type     string // code / note / file
	Title    string
	Content  string // used for code/note
	FileURL  string // used for file
	FileSize int64  // used for file
	MimeType string // used for file
	Language string
	GroupID  *int64 // optional group ID
}

// UpdateSnippetCommand is the service-layer input for updating a snippet.
type UpdateSnippetCommand struct {
	Title    string
	Content  string
	Language string
	GroupID  *int64
}

// MoveSnippetCommand is the service-layer input for moving a snippet.
// It is used to move a snippet between groups or adjust its order within the same group.
type MoveSnippetCommand struct {
	// GroupID: target group ID. nil means move it to the ungrouped inbox.
	GroupID *int64
	// SortOrder: target position. nil means append to the end of the target group (max+1).
	SortOrder *int
}

// SnippetResult is the snippet shape returned by the service layer.
type SnippetResult struct {
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
	SortOrder  int
	TagIDs     []int64 // list of associated tag IDs
	IsFavorite bool
	DeletedAt  string // empty string means not deleted
	CreatedAt  string
	UpdatedAt  string
}

// Notes 4 + 5: compound filters and cursor pagination

// ListQuery contains list query parameters supporting multi-condition filtering and cursor pagination.
// All fields are optional; omitted fields mean no filtering.
type ListQuery struct {
	// Filters
	GroupID       *int64 // filter by group
	TagID         *int64 // filter by tag (via bridge-table JOIN)
	Type          string // filter by type: code / note / file
	Keyword       string // fuzzy-search title
	Status        string // "" (defaults to active), "active", "trashed"
	OnlyFavorites bool   // only return favorited documents

	// Sorting
	SortBy string // "updated_at" | "created_at" | "title" | "manual", defaults to "updated_at"

	// Cursor pagination
	Cursor string // next_cursor returned by the previous page; omit on the first page
	Limit  int    // page size, default 20, maximum 100
}

// ListResult is a paginated response containing items and the next-page cursor.
type ListResult struct {
	Items      []SnippetResult `json:"items"`
	NextCursor string          `json:"next_cursor"` // empty string means there is no next page
	HasMore    bool            `json:"has_more"`
}

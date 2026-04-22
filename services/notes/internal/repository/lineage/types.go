package lineagerepo

import "time"

// Lineage is the repository-layer model for snippet_lineage.
type Lineage struct {
	ID              int64
	SnippetID       int64
	SourceSnippetID *int64
	SourceShareID   *int64
	SourceUserID    *int64
	RelationType    string
	CreatedAt       time.Time
}

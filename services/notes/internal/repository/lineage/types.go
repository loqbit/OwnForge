package lineagerepo

import "time"

// Lineage 是 snippet_lineage 的仓储层模型。
type Lineage struct {
	ID              int64
	SnippetID       int64
	SourceSnippetID *int64
	SourceShareID   *int64
	SourceUserID    *int64
	RelationType    string
	CreatedAt       time.Time
}

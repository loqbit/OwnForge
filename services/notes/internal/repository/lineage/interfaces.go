package lineagerepo

import "context"

// Repository defines the data access interface for snippet_lineage.
type Repository interface {
	Create(ctx context.Context, item *Lineage) (*Lineage, error)
	GetBySnippetID(ctx context.Context, snippetID int64) (*Lineage, error)
}

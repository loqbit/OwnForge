package aimetadata

import "context"

// Repository defines the AI metadata data access interface.
type Repository interface {
	// GetBySnippetID returns AI metadata for a snippet. It should return a clear not-found error when absent.
	GetBySnippetID(ctx context.Context, snippetID int64) (*AIMetadata, error)

	// Upsert updates or inserts AI metadata.
	Upsert(ctx context.Context, in UpsertInput) error
}

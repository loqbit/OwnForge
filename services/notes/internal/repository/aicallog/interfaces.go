package aicallog

import "context"

// Repository defines the data access interface for AI call logs.
//
// Design principle: Insert is append-only and never updates. Queries are mainly used for billing, dashboards, and troubleshooting.
type Repository interface {
	// Insert writes one call record. Entry.ID is provided by the caller through id-generator.
	Insert(ctx context.Context, entry *Entry) error
}

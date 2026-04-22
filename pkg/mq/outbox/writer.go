package outbox

import "context"

// Writer defines the unified interface for appending outbox events inside a transaction.
// Concrete implementations can be built on Ent, GORM, sql.DB, or other persistence layers.
//
// This interface is intentionally kept small:
// the service layer only needs to care about appending an event, without knowing whether the backend uses an old relay, direct DB persistence,
// or some future CDC-friendly implementation.
type Writer interface {
	Append(ctx context.Context, record *Record) error
}

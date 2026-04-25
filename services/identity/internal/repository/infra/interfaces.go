package infrarepo

import (
	"context"
	"encoding/json"
	"time"
)

// TransactionManager defines transactional execution behavior.
type TransactionManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventOutboxWriter defines the ability to write to the event outbox.
type EventOutboxWriter interface {
	Append(ctx context.Context, record *OutboxRecord) error
}

// OutboxRecord is a record written to the event outbox.
// Field names follow common Debezium Outbox Event Router conventions.
type OutboxRecord struct {
	ID            string          // unique event ID
	AggregateType string          // aggregate type (for example, user)
	AggregateID   string          // aggregate ID (for example, user_id)
	EventType     string          // event type (for example, user_registered)
	Payload       json.RawMessage // event payload (JSON)
	Headers       json.RawMessage // event headers (trace_id, span_id, etc.)
	CreatedAt     time.Time       // creation time
}

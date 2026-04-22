package outbox

import (
	"encoding/json"
	"time"
)

// Record is the standard data structure services write into the outbox table.
// Its field names follow common Debezium Outbox Event Router conventions.
//
// It represents a domain-event record that is definitely meant to be published,
// but it does not require the service to publish to Kafka itself.
// The service's job is only to persist it safely to the database.
type Record struct {
	ID            string          // unique event ID
	AggregateType string          // aggregate type (for example, user)
	AggregateID   string          // aggregate ID (for example, user_id)
	EventType     string          // event type (for example, user_registered)
	Payload       json.RawMessage // event payload (JSON)
	Headers       json.RawMessage // event headers (trace_id, span_id, etc.)
	CreatedAt     time.Time       // created at
}

// NewRecord builds an outbox record with unified defaults.
// CreatedAt is filled in here uniformly to reduce business-layer boilerplate.
func NewRecord(id, aggregateType, aggregateID, eventType string, payload json.RawMessage, headers json.RawMessage) *Record {
	return &Record{
		ID:            id,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		Headers:       headers,
		CreatedAt:     time.Now(),
	}
}

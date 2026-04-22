// Event envelope layer (unified wrapper)
// Adds a common outer wrapper around all events.
// Whether the event is "user registration" or "order creation", the outer structure stays the same and only the payload differs.
// This lets downstream consumers inspect EventType first and choose the correct payload struct for deserialization.
package envelope

import (
	"encoding/json"
	"time"
)

// Event is the shared cross-service outer structure for domain events.
// Payload uses json.RawMessage to stay decoupled from specific business event DTOs.
//
// This layer is more of a message semantic model:
// business events are first organized into a unified envelope, then sent directly, written to the Outbox,
// or handed to CDC for delivery to Kafka.
type Event struct {
	Version       string          `json:"version"`
	EventType     string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type,omitempty"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	Timestamp     int64           `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
}

// New builds the unified event envelope.
// Timestamp is generated here uniformly so services do not diverge in field style.
func New(version, eventType, aggregateType, aggregateID string, payload json.RawMessage) Event {
	return Event{
		Version:       version,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Timestamp:     time.Now().Unix(),
		Payload:       payload,
	}
}

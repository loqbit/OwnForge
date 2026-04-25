package infrastore

import (
	"context"
	"encoding/json"

	"github.com/loqbit/ownforge/services/identity/internal/ent"
	infrarepo "github.com/loqbit/ownforge/services/identity/internal/repository/infra"
	"github.com/loqbit/ownforge/services/identity/internal/store/entstore/shared"
)

// EventOutboxStore is the Ent-backed implementation of EventOutboxWriter.
type EventOutboxStore struct {
	client *ent.Client
}

// NewEventOutboxStore creates an EventOutboxWriter instance.
func NewEventOutboxStore(client *ent.Client) infrarepo.EventOutboxWriter {
	return &EventOutboxStore{client: client}
}

func (s *EventOutboxStore) Append(ctx context.Context, record *infrarepo.OutboxRecord) error {
	c := shared.EntClientFromCtx(ctx, s.client)

	builder := c.EventOutbox.Create()

	if record.AggregateType != "" {
		builder.SetAggregatetype(record.AggregateType)
	}
	if record.AggregateID != "" {
		builder.SetAggregateid(record.AggregateID)
	}
	if record.EventType != "" {
		builder.SetType(record.EventType)
	}
	builder.SetPayload(json.RawMessage(record.Payload))
	if len(record.Headers) > 0 {
		builder.SetHeaders(json.RawMessage(record.Headers))
	}
	if !record.CreatedAt.IsZero() {
		builder.SetCreatedAt(record.CreatedAt)
	}

	_, err := builder.Save(ctx)
	return err
}

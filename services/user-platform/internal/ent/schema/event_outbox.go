package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type EventOutbox struct {
	ent.Schema
}

func (EventOutbox) Fields() []ent.Field {
	return []ent.Field{
		// This currently still uses Ent's default auto-increment primary key.
		// If event IDs later need to be fully controlled by the business layer, switching to a string primary key can be reconsidered.
		field.String("aggregatetype").
			Optional().
			Nillable().
			Comment("aggregate type, such as user / paste; mainly indicates which aggregate the event belongs to"),
		field.String("aggregateid").
			Optional().
			Nillable().
			Comment("aggregate root ID, such as userID / pasteID; currently mapped to the Kafka message key by Debezium Outbox Router"),
		field.String("type").
			Optional().
			Nillable().
			Comment("domain event type, such as user.registered; currently used as the Kafka topic routing field by Debezium Outbox Router"),
		field.JSON("payload", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("event payload; currently mapped to the Kafka message value by Debezium Outbox Router"),
		field.JSON("headers", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("optional event headers; currently mapped to Kafka headers by Debezium Outbox Router and reserved for trace/source metadata"),
		field.Time("created_at").
			Default(time.Now).
			Comment("outbox record creation time; mainly used for auditing, troubleshooting, and time-based queries"),
	}
}

func (EventOutbox) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (EventOutbox) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("aggregatetype", "aggregateid"),
		index.Fields("type", "created_at"),
	}
}

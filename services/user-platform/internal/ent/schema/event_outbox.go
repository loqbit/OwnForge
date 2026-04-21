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
		// 当前仍沿用 Ent 默认自增主键。
		// 如果后续希望事件 ID 完全由业务侧控制，可再评估切换为字符串主键。
		field.String("aggregatetype").
			Optional().
			Nillable().
			Comment("聚合类型，如 user / paste；主要用于表达这条事件属于哪个聚合"),
		field.String("aggregateid").
			Optional().
			Nillable().
			Comment("聚合根 ID，如 userID / pasteID；当前会被 Debezium Outbox Router 映射为 Kafka message key"),
		field.String("type").
			Optional().
			Nillable().
			Comment("领域事件类型，如 user.registered；当前会被 Debezium Outbox Router 用作 Kafka topic 路由字段"),
		field.JSON("payload", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("事件消息体；当前会被 Debezium Outbox Router 映射为 Kafka message value"),
		field.JSON("headers", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("可选事件头；当前会被 Debezium Outbox Router 映射为 Kafka headers，预留给 trace/source 等元数据"),
		field.Time("created_at").
			Default(time.Now).
			Comment("Outbox 记录创建时间；主要用于审计、排查和按时间维度查询"),
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

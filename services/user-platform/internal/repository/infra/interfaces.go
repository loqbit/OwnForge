package infrarepo

import (
	"context"
	"encoding/json"
	"time"
)

// TransactionManager 定义了事务执行的能力。
type TransactionManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventOutboxWriter 定义了事件 Outbox 的写入能力。
type EventOutboxWriter interface {
	Append(ctx context.Context, record *OutboxRecord) error
}

// OutboxRecord 是事件 Outbox 的写入记录。
// 字段命名与 Debezium Outbox Event Router 的常见约定保持一致。
type OutboxRecord struct {
	ID            string          // 事件唯一 ID
	AggregateType string          // 聚合类型（如 user）
	AggregateID   string          // 聚合 ID（如 user_id）
	EventType     string          // 事件类型（如 user_registered）
	Payload       json.RawMessage // 事件内容（json）
	Headers       json.RawMessage // 事件头 （trace_id、span_id 等）
	CreatedAt     time.Time       // 创建时间
}

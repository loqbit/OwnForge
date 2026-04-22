package outbox

import (
	"encoding/json"
	"time"
)

// Record 是服务写入 outbox 表的统一数据结构。
// 字段命名与 Debezium Outbox Event Router 的常见约定保持一致。
//
// 它代表“已经确定要发出去的一条领域事件记录”，
// 但此时还没有要求服务自己去发 Kafka。
// 服务的职责只是把它安全写入数据库。
type Record struct {
	ID            string          // 事件唯一ID
	AggregateType string          // 聚合类型（如 user）
	AggregateID   string          // 聚合ID（如 user_id）
	EventType     string          // 事件类型（如 user_registered）
	Payload       json.RawMessage // 事件内容（json）
	Headers       json.RawMessage // 事件头 （trace_id、span_id等）
	CreatedAt     time.Time       // 创建时间
}

// NewRecord 使用统一默认值构造一条 outbox 记录。
// CreatedAt 由这里统一填充，减少业务侧样板代码。
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

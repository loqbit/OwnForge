package event

import "context"

// Topic 事件主题类型。
type Topic string

const (
	// TopicSnippetSaved 文档保存事件（创建或更新后触发）。
	TopicSnippetSaved Topic = "snippet.saved"
)

// SnippetSavedPayload snippet.saved 事件的消息体。
type SnippetSavedPayload struct {
	SnippetID int64  `json:"snippet_id"`
	OwnerID   int64  `json:"owner_id"`
	Action    string `json:"action"` // "create" | "update"
}

// Publisher 事件发布抽象接口。
//
// 当前实现：RedisStreamPublisher（基于 Redis Stream XADD）。
// 未来 Phase 6 可替换为 MemoryPublisher（基于 Go channel，桌面版/NAS 版零依赖）。
type Publisher interface {
	// Publish 发布一条事件。payload 会被 JSON 序列化。
	Publish(ctx context.Context, topic Topic, payload any) error
	// Close 释放资源。
	Close() error
}

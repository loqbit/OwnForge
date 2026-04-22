package event

import "context"

// Topic is the event topic type.
type Topic string

const (
	// TopicSnippetSaved is emitted after a document is created or updated.
	TopicSnippetSaved Topic = "snippet.saved"
)

// SnippetSavedPayload is the payload for the snippet.saved event.
type SnippetSavedPayload struct {
	SnippetID int64  `json:"snippet_id"`
	OwnerID   int64  `json:"owner_id"`
	Action    string `json:"action"` // "create" | "update"
}

// Publisher abstracts event publishing.
//
// Current implementation: RedisStreamPublisher based on Redis Stream XADD.
// In a future phase, this can be swapped with MemoryPublisher based on Go channels for zero-dependency desktop/NAS builds.
type Publisher interface {
	// Publish emits one event. payload is serialized as JSON.
	Publish(ctx context.Context, topic Topic, payload any) error
	// Close releases resources.
	Close() error
}

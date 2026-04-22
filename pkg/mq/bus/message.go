package bus

import "context"

// Message represents a business message decoupled from any specific middleware.
// Business handlers depend only on this abstraction and do not need to know Kafka or NATS client types.
type Message struct {
	Topic    string
	Key      string
	Value    []byte
	Headers  map[string][]byte
	Metadata map[string]string
}

// Handler defines the unified message-processing interface.
// Returning nil means the message was handled successfully, and the upper bus implementation can decide whether to ack/commit.
// Returning an error means processing failed, and the upper bus implementation can decide whether to retry, redeliver, or move it to a dead-letter queue.
type Handler interface {
	Handle(ctx context.Context, msg *Message) error
}

// HandlerFunc lets a plain function implement Handler directly.
type HandlerFunc func(ctx context.Context, msg *Message) error

func (f HandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

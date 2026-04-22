package bus

import "context"

// Subscriber defines a unified message consumption interface.
// Start blocks the current goroutine until ctx is canceled.
// For each received message, it calls handler.Handle.
//
// Concrete implementations are responsible for:
//   - pulling or receiving messages from the underlying broker
//   - converting them into bus.Message
//   - deciding ack/nack based on the handler result
type Subscriber interface {
	Start(ctx context.Context, handler Handler) error
	Close() error
}

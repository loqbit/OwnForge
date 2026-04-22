package bus

import "context"

// Publisher 定义统一的消息发布接口。
// 具体实现可以基于 Kafka、NATS 或其他消息中间件。
// 业务层只依赖这个抽象，不直接感知底层 broker。
type Publisher interface {
	Publish(ctx context.Context, msg *Message) error
	Close() error
}

package bus

import "context"

// Subscriber 定义统一的消息消费接口。
// Start 会阻塞当前 goroutine，直到 ctx 被取消。
// 每收到一条消息，会调用 handler.Handle 处理。
//
// 具体实现负责：
//   - 从底层 broker 拉取/接收消息
//   - 转换为 bus.Message
//   - 根据 handler 返回值决定 ack/nack
type Subscriber interface {
	Start(ctx context.Context, handler Handler) error
	Close() error
}

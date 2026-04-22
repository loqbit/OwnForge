package bus

import "context"

// Message 表示一条与具体中间件解耦后的业务消息。
// 业务 Handler 只依赖这层抽象，不直接感知 Kafka/NATS 的客户端类型。
type Message struct {
	Topic    string
	Key      string
	Value    []byte
	Headers  map[string][]byte
	Metadata map[string]string
}

// Handler 定义统一的消息处理接口。
// 返回 nil 表示消息已成功处理，可由上层总线实现决定是否 ack/commit。
// 返回 error 表示处理失败，由上层总线实现决定重试、重投或进入死信。
type Handler interface {
	Handle(ctx context.Context, msg *Message) error
}

// HandlerFunc 允许直接使用函数实现 Handler。
type HandlerFunc func(ctx context.Context, msg *Message) error

func (f HandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

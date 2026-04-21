package event

import "context"

// Handler 事件处理函数。返回 nil 表示处理成功（会 ACK），返回 error 表示处理失败（不 ACK，留待重试）。
type Handler func(ctx context.Context, data []byte) error

// Subscriber 事件订阅抽象接口。
//
// 当前实现：RedisStreamSubscriber（基于 Redis Stream 消费者组）。
// 未来 Phase 6 可替换为 MemorySubscriber（基于 Go channel）。
type Subscriber interface {
	// Subscribe 以消费者组模式订阅指定 topic。
	//   - group: 消费者组名称（同一组内的多个消费者不重复消费）
	//   - consumer: 当前消费者实例名（用于区分同组内的不同 Worker）
	//   - handler: 消息处理函数
	//
	// 该方法会阻塞直到 ctx 取消。
	Subscribe(ctx context.Context, topic Topic, group, consumer string, handler Handler) error
	// Close 释放资源。
	Close() error
}

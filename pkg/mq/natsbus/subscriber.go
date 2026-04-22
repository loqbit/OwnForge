package natsbus

import (
	"context"
	"sync"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/nats-io/nats.go"
)

// Subscriber 是基于 NATS Core 的 bus.Subscriber 实现。
// 纯内存 Pub/Sub，不持久化。适用于在线实时场景。
// 支持 Queue Group 做多实例负载均衡。
type Subscriber struct {
	conn    *nats.Conn
	subject string
	queue   string // 为空则为普通订阅，非空则为 Queue Group 订阅

	mu  sync.Mutex
	sub *nats.Subscription
}

// NewSubscriber 创建 Core NATS 订阅者。
// subject 支持 NATS 通配符：* 匹配单级，> 匹配多级。
//
// 示例：
//
//	// 订阅所有聊天室消息
//	sub := NewSubscriber(conn, "chat.room.>")
//
//	// 带 Queue Group 的负载均衡订阅
//	sub := NewSubscriber(conn, "task.>", WithQueue("workers"))
func NewSubscriber(conn *nats.Conn, subject string, opts ...Option) *Subscriber {
	s := &Subscriber{
		conn:    conn,
		subject: subject,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// Start 开始订阅消息，阻塞直到 ctx 取消。
// 每条消息会被转换为 bus.Message 后交给 handler 处理。
// Core NATS 没有 ACK 机制，handler 返回的 error 仅用于日志/监控。
func (s *Subscriber) Start(ctx context.Context, handler bus.Handler) error {
	msgHandler := func(m *nats.Msg) {
		msg := coreToMessage(m)
		// Core NATS 没有 ack/nack，handler 的 error 由上层自行处理
		_ = handler.Handle(ctx, msg)
	}

	var (
		sub *nats.Subscription
		err error
	)
	if s.queue != "" {
		sub, err = s.conn.QueueSubscribe(s.subject, s.queue, msgHandler)
	} else {
		sub, err = s.conn.Subscribe(s.subject, msgHandler)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.sub = sub
	s.mu.Unlock()

	// 阻塞等待取消信号
	<-ctx.Done()

	// 优雅退出：Drain 会处理完已接收的消息后再取消订阅
	return sub.Drain()
}

func (s *Subscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sub != nil {
		return s.sub.Drain()
	}
	return nil
}

// coreToMessage 将 NATS Core 消息转换为 bus.Message。
func coreToMessage(m *nats.Msg) *bus.Message {
	headers := make(map[string][]byte, len(m.Header))
	for k, vals := range m.Header {
		if len(vals) > 0 {
			headers[k] = []byte(vals[0])
		}
	}

	// 从 Header 中提取 Key（如果有）
	key := ""
	if v := m.Header.Get(HeaderKey); v != "" {
		key = v
		delete(headers, HeaderKey)
	}

	return &bus.Message{
		Topic:   m.Subject,
		Key:     key,
		Value:   m.Data,
		Headers: headers,
		Metadata: map[string]string{
			"transport": "nats-core",
		},
	}
}

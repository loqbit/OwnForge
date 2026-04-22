package natsbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/nats-io/nats.go/jetstream"
)

// JSSubscriber 是基于 NATS JetStream 的 bus.Subscriber 实现。
// 支持持久化消费、ACK/NAK 确认、Queue Group 负载均衡。
// 适用于任务队列、离线消息等需要可靠投递的场景。
type JSSubscriber struct {
	js      jetstream.JetStream
	stream  string // Stream 名称（JetStream 的存储单元）
	subject string // 要消费的 Subject 过滤规则
	durable string // 持久 Consumer 名称，重启后从上次位置继续
	queue   string // DeliverGroup，多实例负载均衡

	mu      sync.Mutex
	consume jetstream.ConsumeContext
}

// NewJSSubscriber 创建 JetStream 订阅者。
//
// stream 是 JetStream Stream 名称（需要先创建好）。
// subject 是要消费的 Subject 过滤规则。
//
// 示例：
//
//	// 持久消费，带 Queue Group 负载均衡
//	sub := NewJSSubscriber(js, "TASKS", "task.>",
//	    WithJSDurable("task-worker"),
//	    WithQueue("workers"),
//	)
func NewJSSubscriber(js jetstream.JetStream, stream, subject string, opts ...Option) *JSSubscriber {
	s := &JSSubscriber{
		js:      js,
		stream:  stream,
		subject: subject,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// Start 开始消费 JetStream 消息，阻塞直到 ctx 取消。
// handler 返回 nil → ACK（消息处理完成）；返回 error → NAK（消息稍后重投）。
func (s *JSSubscriber) Start(ctx context.Context, handler bus.Handler) error {
	// 构建 Consumer 配置
	consumerCfg := jetstream.ConsumerConfig{
		FilterSubject: s.subject,
	}
	if s.durable != "" {
		consumerCfg.Durable = s.durable
	}
	if s.queue != "" {
		consumerCfg.DeliverGroup = s.queue
	}

	// 创建或更新 Consumer（幂等操作）
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.stream, consumerCfg)
	if err != nil {
		return fmt.Errorf("natsbus: 创建 JetStream Consumer 失败: %w", err)
	}

	// 开始消费
	cc, err := consumer.Consume(func(m jetstream.Msg) {
		msg := jsToMessage(m)
		if err := handler.Handle(ctx, msg); err != nil {
			// 处理失败，NAK 让 JetStream 稍后重投
			_ = m.Nak()
			return
		}
		// 处理成功，ACK 确认
		_ = m.Ack()
	})
	if err != nil {
		return fmt.Errorf("natsbus: 启动 JetStream 消费失败: %w", err)
	}

	s.mu.Lock()
	s.consume = cc
	s.mu.Unlock()

	// 阻塞等待取消信号
	<-ctx.Done()
	cc.Stop()
	return nil
}

func (s *JSSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consume != nil {
		s.consume.Stop()
	}
	return nil
}

// jsToMessage 将 JetStream 消息转换为 bus.Message。
func jsToMessage(m jetstream.Msg) *bus.Message {
	natsHeaders := m.Headers()
	headers := make(map[string][]byte, len(natsHeaders))
	for k, vals := range natsHeaders {
		if len(vals) > 0 {
			headers[k] = []byte(vals[0])
		}
	}

	key := ""
	if v := natsHeaders.Get(HeaderKey); v != "" {
		key = v
		delete(headers, HeaderKey)
	}

	return &bus.Message{
		Topic:   m.Subject(),
		Key:     key,
		Value:   m.Data(),
		Headers: headers,
		Metadata: map[string]string{
			"transport": "nats-jetstream",
			"stream":    m.Headers().Get("Nats-Stream"),
		},
	}
}

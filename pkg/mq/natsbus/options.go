package natsbus

// Option 用于配置 Subscriber / JSSubscriber 的可选参数。
type Option interface {
	apply(any)
}

type optionFunc struct {
	fn func(any)
}

func (f optionFunc) apply(target any) { f.fn(target) }

// WithQueue 设置 Queue Group 名称，用于多实例负载均衡。
// 同一 Queue Group 内，同一条消息只会被一个消费者处理。
//
// 在 Core NATS 中对应 QueueSubscribe；
// 在 JetStream 中对应 DeliverGroup。
func WithQueue(queue string) Option {
	return optionFunc{fn: func(target any) {
		switch t := target.(type) {
		case *Subscriber:
			t.queue = queue
		case *JSSubscriber:
			t.queue = queue
		}
	}}
}

// WithJSDurable 设置 JetStream Consumer 的持久名称。
// 持久 Consumer 在重启后会从上次确认的位置继续消费，不会丢消息。
func WithJSDurable(name string) Option {
	return optionFunc{fn: func(target any) {
		if t, ok := target.(*JSSubscriber); ok {
			t.durable = name
		}
	}}
}

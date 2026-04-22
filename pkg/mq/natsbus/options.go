package natsbus

// Option configures optional parameters for Subscriber / JSSubscriber.
type Option interface {
	apply(any)
}

type optionFunc struct {
	fn func(any)
}

func (f optionFunc) apply(target any) { f.fn(target) }

// WithQueue sets the Queue Group name for multi-instance load balancing.
// Within the same Queue Group, each message is handled by only one consumer.
//
// In Core NATS this maps to QueueSubscribe;
// in JetStream it maps to DeliverGroup.
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

// WithJSDurable sets the durable name of the JetStream consumer.
// A durable consumer resumes from the last acknowledged position after restart and does not lose messages.
func WithJSDurable(name string) Option {
	return optionFunc{fn: func(target any) {
		if t, ok := target.(*JSSubscriber); ok {
			t.durable = name
		}
	}}
}

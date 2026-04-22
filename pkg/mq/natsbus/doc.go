// Package natsbus provides NATS implementations of bus.Publisher and bus.Subscriber.
//
// It supports two modes:
//   - Core NATS: in-memory forwarding, fire-and-forget, ultra-low latency (great for real-time notifications and chat routing)
//   - JetStream: persistence plus ACK confirmation with at-least-once delivery (great for task queues and offline messages)
//
// Usage:
//
//	// Core NATS
//	pub := natsbus.NewPublisher(conn)
//	sub := natsbus.NewSubscriber(conn, "chat.room.>", natsbus.WithQueue("chat-workers"))
//
//	// JetStream
//	pub := natsbus.NewJSPublisher(js)
//	sub := natsbus.NewJSSubscriber(js, "TASKS", "task.>", natsbus.WithJSDurable("task-worker"))
package natsbus

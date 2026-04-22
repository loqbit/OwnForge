// Package natsbus 提供 bus.Publisher 和 bus.Subscriber 的 NATS 实现。
//
// 支持两种模式：
//   - Core NATS：纯内存转发，fire-and-forget，超低延迟（适合实时通知、聊天消息路由）
//   - JetStream：持久化 + ACK 确认，At-Least-Once 投递（适合任务队列、离线消息）
//
// 使用方式：
//
//	// Core NATS
//	pub := natsbus.NewPublisher(conn)
//	sub := natsbus.NewSubscriber(conn, "chat.room.>", natsbus.WithQueue("chat-workers"))
//
//	// JetStream
//	pub := natsbus.NewJSPublisher(js)
//	sub := natsbus.NewJSSubscriber(js, "TASKS", "task.>", natsbus.WithJSDurable("task-worker"))
package natsbus

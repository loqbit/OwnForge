package natsbus

// HeaderKey 是 NATS Header 中用于传递 bus.Message.Key 的键名。
// Key 在 Kafka 里用于 Partition 路由，在 NATS 里没有等价概念，
// 所以通过 Header 透传，保持抽象层语义一致。
const HeaderKey = "X-Bus-Key"

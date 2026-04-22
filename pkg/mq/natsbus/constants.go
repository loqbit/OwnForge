package natsbus

// HeaderKey is the NATS header key used to carry bus.Message.Key.
// Key is used for partition routing in Kafka and has no exact equivalent in NATS,
// so it is forwarded through headers to keep the abstraction semantically consistent.
const HeaderKey = "X-Bus-Key"

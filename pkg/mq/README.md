# common/mq — 统一消息队列抽象

基于接口的消息队列抽象层，支持 Kafka 和 NATS JetStream 双后端，外加 Outbox 模式保证最终一致性。

## 架构

```
mq/
├── bus/          # 核心接口 (Publisher / Subscriber / Handler)
├── kafkabus/     # Kafka 实现
├── natsbus/      # NATS JetStream 实现
├── envelope/     # 标准事件信封
├── events/       # 领域事件定义 (UserRegistered 等)
├── topics/       # Topic 常量
├── outbox/       # Outbox 模式（事务性消息发送）
└── cdc/          # CDC (Change Data Capture) 配置
```

## 核心接口

```go
// 发布消息
type Publisher interface {
    Publish(ctx context.Context, msg *bus.Message) error
    Close() error
}

// 订阅消息
type Subscriber interface {
    Start(ctx context.Context, handler Handler) error
    Close() error
}

// 处理消息
type Handler interface {
    Handle(ctx context.Context, msg *Message) error
}
```

## Kafka 用法

```go
import "github.com/ownforge/ownforge/pkg/mq/kafkabus"

// 发布
pub := kafkabus.NewPublisher([]string{"kafka:9092"})
defer pub.Close()
pub.Publish(ctx, &bus.Message{Topic: "user.registered", Value: data})

// 订阅
sub := kafkabus.NewSubscriber([]string{"kafka:9092"}, "user.registered", "my-group")
sub.Start(ctx, bus.HandlerFunc(func(ctx context.Context, msg *bus.Message) error {
    // 处理消息
    return nil
}))
```

## NATS JetStream 用法

```go
import "github.com/ownforge/ownforge/pkg/mq/natsbus"

pub := natsbus.NewJSPublisher(js)
sub := natsbus.NewJSSubscriber(js, "STREAM", "subject.>")
```

## Outbox 模式

```go
import "github.com/ownforge/ownforge/pkg/mq/outbox"

// 在数据库事务中写入 Outbox 记录（保证原子性）
record, _ := outbox.NewJSONRecord(id, "User", userID, "UserRegistered", payload, nil)
writer.Write(ctx, tx, record)
// CDC (Debezium) 自动将 Outbox 表变更推送到 Kafka
```

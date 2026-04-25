# common/mq — Unified Message Queue Abstraction

An interface-based message-queue abstraction that supports both Kafka and NATS JetStream, plus the Outbox pattern for eventual consistency.

## Architecture

```
mq/
├── bus/          # Core Interfaces (Publisher / Subscriber / Handler)
├── kafkabus/     # Kafka implementation
├── natsbus/      # NATS JetStream implementation
├── envelope/     # standard event envelope
├── events/       # domain event definitions (UserRegistered, etc.)
├── topics/       # Topic Constant
├── outbox/       # Outbox pattern (transactional message publishing)
└── cdc/          # CDC (Change Data Capture) Configuration
```

## Core Interfaces

```go
// Publish messages
type Publisher interface {
    Publish(ctx context.Context, msg *bus.Message) error
    Close() error
}

// Subscribe to messages
type Subscriber interface {
    Start(ctx context.Context, handler Handler) error
    Close() error
}

// Handle messages
type Handler interface {
    Handle(ctx context.Context, msg *Message) error
}
```

## Kafka Usage

```go
import "github.com/loqbit/ownforge/pkg/mq/kafkabus"

// Publish
pub := kafkabus.NewPublisher([]string{"kafka:9092"})
defer pub.Close()
pub.Publish(ctx, &bus.Message{Topic: "user.registered", Value: data})

// Subscribe
sub := kafkabus.NewSubscriber([]string{"kafka:9092"}, "user.registered", "my-group")
sub.Start(ctx, bus.HandlerFunc(func(ctx context.Context, msg *bus.Message) error {
    // Handle messages
    return nil
}))
```

## NATS JetStream Usage

```go
import "github.com/loqbit/ownforge/pkg/mq/natsbus"

pub := natsbus.NewJSPublisher(js)
sub := natsbus.NewJSSubscriber(js, "STREAM", "subject.>")
```

## Outbox Pattern

```go
import "github.com/loqbit/ownforge/pkg/mq/outbox"

// Write the Outbox record within the database transaction to guarantee atomicity
record, _ := outbox.NewJSONRecord(id, "User", userID, "UserRegistered", payload, nil)
writer.Write(ctx, tx, record)
// CDC (Debezium) automatically pushes Outbox table changes to Kafka
```

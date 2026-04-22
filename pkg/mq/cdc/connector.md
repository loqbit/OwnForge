# Debezium Outbox Connector 约定

`common/mq/cdc` 约定服务统一采用 Outbox + CDC 模式接入 Kafka。

推荐的 Outbox 表字段如下：

```text
id
aggregatetype
aggregateid
type
payload
headers
created_at
```

字段用途：

- `id`: 事件唯一标识
- `aggregatetype`: 聚合类型，例如 `user`、`paste`
- `aggregateid`: 聚合根 ID，用于分区键或局部有序语义
- `type`: 领域事件类型，例如 `user.registered`
- `payload`: JSON 事件体
- `headers`: 可选 JSON 头信息
- `created_at`: 事件产生时间

服务侧职责：

1. 在业务事务内写业务表
2. 在同一事务内向 Outbox 表追加一条记录

CDC 职责：

1. 监听 Outbox 表
2. 使用 Debezium Outbox Event Router 将记录转成 Kafka 消息
3. 按统一 Topic/Key 约定投递到消息总线

推荐同时显式配置：

```json
"transforms.outbox.route.by.field": "type",
"transforms.outbox.route.topic.replacement": "${routedByValue}",
"transforms.outbox.table.expand.json.payload": "true"
```

这样最终业务 Topic 会直接等于 Outbox 中的 `type` 值，例如：

- `type = user.registered`
- Topic = `user.registered`
- Kafka value = JSON 对象，而不是 base64 或普通字符串

否则 Debezium 可能会使用默认前缀，生成类似 `outbox.event.user.registered` 的 Topic。

这份约定的目标是让服务只关心“追加事件”，而不是自管 Relay、轮询和重试。

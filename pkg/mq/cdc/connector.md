# Debezium Outbox Connector Conventions

`common/mq/cdc` defines the convention that services integrate with Kafka through the Outbox + CDC pattern.

The recommended Outbox table fields are:

```text
id
aggregatetype
aggregateid
type
payload
headers
created_at
```

Field meanings:

- `id`: unique event identifier
- `aggregatetype`: aggregate type, for example `user`、`paste`
- `aggregateid`: aggregate root ID used as a partition key or for local ordering semantics
- `type`: domain event type, for example `user.registered`
- `payload`: JSON event payload
- `headers`: optional JSON headers
- `created_at`: event creation time

Service responsibilities:

1. Write business tables inside the business transaction
2. Append an Outbox row within the same transaction

CDC responsibilities:

1. Listen to the Outbox table
2. Use Debezium Outbox Event Router to convert rows into Kafka messages
3. Deliver them to the message bus following the unified Topic/Key convention

It is recommended to configure the following explicitly as well:

```json
"transforms.outbox.route.by.field": "type",
"transforms.outbox.route.topic.replacement": "${routedByValue}",
"transforms.outbox.table.expand.json.payload": "true"
```

This makes the final business topic equal the `type` value in the Outbox row, for example:

- `type = user.registered`
- Topic = `user.registered`
- Kafka value = JSON object，rather than base64 or a plain string

Otherwise Debezium may use a default prefix and generate topics such as `outbox.event.user.registered`.

The goal of this convention is to let services care only about appending events, rather than managing relays, polling, and retries themselves.

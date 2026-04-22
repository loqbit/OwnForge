package cdc

import "github.com/ownforge/ownforge/pkg/mq/outbox"

const (
	// DefaultConnectorClass is the class name of the Postgres Debezium connector.
	DefaultConnectorClass = "io.debezium.connector.postgresql.PostgresConnector"

	// DefaultRouterTransform is the transform name of the Debezium Outbox Event Router.
	DefaultRouterTransform = "outbox"

	// DefaultRouterClass is the implementation class name of the Debezium Outbox Event Router.
	DefaultRouterClass = "io.debezium.transforms.outbox.EventRouter"
)

// Config describes the shared Debezium Outbox integration conventions.
// It is not a full connector configuration; it defines the core field mappings that services depend on stably.
//
// In other words, this is more of a service-to-CDC contract than the final JSON submitted to Connect during deployment.
type Config struct {
	TableName           string
	IDColumn            string
	AggregateTypeColumn string
	AggregateIDColumn   string
	EventTypeColumn     string
	PayloadColumn       string
	HeadersColumn       string
	TimestampColumn     string
}

// DefaultConfig returns the recommended Outbox + Debezium field conventions.
// If business services follow these names, connector integration will be smoother later.
func DefaultConfig() Config {
	return Config{
		TableName:           outbox.DefaultTableName,
		IDColumn:            outbox.ColumnID,
		AggregateTypeColumn: outbox.ColumnAggregateType,
		AggregateIDColumn:   outbox.ColumnAggregateID,
		EventTypeColumn:     outbox.ColumnType,
		PayloadColumn:       outbox.ColumnPayload,
		HeadersColumn:       outbox.ColumnHeaders,
		TimestampColumn:     outbox.ColumnCreatedAt,
	}
}

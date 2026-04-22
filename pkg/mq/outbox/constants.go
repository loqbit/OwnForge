package outbox

const (
	// DefaultTableName is the recommended standard outbox table name.
	// Unless there is a special reason, later services should reuse this name to simplify CDC configuration.
	DefaultTableName = "outbox_events"

	// The following column names are chosen to stay close to common Debezium Outbox Event Router conventions,
	// so business services, database schemas, and CDC configuration all speak the same language.
	ColumnID            = "id"
	ColumnAggregateType = "aggregatetype"
	ColumnAggregateID   = "aggregateid"
	ColumnType          = "type"
	ColumnPayload       = "payload"
	ColumnHeaders       = "headers"
	ColumnCreatedAt     = "created_at"
)

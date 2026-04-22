package cdc

import "github.com/ownforge/ownforge/pkg/mq/outbox"

const (
	// DefaultConnectorClass 是 Postgres Debezium Connector 的类名。
	DefaultConnectorClass = "io.debezium.connector.postgresql.PostgresConnector"

	// DefaultRouterTransform 是 Debezium Outbox Event Router 的转换器名称。
	DefaultRouterTransform = "outbox"

	// DefaultRouterClass 是 Debezium Outbox Event Router 的实现类名。
	DefaultRouterClass = "io.debezium.transforms.outbox.EventRouter"
)

// Config 描述公共的 Debezium Outbox 集成约定。
// 它不是完整的连接器配置，而是服务侧需要稳定依赖的核心字段映射。
//
// 也就是说，这里更像“服务与 CDC 的契约”，而不是最终部署时直接提交给 Connect 的全部 JSON。
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

// DefaultConfig 返回推荐的 Outbox + Debezium 字段约定。
// 业务服务如果遵循这套命名，后续接 Connector 时会更顺滑。
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

package outbox

const (
	// DefaultTableName 是统一建议的 Outbox 表名。
	// 后续服务如无特殊原因，尽量共用这个命名，方便 CDC 配置收口。
	DefaultTableName = "outbox_events"

	// 以下列名尽量贴近 Debezium Outbox Event Router 的常见约定，
	// 目的是让业务服务、数据库表结构和 CDC 配置保持同一套语言体系。
	ColumnID            = "id"
	ColumnAggregateType = "aggregatetype"
	ColumnAggregateID   = "aggregateid"
	ColumnType          = "type"
	ColumnPayload       = "payload"
	ColumnHeaders       = "headers"
	ColumnCreatedAt     = "created_at"
)

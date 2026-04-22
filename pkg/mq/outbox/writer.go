package outbox

import "context"

// Writer 定义了服务在事务内追加 outbox 事件的统一接口。
// 具体实现可以基于 Ent、GORM、sql.DB 或其他持久层。
//
// 这个接口刻意保持得很小：
// service 层只需要关心“追加一条事件”，不需要知道底层是旧 Relay、纯数据库落表，
// 还是未来的 CDC 友好实现。
type Writer interface {
	Append(ctx context.Context, record *Record) error
}

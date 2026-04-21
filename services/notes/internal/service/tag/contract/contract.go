package contract

// CreateTagCommand 创建 Tag 的服务层输入参数。
type CreateTagCommand struct {
	Name  string
	Color string
}

// UpdateTagCommand 更新 Tag 的服务层输入参数。
type UpdateTagCommand struct {
	Name  string
	Color string
}

// TagResult 服务层输出的 Tag 数据结构。
type TagResult struct {
	ID        int64
	OwnerID   int64
	Name      string
	Color     string
	CreatedAt string
}

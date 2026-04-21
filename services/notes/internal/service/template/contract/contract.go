package contract

// CreateTemplateCommand 创建模板的服务层输入参数。
type CreateTemplateCommand struct {
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
}

// UpdateTemplateCommand 更新模板的服务层输入参数。
type UpdateTemplateCommand struct {
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
}

// TemplateResult 服务层输出的模板数据结构。
type TemplateResult struct {
	ID          int64
	OwnerID     int64
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
	IsSystem    bool
	CreatedAt   string
	UpdatedAt   string
}

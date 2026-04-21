package contract

// CreateGroupCommand 创建 Group 的服务层输入参数。
type CreateGroupCommand struct {
	ParentID    *int64
	Name        string
	Description string
}

// UpdateGroupCommand 更新 Group 的服务层输入参数。
type UpdateGroupCommand struct {
	Name        string
	Description string
	SortOrder   *int
	ParentID    *int64 // 支持移动分组
}

// GroupResult 服务层输出的 Group 数据结构。
type GroupResult struct {
	ID            int64
	OwnerID       int64
	ParentID      *int64
	Name          string
	Description   string
	SortOrder     int
	ChildrenCount int
	SnippetCount  int
	CreatedAt     string
	UpdatedAt     string
}

// GroupTreeNode 递归树节点，用于 GetTree 返回完整的目录结构。
// 核心思路：一次查全部 → O(n) 内存 hashmap 建树 → 返回顶级节点数组。
type GroupTreeNode struct {
	ID            int64
	ParentID      *int64
	Name          string
	Description   string
	SortOrder     int
	ChildrenCount int
	SnippetCount  int
	CreatedAt     string
	UpdatedAt     string
	Children      []GroupTreeNode // 递归嵌套子节点
}

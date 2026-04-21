package snippetrepo

import "time"

// Snippet 是 snippet 的仓储层领域模型。
type Snippet struct {
	ID         int64
	OwnerID    int64
	Type       string
	Title      string
	Content    string
	FileURL    string
	FileSize   int64
	MimeType   string
	Language   string
	GroupID    *int64
	SortOrder  int        // 在所属分组内的排序权重
	TagIDs     []int64    // 关联的标签 ID 列表
	IsFavorite bool
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

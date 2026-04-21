package contract

// CreateSnippetCommand 创建 Snippet 的服务层输入参数。
type CreateSnippetCommand struct {
	Type       string // code / note / file
	Title      string
	Content    string // code/note 时使用
	FileURL    string // file 时使用
	FileSize   int64  // file 时使用
	MimeType   string // file 时使用
	Language   string
	GroupID    *int64 // 可选的分组 ID
}

// UpdateSnippetCommand 更新 Snippet 的服务层输入参数。
type UpdateSnippetCommand struct {
	Title      string
	Content    string
	Language   string
	GroupID    *int64
}

// MoveSnippetCommand 移动 Snippet 的服务层输入参数。
// 用于将 snippet 在分组间移动、或在同一分组内调整排序。
type MoveSnippetCommand struct {
	// GroupID: 目标分组 ID。nil 表示移动到未分组（收集箱）。
	GroupID *int64
	// SortOrder: 目标排序位置。nil 表示追加到目标分组末尾（max+1）。
	SortOrder *int
}

// SnippetResult 服务层输出的 Snippet 数据结构。
type SnippetResult struct {
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
	SortOrder  int
	TagIDs     []int64 // 关联的标签 ID 列表
	IsFavorite bool
	DeletedAt  string  // 空字符串表示未删除
	CreatedAt  string
	UpdatedAt  string
}

// ── 技术点 4 + 5：复合筛选 + 游标分页 ─────────────────────────

// ListQuery 列表查询参数，支持多条件筛选 + 游标分页。
// 所有字段都是可选的，不传 = 不筛选。
type ListQuery struct {
	// ── 筛选条件 ──
	GroupID    *int64 // 按分组筛选
	TagID     *int64 // 按标签筛选（通过桥接表 JOIN）
	Type      string // 按类型筛选: code / note / file
	Keyword   string // 模糊搜索标题
	Status    string // ""(默认 active), "active", "trashed"
	OnlyFavorites bool // 只过滤出被收藏的文档

	// ── 排序 ──
	SortBy string // "updated_at" | "created_at" | "title" | "manual"，默认 "updated_at"

	// ── 游标分页 ──
	Cursor string // 上一页返回的 next_cursor，首页不传
	Limit  int    // 每页条数，默认 20，最大 100
}

// ListResult 分页列表响应，包含数据和下一页游标。
type ListResult struct {
	Items      []SnippetResult `json:"items"`
	NextCursor string          `json:"next_cursor"` // 空字符串 = 没有下一页
	HasMore    bool            `json:"has_more"`
}


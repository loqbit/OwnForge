package contract

import snippetcontract "github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"

// CreateShareCommand 创建分享请求。
type CreateShareCommand struct {
	SnippetID int64
	Kind      string
	Password  string
	ExpiresAt string
}

// ListSharesQuery 分享列表筛选参数。
type ListSharesQuery struct {
	Kind string
}

// ShareResult 服务层输出的分享结构。
type ShareResult struct {
	ID          int64
	Token       string
	Kind        string
	SnippetID   int64
	OwnerID     int64
	HasPassword bool
	ExpiresAt   string
	ViewCount   int
	ForkCount   int
	CreatedAt   string
}

// PublicShareResult 公开访问返回的分享数据。
type PublicShareResult struct {
	Share   *ShareResult
	Snippet *snippetcontract.SnippetResult
}

// ShareSource 用于导入副本时获取来源信息。
type ShareSource struct {
	Share   *ShareResult
	Snippet *snippetcontract.SnippetResult
}

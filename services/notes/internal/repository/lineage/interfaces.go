package lineagerepo

import "context"

// Repository 定义 snippet_lineage 数据访问接口。
type Repository interface {
	Create(ctx context.Context, item *Lineage) (*Lineage, error)
	GetBySnippetID(ctx context.Context, snippetID int64) (*Lineage, error)
}

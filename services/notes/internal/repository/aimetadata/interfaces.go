package aimetadata

import "context"

// Repository 定义 AI 元数据的数据访问接口。
type Repository interface {
	// GetBySnippetID 获取片段的 AI 元数据。如果不存在，应返回明确的错误（对应 NotFound）。
	GetBySnippetID(ctx context.Context, snippetID int64) (*AIMetadata, error)

	// Upsert 更新或插入 AI 元数据。
	Upsert(ctx context.Context, in UpsertInput) error
}

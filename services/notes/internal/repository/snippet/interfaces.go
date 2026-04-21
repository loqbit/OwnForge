package snippetrepo

import (
	"context"

	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"
)

// Repository 定义 snippet 数据访问接口。
type Repository interface {
	Create(ctx context.Context, id, ownerID int64, params *contract.CreateSnippetCommand) (*Snippet, error)
	GetByID(ctx context.Context, id int64) (*Snippet, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]Snippet, error)
	ListFiltered(ctx context.Context, ownerID int64, query *contract.ListQuery) ([]Snippet, error) // 复合筛选 + 游标分页
	Update(ctx context.Context, ownerID, id int64, params *contract.UpdateSnippetCommand) (*Snippet, error)
	Delete(ctx context.Context, ownerID, id int64) error // 物理删除
	SoftDelete(ctx context.Context, ownerID, id int64) error
	Restore(ctx context.Context, ownerID, id int64) error
	SetFavorite(ctx context.Context, ownerID, id int64, isFavorite bool) error
	// groupID 为 nil 表示移动到收集箱（未分组）；sortOrder 直接写入。
	Move(ctx context.Context, ownerID, id int64, groupID *int64, sortOrder int) (*Snippet, error)
	// MaxSortOrderInGroup 返回目标分组内当前最大的 sort_order，用于追加到末尾。
	// 目标分组为 nil 表示收集箱（group_id IS NULL）。
	MaxSortOrderInGroup(ctx context.Context, ownerID int64, groupID *int64) (int, error)

	// ── Snippet-Tag M2M 关联 ──
	// SetTags 替换片段的所有标签（先清空再添加）
	SetTags(ctx context.Context, snippetID int64, tagIDs []int64) error
	// GetTagIDs 查询片段关联的所有标签 ID
	GetTagIDs(ctx context.Context, snippetID int64) ([]int64, error)
}

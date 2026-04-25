package snippetrepo

import (
	"context"

	"github.com/loqbit/ownforge/services/notes/internal/service/snippet/contract"
)

// Repository defines the snippet data access interface.
type Repository interface {
	Create(ctx context.Context, id, ownerID int64, params *contract.CreateSnippetCommand) (*Snippet, error)
	GetByID(ctx context.Context, id int64) (*Snippet, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]Snippet, error)
	ListFiltered(ctx context.Context, ownerID int64, query *contract.ListQuery) ([]Snippet, error) // Compound filters with cursor pagination.
	Update(ctx context.Context, ownerID, id int64, params *contract.UpdateSnippetCommand) (*Snippet, error)
	Delete(ctx context.Context, ownerID, id int64) error // Hard delete.
	SoftDelete(ctx context.Context, ownerID, id int64) error
	Restore(ctx context.Context, ownerID, id int64) error
	SetFavorite(ctx context.Context, ownerID, id int64, isFavorite bool) error
	// groupID == nil moves the snippet to the inbox (ungrouped); sortOrder is written as-is.
	Move(ctx context.Context, ownerID, id int64, groupID *int64, sortOrder int) (*Snippet, error)
	// MaxSortOrderInGroup returns the current maximum sort_order in the target group for append-to-end behavior.
	// groupID == nil refers to the inbox (group_id IS NULL).
	MaxSortOrderInGroup(ctx context.Context, ownerID int64, groupID *int64) (int, error)

	// Snippet-tag many-to-many association.
	// SetTags replaces all tags on the snippet by clearing first and then adding.
	SetTags(ctx context.Context, snippetID int64, tagIDs []int64) error
	// GetTagIDs returns all tag IDs associated with the snippet.
	GetTagIDs(ctx context.Context, snippetID int64) ([]int64, error)
}

package grouprepo

import (
	"context"

	"github.com/ownforge/ownforge/services/notes/internal/service/group/contract"
)

// Repository defines the group data access interface.
type Repository interface {
	Create(ctx context.Context, id, ownerID int64, params *contract.CreateGroupCommand) (*Group, error)
	GetByID(ctx context.Context, id int64) (*Group, error)
	ListByOwner(ctx context.Context, ownerID int64, parentID *int64) ([]Group, error)
	ListAllByOwner(ctx context.Context, ownerID int64) ([]Group, error) // Used by GetTree: query everything once.
	Update(ctx context.Context, ownerID, id int64, params *contract.UpdateGroupCommand) (*Group, error)
	Delete(ctx context.Context, ownerID, id int64) error
	CountChildren(ctx context.Context, id int64) (int, error)
	CountSnippets(ctx context.Context, id int64) (int, error)
}

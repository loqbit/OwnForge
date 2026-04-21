package tagrepo

import (
	"context"

	"github.com/ownforge/ownforge/services/notes/internal/service/tag/contract"
)

// Repository 定义 tag 数据访问接口。
type Repository interface {
	Create(ctx context.Context, id, ownerID int64, params *contract.CreateTagCommand) (*Tag, error)
	GetByID(ctx context.Context, id int64) (*Tag, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]Tag, error)
	ListByIDs(ctx context.Context, ownerID int64, ids []int64) ([]Tag, error)
	Update(ctx context.Context, ownerID, id int64, params *contract.UpdateTagCommand) (*Tag, error)
	Delete(ctx context.Context, ownerID, id int64) error
}

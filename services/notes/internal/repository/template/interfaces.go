package templaterepo

import (
	"context"

	"github.com/loqbit/ownforge/services/notes/internal/service/template/contract"
)

// Repository defines the template data access interface.
type Repository interface {
	Create(ctx context.Context, id, ownerID int64, params *contract.CreateTemplateCommand) (*Template, error)
	GetByID(ctx context.Context, id int64) (*Template, error)
	List(ctx context.Context, ownerID int64, category string) ([]Template, error)
	Update(ctx context.Context, ownerID, id int64, params *contract.UpdateTemplateCommand) (*Template, error)
	Delete(ctx context.Context, ownerID, id int64) error
	CountSystem(ctx context.Context) (int, error)
	CreateBatch(ctx context.Context, templates []Template) error
}

package sharerepo

import (
	"context"
	"time"
)

// Repository 定义 share 数据访问接口。
type Repository interface {
	Create(ctx context.Context, share *Share) (*Share, error)
	GetByID(ctx context.Context, id int64) (*Share, error)
	GetByToken(ctx context.Context, token string) (*Share, error)
	ListByOwner(ctx context.Context, ownerID int64, kind string) ([]Share, error)
	Delete(ctx context.Context, ownerID, id int64) error
	IncrementViewCount(ctx context.Context, id int64) error
	IncrementForkCount(ctx context.Context, id int64) error
	FindActiveToken(ctx context.Context, ownerID, snippetID int64, kind string, now time.Time) (*Share, error)
}

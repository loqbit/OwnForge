package sharestore

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	entshare "github.com/ownforge/ownforge/services/notes/internal/ent/share"
	sharerepo "github.com/ownforge/ownforge/services/notes/internal/repository/share"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the share repository.
type Store struct {
	client *ent.Client
}

// New creates an Ent-backed share repository.
func New(client *ent.Client) sharerepo.Repository {
	return &Store{client: client}
}

// Create inserts a share record.
func (s *Store) Create(ctx context.Context, item *sharerepo.Share) (*sharerepo.Share, error) {
	builder := s.client.Share.Create().
		SetID(item.ID).
		SetToken(item.Token).
		SetKind(entshare.Kind(item.Kind)).
		SetSnippetID(item.SnippetID).
		SetOwnerID(item.OwnerID).
		SetViewCount(item.ViewCount).
		SetForkCount(item.ForkCount)

	if item.PasswordHash != "" {
		builder.SetPasswordHash(item.PasswordHash)
	}
	if item.ExpiresAt != nil {
		builder.SetExpiresAt(*item.ExpiresAt)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapShare(entity), nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*sharerepo.Share, error) {
	entity, err := s.client.Share.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapShare(entity), nil
}

func (s *Store) GetByToken(ctx context.Context, token string) (*sharerepo.Share, error) {
	entity, err := s.client.Share.Query().Where(entshare.TokenEQ(token)).Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapShare(entity), nil
}

func (s *Store) ListByOwner(ctx context.Context, ownerID int64, kind string) ([]sharerepo.Share, error) {
	query := s.client.Share.Query().
		Where(entshare.OwnerIDEQ(ownerID)).
		Order(entshare.ByCreatedAt(sql.OrderDesc()))
	if kind != "" {
		query = query.Where(entshare.KindEQ(entshare.Kind(kind)))
	}

	entities, err := query.All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]sharerepo.Share, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapShare(entity))
	}
	return results, nil
}

func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	count, err := s.client.Share.Query().
		Where(entshare.IDEQ(id), entshare.OwnerIDEQ(ownerID)).
		Count(ctx)
	if err != nil {
		return shared.ParseEntError(err)
	}
	if count == 0 {
		return sharedrepo.ErrNoRows
	}

	return shared.ParseEntError(s.client.Share.DeleteOneID(id).Exec(ctx))
}

func (s *Store) IncrementViewCount(ctx context.Context, id int64) error {
	_, err := s.client.Share.UpdateOneID(id).AddViewCount(1).Save(ctx)
	return shared.ParseEntError(err)
}

func (s *Store) IncrementForkCount(ctx context.Context, id int64) error {
	_, err := s.client.Share.UpdateOneID(id).AddForkCount(1).Save(ctx)
	return shared.ParseEntError(err)
}

func (s *Store) FindActiveToken(ctx context.Context, ownerID, snippetID int64, kind string, now time.Time) (*sharerepo.Share, error) {
	query := s.client.Share.Query().
		Where(
			entshare.OwnerIDEQ(ownerID),
			entshare.SnippetIDEQ(snippetID),
			entshare.KindEQ(entshare.Kind(kind)),
			entshare.Or(
				entshare.ExpiresAtIsNil(),
				entshare.ExpiresAtGT(now),
			),
		).
		Order(entshare.ByCreatedAt(sql.OrderDesc()))

	entity, err := query.First(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapShare(entity), nil
}

func mapShare(entity *ent.Share) *sharerepo.Share {
	if entity == nil {
		return nil
	}

	return &sharerepo.Share{
		ID:           entity.ID,
		Token:        entity.Token,
		Kind:         string(entity.Kind),
		SnippetID:    entity.SnippetID,
		OwnerID:      entity.OwnerID,
		PasswordHash: entity.PasswordHash,
		ExpiresAt:    entity.ExpiresAt,
		ViewCount:    entity.ViewCount,
		ForkCount:    entity.ForkCount,
		CreatedAt:    entity.CreatedAt,
	}
}

var _ sharerepo.Repository = (*Store)(nil)

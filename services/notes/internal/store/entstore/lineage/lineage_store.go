package lineagestore

import (
	"context"

	"github.com/loqbit/ownforge/services/notes/internal/ent"
	entsnippetlineage "github.com/loqbit/ownforge/services/notes/internal/ent/snippetlineage"
	lineagerepo "github.com/loqbit/ownforge/services/notes/internal/repository/lineage"
	"github.com/loqbit/ownforge/services/notes/internal/store/entstore/shared"
)

type Store struct {
	client *ent.Client
}

func New(client *ent.Client) lineagerepo.Repository {
	return &Store{client: client}
}

func (s *Store) Create(ctx context.Context, item *lineagerepo.Lineage) (*lineagerepo.Lineage, error) {
	builder := s.client.SnippetLineage.Create().
		SetID(item.ID).
		SetSnippetID(item.SnippetID).
		SetRelationType(entsnippetlineage.RelationType(item.RelationType))

	if item.SourceSnippetID != nil {
		builder.SetSourceSnippetID(*item.SourceSnippetID)
	}
	if item.SourceShareID != nil {
		builder.SetSourceShareID(*item.SourceShareID)
	}
	if item.SourceUserID != nil {
		builder.SetSourceUserID(*item.SourceUserID)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapLineage(entity), nil
}

func (s *Store) GetBySnippetID(ctx context.Context, snippetID int64) (*lineagerepo.Lineage, error) {
	entity, err := s.client.SnippetLineage.Query().
		Where(entsnippetlineage.SnippetIDEQ(snippetID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapLineage(entity), nil
}

func mapLineage(entity *ent.SnippetLineage) *lineagerepo.Lineage {
	if entity == nil {
		return nil
	}
	return &lineagerepo.Lineage{
		ID:              entity.ID,
		SnippetID:       entity.SnippetID,
		SourceSnippetID: entity.SourceSnippetID,
		SourceShareID:   entity.SourceShareID,
		SourceUserID:    entity.SourceUserID,
		RelationType:    string(entity.RelationType),
		CreatedAt:       entity.CreatedAt,
	}
}

package aimetadata

import (
	"context"

	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/snippetaimetadata"
	aimetadatarepo "github.com/ownforge/ownforge/services/notes/internal/repository/aimetadata"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the AI metadata repository.
type Store struct {
	client *ent.Client
}

// New creates a new AI metadata repository implementation.
func New(client *ent.Client) aimetadatarepo.Repository {
	return &Store{
		client: client,
	}
}

// GetBySnippetID returns AI metadata for the given snippet_id.
func (s *Store) GetBySnippetID(ctx context.Context, snippetID int64) (*aimetadatarepo.AIMetadata, error) {
	record, err := s.client.SnippetAIMetadata.Query().
		Where(snippetaimetadata.IDEQ(snippetID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapAIMetadata(record), nil
}

// Upsert inserts or updates AI metadata.
func (s *Store) Upsert(ctx context.Context, in aimetadatarepo.UpsertInput) error {
	err := s.client.SnippetAIMetadata.Create().
		SetID(in.SnippetID).
		SetOwnerID(in.OwnerID).
		SetSummary(in.Summary).
		SetSuggestedTags(in.SuggestedTags).
		SetExtractedTodos(in.ExtractedTodos).
		SetContentHash(in.ContentHash).
		SetPromptVersion(in.PromptVersion).
		SetModel(in.Model).
		OnConflictColumns(snippetaimetadata.FieldID).
		UpdateSummary().
		UpdateSuggestedTags().
		UpdateExtractedTodos().
		UpdateContentHash().
		UpdatePromptVersion().
		UpdateModel().
		UpdateUpdatedAt().
		Exec(ctx)

	return shared.ParseEntError(err)
}

func mapAIMetadata(entity *ent.SnippetAIMetadata) *aimetadatarepo.AIMetadata {
	return &aimetadatarepo.AIMetadata{
		SnippetID:      entity.ID,
		OwnerID:        entity.OwnerID,
		Summary:        entity.Summary,
		SuggestedTags:  entity.SuggestedTags,
		ExtractedTodos: entity.ExtractedTodos,
		ContentHash:    entity.ContentHash,
		PromptVersion:  entity.PromptVersion,
		Model:          entity.Model,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}
}

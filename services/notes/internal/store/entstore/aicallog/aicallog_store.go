package aicallog

import (
	"context"

	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/aicalllog"
	aicallogrepo "github.com/ownforge/ownforge/services/notes/internal/repository/aicallog"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the aicallog repository.
type Store struct {
	client *ent.Client
}

// New creates an aicallog repository.
func New(client *ent.Client) aicallogrepo.Repository {
	return &Store{client: client}
}

// Insert appends one AI call log entry.
func (s *Store) Insert(ctx context.Context, e *aicallogrepo.Entry) error {
	builder := s.client.AICallLog.Create().
		SetID(e.ID).
		SetOwnerID(e.OwnerID).
		SetSkill(e.Skill).
		SetProvider(e.Provider).
		SetModel(e.Model).
		SetPromptVersion(e.PromptVersion).
		SetInputHash(e.InputHash).
		SetInputTokens(e.InputTokens).
		SetOutputTokens(e.OutputTokens).
		SetCachedTokens(e.CachedTokens).
		SetCostUsd(e.CostUSD).
		SetLatencyMs(e.LatencyMS).
		SetStatus(aicalllog.Status(e.Status))

	if e.SnippetID != nil {
		builder.SetSnippetID(*e.SnippetID)
	}
	if e.Error != "" {
		builder.SetError(truncateError(e.Error, 500))
	}

	_, err := builder.Save(ctx)
	return shared.ParseEntError(err)
}

func truncateError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

package aimetadata

import "time"

// AIMetadata is the repository-layer domain model for AI enrichment output.
type AIMetadata struct {
	SnippetID      int64
	OwnerID        int64 // Owning user, used for authorization checks.
	Summary        string
	SuggestedTags  []string
	ExtractedTodos []map[string]any
	ContentHash    uint32 // FNV-1a hash of the content, used for idempotency checks.
	PromptVersion  string // Prompt version used to generate this result.
	Model          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertInput groups Upsert parameters to keep the function signature short.
type UpsertInput struct {
	SnippetID      int64
	OwnerID        int64
	Summary        string
	SuggestedTags  []string
	ExtractedTodos []map[string]any
	ContentHash    uint32
	PromptVersion  string
	Model          string
}

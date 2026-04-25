package ai

import (
	"context"
	"time"

	"github.com/loqbit/ownforge/services/notes/internal/platform/llm"
	"github.com/loqbit/ownforge/services/notes/internal/service/ai/contract"
	"github.com/loqbit/ownforge/services/notes/internal/service/ai/prompt"
)

// EnrichStats holds observability data for one enrich call and is used by the evaluation harness.
type EnrichStats struct {
	InputTokens  int
	OutputTokens int
	LatencyMS    int
	Model        string
	PromptVer    string
}

// EnrichOnce runs a pure-function version of document enrichment with no DB, idgen, lock, or call log dependencies.
//
// It is only used for:
//   - the ai-eval harness for golden-set regression tests
//   - one-off validation while manually debugging prompts
//
// Production code should use EnrichService.Enrich, which includes idempotency, tracing, and locking.
func EnrichOnce(
	ctx context.Context,
	client llm.Client,
	model string,
	maxTokens int,
	title, content string,
	existingTags []string,
) (*contract.EnrichResult, *EnrichStats, error) {
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	messages := prompt.BuildEnrichMessages(title, content, existingTags)
	started := time.Now()
	resp, err := client.Complete(ctx, &llm.CompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: 0.3,
	})
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return nil, nil, err
	}

	result, err := parseEnrichResponse(resp.Content)
	if err != nil {
		return nil, nil, err
	}

	return result, &EnrichStats{
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		LatencyMS:    latency,
		Model:        model,
		PromptVer:    prompt.PromptVersionEnrich,
	}, nil
}

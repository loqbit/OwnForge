package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"

	"github.com/loqbit/ownforge/services/notes/internal/platform/idgen"
	"github.com/loqbit/ownforge/services/notes/internal/platform/llm"
	"github.com/loqbit/ownforge/services/notes/internal/platform/lock"
	aicallogrepo "github.com/loqbit/ownforge/services/notes/internal/repository/aicallog"
	aimetadatarepo "github.com/loqbit/ownforge/services/notes/internal/repository/aimetadata"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	snippetrepo "github.com/loqbit/ownforge/services/notes/internal/repository/snippet"
	tagrepo "github.com/loqbit/ownforge/services/notes/internal/repository/tag"
	"github.com/loqbit/ownforge/services/notes/internal/service/ai/contract"
	"github.com/loqbit/ownforge/services/notes/internal/service/ai/prompt"
	"go.uber.org/zap"
)

// EnrichService provides AI-based document enrichment.
//
// Main flow: load -> idempotency check -> existing tags -> LLM call -> JSON repair -> write back and record trace data.
// Every LLM call writes a record to ai_call_log, including skipped, failed, and successful runs.
type EnrichService struct {
	snippetRepo    snippetrepo.Repository
	tagRepo        tagrepo.Repository
	aiMetadataRepo aimetadatarepo.Repository
	callLogRepo    aicallogrepo.Repository
	idgenClient    idgen.Client
	llmClient      llm.Client
	locker         lock.Locker
	provider       string // "openai" / "anthropic" / "ollama"...
	model          string // Concrete model ID
	maxTokens      int
	minContentLen  int // Minimum content length; requests below this threshold are skipped
	logger         *zap.Logger
}

// NewEnrichService creates an EnrichService.
func NewEnrichService(
	snippetRepo snippetrepo.Repository,
	tagRepo tagrepo.Repository,
	aiMetadataRepo aimetadatarepo.Repository,
	callLogRepo aicallogrepo.Repository,
	idgenClient idgen.Client,
	llmClient llm.Client,
	locker lock.Locker,
	provider, model string,
	maxTokens int,
	minContentLen int,
	logger *zap.Logger,
) *EnrichService {
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	if minContentLen <= 0 {
		minContentLen = 50
	}
	return &EnrichService{
		snippetRepo:    snippetRepo,
		tagRepo:        tagRepo,
		aiMetadataRepo: aiMetadataRepo,
		callLogRepo:    callLogRepo,
		idgenClient:    idgenClient,
		llmClient:      llmClient,
		locker:         locker,
		provider:       provider,
		model:          model,
		maxTokens:      maxTokens,
		minContentLen:  minContentLen,
		logger:         logger,
	}
}

// Enrich performs AI enrichment on the specified snippet, producing tags, TODOs, and a summary.
func (s *EnrichService) Enrich(ctx context.Context, snippetID int64) (*contract.EnrichResult, error) {
	// 1. Load the document.
	snippet, err := s.snippetRepo.GetByID(ctx, snippetID)
	if err != nil {
		return nil, fmt.Errorf("ai: failed to query snippet %d: %w", snippetID, err)
	}

	// 2. Skip short content without writing a call log; only emit a debug log.
	contentLen := len([]rune(snippet.Content))
	if contentLen < s.minContentLen {
		s.logger.Debug("ai: content too short, skipping enrichment",
			zap.Int64("snippet_id", snippetID),
			zap.Int("content_len", contentLen),
		)
		return nil, nil
	}

	// 3. Mutex: allow only one worker to process a given snippet at a time to avoid duplicate LLM calls.
	//    A 60-second TTL prevents deadlocks. Single calls usually take under 3 seconds, so 60 seconds is ample without blocking later requests for too long.
	//    If the lock cannot be acquired, skip immediately. Another worker will handle it, and the content_hash it writes back
	//    is the same one this run would produce, so there is no need to wait in line.
	lockKey := fmt.Sprintf("ai:enrich:lock:%d", snippetID)
	release, err := s.locker.Acquire(ctx, lockKey, 60*time.Second)
	if err != nil {
		if errors.Is(err, lock.ErrNotAcquired) {
			s.logger.Debug("ai: the same snippet is already being processed by another worker, skipping",
				zap.Int64("snippet_id", snippetID),
			)
			return nil, nil
		}
		// Non-conflict lock errors such as Redis failures are logged but do not block the main flow; processing falls back to lock-free mode.
		s.logger.Warn("ai: failed to acquire lock, falling back to unlocked processing",
			zap.Int64("snippet_id", snippetID), zap.Error(err),
		)
	} else {
		defer release()
	}

	// 4. Idempotency check: skip when both content_hash and prompt_version are unchanged.
	currentHash := contentHash(snippet.Content)
	aiMeta, err := s.aiMetadataRepo.GetBySnippetID(ctx, snippetID)
	if err != nil && !errors.Is(err, sharedrepo.ErrNoRows) {
		return nil, fmt.Errorf("ai: failed to query AI metadata: %w", err)
	}
	if aiMeta != nil && aiMeta.ContentHash == currentHash && aiMeta.PromptVersion == prompt.PromptVersionEnrich {
		s.logger.Debug("ai: content and prompt are unchanged, skipping",
			zap.Int64("snippet_id", snippetID),
		)
		return nil, nil
	}

	// 4. Existing tag context.
	existingTagNames := s.getExistingTagNames(ctx, snippet.OwnerID)

	// 5. Call the LLM with tracing.
	messages := prompt.BuildEnrichMessages(snippet.Title, snippet.Content, existingTagNames)
	started := time.Now()
	resp, llmErr := s.llmClient.Complete(ctx, &llm.CompletionRequest{
		Model:       s.model,
		Messages:    messages,
		MaxTokens:   s.maxTokens,
		Temperature: 0.3,
	})
	latency := int(time.Since(started).Milliseconds())
	inputHash := hashMessages(messages)

	// 5a. On LLM error, write the call log and return.
	if llmErr != nil {
		s.writeCallLog(ctx, &aicallogrepo.Entry{
			OwnerID:       snippet.OwnerID,
			Skill:         "enrich",
			SnippetID:     &snippetID,
			Provider:      s.provider,
			Model:         s.model,
			PromptVersion: prompt.PromptVersionEnrich,
			InputHash:     inputHash,
			LatencyMS:     latency,
			Status:        aicallogrepo.StatusLLMError,
			Error:         llmErr.Error(),
		})
		return nil, fmt.Errorf("ai: LLM call failed: %w", llmErr)
	}

	// 6. Parse the response with JSON repair.
	result, parseErr := parseEnrichResponse(resp.Content)
	if parseErr != nil {
		s.writeCallLog(ctx, &aicallogrepo.Entry{
			OwnerID:       snippet.OwnerID,
			Skill:         "enrich",
			SnippetID:     &snippetID,
			Provider:      s.provider,
			Model:         s.model,
			PromptVersion: prompt.PromptVersionEnrich,
			InputHash:     inputHash,
			InputTokens:   resp.InputTokens,
			OutputTokens:  resp.OutputTokens,
			LatencyMS:     latency,
			Status:        aicallogrepo.StatusParseError,
			Error:         parseErr.Error(),
		})
		return nil, fmt.Errorf("ai: failed to parse LLM response: %w", parseErr)
	}

	// 7. Write back AI metadata.
	if err := s.aiMetadataRepo.Upsert(ctx, aimetadatarepo.UpsertInput{
		SnippetID:      snippetID,
		OwnerID:        snippet.OwnerID,
		Summary:        result.Summary,
		SuggestedTags:  result.AutoTags,
		ExtractedTodos: todosToMap(result.Todos),
		ContentHash:    currentHash,
		PromptVersion:  prompt.PromptVersionEnrich,
		Model:          s.model,
	}); err != nil {
		return nil, fmt.Errorf("ai: failed to write back AI metadata: %w", err)
	}

	// 8. Record a successful trace entry.
	s.writeCallLog(ctx, &aicallogrepo.Entry{
		OwnerID:       snippet.OwnerID,
		Skill:         "enrich",
		SnippetID:     &snippetID,
		Provider:      s.provider,
		Model:         s.model,
		PromptVersion: prompt.PromptVersionEnrich,
		InputHash:     inputHash,
		InputTokens:   resp.InputTokens,
		OutputTokens:  resp.OutputTokens,
		LatencyMS:     latency,
		Status:        aicallogrepo.StatusSuccess,
	})

	s.logger.Info("ai: enrichment completed",
		zap.Int64("snippet_id", snippetID),
		zap.Int("latency_ms", latency),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
	)

	return result, nil
}

// writeCallLog writes one call-log record. Failures are only logged and do not affect the main flow.
func (s *EnrichService) writeCallLog(ctx context.Context, entry *aicallogrepo.Entry) {
	// Call-log writes use a separate context so trace data is not lost if the main context has already been canceled.
	logCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id, err := s.idgenClient.NextID(logCtx)
	if err != nil {
		s.logger.Warn("ai: failed to allocate call log ID, skipping log write", zap.Error(err))
		return
	}
	entry.ID = id
	if err := s.callLogRepo.Insert(logCtx, entry); err != nil {
		s.logger.Warn("ai: failed to write call log", zap.Error(err))
	}
}

// getExistingTagNames fetches the user's existing tag names. Failures do not block the main flow.
func (s *EnrichService) getExistingTagNames(ctx context.Context, ownerID int64) []string {
	tags, err := s.tagRepo.ListByOwner(ctx, ownerID)
	if err != nil {
		s.logger.Warn("ai: failed to query existing tags",
			zap.Int64("owner_id", ownerID),
			zap.Error(err),
		)
		return nil
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names
}

// contentHash computes the FNV-1a hash of the content for idempotency checks.
func contentHash(content string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(content))
	return h.Sum32()
}

// hashMessages fingerprints the prompt messages for trace correlation and deduplication analysis.
func hashMessages(msgs []llm.Message) string {
	h := sha256.New()
	for _, m := range msgs {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8]) // 16 hex chars = 64 bits, enough for deduplication.
}

// jsonBlockRE matches ```json ... ``` or ``` ... ``` code blocks.
var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// parseEnrichResponse parses enrichment output from the LLM.
//
// Robustness strategy, tried in order:
//  1. Direct JSON parsing
//  2. Extract the first ```...``` code block
//  3. Extract the content between the first { and the last }
//
// Local small models such as Qwen often return markdown or leading explanations, so repair is required.
func parseEnrichResponse(raw string) (*contract.EnrichResult, error) {
	candidates := extractJSONCandidates(raw)
	var lastErr error
	for _, c := range candidates {
		var result contract.EnrichResult
		if err := json.Unmarshal([]byte(c), &result); err == nil {
			return &result, nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no JSON candidate found")
	}
	return nil, fmt.Errorf("JSON parse failed (raw=%q): %w", truncate(raw, 200), lastErr)
}

// extractJSONCandidates extracts possible JSON candidates from text in priority order.
func extractJSONCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	out := []string{trimmed}

	// Candidate 2: JSON inside a code block
	if m := jsonBlockRE.FindStringSubmatch(raw); len(m) > 1 {
		out = append(out, strings.TrimSpace(m[1]))
	}

	// Candidate 3: the content from the first { to the last }
	if start := strings.IndexByte(trimmed, '{'); start >= 0 {
		if end := strings.LastIndexByte(trimmed, '}'); end > start {
			out = append(out, trimmed[start:end+1])
		}
	}
	return out
}

// todosToMap converts TodoItem values into map form, compatible with Ent JSON storage fields.
func todosToMap(todos []contract.TodoItem) []map[string]any {
	result := make([]map[string]any, len(todos))
	for i, t := range todos {
		result[i] = map[string]any{
			"text":     t.Text,
			"priority": t.Priority,
			"done":     t.Done,
		}
	}
	return result
}

// truncate shortens strings for logging.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

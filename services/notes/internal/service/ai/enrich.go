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

	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	"github.com/ownforge/ownforge/services/notes/internal/platform/llm"
	"github.com/ownforge/ownforge/services/notes/internal/platform/lock"
	aicallogrepo "github.com/ownforge/ownforge/services/notes/internal/repository/aicallog"
	aimetadatarepo "github.com/ownforge/ownforge/services/notes/internal/repository/aimetadata"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	snippetrepo "github.com/ownforge/ownforge/services/notes/internal/repository/snippet"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/contract"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/prompt"
	"go.uber.org/zap"
)

// EnrichService 文档 AI 增值服务。
//
// 核心流程：读取 → 幂等检查 → 已有标签 → LLM 调用 → JSON 修复 → 写回 + 落 trace。
// 每次 LLM 调用都会在 ai_call_log 表落一条记录（包括跳过/失败/成功）。
type EnrichService struct {
	snippetRepo    snippetrepo.Repository
	tagRepo        tagrepo.Repository
	aiMetadataRepo aimetadatarepo.Repository
	callLogRepo    aicallogrepo.Repository
	idgenClient    idgen.Client
	llmClient      llm.Client
	locker         lock.Locker
	provider       string // "openai" / "anthropic" / "ollama"...
	model          string // 具体模型 ID
	maxTokens      int
	minContentLen  int // 内容最小长度，低于此值跳过处理
	logger         *zap.Logger
}

// NewEnrichService 创建 EnrichService。
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

// Enrich 对指定 snippet 执行 AI 增值（标签 + TODO + 摘要）。
func (s *EnrichService) Enrich(ctx context.Context, snippetID int64) (*contract.EnrichResult, error) {
	// 1. 读取文档
	snippet, err := s.snippetRepo.GetByID(ctx, snippetID)
	if err != nil {
		return nil, fmt.Errorf("ai: 查询 snippet %d 失败: %w", snippetID, err)
	}

	// 2. 短文本过滤（不落 call log，只 debug 日志）
	contentLen := len([]rune(snippet.Content))
	if contentLen < s.minContentLen {
		s.logger.Debug("ai: 内容过短，跳过增值",
			zap.Int64("snippet_id", snippetID),
			zap.Int("content_len", contentLen),
		)
		return nil, nil
	}

	// 3. 互斥锁：同一 snippet 同时只允许一个 worker 处理，避免重复 LLM 调用。
	//    TTL 60s 防死锁（单次调用通常 <3s，60s 足够且不会卡后续请求太久）。
	//    获取不到就直接跳过——另一个 worker 会处理，它写回的 content_hash
	//    就是本次要写的那个，没必要排队等。
	lockKey := fmt.Sprintf("ai:enrich:lock:%d", snippetID)
	release, err := s.locker.Acquire(ctx, lockKey, 60*time.Second)
	if err != nil {
		if errors.Is(err, lock.ErrNotAcquired) {
			s.logger.Debug("ai: 同 snippet 正在被其他 worker 处理，跳过",
				zap.Int64("snippet_id", snippetID),
			)
			return nil, nil
		}
		// Redis 故障等非锁冲突错误：记日志但不阻塞主流程，退化为无锁模式
		s.logger.Warn("ai: 获取锁失败，退化为无锁处理",
			zap.Int64("snippet_id", snippetID), zap.Error(err),
		)
	} else {
		defer release()
	}

	// 4. 幂等检查：content_hash 未变 且 prompt_version 未变 则跳过
	currentHash := contentHash(snippet.Content)
	aiMeta, err := s.aiMetadataRepo.GetBySnippetID(ctx, snippetID)
	if err != nil && !errors.Is(err, sharedrepo.ErrNoRows) {
		return nil, fmt.Errorf("ai: 查询 AI 元数据失败: %w", err)
	}
	if aiMeta != nil && aiMeta.ContentHash == currentHash && aiMeta.PromptVersion == prompt.PromptVersionEnrich {
		s.logger.Debug("ai: content 与 prompt 均未变，跳过",
			zap.Int64("snippet_id", snippetID),
		)
		return nil, nil
	}

	// 4. 已有标签上下文
	existingTagNames := s.getExistingTagNames(ctx, snippet.OwnerID)

	// 5. 调用 LLM（带 trace）
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

	// 5a. LLM 错误 → 落 call log → 返回
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
		return nil, fmt.Errorf("ai: LLM 调用失败: %w", llmErr)
	}

	// 6. 解析响应（带 JSON 修复）
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
		return nil, fmt.Errorf("ai: 解析 LLM 响应失败: %w", parseErr)
	}

	// 7. 写回 AI 元数据
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
		return nil, fmt.Errorf("ai: 写回 AI 元数据失败: %w", err)
	}

	// 8. 成功落 trace
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

	s.logger.Info("ai: 增值完成",
		zap.Int64("snippet_id", snippetID),
		zap.Int("latency_ms", latency),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
	)

	return result, nil
}

// writeCallLog 落一条 call log，失败只记日志，不影响主流程。
func (s *EnrichService) writeCallLog(ctx context.Context, entry *aicallogrepo.Entry) {
	// call log 的写入独立用一个 context，避免主 ctx 已 cancel 导致 trace 丢失
	logCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	id, err := s.idgenClient.NextID(logCtx)
	if err != nil {
		s.logger.Warn("ai: 分配 call log ID 失败，放弃记录", zap.Error(err))
		return
	}
	entry.ID = id
	if err := s.callLogRepo.Insert(logCtx, entry); err != nil {
		s.logger.Warn("ai: 写入 call log 失败", zap.Error(err))
	}
}

// getExistingTagNames 查询用户已有的标签名称列表。失败不阻断主流程。
func (s *EnrichService) getExistingTagNames(ctx context.Context, ownerID int64) []string {
	tags, err := s.tagRepo.ListByOwner(ctx, ownerID)
	if err != nil {
		s.logger.Warn("ai: 查询已有标签失败",
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

// contentHash 计算内容的 FNV-1a hash（用于幂等检查）。
func contentHash(content string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(content))
	return h.Sum32()
}

// hashMessages 对 prompt messages 生成指纹，用于 trace 关联 / 去重分析。
func hashMessages(msgs []llm.Message) string {
	h := sha256.New()
	for _, m := range msgs {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8]) // 16 hex chars = 64 bit，足够去重
}

// jsonBlockRE 匹配 ```json ... ``` 或 ``` ... ``` 代码块。
var jsonBlockRE = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// parseEnrichResponse 从 LLM 输出解析增值结果。
//
// 健壮性策略（按顺序尝试）：
//   1. 直接 JSON 解析
//   2. 提取第一个 ```...``` 代码块
//   3. 提取第一个 { 到最后一个 } 之间的内容
//
// 本地小模型（Qwen 等）常会带 markdown / 前置说明，必须做 repair。
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
		lastErr = errors.New("未找到 JSON 候选")
	}
	return nil, fmt.Errorf("JSON 解析失败 (raw=%q): %w", truncate(raw, 200), lastErr)
}

// extractJSONCandidates 从文本中抽取可能的 JSON 候选，按优先级排序。
func extractJSONCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	out := []string{trimmed}

	// 候选 2：代码块内的 JSON
	if m := jsonBlockRE.FindStringSubmatch(raw); len(m) > 1 {
		out = append(out, strings.TrimSpace(m[1]))
	}

	// 候选 3：第一个 { 到最后一个 }
	if start := strings.IndexByte(trimmed, '{'); start >= 0 {
		if end := strings.LastIndexByte(trimmed, '}'); end > start {
			out = append(out, trimmed[start:end+1])
		}
	}
	return out
}

// todosToMap 将 TodoItem 转为 map 格式（兼容 ent JSON 字段存储）。
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

// truncate 截断字符串用于日志。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

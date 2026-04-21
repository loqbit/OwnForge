package ai

import (
	"context"
	"time"

	"github.com/ownforge/ownforge/services/notes/internal/platform/llm"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/contract"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/prompt"
)

// EnrichStats 一次 enrich 调用的观测数据（给 eval harness 用）。
type EnrichStats struct {
	InputTokens  int
	OutputTokens int
	LatencyMS    int
	Model        string
	PromptVer    string
}

// EnrichOnce 执行一次文档增值的纯函数版本：不依赖 DB / idgen / lock / call log。
//
// 仅用于：
//   - ai-eval harness：golden set 回归测试
//   - 手动调试 prompt 时的一次性验证
//
// 生产路径请使用 EnrichService.Enrich（带幂等、trace、锁）。
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

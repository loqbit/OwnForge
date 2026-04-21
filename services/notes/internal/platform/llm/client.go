package llm

import "context"

// Message 对话消息。
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// CompletionRequest LLM 调用请求。
type CompletionRequest struct {
	Model       string    // 由配置决定，如 "claude-3-haiku-20241022" 或 "qwen2.5:7b"
	Messages    []Message
	MaxTokens   int
	Temperature float64
}

// CompletionResponse LLM 调用响应。
type CompletionResponse struct {
	Content      string // 模型输出的文本内容
	InputTokens  int    // 输入 token 数（用于计费和观测）
	OutputTokens int    // 输出 token 数
}

// Client 统一的 LLM 调用接口。
//
// 所有 Provider（Anthropic / OpenAI / Ollama / Qwen / 任意 OpenAI Compatible 端点）
// 都实现此接口。业务代码只依赖此接口，不感知具体 Provider。
type Client interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

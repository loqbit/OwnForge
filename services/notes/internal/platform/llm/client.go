package llm

import "context"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// CompletionRequest is an LLM completion request.
type CompletionRequest struct {
	Model       string // configured model, such as "claude-3-haiku-20241022" or "qwen2.5:7b"
	Messages    []Message
	MaxTokens   int
	Temperature float64
}

// CompletionResponse is an LLM completion response.
type CompletionResponse struct {
	Content      string // text content returned by the model
	InputTokens  int    // input token count (for billing and observability)
	OutputTokens int    // output token count
}

// Client is the common LLM invocation interface.
//
// All providers, including Anthropic, OpenAI, Ollama, Qwen, and any OpenAI-compatible endpoint,
// implement this interface. Business code depends only on it and stays provider-agnostic.
type Client interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAICompatClient 基于 OpenAI Chat Completions API 协议的通用实现。
//
// 兼容所有支持 /v1/chat/completions 端点的服务：
//   - OpenAI:  baseURL = "https://api.openai.com/v1"
//   - Ollama:  baseURL = "http://localhost:11434/v1"
//   - Qwen:    baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
//   - 任意代理: 用户自填 baseURL
type OpenAICompatClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewOpenAICompatClient 创建 OpenAI Compatible 客户端。
func NewOpenAICompatClient(baseURL, apiKey string) *OpenAICompatClient {
	return &OpenAICompatClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ── OpenAI API 请求/响应结构 ──

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete 调用 OpenAI Compatible API 完成对话。
func (c *OpenAICompatClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 构造请求体
	messages := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}

	body := openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm: 序列化请求失败: %w", err)
	}

	// 构造 HTTP 请求
	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm: 读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm: API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("llm: 解析响应失败: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("llm: API 错误: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("llm: API 返回空 choices")
	}

	return &CompletionResponse{
		Content:      result.Choices[0].Message.Content,
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}, nil
}

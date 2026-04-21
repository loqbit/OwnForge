package aimetadata

import "time"

// AIMetadata 是 AI 增值结果的仓储层领域模型。
type AIMetadata struct {
	SnippetID      int64
	OwnerID        int64 // 所属用户，用于权限校验
	Summary        string
	SuggestedTags  []string
	ExtractedTodos []map[string]any
	ContentHash    uint32 // 内容 FNV-1a hash，幂等检查用
	PromptVersion  string // 生成该结果使用的 prompt 版本
	Model          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertInput 封装 Upsert 的参数，避免参数列表过长。
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

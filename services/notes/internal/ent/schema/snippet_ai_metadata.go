package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SnippetAIMetadata 存储 AI 对文档的增值分析结果。
//
// 与 Snippet 是 1:1 关系（snippet_id 是主键）。
// 独立于 Snippet 表，AI 数据可以独立清空、重跑、迁移，不影响业务。
type SnippetAIMetadata struct {
	ent.Schema
}

// Fields of the SnippetAIMetadata.
func (SnippetAIMetadata) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Comment("snippet_id，与 snippet 表 1:1 对应"),

		field.Int64("owner_id").
			Comment("文档所有者 ID（冗余存储，方便查询）"),

		field.Text("summary").
			Optional().
			Comment("AI 生成的一句话摘要"),

		field.JSON("suggested_tags", []string{}).
			Optional().
			Comment("AI 建议的标签候选列表（用户确认后才走正式 Tag 系统）"),

		field.JSON("extracted_todos", []map[string]any{}).
			Optional().
			Comment("AI 提取的结构化待办 [{text, priority, done}]"),

		// ── 幂等与版本追踪 ──
		field.Uint32("content_hash").
			Default(0).
			Comment("内容的 FNV-1a hash，内容未变则跳过重算（原 ai_version）"),

		field.String("prompt_version").
			Default("v1").
			MaxLen(32).
			Comment("生成该结果使用的 Prompt 版本，prompt 改版后可灰度重算"),

		field.String("model").
			Optional().
			MaxLen(100).
			Comment("生成该结果的 LLM 模型名称"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("首次生成时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("最后更新时间"),
	}
}

// Indexes of the SnippetAIMetadata.
func (SnippetAIMetadata) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
	}
}

package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AICallLog 记录每次 LLM 调用的 trace 信息。
//
// 这是 AI 产品的"黑匣子"：每次调用的入参 hash、模型、token、耗时、成本、结果都落库。
// 用途：成本分析、prompt A/B、失败排查、用户配额、效果评估。
//
// 保留策略：30 天热数据完整保留；>30 天聚合到 ai_usage_daily 后可清理。
type AICallLog struct {
	ent.Schema
}

// Fields of the AICallLog.
func (AICallLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Comment("雪花 ID"),

		field.Int64("owner_id").
			Comment("归属用户（用于配额、账单、数据隔离）"),

		// ── 业务上下文 ──
		field.String("skill").
			MaxLen(64).
			Comment("技能名称，如 enrich / weekly_report / smart_search"),

		field.Int64("snippet_id").
			Optional().
			Nillable().
			Comment("关联的 snippet（如有），方便按文档回溯"),

		// ── 模型与 Prompt ──
		field.String("provider").
			MaxLen(32).
			Comment("openai / anthropic / ollama / ..."),

		field.String("model").
			MaxLen(100).
			Comment("具体模型 ID，如 gpt-4o-mini / claude-3-5-sonnet"),

		field.String("prompt_version").
			MaxLen(32).
			Comment("Prompt 版本号，用于回归对比"),

		// ── 调用指纹（便于命中 prompt cache + 去重分析） ──
		field.String("input_hash").
			MaxLen(64).
			Comment("input messages 的 hash（sha256 前 16 字节 hex）"),

		// ── Token & 成本 ──
		field.Int("input_tokens").
			Default(0).
			Comment("输入 token 数"),

		field.Int("output_tokens").
			Default(0).
			Comment("输出 token 数"),

		field.Int("cached_tokens").
			Default(0).
			Comment("命中 prompt cache 的 token 数"),

		field.Float("cost_usd").
			Default(0).
			Comment("本次调用估算成本（美元）"),

		// ── 性能与结果 ──
		field.Int("latency_ms").
			Default(0).
			Comment("调用耗时（毫秒）"),

		field.Enum("status").
			Values("success", "parse_error", "llm_error", "skipped", "timeout").
			Comment("调用结果状态"),

		field.String("error").
			Optional().
			MaxLen(500).
			Comment("错误信息（截断），status != success 时填充"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("调用时间"),
	}
}

// Indexes of the AICallLog.
func (AICallLog) Indexes() []ent.Index {
	return []ent.Index{
		// 按用户 + 时间查询（账单、配额）
		index.Fields("owner_id", "created_at"),
		// 按 skill + 时间（效果分析）
		index.Fields("skill", "created_at"),
		// 按 snippet 回溯
		index.Fields("snippet_id"),
		// 按 created_at 单独建索引，方便按时间清理/归档
		index.Fields("created_at"),
	}
}

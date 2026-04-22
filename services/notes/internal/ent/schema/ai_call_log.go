package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AICallLog records trace information for each LLM call.
//
// This is the AI product's black box: input hashes, model, token counts, latency, cost, and result are all persisted for every call.
// It is used for cost analysis, prompt A/B testing, failure investigation, user quotas, and quality evaluation.
//
// Retention policy: keep full hot data for 30 days; after that, once it is aggregated into ai_usage_daily, it can be cleaned up.
type AICallLog struct {
	ent.Schema
}

// Fields of the AICallLog.
func (AICallLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Comment("Snowflake ID"),

		field.Int64("owner_id").
			Comment("owner user (for quota, billing, and data isolation)"),

		// Business context
		field.String("skill").
			MaxLen(64).
			Comment("skill name, for example enrich / weekly_report / smart_search"),

		field.Int64("snippet_id").
			Optional().
			Nillable().
			Comment("associated snippet if any, for document-level tracing"),

		// Model and prompt
		field.String("provider").
			MaxLen(32).
			Comment("openai / anthropic / ollama / ..."),

		field.String("model").
			MaxLen(100).
			Comment("specific model ID, for example gpt-4o-mini / claude-3-5-sonnet"),

		field.String("prompt_version").
			MaxLen(32).
			Comment("prompt version, used for regression comparison"),

		// Call fingerprint, useful for prompt-cache hits and deduplication analysis
		field.String("input_hash").
			MaxLen(64).
			Comment("hash of input messages (first 16 bytes of sha256 as hex)"),

		// Tokens and cost
		field.Int("input_tokens").
			Default(0).
			Comment("input token count"),

		field.Int("output_tokens").
			Default(0).
			Comment("output token count"),

		field.Int("cached_tokens").
			Default(0).
			Comment("prompt cache hit token count"),

		field.Float("cost_usd").
			Default(0).
			Comment("estimated cost of this call (USD)"),

		// Performance and result
		field.Int("latency_ms").
			Default(0).
			Comment("call latency (ms)"),

		field.Enum("status").
			Values("success", "parse_error", "llm_error", "skipped", "timeout").
			Comment("call result status"),

		field.String("error").
			Optional().
			MaxLen(500).
			Comment("truncated error message, populated when status != success"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("call time"),
	}
}

// Indexes of the AICallLog.
func (AICallLog) Indexes() []ent.Index {
	return []ent.Index{
		// Query by user and time for billing and quotas
		index.Fields("owner_id", "created_at"),
		// Query by skill and time for effectiveness analysis
		index.Fields("skill", "created_at"),
		// Trace back by snippet
		index.Fields("snippet_id"),
		// Add a standalone index on created_at to simplify time-based cleanup and archiving
		index.Fields("created_at"),
	}
}

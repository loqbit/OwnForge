package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AIUsageDaily 按天/用户/模型/skill 聚合的 AI 使用量统计。
//
// 由定时任务（cron / scheduled worker）从 ai_call_log 聚合产生。
// 长期保留：一行 ~100 字节，单用户单年 ~10 KB，千人级别长期总量也在 MB 级。
//
// 用途：用户月账单、成本 Dashboard、模型使用占比、skill 热度分析。
type AIUsageDaily struct {
	ent.Schema
}

// Fields of the AIUsageDaily.
func (AIUsageDaily) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Unique().
			Comment("雪花 ID"),

		// ── 聚合维度（复合唯一键 date + owner_id + model + skill） ──
		field.Time("date").
			SchemaType(map[string]string{
				"postgres": "date",
				"sqlite3":  "date",
				"mysql":    "date",
			}).
			Comment("统计日期（仅 YYYY-MM-DD，UTC）"),

		field.Int64("owner_id").
			Comment("归属用户"),

		field.String("provider").
			MaxLen(32).
			Comment("LLM 服务商"),

		field.String("model").
			MaxLen(100).
			Comment("模型 ID"),

		field.String("skill").
			MaxLen(64).
			Comment("技能名称"),

		// ── 聚合指标 ──
		field.Int("call_count").
			Default(0).
			Comment("当日调用次数"),

		field.Int("success_count").
			Default(0).
			Comment("成功次数"),

		field.Int("error_count").
			Default(0).
			Comment("失败次数"),

		field.Int64("input_tokens").
			Default(0).
			Comment("输入 token 累计"),

		field.Int64("output_tokens").
			Default(0).
			Comment("输出 token 累计"),

		field.Int64("cached_tokens").
			Default(0).
			Comment("prompt cache 命中 token 累计"),

		field.Float("cost_usd").
			Default(0).
			Comment("当日估算成本（美元）"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("聚合任务最后更新时间"),
	}
}

// Indexes of the AIUsageDaily.
func (AIUsageDaily) Indexes() []ent.Index {
	return []ent.Index{
		// 聚合维度的唯一约束（UPSERT 的依据）
		index.Fields("date", "owner_id", "provider", "model", "skill").Unique(),
		// 用户账单：按用户 + 时间范围
		index.Fields("owner_id", "date"),
	}
}

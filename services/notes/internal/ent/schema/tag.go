package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Tag 标签实体，通过多对多关系标注 Snippet。
type Tag struct {
	ent.Schema
}

// Fields of the Tag.
func (Tag) Fields() []ent.Field {
	return []ent.Field{
		// 主键：由 id-generator 雪花算法生成
		field.Int64("id").
			Comment("雪花算法生成的全局唯一 ID"),

		field.Int64("owner_id").
			Positive().
			Comment("标签拥有者的用户 ID"),

		field.String("name").
			NotEmpty().
			MaxLen(30).
			Comment("标签名称"),

		field.String("color").
			Optional().
			MaxLen(7).
			Default("#6366f1").
			Comment("标签颜色 HEX 值"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
	}
}

// Edges of the Tag.
func (Tag) Edges() []ent.Edge {
	return []ent.Edge{
		// Tag ↔ Snippet（多对多，反向引用）
		edge.From("snippets", Snippet.Type).
			Ref("tags"),
	}
}

// Indexes of the Tag.
func (Tag) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("owner_id", "name").Unique(),
	}
}

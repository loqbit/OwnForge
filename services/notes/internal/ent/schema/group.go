package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Group 分组实体，用于组织和分类 Snippet。
// 支持嵌套目录结构（通过 parent_id 自引用）。
type Group struct {
	ent.Schema
}

// Fields of the Group.
func (Group) Fields() []ent.Field {
	return []ent.Field{
		// 主键：由 id-generator 雪花算法生成
		field.Int64("id").
			Comment("雪花算法生成的全局唯一 ID"),

		field.Int64("owner_id").
			Positive().
			Comment("分组拥有者的用户 ID"),

		field.Int64("parent_id").
			Optional().
			Nillable().
			Comment("父分组 ID，nil 表示顶级分组"),

		field.String("name").
			NotEmpty().
			MaxLen(60).
			Comment("分组名称"),

		field.String("description").
			Optional().
			MaxLen(200).
			Default("").
			Comment("分组描述"),

		field.Int("sort_order").
			Default(0).
			Comment("排序权重，值越小越靠前"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("最后更新时间"),
	}
}

// Edges of the Group.
func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		// Group → Snippet（一对多）
		edge.To("snippets", Snippet.Type),

		// 自引用：parent → children
		edge.To("children", Group.Type).
			From("parent").
			Field("parent_id").
			Unique(),
	}
}

// Indexes of the Group.
func (Group) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("owner_id", "name").Unique(),
		index.Fields("owner_id", "parent_id"),
	}
}

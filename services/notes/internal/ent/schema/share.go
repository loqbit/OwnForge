package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Share 文档分享实体。
type Share struct {
	ent.Schema
}

// Fields of the Share.
func (Share) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("雪花算法生成的全局唯一 ID"),
		field.String("token").
			Unique().
			NotEmpty().
			MaxLen(64).
			Comment("公开分享 token"),
		field.Enum("kind").
			Values("article", "template").
			Default("article").
			Comment("分享类型：article=文章 template=模板"),
		field.Int64("snippet_id").
			Positive().
			Comment("关联的文档 ID"),
		field.Int64("owner_id").
			Positive().
			Comment("分享创建者 ID"),
		field.String("password_hash").
			Optional().
			MaxLen(255).
			Comment("可选的密码 hash"),
		field.Time("expires_at").
			Optional().
			Nillable().
			Comment("可选的过期时间"),
		field.Int("view_count").
			Default(0).
			Comment("浏览次数"),
		field.Int("fork_count").
			Default(0).
			Comment("模板 Fork 次数"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
	}
}

// Edges of the Share.
func (Share) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("snippet", Snippet.Type).
			Ref("shares").
			Field("snippet_id").
			Unique().
			Required().
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

// Indexes of the Share.
func (Share) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("snippet_id"),
		index.Fields("kind"),
	}
}

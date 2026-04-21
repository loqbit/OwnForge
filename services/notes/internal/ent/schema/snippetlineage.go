package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SnippetLineage 记录文档来源关系。
type SnippetLineage struct {
	ent.Schema
}

func (SnippetLineage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("雪花算法生成的全局唯一 ID"),
		field.Int64("snippet_id").
			Positive().
			Unique().
			Comment("当前文档 ID"),
		field.Int64("source_snippet_id").
			Optional().
			Nillable().
			Comment("来源文档 ID"),
		field.Int64("source_share_id").
			Optional().
			Nillable().
			Comment("来源分享 ID"),
		field.Int64("source_user_id").
			Optional().
			Nillable().
			Comment("来源作者 ID"),
		field.Enum("relation_type").
			Values("fork", "import", "duplicate", "template").
			Default("import").
			Comment("来源关系类型"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
	}
}

func (SnippetLineage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_snippet_id"),
		index.Fields("source_share_id"),
		index.Fields("relation_type"),
	}
}

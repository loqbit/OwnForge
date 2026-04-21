package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Template 文档模板实体。
// 支持系统预置模板（is_system=true, owner_id=0）和用户个人模板。
type Template struct {
	ent.Schema
}

// Fields of the Template.
func (Template) Fields() []ent.Field {
	return []ent.Field{
		// 主键：由 id-generator 雪花算法生成
		field.Int64("id").
			Comment("雪花算法生成的全局唯一 ID"),

		field.Int64("owner_id").
			Default(0).
			Comment("创建者用户 ID，0 = 系统内置"),

		field.String("name").
			NotEmpty().
			MaxLen(100).
			Comment("模板名称"),

		field.String("description").
			Optional().
			Default("").
			MaxLen(500).
			Comment("模板描述"),

		field.Text("content").
			Comment("模板内容（Markdown）"),

		field.String("language").
			Default("markdown").
			MaxLen(30).
			Comment("语言标识"),

		field.String("category").
			Default("general").
			MaxLen(30).
			Comment("分类：general / meeting / tech_design / weekly_report"),

		field.Bool("is_system").
			Default(false).
			Comment("是否系统预置模板"),

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

// Indexes of the Template.
func (Template) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("category"),
		index.Fields("is_system"),
	}
}

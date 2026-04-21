package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Profile holds the schema definition for the Profile entity.
type Profile struct {
	ent.Schema
}

// Fields of the Profile.
func (Profile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive(),

		field.String("nickname").
			MaxLen(32).
			Default("").
			Comment("用户昵称"),

		field.String("avatar_url").
			MaxLen(512).
			Default("").
			Comment("用户头像URL"),

		field.String("bio").
			MaxLen(256).
			Default("").
			Comment("个性签名"),

		field.String("birthday").
			MaxLen(10).
			Default("").
			Comment("生日，格式 YYYY-MM-DD"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("最后更新时间"),
	}
}

// Edges of the Profile.
func (Profile) Edges() []ent.Edge {
	return []ent.Edge{
		// 1对1：一个 Profile 属于一个 User
		edge.From("user", User.Type).Ref("profile").Unique().Required(),
	}
}

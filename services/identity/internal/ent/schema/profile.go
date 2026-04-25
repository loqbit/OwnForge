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
			Comment("user nickname"),

		field.String("avatar_url").
			MaxLen(512).
			Default("").
			Comment("user avatar URL"),

		field.String("bio").
			MaxLen(256).
			Default("").
			Comment("bio"),

		field.String("birthday").
			MaxLen(10).
			Default("").
			Comment("birthday in YYYY-MM-DD format"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("last update time"),
	}
}

// Edges of the Profile.
func (Profile) Edges() []ent.Edge {
	return []ent.Edge{
		// One-to-one: one Profile belongs to one User
		edge.From("user", User.Type).Ref("profile").Unique().Required(),
	}
}

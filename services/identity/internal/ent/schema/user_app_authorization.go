package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserAppAuthorization struct {
	ent.Schema
}

func (UserAppAuthorization) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").
			Values("active", "revoked", "banned").
			Default("active").
			Comment("authorization status"),
		field.JSON("scopes", []string{}).
			Default([]string{}).
			Comment("authorization scopes"),
		field.JSON("ext_profile", map[string]any{}).
			Default(map[string]any{}).
			Comment("app-level metadata"),
		field.Time("first_authorized_at").
			Default(time.Now).
			Immutable().
			Comment("first authorization time"),
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("last login time"),
		field.Time("last_active_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("last active time"),
	}
}

func (UserAppAuthorization) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("authorizations").
			Unique().
			Required(),
		edge.From("app", App.Type).
			Ref("authorizations").
			Unique().
			Required(),
		edge.From("source_identity", UserIdentity.Type).
			Ref("authorizations").
			Unique(),
	}
}

func (UserAppAuthorization) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("user", "app").Unique(),
	}
}

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type App struct {
	ent.Schema
}

func (App) Fields() []ent.Field {
	return []ent.Field{
		// For example, "nakama_game" or "gopher_paste"
		field.String("app_code").Unique().NotEmpty().Comment("unique application identifier"),
		field.String("app_name").NotEmpty().Comment("application display name"),
	}
}

func (App) Edges() []ent.Edge {
	return []ent.Edge{
		// One-to-many: one App can have multiple user app authorization records
		edge.To("authorizations", UserAppAuthorization.Type),
		// One-to-many: one App can have multiple sessions
		edge.To("sessions", Session.Type),
	}
}

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
		// 比如 "nakama_game", "gopher_paste"
		field.String("app_code").Unique().NotEmpty().Comment("应用唯一标识"),
		field.String("app_name").NotEmpty().Comment("应用展示名称"),
	}
}

func (App) Edges() []ent.Edge {
	return []ent.Edge{
		// 1对多：一个 App 可以有多个新版授权记录
		edge.To("authorizations", UserAppAuthorization.Type),
		// 1对多：一个 App 可以有多个会话
		edge.To("sessions", Session.Type),
	}
}

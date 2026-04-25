package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserIdentity struct {
	ent.Schema
}

func (UserIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("provider").
			Values("phone", "email", "username", "github", "qq", "wechat").
			Comment("identity provider"),
		field.String("provider_uid").
			NotEmpty().
			MaxLen(255).
			Comment("provider unique identifier: phone/email/github_id/openid"),
		field.String("provider_union_id").
			Optional().
			Nillable().
			MaxLen(255).
			Comment("union ID for the WeChat/QQ ecosystem"),
		field.String("login_name").
			Optional().
			Nillable().
			MaxLen(255).
			Comment("readable login name: username/email"),
		field.String("credential_hash").
			Optional().
			Sensitive().
			Comment("local password hash; empty for third-party login"),
		field.Time("verified_at").
			Optional().
			Nillable().
			Comment("identity verification time"),
		field.Time("linked_at").
			Default(time.Now).
			Immutable().
			Comment("identity binding time"),
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("last login time using this identity"),
		field.JSON("meta", map[string]any{}).
			Default(map[string]any{}).
			Comment("extended metadata such as OAuth scopes and avatar_url"),
	}
}

func (UserIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("identities").
			Unique().
			Required(),
		edge.To("authorizations", UserAppAuthorization.Type),
		edge.To("sso_sessions", SsoSession.Type),
		edge.To("sessions", Session.Type),
	}
}

func (UserIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("provider", "provider_uid").Unique(),
	}
}

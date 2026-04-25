package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type SsoSession struct {
	ent.Schema
}

func (SsoSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("sso_token_hash").
			Unique().
			NotEmpty().
			Comment("global login-state token hash, mainly used for cookie-based SSO"),
		field.String("device_id").
			Optional().
			Nillable().
			MaxLen(128).
			Comment("device identifier"),
		field.String("user_agent").
			Optional().
			Nillable().
			MaxLen(512).
			Comment("user agent / client identifier"),
		field.String("ip").
			Optional().
			Nillable().
			Comment("client IP"),
		field.Enum("status").
			Values("active", "revoked", "expired").
			Default("active").
			Comment("global login-state status"),
		field.Int64("sso_version").
			Default(1).
			Comment("current global login-state version"),
		field.Int64("user_version").
			Default(1).
			Comment("snapshot of the user global version when the global login state was created"),
		field.Time("expires_at").
			Comment("global login-state expiration time"),
		field.Time("last_seen_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("last heartbeat time"),
		field.Time("revoked_at").
			Optional().
			Nillable().
			Comment("revocation time"),
	}
}

func (SsoSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("sso_sessions").
			Unique().
			Required(),
		edge.From("identity", UserIdentity.Type).
			Ref("sso_sessions").
			Unique(),
		edge.To("sessions", Session.Type),
	}
}

func (SsoSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("user"),
	}
}

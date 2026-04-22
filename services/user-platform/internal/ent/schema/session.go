package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Session struct {
	ent.Schema
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("session_token_hash").
			Unique().
			NotEmpty().
			Comment("session token hash (currently mainly stores the refresh token hash)"),
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
			Comment("session status"),
		field.Int64("version").
			Default(1).
			Comment("current session version, affecting only this session"),
		field.Int64("user_version").
			Default(1).
			Comment("snapshot of the user's global version when the app session was created"),
		field.Time("expires_at").
			Comment("session expiration time"),
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

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("sessions").
			Unique().
			Required(),
		edge.From("app", App.Type).
			Ref("sessions").
			Unique().
			Required(),
		// Optionally linked to a global SSO session.
		// If the app session is derived from cookie SSO or a global SSO session, attach it to the corresponding sso_session.
		// Leave it empty for an app-specific standalone login.
		edge.From("sso_session", SsoSession.Type).
			Ref("sessions").
			Unique(),
		edge.From("identity", UserIdentity.Type).
			Ref("sessions").
			Unique(),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("user"),
		index.Edges("user", "app"),
	}
}

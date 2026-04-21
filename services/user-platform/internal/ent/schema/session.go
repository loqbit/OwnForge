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
			Comment("会话令牌哈希（当前主要承载 refresh token 哈希）"),
		field.String("device_id").
			Optional().
			Nillable().
			MaxLen(128).
			Comment("设备标识"),
		field.String("user_agent").
			Optional().
			Nillable().
			MaxLen(512).
			Comment("用户 agent / 客户端标识"),
		field.String("ip").
			Optional().
			Nillable().
			Comment("客户端 IP"),
		field.Enum("status").
			Values("active", "revoked", "expired").
			Default("active").
			Comment("会话状态"),
		field.Int64("version").
			Default(1).
			Comment("当前会话版本，仅影响该 session"),
		field.Int64("user_version").
			Default(1).
			Comment("创建该应用会话时的用户全局版本快照"),
		field.Time("expires_at").
			Comment("会话过期时间"),
		field.Time("last_seen_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("最近一次心跳时间"),
		field.Time("revoked_at").
			Optional().
			Nillable().
			Comment("撤销时间"),
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
		// 可选关联到某次全局登录态。
		// 如果应用会话是通过 Cookie SSO / 全局登录态派生出来的，就挂到对应 sso_session；
		// 如果是应用自身独立登录，则保持为空。
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

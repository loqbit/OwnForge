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
			Comment("全局登录态令牌哈希，主要承载 Cookie SSO"),
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
			Comment("全局登录态状态"),
		field.Int64("sso_version").
			Default(1).
			Comment("当前全局登录态版本"),
		field.Int64("user_version").
			Default(1).
			Comment("创建该全局登录态时的用户全局版本快照"),
		field.Time("expires_at").
			Comment("全局登录态过期时间"),
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

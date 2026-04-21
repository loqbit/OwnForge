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
			Comment("身份提供方"),
		field.String("provider_uid").
			NotEmpty().
			MaxLen(255).
			Comment("提供方唯一标识：手机号/邮箱/github_id/openid"),
		field.String("provider_union_id").
			Optional().
			Nillable().
			MaxLen(255).
			Comment("微信/QQ 生态联合 ID"),
		field.String("login_name").
			Optional().
			Nillable().
			MaxLen(255).
			Comment("可读登录名：username/email"),
		field.String("credential_hash").
			Optional().
			Sensitive().
			Comment("本地密码哈希；第三方登录为空"),
		field.Time("verified_at").
			Optional().
			Nillable().
			Comment("身份验证通过时间"),
		field.Time("linked_at").
			Default(time.Now).
			Immutable().
			Comment("身份绑定时间"),
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("最近一次使用该身份登录的时间"),
		field.JSON("meta", map[string]any{}).
			Default(map[string]any{}).
			Comment("扩展元数据（如 OAuth scope、avatar_url 等）"),
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

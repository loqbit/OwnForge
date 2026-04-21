package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// 1. 主键：已接入雪花算法 (Snowflake)
		field.Int64("id").
			Positive(),

		// 1.1 用户全局版本：用于全局失效控制
		field.Int64("user_version").
			Default(1).
			Comment("用户全局版本"),

		// 2. 状态：主体只保留账号生命周期状态
		field.Enum("status").
			Values("active", "banned", "deleted").
			Default("active").
			Comment("用户账号状态"),

		// 3. 创建时间：数据写入时自动生成，且不可篡改
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		// 4. 更新时间：每次 Update 操作时，Ent 会自动帮你把这个字段更新为当前时间
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now). // 不需要在业务代码里手动写更新时间逻辑
			Comment("最后更新时间"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// 1对1：一个 User 拥有一个 Profile
		edge.To("profile", Profile.Type).Unique(),
		// 1对多：一个 User 可以绑定多个登录身份
		edge.To("identities", UserIdentity.Type),
		// 1对多：一个 User 可以有多个应用授权
		edge.To("authorizations", UserAppAuthorization.Type),
		// 1对多：一个 User 可以有多个全局登录态
		edge.To("sso_sessions", SsoSession.Type),
		// 1对多：一个 User 可以有多个会话
		edge.To("sessions", Session.Type),
	}
}

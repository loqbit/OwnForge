// 事件定义层
// producer 和 consumer 之间的 契约。
// 两边都 import 这个包，保证序列化/反序列化一致
package events

const UserRegisteredVersion = "v1"

// UserRegistered 事件结构体
type UserRegistered struct {
	Version   string `json:"version"`
	EventType string `json:"event_type"`
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Timestamp int64  `json:"timestamp"`
}

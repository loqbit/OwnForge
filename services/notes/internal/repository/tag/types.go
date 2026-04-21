package tagrepo

import "time"

// Tag 是 tag 的仓储层领域模型。
type Tag struct {
	ID        int64
	OwnerID   int64
	Name      string
	Color     string
	CreatedAt time.Time
}

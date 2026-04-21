package templaterepo

import "time"

// Template 是 template 的仓储层领域模型。
type Template struct {
	ID          int64
	OwnerID     int64
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

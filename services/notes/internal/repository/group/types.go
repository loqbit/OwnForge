package grouprepo

import "time"

// Group 是 group 的仓储层领域模型。
type Group struct {
	ID          int64
	OwnerID     int64
	ParentID    *int64
	Name        string
	Description string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

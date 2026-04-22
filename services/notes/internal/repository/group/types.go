package grouprepo

import "time"

// Group is the repository-layer domain model for group.
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

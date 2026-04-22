package tagrepo

import "time"

// Tag is the repository-layer domain model for tag.
type Tag struct {
	ID        int64
	OwnerID   int64
	Name      string
	Color     string
	CreatedAt time.Time
}

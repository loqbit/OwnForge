package templaterepo

import "time"

// Template is the repository-layer domain model for template.
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

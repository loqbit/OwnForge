package contract

// CreateTagCommand is the service-layer input for creating a tag.
type CreateTagCommand struct {
	Name  string
	Color string
}

// UpdateTagCommand is the service-layer input for updating a tag.
type UpdateTagCommand struct {
	Name  string
	Color string
}

// TagResult is the tag shape returned by the service layer.
type TagResult struct {
	ID        int64
	OwnerID   int64
	Name      string
	Color     string
	CreatedAt string
}

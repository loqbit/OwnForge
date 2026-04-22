package contract

// CreateTemplateCommand is the service-layer input for creating a template.
type CreateTemplateCommand struct {
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
}

// UpdateTemplateCommand is the service-layer input for updating a template.
type UpdateTemplateCommand struct {
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
}

// TemplateResult is the template shape returned by the service layer.
type TemplateResult struct {
	ID          int64
	OwnerID     int64
	Name        string
	Description string
	Content     string
	Language    string
	Category    string
	IsSystem    bool
	CreatedAt   string
	UpdatedAt   string
}

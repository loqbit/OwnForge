package contract

// CreateGroupCommand is the service-layer input for creating a group.
type CreateGroupCommand struct {
	ParentID    *int64
	Name        string
	Description string
}

// UpdateGroupCommand is the service-layer input for updating a group.
type UpdateGroupCommand struct {
	Name        string
	Description string
	SortOrder   *int
	ParentID    *int64 // supports moving groups
}

// GroupResult is the group shape returned by the service layer.
type GroupResult struct {
	ID            int64
	OwnerID       int64
	ParentID      *int64
	Name          string
	Description   string
	SortOrder     int
	ChildrenCount int
	SnippetCount  int
	CreatedAt     string
	UpdatedAt     string
}

// GroupTreeNode is a recursive tree node used by GetTree to return the full directory structure.
// Approach: fetch everything once, build the tree in O(n) memory with a hashmap, then return the top-level nodes.
type GroupTreeNode struct {
	ID            int64
	ParentID      *int64
	Name          string
	Description   string
	SortOrder     int
	ChildrenCount int
	SnippetCount  int
	CreatedAt     string
	UpdatedAt     string
	Children      []GroupTreeNode // recursively nested child nodes
}

package groupstore

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/group"
	"github.com/ownforge/ownforge/services/notes/internal/ent/snippet"
	grouprepo "github.com/ownforge/ownforge/services/notes/internal/repository/group"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"github.com/ownforge/ownforge/services/notes/internal/service/group/contract"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the group repository.
type Store struct {
	client *ent.Client
}

// New creates an Ent-backed group repository.
func New(client *ent.Client) grouprepo.Repository {
	return &Store{client: client}
}

// Create inserts a group record.
func (s *Store) Create(ctx context.Context, id, ownerID int64, params *contract.CreateGroupCommand) (*grouprepo.Group, error) {
	builder := s.client.Group.Create().
		SetID(id).
		SetOwnerID(ownerID).
		SetName(params.Name).
		SetDescription(params.Description)

	if params.ParentID != nil {
		builder.SetParentID(*params.ParentID)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapGroup(entity), nil
}

// GetByID looks up a single group by ID.
func (s *Store) GetByID(ctx context.Context, id int64) (*grouprepo.Group, error) {
	entity, err := s.client.Group.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapGroup(entity), nil
}

// ListByOwner returns the owner's groups and optionally filters by parentID.
// parentID == nil returns top-level groups; parentID != nil returns children of the given parent.
func (s *Store) ListByOwner(ctx context.Context, ownerID int64, parentID *int64) ([]grouprepo.Group, error) {
	query := s.client.Group.
		Query().
		Where(group.OwnerIDEQ(ownerID))

	if parentID != nil {
		query = query.Where(group.ParentIDEQ(*parentID))
	} else {
		query = query.Where(group.ParentIDIsNil())
	}

	entities, err := query.
		Order(group.BySortOrder(sql.OrderAsc()), group.ByName(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]grouprepo.Group, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapGroup(entity))
	}

	return results, nil
}

// ListAllByOwner returns all groups for the owner regardless of hierarchy, for in-memory tree building in GetTree.
// This uses one SQL query and O(n) memory, which is a good tradeoff for per-user group counts under 500.
func (s *Store) ListAllByOwner(ctx context.Context, ownerID int64) ([]grouprepo.Group, error) {
	entities, err := s.client.Group.
		Query().
		Where(group.OwnerIDEQ(ownerID)).
		Order(group.BySortOrder(sql.OrderAsc()), group.ByName(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]grouprepo.Group, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapGroup(entity))
	}

	return results, nil
}

// Update updates the specified group after verifying ownership.
func (s *Store) Update(ctx context.Context, ownerID, id int64, params *contract.UpdateGroupCommand) (*grouprepo.Group, error) {
	entity, err := s.client.Group.
		Query().
		Where(group.IDEQ(id), group.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := entity.Update().
		SetName(params.Name).
		SetDescription(params.Description)

	if params.SortOrder != nil {
		builder.SetSortOrder(*params.SortOrder)
	}

	if params.ParentID != nil {
		builder.SetParentID(*params.ParentID)
	} else if entity.ParentID != nil {
		// An explicit nil moves the group to the top level.
		builder.ClearParentID()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapGroup(updated), nil
}

// Delete removes the specified group after verifying ownership.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	count, err := s.client.Group.
		Query().
		Where(group.IDEQ(id), group.OwnerIDEQ(ownerID)).
		Count(ctx)
	if err != nil {
		return shared.ParseEntError(err)
	}
	if count == 0 {
		return sharedrepo.ErrNoRows
	}

	return shared.ParseEntError(s.client.Group.DeleteOneID(id).Exec(ctx))
}

// CountChildren returns the number of child groups.
func (s *Store) CountChildren(ctx context.Context, id int64) (int, error) {
	count, err := s.client.Group.
		Query().
		Where(group.ParentIDEQ(id)).
		Count(ctx)
	if err != nil {
		return 0, shared.ParseEntError(err)
	}
	return count, nil
}

// CountSnippets returns the number of direct snippets in the group.
func (s *Store) CountSnippets(ctx context.Context, id int64) (int, error) {
	count, err := s.client.Snippet.
		Query().
		Where(snippet.GroupIDEQ(id)).
		Count(ctx)
	if err != nil {
		return 0, shared.ParseEntError(err)
	}
	return count, nil
}

// mapGroup converts an Ent Group entity into the repository-layer Group struct.
func mapGroup(entity *ent.Group) *grouprepo.Group {
	if entity == nil {
		return nil
	}

	return &grouprepo.Group{
		ID:          entity.ID,
		OwnerID:     entity.OwnerID,
		ParentID:    entity.ParentID,
		Name:        entity.Name,
		Description: entity.Description,
		SortOrder:   entity.SortOrder,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

package tagstore

import (
	"context"
	"sort"

	"entgo.io/ent/dialect/sql"
	"github.com/loqbit/ownforge/services/notes/internal/ent"
	"github.com/loqbit/ownforge/services/notes/internal/ent/tag"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	tagrepo "github.com/loqbit/ownforge/services/notes/internal/repository/tag"
	"github.com/loqbit/ownforge/services/notes/internal/service/tag/contract"
	"github.com/loqbit/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the tag repository.
type Store struct {
	client *ent.Client
}

// New creates an Ent-backed tag repository.
func New(client *ent.Client) tagrepo.Repository {
	return &Store{client: client}
}

// Create inserts a tag record.
func (s *Store) Create(ctx context.Context, id, ownerID int64, params *contract.CreateTagCommand) (*tagrepo.Tag, error) {
	entity, err := s.client.Tag.Create().
		SetID(id).
		SetOwnerID(ownerID).
		SetName(params.Name).
		SetColor(resolveColor(params.Color)).
		Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTag(entity), nil
}

// GetByID looks up a single tag by ID.
func (s *Store) GetByID(ctx context.Context, id int64) (*tagrepo.Tag, error) {
	entity, err := s.client.Tag.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTag(entity), nil
}

// ListByOwner returns all tags for the owner, ordered by name.
func (s *Store) ListByOwner(ctx context.Context, ownerID int64) ([]tagrepo.Tag, error) {
	entities, err := s.client.Tag.
		Query().
		Where(tag.OwnerIDEQ(ownerID)).
		Order(tag.ByName(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]tagrepo.Tag, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapTag(entity))
	}

	return results, nil
}

// ListByIDs returns the specified tags owned by the current user.
func (s *Store) ListByIDs(ctx context.Context, ownerID int64, ids []int64) ([]tagrepo.Tag, error) {
	if len(ids) == 0 {
		return []tagrepo.Tag{}, nil
	}

	normalized := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	if len(normalized) == 0 {
		return []tagrepo.Tag{}, nil
	}

	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })

	entities, err := s.client.Tag.
		Query().
		Where(tag.OwnerIDEQ(ownerID), tag.IDIn(normalized...)).
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]tagrepo.Tag, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapTag(entity))
	}

	return results, nil
}

// Update updates a tag after verifying ownership.
func (s *Store) Update(ctx context.Context, ownerID, id int64, params *contract.UpdateTagCommand) (*tagrepo.Tag, error) {
	entity, err := s.client.Tag.
		Query().
		Where(tag.IDEQ(id), tag.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := entity.Update().
		SetName(params.Name)

	if params.Color != "" {
		builder.SetColor(params.Color)
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTag(updated), nil
}

// Delete removes a tag after verifying ownership.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	count, err := s.client.Tag.
		Query().
		Where(tag.IDEQ(id), tag.OwnerIDEQ(ownerID)).
		Count(ctx)
	if err != nil {
		return shared.ParseEntError(err)
	}
	if count == 0 {
		return sharedrepo.ErrNoRows
	}

	return shared.ParseEntError(s.client.Tag.DeleteOneID(id).Exec(ctx))
}

func resolveColor(value string) string {
	if value == "" {
		return "#6366f1"
	}
	return value
}

func mapTag(entity *ent.Tag) *tagrepo.Tag {
	if entity == nil {
		return nil
	}

	return &tagrepo.Tag{
		ID:        entity.ID,
		OwnerID:   entity.OwnerID,
		Name:      entity.Name,
		Color:     entity.Color,
		CreatedAt: entity.CreatedAt,
	}
}

package tagstore

import (
	"context"
	"sort"

	"entgo.io/ent/dialect/sql"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/tag"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/tag/contract"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store 是 tag Repository 的 Ent 实现。
type Store struct {
	client *ent.Client
}

// New 创建一个基于 Ent 的 tag Repository。
func New(client *ent.Client) tagrepo.Repository {
	return &Store{client: client}
}

// Create 创建一条 Tag 记录。
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

// GetByID 根据 ID 查询单个 Tag。
func (s *Store) GetByID(ctx context.Context, id int64) (*tagrepo.Tag, error) {
	entity, err := s.client.Tag.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTag(entity), nil
}

// ListByOwner 按 owner_id 查询用户的所有 Tag，按名称排序。
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

// ListByIDs 查询当前用户指定 ID 集合中的标签。
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

// Update 更新指定 Tag，需要校验 ownerID 所有权。
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

// Delete 删除指定 Tag，需要校验 ownerID 所有权。
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

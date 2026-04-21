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

// Store 是 group Repository 的 Ent 实现。
type Store struct {
	client *ent.Client
}

// New 创建一个基于 Ent 的 group Repository。
func New(client *ent.Client) grouprepo.Repository {
	return &Store{client: client}
}

// Create 创建一条 Group 记录。
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

// GetByID 根据 ID 查询单个 Group。
func (s *Store) GetByID(ctx context.Context, id int64) (*grouprepo.Group, error) {
	entity, err := s.client.Group.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapGroup(entity), nil
}

// ListByOwner 按 owner_id 查询用户的分组，支持按 parentID 筛选。
// parentID == nil → 返回顶级分组；parentID != nil → 返回指定父级的子分组。
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

// ListAllByOwner 查询用户的**所有**分组（不区分层级），用于 GetTree 内存建树。
// 一次查询 O(1) SQL，O(n) 内存，是用户级分组量（<500）下的最优解。
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

// Update 更新指定 Group，需要校验 ownerID 所有权。
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
		// 显式传 nil 表示移到顶级
		builder.ClearParentID()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapGroup(updated), nil
}

// Delete 删除指定 Group，需要校验 ownerID 所有权。
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

// CountChildren 统计分组的子分组数量。
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

// CountSnippets 统计分组下直属的片段数量。
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

// mapGroup 将 Ent 的 Group 实体映射为 Repository 层需要的 Group 结构体。
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

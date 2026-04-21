package templatestore

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	enttemplate "github.com/ownforge/ownforge/services/notes/internal/ent/template"
	templaterepo "github.com/ownforge/ownforge/services/notes/internal/repository/template"
	"github.com/ownforge/ownforge/services/notes/internal/service/template/contract"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store 是 template Repository 的 Ent 实现。
type Store struct {
	client *ent.Client
}

// New 创建一个基于 Ent 的 template Repository。
func New(client *ent.Client) templaterepo.Repository {
	return &Store{client: client}
}

// Create 创建一条 Template 记录。
func (s *Store) Create(ctx context.Context, id, ownerID int64, params *contract.CreateTemplateCommand) (*templaterepo.Template, error) {
	entity, err := s.client.Template.Create().
		SetID(id).
		SetOwnerID(ownerID).
		SetName(params.Name).
		SetDescription(params.Description).
		SetContent(params.Content).
		SetLanguage(params.Language).
		SetCategory(params.Category).
		Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTemplate(entity), nil
}

// GetByID 根据 ID 查询单个 Template。
func (s *Store) GetByID(ctx context.Context, id int64) (*templaterepo.Template, error) {
	entity, err := s.client.Template.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTemplate(entity), nil
}

// List 返回系统模板 + 指定用户的个人模板，可按 category 过滤。
func (s *Store) List(ctx context.Context, ownerID int64, category string) ([]templaterepo.Template, error) {
	query := s.client.Template.
		Query().
		Where(
			// 系统模板 OR 自己的模板
			enttemplate.Or(
				enttemplate.IsSystem(true),
				enttemplate.OwnerIDEQ(ownerID),
			),
		).
		Order(enttemplate.ByCreatedAt(sql.OrderAsc()))

	if category != "" {
		query = query.Where(enttemplate.CategoryEQ(category))
	}

	entities, err := query.All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]templaterepo.Template, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapTemplate(entity))
	}

	return results, nil
}

// Update 更新指定模板，需要校验 ownerID 所有权。
func (s *Store) Update(ctx context.Context, ownerID, id int64, params *contract.UpdateTemplateCommand) (*templaterepo.Template, error) {
	entity, err := s.client.Template.
		Query().
		Where(enttemplate.IDEQ(id), enttemplate.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := entity.Update().
		SetName(params.Name).
		SetDescription(params.Description).
		SetContent(params.Content).
		SetLanguage(params.Language).
		SetCategory(params.Category)

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapTemplate(updated), nil
}

// Delete 删除指定模板，需要校验 ownerID（系统模板不可删除，owner_id=0）。
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	count, err := s.client.Template.
		Query().
		Where(enttemplate.IDEQ(id), enttemplate.OwnerIDEQ(ownerID)).
		Count(ctx)
	if err != nil {
		return shared.ParseEntError(err)
	}
	if count == 0 {
		return shared.ParseEntError(
			&ent.NotFoundError{},
		)
	}

	return shared.ParseEntError(s.client.Template.DeleteOneID(id).Exec(ctx))
}

// CountSystem 统计系统预置模板数量，用于判断是否需要 seed。
func (s *Store) CountSystem(ctx context.Context) (int, error) {
	count, err := s.client.Template.
		Query().
		Where(enttemplate.IsSystem(true)).
		Count(ctx)
	if err != nil {
		return 0, shared.ParseEntError(err)
	}
	return count, nil
}

// CreateBatch 批量创建模板（用于 seed 系统模板）。
func (s *Store) CreateBatch(ctx context.Context, templates []templaterepo.Template) error {
	builders := make([]*ent.TemplateCreate, 0, len(templates))
	for _, t := range templates {
		builders = append(builders, s.client.Template.Create().
			SetID(t.ID).
			SetOwnerID(t.OwnerID).
			SetName(t.Name).
			SetDescription(t.Description).
			SetContent(t.Content).
			SetLanguage(t.Language).
			SetCategory(t.Category).
			SetIsSystem(t.IsSystem),
		)
	}

	_, err := s.client.Template.CreateBulk(builders...).Save(ctx)
	return shared.ParseEntError(err)
}

func mapTemplate(entity *ent.Template) *templaterepo.Template {
	if entity == nil {
		return nil
	}

	return &templaterepo.Template{
		ID:          entity.ID,
		OwnerID:     entity.OwnerID,
		Name:        entity.Name,
		Description: entity.Description,
		Content:     entity.Content,
		Language:    entity.Language,
		Category:    entity.Category,
		IsSystem:    entity.IsSystem,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

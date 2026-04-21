package snippetstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/snippet"
	"github.com/ownforge/ownforge/services/notes/internal/ent/tag"
	snippetrepo "github.com/ownforge/ownforge/services/notes/internal/repository/snippet"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"
	"github.com/ownforge/ownforge/services/notes/internal/store/entstore/shared"
)

// Store 是 snippet Repository 的 Ent 实现。
type Store struct {
	client *ent.Client
}

// New 创建一个基于 Ent 的 snippet Repository。
func New(client *ent.Client) snippetrepo.Repository {
	return &Store{client: client}
}

// Create 创建一条 Snippet 记录，ID 由外部（id-generator）传入。
func (s *Store) Create(ctx context.Context, id, ownerID int64, params *contract.CreateSnippetCommand) (*snippetrepo.Snippet, error) {
	builder := s.client.Snippet.Create().
		SetID(id).
		SetOwnerID(ownerID).
		SetType(resolveType(params.Type)).
		SetTitle(params.Title).
		SetLanguage(params.Language)

	if params.Content != "" {
		builder.SetContent(params.Content)
	}
	if params.FileURL != "" {
		builder.SetFileURL(params.FileURL)
	}
	if params.FileSize > 0 {
		builder.SetFileSize(params.FileSize)
	}
	if params.MimeType != "" {
		builder.SetMimeType(params.MimeType)
	}
	if params.GroupID != nil && *params.GroupID != 0 {
		builder.SetGroupID(*params.GroupID)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapSnippet(entity), nil
}

// GetByID 根据 ID 查询单个 Snippet。
func (s *Store) GetByID(ctx context.Context, id int64) (*snippetrepo.Snippet, error) {
	entity, err := s.client.Snippet.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapSnippet(entity), nil
}

// ListByOwner 按 owner_id 查询用户的所有 Snippet（仅查活着的文档），按更新时间倒序。
func (s *Store) ListByOwner(ctx context.Context, ownerID int64) ([]snippetrepo.Snippet, error) {
	entities, err := s.client.Snippet.
		Query().
		Where(snippet.OwnerIDEQ(ownerID)).
		Where(snippet.DeletedAtIsNil()).
		Order(snippet.ByUpdatedAt(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]snippetrepo.Snippet, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapSnippet(entity))
	}

	return results, nil
}

// ─── 技术点 4 + 5：复合筛选 + 游标分页 ──────────────────────────
//
// 复合筛选（Predicate 组合）：
//   Ent 的 Where() 接受 ...predicate.Snippet，多个条件自动 AND。
//   我们根据用户传了哪些参数，动态构建 predicate 列表：
//     predicates := []predicate.Snippet{snippet.OwnerIDEQ(ownerID)}
//     if groupID != nil { predicates = append(predicates, ...) }
//     if tagID != nil { predicates = append(predicates, ...) }  ← 这个涉及 JOIN
//     query.Where(predicates...)
//   这比 if/else 层层嵌套清晰得多。
//
// 游标分页（Cursor-based Pagination）：
//   排序键: (updated_at DESC, id DESC)  ← 复合键保证唯一性
//   游标编码: base64(JSON({"updated_at": "...", "id": 123}))
//   WHERE 条件:
//     updated_at < cursor.updated_at
//     或者 (updated_at = cursor.updated_at AND id < cursor.id)
//   这保证了分页的稳定性，即使有新数据插入也不会重复/遗漏。
// ──────────────────────────────────────────────────────────────

// cursorPayload 游标的序列化结构。
// SortKey 是通用排序键（可以是 updated_at、created_at、title 或 sort_order 的值）。
type cursorPayload struct {
	UpdatedAt time.Time `json:"u"`
	CreatedAt time.Time `json:"c,omitempty"`
	Title     string    `json:"t,omitempty"`
	SortOrder int       `json:"o,omitempty"`
	ID        int64     `json:"i"`
	SortBy    string    `json:"s,omitempty"` // 记录游标对应的排序方式
}


func decodeCursor(cursor string) (*cursorPayload, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// normalizeSortBy 标准化排序字段名。
func normalizeSortBy(sortBy string) string {
	switch strings.TrimSpace(strings.ToLower(sortBy)) {
	case "created_at":
		return "created_at"
	case "title":
		return "title"
	case "manual", "sort_order":
		return "manual"
	default:
		return "updated_at"
	}
}

// ListFiltered 复合筛选 + 游标分页查询。
func (s *Store) ListFiltered(ctx context.Context, ownerID int64, query *contract.ListQuery) ([]snippetrepo.Snippet, error) {
	// ── 第一步：构建基础查询 + 动态添加 predicates ──
	q := s.client.Snippet.Query().
		Where(snippet.OwnerIDEQ(ownerID)) // 必有条件：只查自己的

	// 按分组筛选
	if query.GroupID != nil {
		if *query.GroupID == 0 {
			q = q.Where(snippet.GroupIDIsNil()) // 0 表示收集箱/未分组
		} else {
			q = q.Where(snippet.GroupIDEQ(*query.GroupID))
		}
	}

	// 按类型筛选
	if query.Type != "" {
		q = q.Where(snippet.TypeEQ(snippet.Type(query.Type)))
	}

	// 按收藏筛选
	if query.OnlyFavorites {
		q = q.Where(snippet.IsFavoriteEQ(true))
	}

	// 按状态（活跃/回收站）筛选
	if query.Status == "trashed" {
		q = q.Where(snippet.DeletedAtNotNil())
	} else {
		// 默认只查活跃记录
		q = q.Where(snippet.DeletedAtIsNil())
	}

	// 按关键词模糊搜索标题
	if query.Keyword != "" {
		q = q.Where(snippet.TitleContainsFold(query.Keyword))
	}

	// 按标签筛选（通过桥接表 JOIN）
	// 这是 Ent 特有的 HasTagsWith：生成 EXISTS (SELECT 1 FROM snippet_tags ...)
	if query.TagID != nil {
		q = q.Where(snippet.HasTagsWith(tag.IDEQ(*query.TagID)))
	}

	// ── 第二步：确定排序方式 ──
	sortBy := normalizeSortBy(query.SortBy)

	// ── 第三步：游标分页（根据排序方式选择不同的游标条件）──
	if query.Cursor != "" {
		cur, err := decodeCursor(query.Cursor)
		if err == nil { // 游标无效时忽略，退化为首页
			switch sortBy {
			case "created_at":
				// WHERE (created_at < cursor.created_at)
				//    或者 (created_at = cursor.created_at AND id < cursor.id)
				q = q.Where(
					snippet.Or(
						snippet.CreatedAtLT(cur.CreatedAt),
						snippet.And(
							snippet.CreatedAtEQ(cur.CreatedAt),
							snippet.IDLT(cur.ID),
						),
					),
				)
			case "title":
				// WHERE (title > cursor.title)
				//    或者 (title = cursor.title AND id > cursor.id)
				q = q.Where(
					snippet.Or(
						snippet.TitleGT(cur.Title),
						snippet.And(
							snippet.TitleEQ(cur.Title),
							snippet.IDGT(cur.ID),
						),
					),
				)
			case "manual":
				// WHERE (sort_order > cursor.sort_order)
				//    或者 (sort_order = cursor.sort_order AND id > cursor.id)
				q = q.Where(
					snippet.Or(
						snippet.SortOrderGT(cur.SortOrder),
						snippet.And(
							snippet.SortOrderEQ(cur.SortOrder),
							snippet.IDGT(cur.ID),
						),
					),
				)
			default: // updated_at
				// WHERE (updated_at < cursor.updated_at)
				//    或者 (updated_at = cursor.updated_at AND id < cursor.id)
				q = q.Where(
					snippet.Or(
						snippet.UpdatedAtLT(cur.UpdatedAt),
						snippet.And(
							snippet.UpdatedAtEQ(cur.UpdatedAt),
							snippet.IDLT(cur.ID),
						),
					),
				)
			}
		}
	}

	// ── 第四步：排序 + 限制条数 ──
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	switch sortBy {
	case "created_at":
		q = q.Order(snippet.ByCreatedAt(sql.OrderDesc()), snippet.ByID(sql.OrderDesc()))
	case "title":
		q = q.Order(snippet.ByTitle(sql.OrderAsc()), snippet.ByID(sql.OrderAsc()))
	case "manual":
		q = q.Order(snippet.BySortOrder(sql.OrderAsc()), snippet.ByID(sql.OrderAsc()))
	default:
		q = q.Order(snippet.ByUpdatedAt(sql.OrderDesc()), snippet.ByID(sql.OrderDesc()))
	}

	entities, err := q.
		Limit(limit + 1). // 多查一条，用来判断是否还有下一页
		All(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	results := make([]snippetrepo.Snippet, 0, len(entities))
	for _, entity := range entities {
		results = append(results, *mapSnippet(entity))
	}

	return results, nil
}


// Update 更新指定 Snippet，需要校验 ownerID 所有权。
func (s *Store) Update(ctx context.Context, ownerID, id int64, params *contract.UpdateSnippetCommand) (*snippetrepo.Snippet, error) {
	entity, err := s.client.Snippet.
		Query().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	builder := entity.Update().
		SetTitle(params.Title).
		SetContent(params.Content).
		SetLanguage(params.Language)

	if params.GroupID != nil && *params.GroupID != 0 {
		builder.SetGroupID(*params.GroupID)
	} else if entity.GroupID != nil {
		builder.ClearGroupID()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapSnippet(updated), nil
}

// Delete 删除指定 Snippet，需要校验 ownerID 所有权。
// 这里执行硬删除。
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	count, err := s.client.Snippet.
		Query().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		Count(ctx)
	if err != nil {
		return shared.ParseEntError(err)
	}
	if count == 0 {
		return sharedrepo.ErrNoRows
	}

	return shared.ParseEntError(s.client.Snippet.DeleteOneID(id).Exec(ctx))
}

// SoftDelete 将文档移入回收站
func (s *Store) SoftDelete(ctx context.Context, ownerID, id int64) error {
	_, err := s.client.Snippet.
		Update().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return shared.ParseEntError(err)
}

// Restore 将文档从回收站恢复
func (s *Store) Restore(ctx context.Context, ownerID, id int64) error {
	_, err := s.client.Snippet.
		Update().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		ClearDeletedAt().
		Save(ctx)
	return shared.ParseEntError(err)
}

// SetFavorite 设置收藏状态
func (s *Store) SetFavorite(ctx context.Context, ownerID, id int64, isFavorite bool) error {
	_, err := s.client.Snippet.
		Update().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		SetIsFavorite(isFavorite).
		Save(ctx)
	return shared.ParseEntError(err)
}

func resolveType(value string) snippet.Type {
	switch strings.TrimSpace(value) {
	case "note":
		return snippet.TypeNote
	case "file":
		return snippet.TypeFile
	default:
		return snippet.TypeCode
	}
}



func mapSnippet(entity *ent.Snippet) *snippetrepo.Snippet {
	if entity == nil {
		return nil
	}

	return &snippetrepo.Snippet{
		ID:         entity.ID,
		OwnerID:    entity.OwnerID,
		Type:       string(entity.Type),
		Title:      entity.Title,
		Content:    entity.Content,
		FileURL:    entity.FileURL,
		FileSize:   entity.FileSize,
		MimeType:   entity.MimeType,
		Language:   entity.Language,
		GroupID:    entity.GroupID,
		SortOrder:  entity.SortOrder,
		IsFavorite: entity.IsFavorite,
		DeletedAt:  entity.DeletedAt,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}

// ─── 技术点 3：Ent M2M Edge 操作 ──────────────────────────────
//
// Ent 的多对多关系通过 Edge 定义（见 schema/snippet.go: edge.To("tags", Tag.Type)）。
// Ent 自动创建桥接表 snippet_tags(snippet_id, tag_id)，不需要手动管理。
//
// 关键 API：
//   entity.Update().ClearTags()           → 清空所有关联
//   entity.Update().AddTagIDs(1, 2, 3)    → 添加关联
//   entity.Update().RemoveTagIDs(1, 2)    → 移除关联
//   entity.QueryTags()                    → 查询关联的 Tag 实体
//
// SetTags 的实现选择 "先 Clear 再 Add" 而不是 diff：
//   优点：逻辑简单，无状态比较
//   代价：多一条 DELETE 语句，但桥接表行数极少（<100），可忽略
// ───────────────────────────────────────────────────────────────

// SetTags 替换片段的所有标签关联。
// 实现：先 ClearTags（删除桥接表中该 snippet 的所有行）→ 再 AddTagIDs（插入新行）
func (s *Store) SetTags(ctx context.Context, snippetID int64, tagIDs []int64) error {
	builder := s.client.Snippet.UpdateOneID(snippetID).
		ClearTags() // 第一步：DELETE FROM snippet_tags WHERE snippet_id = ?

	if len(tagIDs) > 0 {
		builder.AddTagIDs(tagIDs...) // 第二步：INSERT INTO snippet_tags (snippet_id, tag_id) VALUES ...
	}

	_, err := builder.Save(ctx)
	return shared.ParseEntError(err)
}

// Move 将 Snippet 移动到目标分组并写入新的 sort_order。
// groupID 为 nil 表示移动到收集箱（清除 group_id）。
func (s *Store) Move(ctx context.Context, ownerID, id int64, groupID *int64, sortOrder int) (*snippetrepo.Snippet, error) {
	// 1. 先校验归属
	entity, err := s.client.Snippet.
		Query().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	// 2. 构造更新
	builder := entity.Update().SetSortOrder(sortOrder)
	if groupID != nil && *groupID != 0 {
		builder.SetGroupID(*groupID)
	} else {
		builder.ClearGroupID()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}
	return mapSnippet(updated), nil
}

// MaxSortOrderInGroup 返回目标分组内当前最大的 sort_order。分组为空返回 0。
func (s *Store) MaxSortOrderInGroup(ctx context.Context, ownerID int64, groupID *int64) (int, error) {
	q := s.client.Snippet.
		Query().
		Where(snippet.OwnerIDEQ(ownerID))
	if groupID == nil || *groupID == 0 {
		q = q.Where(snippet.GroupIDIsNil())
	} else {
		q = q.Where(snippet.GroupIDEQ(*groupID))
	}

	entity, err := q.Order(snippet.BySortOrder(sql.OrderDesc())).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}
		return 0, shared.ParseEntError(err)
	}
	return entity.SortOrder, nil
}

// GetTagIDs 查询片段关联的所有标签 ID。
// 通过 Ent 的 Edge Query 实现，不需要手写 JOIN。
func (s *Store) GetTagIDs(ctx context.Context, snippetID int64) ([]int64, error) {
	// entity.QueryTags() 生成：
	// SELECT tag.id FROM tags JOIN snippet_tags ON ... WHERE snippet_tags.snippet_id = ?
	tags, err := s.client.Snippet.
		Query().
		Where(snippet.IDEQ(snippetID)).
		QueryTags().
		IDs(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return tags, nil
}

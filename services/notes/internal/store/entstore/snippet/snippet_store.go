package snippetstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/loqbit/ownforge/services/notes/internal/ent"
	"github.com/loqbit/ownforge/services/notes/internal/ent/snippet"
	"github.com/loqbit/ownforge/services/notes/internal/ent/tag"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	snippetrepo "github.com/loqbit/ownforge/services/notes/internal/repository/snippet"
	"github.com/loqbit/ownforge/services/notes/internal/service/snippet/contract"
	"github.com/loqbit/ownforge/services/notes/internal/store/entstore/shared"
)

// Store is the Ent-backed implementation of the snippet repository.
type Store struct {
	client *ent.Client
}

// New creates an Ent-backed snippet repository.
func New(client *ent.Client) snippetrepo.Repository {
	return &Store{client: client}
}

// Create inserts a snippet record. The ID is provided externally by id-generator.
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

// GetByID looks up a single snippet by ID.
func (s *Store) GetByID(ctx context.Context, id int64) (*snippetrepo.Snippet, error) {
	entity, err := s.client.Snippet.Get(ctx, id)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	return mapSnippet(entity), nil
}

// ListByOwner returns all active snippets for the owner, ordered by last update time descending.
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

// ─── Notes 4 + 5: compound filters + cursor pagination ─────────────────────
//
// Compound filters (predicate composition):
//   Ent's Where() accepts ...predicate.Snippet, and multiple conditions are ANDed automatically.
//   We build the predicate list dynamically based on the request parameters:
//     predicates := []predicate.Snippet{snippet.OwnerIDEQ(ownerID)}
//     if groupID != nil { predicates = append(predicates, ...) }
//     if tagID != nil { predicates = append(predicates, ...) }  <- this involves a JOIN
//     query.Where(predicates...)
//   This is much clearer than deeply nested if/else blocks.
//
// Cursor-based pagination:
//   Sort key: (updated_at DESC, id DESC)  <- the composite key guarantees uniqueness
//   Cursor encoding: base64(JSON({"updated_at": "...", "id": 123}))
//   WHERE clause:
//     updated_at < cursor.updated_at
//     or (updated_at = cursor.updated_at AND id < cursor.id)
//   This keeps pagination stable even when new rows are inserted, without duplicates or gaps.
// ──────────────────────────────────────────────────────────────

// cursorPayload is the serialized cursor shape.
// SortKey is the generic sort key and may hold updated_at, created_at, title, or sort_order.
type cursorPayload struct {
	UpdatedAt time.Time `json:"u"`
	CreatedAt time.Time `json:"c,omitempty"`
	Title     string    `json:"t,omitempty"`
	SortOrder int       `json:"o,omitempty"`
	ID        int64     `json:"i"`
	SortBy    string    `json:"s,omitempty"` // Records the sort mode used by this cursor.
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

// normalizeSortBy normalizes the sort field name.
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

// ListFiltered applies compound filters and cursor pagination.
func (s *Store) ListFiltered(ctx context.Context, ownerID int64, query *contract.ListQuery) ([]snippetrepo.Snippet, error) {
	// Step 1: build the base query and add predicates dynamically.
	q := s.client.Snippet.Query().
		Where(snippet.OwnerIDEQ(ownerID)) // Required condition: only query the caller's own snippets.

	// Filter by group.
	if query.GroupID != nil {
		if *query.GroupID == 0 {
			q = q.Where(snippet.GroupIDIsNil()) // 0 means inbox / ungrouped.
		} else {
			q = q.Where(snippet.GroupIDEQ(*query.GroupID))
		}
	}

	// Filter by type.
	if query.Type != "" {
		q = q.Where(snippet.TypeEQ(snippet.Type(query.Type)))
	}

	// Filter favorites.
	if query.OnlyFavorites {
		q = q.Where(snippet.IsFavoriteEQ(true))
	}

	// Filter by status (active / trash).
	if query.Status == "trashed" {
		q = q.Where(snippet.DeletedAtNotNil())
	} else {
		// Query active records by default.
		q = q.Where(snippet.DeletedAtIsNil())
	}

	// Fuzzy-match the title by keyword.
	if query.Keyword != "" {
		q = q.Where(snippet.TitleContainsFold(query.Keyword))
	}

	// Filter by tag through the join table.
	// Ent's HasTagsWith generates EXISTS (SELECT 1 FROM snippet_tags ...).
	if query.TagID != nil {
		q = q.Where(snippet.HasTagsWith(tag.IDEQ(*query.TagID)))
	}

	// Step 2: determine the sort mode.
	sortBy := normalizeSortBy(query.SortBy)

	// Step 3: apply cursor pagination with sort-specific cursor conditions.
	if query.Cursor != "" {
		cur, err := decodeCursor(query.Cursor)
		if err == nil { // Ignore invalid cursors and fall back to the first page.
			switch sortBy {
			case "created_at":
				// WHERE (created_at < cursor.created_at)
				//    or (created_at = cursor.created_at AND id < cursor.id)
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
				//    or (title = cursor.title AND id > cursor.id)
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
				//    or (sort_order = cursor.sort_order AND id > cursor.id)
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
				//    or (updated_at = cursor.updated_at AND id < cursor.id)
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

	// Step 4: apply ordering and page size.
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
		Limit(limit + 1). // Fetch one extra row to determine whether another page exists.
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

// Update updates the specified snippet after verifying ownership.
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

// Delete removes the specified snippet after verifying ownership.
// This performs a hard delete.
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

// SoftDelete moves the document to the trash.
func (s *Store) SoftDelete(ctx context.Context, ownerID, id int64) error {
	_, err := s.client.Snippet.
		Update().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		SetDeletedAt(time.Now()).
		Save(ctx)
	return shared.ParseEntError(err)
}

// Restore brings the document back from the trash.
func (s *Store) Restore(ctx context.Context, ownerID, id int64) error {
	_, err := s.client.Snippet.
		Update().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		ClearDeletedAt().
		Save(ctx)
	return shared.ParseEntError(err)
}

// SetFavorite updates the favorite flag.
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

// ─── Note 3: Ent many-to-many edge operations ─────────────────────────────
//
// Ent defines many-to-many relationships through edges; see schema/snippet.go: edge.To("tags", Tag.Type).
// Ent automatically creates the snippet_tags(snippet_id, tag_id) join table, so no manual management is needed.
//
// Key APIs:
//   entity.Update().ClearTags()           -> clear all associations
//   entity.Update().AddTagIDs(1, 2, 3)    -> add associations
//   entity.Update().RemoveTagIDs(1, 2)    -> remove associations
//   entity.QueryTags()                    -> query associated Tag entities
//
// SetTags intentionally uses "clear first, then add" instead of diffing:
//   Benefit: simple logic with no state comparison
//   Cost: one extra DELETE statement, which is negligible because the join table is tiny (<100 rows)
// ───────────────────────────────────────────────────────────────

// SetTags replaces all tag associations on a snippet.
// Implementation: call ClearTags first to remove all join-table rows for the snippet, then AddTagIDs to insert the new ones.
func (s *Store) SetTags(ctx context.Context, snippetID int64, tagIDs []int64) error {
	builder := s.client.Snippet.UpdateOneID(snippetID).
		ClearTags() // Step 1: DELETE FROM snippet_tags WHERE snippet_id = ?

	if len(tagIDs) > 0 {
		builder.AddTagIDs(tagIDs...) // Step 2: INSERT INTO snippet_tags (snippet_id, tag_id) VALUES ...
	}

	_, err := builder.Save(ctx)
	return shared.ParseEntError(err)
}

// Move sends the snippet to the target group and writes the new sort_order.
// groupID == nil moves the snippet to the inbox by clearing group_id.
func (s *Store) Move(ctx context.Context, ownerID, id int64, groupID *int64, sortOrder int) (*snippetrepo.Snippet, error) {
	// 1. Verify ownership first.
	entity, err := s.client.Snippet.
		Query().
		Where(snippet.IDEQ(id), snippet.OwnerIDEQ(ownerID)).
		Only(ctx)
	if err != nil {
		return nil, shared.ParseEntError(err)
	}

	// 2. Build the update.
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

// MaxSortOrderInGroup returns the current maximum sort_order in the target group. It returns 0 when the group is empty.
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

// GetTagIDs returns all tag IDs associated with the snippet.
// This uses Ent edge queries, so no manual JOIN is needed.
func (s *Store) GetTagIDs(ctx context.Context, snippetID int64) ([]int64, error) {
	// entity.QueryTags() generates:
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

package snippet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/event"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	snippetrepo "github.com/ownforge/ownforge/services/notes/internal/repository/snippet"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"

	"go.uber.org/zap"
)

// Domain errors.
var (
	ErrIDGeneration  = pkgerrs.NewServerErr(errors.New("failed to generate snippet ID"))
	ErrForbidden     = pkgerrs.New(pkgerrs.Forbidden, "no permission to operate on this snippet", nil)
	ErrInvalidTagIDs = pkgerrs.NewParamErr("tag does not exist or cannot be used", nil)
	ErrTrashed       = pkgerrs.New(pkgerrs.Forbidden, "this operation is not allowed on documents in the recycle bin", nil)
	ErrNotTrashed    = pkgerrs.NewParamErr("document is not in the recycle bin", nil)
)

// SnippetService defines the snippet service interface.
type SnippetService interface {
	Create(ctx context.Context, userID int64, req *contract.CreateSnippetCommand) (*contract.SnippetResult, error)
	GetMineByID(ctx context.Context, userID, id int64) (*contract.SnippetResult, error)
	ListMine(ctx context.Context, userID int64) ([]contract.SnippetResult, error)
	ListMineFiltered(ctx context.Context, userID int64, query *contract.ListQuery) (*contract.ListResult, error)
	Update(ctx context.Context, userID, id int64, req *contract.UpdateSnippetCommand) (*contract.SnippetResult, error)
	Delete(ctx context.Context, userID, id int64) error
	Restore(ctx context.Context, userID, id int64) error
	Move(ctx context.Context, userID, id int64, req *contract.MoveSnippetCommand) (*contract.SnippetResult, error)
	SetTags(ctx context.Context, userID, snippetID int64, tagIDs []int64) error
	SetFavorite(ctx context.Context, userID, snippetID int64, isFavorite bool) error
}

type snippetService struct {
	repo      snippetrepo.Repository
	tagRepo   tagrepo.Repository
	idgen     idgen.Client
	publisher event.Publisher // Event publisher. It may be nil without affecting the core flow.
	logger    *zap.Logger
}

// NewSnippetService creates a SnippetService instance.
// publisher may be nil, in which case event publishing is skipped and core CRUD behavior is unaffected.
func NewSnippetService(repo snippetrepo.Repository, tagRepo tagrepo.Repository, idgenClient idgen.Client, publisher event.Publisher, logger *zap.Logger) SnippetService {
	return &snippetService{repo: repo, tagRepo: tagRepo, idgen: idgenClient, publisher: publisher, logger: logger}
}

func (s *snippetService) Create(ctx context.Context, userID int64, req *contract.CreateSnippetCommand) (*contract.SnippetResult, error) {
	params := normalizeCreateCommand(req)
	if err := validateCreateCommand(params); err != nil {
		return nil, err
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate snippet ID", zap.Error(err))
		return nil, ErrIDGeneration
	}

	snippet, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to create snippet",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	result := toSnippetResult(snippet)
	s.publishSnippetSaved(ctx, snippet.ID, userID, "create")
	return result, nil
}

func (s *snippetService) GetMineByID(ctx context.Context, userID, id int64) (*contract.SnippetResult, error) {
	snippet, err := s.repo.GetByID(ctx, id)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to query snippet",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return nil, err
	}

	if snippet.OwnerID != userID {
		return nil, ErrForbidden
	}

	result := toSnippetResult(snippet)
	if err := s.hydrateTagIDs(ctx, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *snippetService) ListMine(ctx context.Context, userID int64) ([]contract.SnippetResult, error) {
	list, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to query user snippets",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	results := make([]contract.SnippetResult, 0, len(list))
	for _, item := range list {
		results = append(results, *toSnippetResult(&item))
	}

	if err := s.hydrateTagIDsForList(ctx, results); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *snippetService) ListMineFiltered(ctx context.Context, userID int64, query *contract.ListQuery) (*contract.ListResult, error) {
	if query == nil {
		query = &contract.ListQuery{}
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
		query.Limit = limit
	}

	list, err := s.repo.ListFiltered(ctx, userID, query)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to filter snippets",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	// The store fetches one extra row to determine whether another page exists.
	hasMore := len(list) > limit
	if hasMore {
		list = list[:limit] // Trim the extra record used for pagination.
	}

	items := make([]contract.SnippetResult, 0, len(list))
	for _, item := range list {
		items = append(items, *toSnippetResult(&item))
	}

	if err := s.hydrateTagIDsForList(ctx, items); err != nil {
		return nil, err
	}

	var nextCursor string
	if hasMore && len(list) > 0 {
		last := &list[len(list)-1]
		nextCursor = encodeSnippetCursor(last, query.SortBy)
	}

	return &contract.ListResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *snippetService) Update(ctx context.Context, userID, id int64, req *contract.UpdateSnippetCommand) (*contract.SnippetResult, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if current.OwnerID != userID {
		return nil, ErrForbidden
	}

	params := normalizeUpdateCommand(current, req)
	if err := validateUpdateCommand(params); err != nil {
		return nil, err
	}

	snippet, err := s.repo.Update(ctx, userID, id, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to update snippet",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	result := toSnippetResult(snippet)
	s.publishSnippetSaved(ctx, id, userID, "update")
	return result, nil
}

func (s *snippetService) Delete(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if current.OwnerID != userID {
		return ErrForbidden
	}

	if current.DeletedAt != nil {
		// Already in the trash, so perform a hard delete.
		if err := s.repo.Delete(ctx, userID, id); err != nil {
			commonlogger.Ctx(ctx, s.logger).Error("failed to permanently delete snippet",
				zap.Int64("id", id),
				zap.Int64("userID", userID),
				zap.Error(err),
			)
			return err
		}
		return nil
	}

	// Perform a soft delete by moving it to the trash.
	if err := s.repo.SoftDelete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to soft-delete snippet",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (s *snippetService) Restore(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if current.OwnerID != userID {
		return ErrForbidden
	}
	if current.DeletedAt == nil {
		return ErrNotTrashed
	}

	if err := s.repo.Restore(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to restore snippet",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *snippetService) SetFavorite(ctx context.Context, userID, snippetID int64, isFavorite bool) error {
	current, err := s.repo.GetByID(ctx, snippetID)
	if err != nil {
		return err
	}
	if current.OwnerID != userID {
		return ErrForbidden
	}
	if current.DeletedAt != nil {
		return ErrTrashed
	}

	if err := s.repo.SetFavorite(ctx, userID, snippetID, isFavorite); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to set snippet favorite state",
			zap.Int64("id", snippetID),
			zap.Int64("userID", userID),
			zap.Bool("isFavorite", isFavorite),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// Move sends a snippet to the target group or adjusts its order within the group.
// - req.GroupID == nil moves the snippet to the inbox (ungrouped)
// - req.SortOrder == nil automatically appends the snippet to the end of the target group (max+1)
func (s *snippetService) Move(ctx context.Context, userID, id int64, req *contract.MoveSnippetCommand) (*contract.SnippetResult, error) {
	if req == nil {
		req = &contract.MoveSnippetCommand{}
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.OwnerID != userID {
		return nil, ErrForbidden
	}

	// Calculate the target sort_order. When unspecified, use the current max in the target group plus 1.
	targetSortOrder := 0
	if req.SortOrder != nil {
		targetSortOrder = *req.SortOrder
	} else {
		max, err := s.repo.MaxSortOrderInGroup(ctx, userID, req.GroupID)
		if err != nil {
			commonlogger.Ctx(ctx, s.logger).Error("failed to query the maximum sort_order in the group",
				zap.Int64("userID", userID),
				zap.Error(err),
			)
			return nil, err
		}
		targetSortOrder = max + 1
	}

	snippet, err := s.repo.Move(ctx, userID, id, req.GroupID, targetSortOrder)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to move snippet",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	result := toSnippetResult(snippet)
	if err := s.hydrateTagIDs(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetTags replaces all tag associations on the snippet.
func (s *snippetService) SetTags(ctx context.Context, userID, snippetID int64, tagIDs []int64) error {
	current, err := s.repo.GetByID(ctx, snippetID)
	if err != nil {
		return err
	}
	if current.OwnerID != userID {
		return ErrForbidden
	}

	normalizedTagIDs, err := s.validateOwnedTagIDs(ctx, userID, tagIDs)
	if err != nil {
		return err
	}

	if err := s.repo.SetTags(ctx, snippetID, normalizedTagIDs); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to set snippet tags",
			zap.Int64("snippetID", snippetID),
			zap.Int64s("tagIDs", normalizedTagIDs),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func normalizeCreateCommand(req *contract.CreateSnippetCommand) *contract.CreateSnippetCommand {
	if req == nil {
		return &contract.CreateSnippetCommand{
			Type:     "code",
			Language: "text",
		}
	}

	params := *req
	params.Type = normalizeType(params.Type)
	params.Title = strings.TrimSpace(params.Title)
	params.FileURL = strings.TrimSpace(params.FileURL)
	params.MimeType = strings.TrimSpace(params.MimeType)
	params.Language = normalizeLanguage(params.Language)
	return &params
}

func normalizeUpdateCommand(current *snippetrepo.Snippet, req *contract.UpdateSnippetCommand) *contract.UpdateSnippetCommand {
	params := &contract.UpdateSnippetCommand{
		Title:    current.Title,
		Content:  current.Content,
		Language: current.Language,
		GroupID:  current.GroupID,
	}
	if req == nil {
		return params
	}

	if trimmed := strings.TrimSpace(req.Title); trimmed != "" {
		params.Title = trimmed
	}
	if req.Content != "" {
		params.Content = req.Content
	}
	if req.Language != "" {
		params.Language = normalizeLanguage(req.Language)
	}
	if req.GroupID != nil {
		params.GroupID = req.GroupID
	}

	return params
}

func validateCreateCommand(req *contract.CreateSnippetCommand) error {
	if strings.TrimSpace(req.Title) == "" {
		return pkgerrs.NewParamErr("title cannot be empty", nil)
	}
	if !isSupportedType(req.Type) {
		return pkgerrs.NewParamErr("type only supports code, note, or file", nil)
	}
	if req.Type == "file" && req.FileURL == "" {
		return pkgerrs.NewParamErr("file snippets must provide file_url", nil)
	}
	if req.Type != "file" && req.Content == "" {
		return pkgerrs.NewParamErr("text snippets must provide content", nil)
	}

	return nil
}

func validateUpdateCommand(req *contract.UpdateSnippetCommand) error {
	if strings.TrimSpace(req.Title) == "" {
		return pkgerrs.NewParamErr("title cannot be empty", nil)
	}
	return nil
}

func normalizeType(t string) string {
	switch strings.TrimSpace(t) {
	case "note":
		return "note"
	case "file":
		return "file"
	default:
		return "code"
	}
}

func normalizeLanguage(v string) string {
	if strings.TrimSpace(v) == "" {
		return "text"
	}
	return strings.TrimSpace(v)
}

func isSupportedType(v string) bool {
	return v == "code" || v == "note" || v == "file"
}

func toSnippetResult(item *snippetrepo.Snippet) *contract.SnippetResult {
	if item == nil {
		return nil
	}

	return &contract.SnippetResult{
		ID:         item.ID,
		OwnerID:    item.OwnerID,
		Type:       item.Type,
		Title:      item.Title,
		Content:    item.Content,
		FileURL:    item.FileURL,
		FileSize:   item.FileSize,
		MimeType:   item.MimeType,
		Language:   item.Language,
		GroupID:    item.GroupID,
		SortOrder:  item.SortOrder,
		TagIDs:     item.TagIDs,
		IsFavorite: item.IsFavorite,
		DeletedAt:  formatOptionalTime(item.DeletedAt),
		CreatedAt:  formatTime(item.CreatedAt),
		UpdatedAt:  formatTime(item.UpdatedAt),
	}
}

// formatTime formats time.Time as an RFC3339 string.
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// formatOptionalTime formats *time.Time as an RFC3339 string and returns an empty string for nil.
func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func (s *snippetService) hydrateTagIDs(ctx context.Context, item *contract.SnippetResult) error {
	if item == nil {
		return nil
	}

	tagIDs, err := s.repo.GetTagIDs(ctx, item.ID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to query snippet tags",
			zap.Int64("snippetID", item.ID),
			zap.Error(err),
		)
		return err
	}

	item.TagIDs = tagIDs
	return nil
}

func (s *snippetService) hydrateTagIDsForList(ctx context.Context, items []contract.SnippetResult) error {
	for i := range items {
		if err := s.hydrateTagIDs(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *snippetService) validateOwnedTagIDs(ctx context.Context, userID int64, tagIDs []int64) ([]int64, error) {
	if len(tagIDs) == 0 {
		return []int64{}, nil
	}

	normalized := make([]int64, 0, len(tagIDs))
	seen := make(map[int64]struct{}, len(tagIDs))
	for _, id := range tagIDs {
		if id <= 0 {
			return nil, ErrInvalidTagIDs
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	tags, err := s.tagRepo.ListByIDs(ctx, userID, normalized)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to validate snippet tags",
			zap.Int64("userID", userID),
			zap.Int64s("tagIDs", normalized),
			zap.Error(err),
		)
		return nil, err
	}
	if len(tags) != len(normalized) {
		return nil, ErrInvalidTagIDs
	}

	return normalized, nil
}

// encodeSnippetCursor encodes the sort key as a cursor string.
// It uses base64(JSON) encoding. The frontend does not need to parse it and can return it unchanged.
func encodeSnippetCursor(s *snippetrepo.Snippet, sortBy string) string {
	payload := map[string]any{
		"u": s.UpdatedAt.Format(time.RFC3339Nano),
		"c": s.CreatedAt.Format(time.RFC3339Nano),
		"t": s.Title,
		"o": s.SortOrder,
		"i": s.ID,
		"s": sortBy,
	}
	data, _ := json.Marshal(payload)
	return base64.URLEncoding.EncodeToString(data)
}

// publishSnippetSaved emits the snippet.saved event asynchronously.
// A nil publisher or publish failure does not block the main flow; it is only logged.
func (s *snippetService) publishSnippetSaved(ctx context.Context, snippetID, ownerID int64, action string) {
	if s.publisher == nil {
		return
	}
	go func() {
		if err := s.publisher.Publish(context.Background(), event.TopicSnippetSaved, &event.SnippetSavedPayload{
			SnippetID: snippetID,
			OwnerID:   ownerID,
			Action:    action,
		}); err != nil {
			s.logger.Warn("failed to publish snippet.saved event (does not affect the main flow)",
				zap.Int64("snippet_id", snippetID),
				zap.Error(err),
			)
		}
	}()
}

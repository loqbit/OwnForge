package snippet

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/luckysxx/common/errs"
	commonlogger "github.com/luckysxx/common/logger"
	"github.com/ownforge/ownforge/services/notes/internal/event"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	snippetrepo "github.com/ownforge/ownforge/services/notes/internal/repository/snippet"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"

	"go.uber.org/zap"
)

// 领域错误定义
var (
	ErrIDGeneration  = pkgerrs.NewServerErr(errors.New("生成片段 ID 失败"))
	ErrForbidden     = pkgerrs.New(pkgerrs.Forbidden, "无权限操作该代码片段", nil)
	ErrInvalidTagIDs = pkgerrs.NewParamErr("标签不存在或无权限使用", nil)
	ErrTrashed       = pkgerrs.New(pkgerrs.Forbidden, "回收站中的文档不可执行该操作", nil)
	ErrNotTrashed    = pkgerrs.NewParamErr("文档未在回收站中", nil)
)

// SnippetService 定义 snippet 业务接口。
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
	publisher event.Publisher // 事件发布（可为 nil，不影响核心流程）
	logger    *zap.Logger
}

// NewSnippetService 创建 SnippetService 实例。
// publisher 可传 nil（事件发布被跳过，核心 CRUD 不受影响）。
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
		commonlogger.Ctx(ctx, s.logger).Error("生成 snippet ID 失败", zap.Error(err))
		return nil, ErrIDGeneration
	}

	snippet, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("创建 snippet 失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("查询 snippet 失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("查询用户 snippet 列表失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("筛选查询 snippet 失败",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	// store 多查了 1 条来判断是否有下一页
	hasMore := len(list) > limit
	if hasMore {
		list = list[:limit] // 截掉多余的那一条
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
		commonlogger.Ctx(ctx, s.logger).Error("更新 snippet 失败",
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
		// 已在回收站，执行物理删除
		if err := s.repo.Delete(ctx, userID, id); err != nil {
			commonlogger.Ctx(ctx, s.logger).Error("物理删除 snippet 失败",
				zap.Int64("id", id),
				zap.Int64("userID", userID),
				zap.Error(err),
			)
			return err
		}
		return nil
	}

	// 执行软删除：移入回收站
	if err := s.repo.SoftDelete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("软删除 snippet 失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("恢复 snippet 失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("设置 snippet 收藏状态失败",
			zap.Int64("id", snippetID),
			zap.Int64("userID", userID),
			zap.Bool("isFavorite", isFavorite),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// Move 移动 Snippet 到目标分组或在分组内调整排序。
// - req.GroupID 为 nil 表示移动到收集箱（未分组）
// - req.SortOrder 为 nil 时自动追加到目标分组末尾（max+1）
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

	// 计算目标 sort_order：未指定则查目标分组内最大值 + 1
	targetSortOrder := 0
	if req.SortOrder != nil {
		targetSortOrder = *req.SortOrder
	} else {
		max, err := s.repo.MaxSortOrderInGroup(ctx, userID, req.GroupID)
		if err != nil {
			commonlogger.Ctx(ctx, s.logger).Error("查询分组最大 sort_order 失败",
				zap.Int64("userID", userID),
				zap.Error(err),
			)
			return nil, err
		}
		targetSortOrder = max + 1
	}

	snippet, err := s.repo.Move(ctx, userID, id, req.GroupID, targetSortOrder)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("移动 snippet 失败",
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

// SetTags 替换片段的所有标签关联。
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
		commonlogger.Ctx(ctx, s.logger).Error("设置 snippet 标签失败",
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
			Type:       "code",
			Language:   "text",
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
		Title:      current.Title,
		Content:    current.Content,
		Language:   current.Language,
		GroupID:    current.GroupID,
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
		return pkgerrs.NewParamErr("标题不能为空", nil)
	}
	if !isSupportedType(req.Type) {
		return pkgerrs.NewParamErr("type 仅支持 code、note、file", nil)
	}
	if req.Type == "file" && req.FileURL == "" {
		return pkgerrs.NewParamErr("文件片段必须提供 file_url", nil)
	}
	if req.Type != "file" && req.Content == "" {
		return pkgerrs.NewParamErr("文本片段必须提供 content", nil)
	}

	return nil
}

func validateUpdateCommand(req *contract.UpdateSnippetCommand) error {
	if strings.TrimSpace(req.Title) == "" {
		return pkgerrs.NewParamErr("标题不能为空", nil)
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

// formatTime 将 time.Time 格式化为 RFC3339 字符串。
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// formatOptionalTime 将 *time.Time 格式化为 RFC3339 字符串，nil 返回空串。
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
		commonlogger.Ctx(ctx, s.logger).Error("查询 snippet 标签失败",
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
		commonlogger.Ctx(ctx, s.logger).Error("校验 snippet 标签失败",
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

// encodeSnippetCursor 将排序键编码为游标字符串。
// 使用 base64(JSON) 编码，前端不需要解析，只需原样回传。
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

// publishSnippetSaved 异步发布 snippet.saved 事件。
// Publisher 为 nil 或发布失败均不阻塞主流程，仅打日志。
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
			s.logger.Warn("发布 snippet.saved 事件失败（不影响主流程）",
				zap.Int64("snippet_id", snippetID),
				zap.Error(err),
			)
		}
	}()
}

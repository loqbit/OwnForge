package grpcserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	notepb "github.com/ownforge/ownforge/pkg/proto/note"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	groupcontract "github.com/ownforge/ownforge/services/notes/internal/service/group/contract"
	lineagecontract "github.com/ownforge/ownforge/services/notes/internal/service/lineage/contract"
	sharecontract "github.com/ownforge/ownforge/services/notes/internal/service/share/contract"
	snippetcontract "github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"
	tagcontract "github.com/ownforge/ownforge/services/notes/internal/service/tag/contract"
	templatecontract "github.com/ownforge/ownforge/services/notes/internal/service/template/contract"
	uploadcontract "github.com/ownforge/ownforge/services/notes/internal/service/upload/contract"
	grpcerrs "github.com/ownforge/ownforge/services/notes/internal/transport/grpc/codec/errs"
	"github.com/ownforge/ownforge/services/notes/internal/transport/grpc/interceptor"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =========================================================================
// 片段扩展能力 (Snippet Extensions)
// =========================================================================

// DeleteSnippet 删除指定的代码片段
func (s *NoteServer) DeleteSnippet(ctx context.Context, req *notepb.DeleteSnippetRequest) (*notepb.DeleteSnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.snippetSvc.Delete(ctx, userID, req.SnippetId); err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteSnippetResponse{Id: req.SnippetId}, nil
}

// RestoreSnippet 将片段从回收站恢复
func (s *NoteServer) RestoreSnippet(ctx context.Context, req *notepb.RestoreSnippetRequest) (*notepb.RestoreSnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.snippetSvc.Restore(ctx, userID, req.SnippetId); err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.RestoreSnippetResponse{Id: req.SnippetId}, nil
}

// MoveSnippet 移动片段到目标分组并可选地设置排序权重
func (s *NoteServer) MoveSnippet(ctx context.Context, req *notepb.MoveSnippetRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &snippetcontract.MoveSnippetCommand{
		GroupID:   req.GroupId,
		SortOrder: optionalInt32ToInt(req.SortOrder),
	}

	result, err := s.snippetSvc.Move(ctx, userID, req.SnippetId, cmd)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC MoveSnippet 失败", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProto(result), nil
}

// SearchSnippets 根据查询条件搜索代码片段
func (s *NoteServer) SearchSnippets(ctx context.Context, req *notepb.SearchSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.snippetSvc.ListMineFiltered(ctx, userID, &snippetcontract.ListQuery{
		Keyword: req.Keyword,
		Limit:   int(req.Limit),
	})
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	snippets := make([]*notepb.SnippetResponse, 0, len(results.Items))
	for i := range results.Items {
		snippets = append(snippets, toProto(&results.Items[i]))
	}

	return &notepb.ListSnippetsResponse{
		Snippets:   snippets,
		NextCursor: results.NextCursor,
		HasMore:    results.HasMore,
	}, nil
}

// GetPublicSnippet — 已废弃，公开访问统一走 Share 短链。
func (s *NoteServer) GetPublicSnippet(ctx context.Context, req *notepb.GetPublicSnippetRequest) (*notepb.SnippetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "public snippet access has been removed, use share tokens instead")
}

// FavoriteSnippet 收藏指定的代码片段
func (s *NoteServer) FavoriteSnippet(ctx context.Context, req *notepb.FavoriteSnippetRequest) (*notepb.FavoriteSnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.snippetSvc.SetFavorite(ctx, userID, req.SnippetId, true); err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.FavoriteSnippetResponse{SnippetId: req.SnippetId, Favorite: true}, nil
}

// UnfavoriteSnippet 取消收藏指定的代码片段
func (s *NoteServer) UnfavoriteSnippet(ctx context.Context, req *notepb.UnfavoriteSnippetRequest) (*notepb.FavoriteSnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.snippetSvc.SetFavorite(ctx, userID, req.SnippetId, false); err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.FavoriteSnippetResponse{SnippetId: req.SnippetId, Favorite: false}, nil
}

// CreateSnippetFromTemplate 基于已有模板创建新代码片段
func (s *NoteServer) CreateSnippetFromTemplate(ctx context.Context, req *notepb.CreateSnippetFromTemplateRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	tpl, err := s.templateSvc.GetByID(ctx, req.TemplateId)
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = tpl.Name
	}

	result, err := s.snippetSvc.Create(ctx, userID, &snippetcontract.CreateSnippetCommand{
		Type:     "note",
		Title:    title,
		Content:  tpl.Content,
		Language: tpl.Language,
	})
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	sourceUserID := tpl.OwnerID
	if err := s.lineageSvc.Record(ctx, &lineagecontract.RecordCommand{
		SnippetID:    result.ID,
		SourceUserID: &sourceUserID,
		RelationType: "template",
	}); err != nil {
		commonlogger.Ctx(ctx, s.log).Warn("记录模板来源失败", zap.Int64("snippet_id", result.ID), zap.Error(err))
	}

	return toProto(result), nil
}

// CreateSnippetFromShare 从分享导入到当前用户自己的知识库。
func (s *NoteServer) CreateSnippetFromShare(ctx context.Context, req *notepb.CreateSnippetFromShareRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	source, err := s.shareSvc.GetSourceByToken(ctx, req.Token, req.Password)
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = source.Snippet.Title
	}

	// 文件类型 snippet 需要复制对象存储中的文件，避免共享 key。
	fileURL := source.Snippet.FileURL
	if source.Snippet.Type == "file" && fileURL != "" {
		copiedURL, err := s.uploadSvc.CopyObject(ctx, fileURL, userID)
		if err != nil {
			commonlogger.Ctx(ctx, s.log).Warn("fork 文件复制失败，回退为共享 URL",
				zap.String("src_url", fileURL), zap.Error(err))
			// 降级：复制失败仍使用原 URL，不阻断核心流程
		} else {
			fileURL = copiedURL
		}
	}

	result, err := s.snippetSvc.Create(ctx, userID, &snippetcontract.CreateSnippetCommand{
		Type:     source.Snippet.Type,
		Title:    title,
		Content:  source.Snippet.Content,
		FileURL:  fileURL,
		FileSize: source.Snippet.FileSize,
		MimeType: source.Snippet.MimeType,
		Language: source.Snippet.Language,
	})
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	sourceSnippetID := source.Snippet.ID
	sourceShareID := source.Share.ID
	sourceUserID := source.Share.OwnerID
	relationType := "import"
	if source.Share.Kind == "template" {
		relationType = "fork"
	}
	if err := s.lineageSvc.Record(ctx, &lineagecontract.RecordCommand{
		SnippetID:       result.ID,
		SourceSnippetID: &sourceSnippetID,
		SourceShareID:   &sourceShareID,
		SourceUserID:    &sourceUserID,
		RelationType:    relationType,
	}); err != nil {
		commonlogger.Ctx(ctx, s.log).Warn("记录分享导入来源失败", zap.Int64("snippet_id", result.ID), zap.Error(err))
	}

	if source.Share.Kind == "template" {
		if err := s.shareSvc.IncrementForkCount(ctx, source.Share.ID); err != nil {
			commonlogger.Ctx(ctx, s.log).Warn("递增分享 fork_count 失败", zap.Int64("share_id", source.Share.ID), zap.Error(err))
		}
	}

	return toProto(result), nil
}

// =========================================================================
// 工作区列表能力 (Workspace Lists)
// =========================================================================

// ListRecentSnippets 获取用户最近访问的代码片段列表
func (s *NoteServer) ListRecentSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	return s.ListSnippets(ctx, req)
}

// ListSharedSnippets 获取与当前用户共享的代码片段列表
func (s *NoteServer) ListSharedSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	return &notepb.ListSnippetsResponse{Snippets: []*notepb.SnippetResponse{}}, nil
}

// ListFavoriteSnippets 获取当前用户已经收藏的代码片段列表
func (s *NoteServer) ListFavoriteSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	req.OnlyFavorites = true
	return s.ListSnippets(ctx, req)
}

// =========================================================================
// 资源管理：分组 (Groups)
// =========================================================================

// ListGroups 获取当前用户拥有的所有分组列表
func (s *NoteServer) ListGroups(ctx context.Context, req *notepb.ListGroupsRequest) (*notepb.ListGroupsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.groupSvc.List(ctx, userID, nil)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListGroups 失败", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	groups := make([]*notepb.GroupResponse, 0, len(results))
	for i := range results {
		groups = append(groups, toProtoGroup(&results[i]))
	}

	return &notepb.ListGroupsResponse{Groups: groups}, nil
}

// GetGroup 获取单个分组详情
func (s *NoteServer) GetGroup(ctx context.Context, req *notepb.GetGroupRequest) (*notepb.GroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.groupSvc.GetByID(ctx, userID, req.GroupId)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC GetGroup 失败", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// CreateGroup 创建一个全新的分组
func (s *NoteServer) CreateGroup(ctx context.Context, req *notepb.CreateGroupRequest) (*notepb.GroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.groupSvc.Create(ctx, userID, &groupcontract.CreateGroupCommand{
		ParentID:    req.ParentId,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateGroup 失败", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// UpdateGroup 更新现有分组的信息 (如重命名)
func (s *NoteServer) UpdateGroup(ctx context.Context, req *notepb.UpdateGroupRequest) (*notepb.GroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.groupSvc.Update(ctx, userID, req.GroupId, &groupcontract.UpdateGroupCommand{
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentId,
		SortOrder:   optionalInt32ToInt(req.SortOrder),
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateGroup 失败", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// DeleteGroup 删除指定的分组
func (s *NoteServer) DeleteGroup(ctx context.Context, req *notepb.DeleteGroupRequest) (*notepb.DeleteGroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.groupSvc.Delete(ctx, userID, req.GroupId); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteGroup 失败", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteGroupResponse{Id: req.GroupId}, nil
}

// =========================================================================
// 资源管理：标签 (Tags)
// =========================================================================

// ListTags 获取当前用户创建的所有标签
func (s *NoteServer) ListTags(ctx context.Context, req *notepb.ListTagsRequest) (*notepb.ListTagsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.tagSvc.List(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListTags 失败", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	tags := make([]*notepb.TagResponse, 0, len(results))
	for i := range results {
		tags = append(tags, toProtoTag(&results[i]))
	}

	return &notepb.ListTagsResponse{Tags: tags}, nil
}

// CreateTag 为某代码片段创建一个新的个人标签
func (s *NoteServer) CreateTag(ctx context.Context, req *notepb.CreateTagRequest) (*notepb.TagResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.tagSvc.Create(ctx, userID, &tagcontract.CreateTagCommand{
		Name:  req.Name,
		Color: req.Color,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateTag 失败", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTag(result), nil
}

// UpdateTag 更新标签
func (s *NoteServer) UpdateTag(ctx context.Context, req *notepb.UpdateTagRequest) (*notepb.TagResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.tagSvc.Update(ctx, userID, req.TagId, &tagcontract.UpdateTagCommand{
		Name:  req.Name,
		Color: req.Color,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateTag 失败", zap.Int64("tag_id", req.TagId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTag(result), nil
}

// DeleteTag 删除某一个标签
func (s *NoteServer) DeleteTag(ctx context.Context, req *notepb.DeleteTagRequest) (*notepb.DeleteTagResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.tagSvc.Delete(ctx, userID, req.TagId); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteTag 失败", zap.Int64("tag_id", req.TagId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteTagResponse{Id: req.TagId}, nil
}

// =========================================================================
// 模板与附件上传 (Templates & Uploads)
// =========================================================================

// ListTemplates 获取系统或个人可用的模板列表
func (s *NoteServer) ListTemplates(ctx context.Context, req *notepb.ListTemplatesRequest) (*notepb.ListTemplatesResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.templateSvc.List(ctx, userID, req.Category)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListTemplates 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	templates := make([]*notepb.TemplateResponse, 0, len(results))
	for i := range results {
		templates = append(templates, toProtoTemplate(&results[i]))
	}
	return &notepb.ListTemplatesResponse{Templates: templates}, nil
}

// GetTemplate 获取单个模板的内容和详细信息
func (s *NoteServer) GetTemplate(ctx context.Context, req *notepb.GetTemplateRequest) (*notepb.TemplateResponse, error) {
	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "无效的模板 ID")
	}

	result, err := s.templateSvc.GetByID(ctx, id)
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// CreateTemplate 创建个人模板
func (s *NoteServer) CreateTemplate(ctx context.Context, req *notepb.CreateTemplateRequest) (*notepb.TemplateResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.templateSvc.Create(ctx, userID, &templatecontract.CreateTemplateCommand{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Language:    req.Language,
		Category:    req.Category,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateTemplate 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// UpdateTemplate 更新个人模板
func (s *NoteServer) UpdateTemplate(ctx context.Context, req *notepb.UpdateTemplateRequest) (*notepb.TemplateResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "无效的模板 ID")
	}

	result, err := s.templateSvc.Update(ctx, userID, id, &templatecontract.UpdateTemplateCommand{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Language:    req.Language,
		Category:    req.Category,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateTemplate 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// DeleteTemplate 删除个人模板
func (s *NoteServer) DeleteTemplate(ctx context.Context, req *notepb.DeleteTemplateRequest) (*notepb.DeleteTemplateResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "无效的模板 ID")
	}

	if err := s.templateSvc.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteTemplate 失败", zap.Int64("id", id), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteTemplateResponse{Id: req.TemplateId}, nil
}

// CreateShare 创建新的分享链接。
func (s *NoteServer) CreateShare(ctx context.Context, req *notepb.CreateShareRequest) (*notepb.ShareResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.shareSvc.Create(ctx, userID, &sharecontract.CreateShareCommand{
		SnippetID: req.SnippetId,
		Kind:      req.Kind,
		Password:  req.Password,
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateShare 失败", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoShare(result), nil
}

// ListMyShares 获取当前用户创建的分享列表。
func (s *NoteServer) ListMyShares(ctx context.Context, req *notepb.ListMySharesRequest) (*notepb.ListSharesResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.shareSvc.ListMine(ctx, userID, &sharecontract.ListSharesQuery{Kind: req.Kind})
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	items := make([]*notepb.ShareResponse, 0, len(results))
	for i := range results {
		items = append(items, toProtoShare(&results[i]))
	}

	return &notepb.ListSharesResponse{Shares: items}, nil
}

// DeleteShare 删除分享。
func (s *NoteServer) DeleteShare(ctx context.Context, req *notepb.DeleteShareRequest) (*notepb.DeleteShareResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.shareSvc.Delete(ctx, userID, req.ShareId); err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteShareResponse{Id: req.ShareId}, nil
}

// GetPublicShareByToken 匿名读取公开分享。
func (s *NoteServer) GetPublicShareByToken(ctx context.Context, req *notepb.GetPublicShareByTokenRequest) (*notepb.PublicShareResponse, error) {
	result, err := s.shareSvc.GetPublicByToken(ctx, req.Token, req.Password)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Warn("gRPC GetPublicShareByToken 失败", zap.String("token", req.Token), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.PublicShareResponse{
		Share:   toProtoShare(result.Share),
		Snippet: toProto(result.Snippet),
	}, nil
}

// PresignUpload 生成浏览器直传 MinIO 的预签名 URL。
func (s *NoteServer) PresignUpload(ctx context.Context, req *notepb.PresignUploadRequest) (*notepb.PresignUploadResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.uploadSvc.Presign(ctx, &uploadcontract.PresignCommand{
		OwnerID:     userID,
		Filename:    req.Filename,
		Size:        req.Size,
		ContentType: req.MimeType,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC PresignUpload 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.PresignUploadResponse{
		Url:       result.URL,
		ObjectKey: result.ObjectKey,
		ExpiresAt: result.ExpiresAt,
		Headers:   result.Headers,
		PublicUrl: result.PublicURL,
	}, nil
}

// CompleteUpload 确认浏览器直传已完成。
func (s *NoteServer) CompleteUpload(ctx context.Context, req *notepb.CompleteUploadRequest) (*notepb.CompleteUploadResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.uploadSvc.Complete(ctx, &uploadcontract.CompleteUploadCommand{
		OwnerID:     userID,
		ObjectKey:   req.ObjectKey,
		Filename:    req.Filename,
		Size:        req.Size,
		ContentType: req.MimeType,
		SnippetID:   req.SnippetId,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CompleteUpload 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.CompleteUploadResponse{
		Filename:     result.Filename,
		Size:         result.Size,
		Url:          result.URL,
		MimeType:     result.MimeType,
		ThumbnailUrl: result.ThumbnailURL,
		ObjectKey:    result.ObjectKey,
	}, nil
}

// UploadFile 基于 gRPC 实现的文件字节流上传入口。
func (s *NoteServer) UploadFile(ctx context.Context, req *notepb.UploadFileRequest) (*notepb.UploadFileResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	contentType := http.DetectContentType(req.FileData)
	cmd := &uploadcontract.UploadCommand{
		OwnerID:     userID,
		Filename:    req.Filename,
		Size:        int64(len(req.FileData)),
		ContentType: contentType,
	}

	result, err := s.uploadSvc.Upload(ctx, cmd, bytes.NewReader(req.FileData))
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UploadFile 失败", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.UploadFileResponse{
		Id:           req.Filename,
		Filename:     result.Filename,
		Size:         result.Size,
		Url:          result.URL,
		MimeType:     result.MimeType,
		ThumbnailUrl: result.ThumbnailURL,
	}, nil
}

func toProtoGroup(r *groupcontract.GroupResult) *notepb.GroupResponse {
	resp := &notepb.GroupResponse{
		Id:            r.ID,
		Name:          r.Name,
		OwnerId:       r.OwnerID,
		Description:   r.Description,
		SortOrder:     int32(r.SortOrder),
		ChildrenCount: int32(r.ChildrenCount),
		SnippetCount:  int32(r.SnippetCount),
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if r.ParentID != nil {
		resp.ParentId = r.ParentID
	}
	return resp
}

func (s *NoteServer) GetSnippetAIMetadata(ctx context.Context, req *notepb.GetSnippetAIMetadataRequest) (*notepb.SnippetAIMetadataResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	md, err := s.aimetadataRepo.GetBySnippetID(ctx, req.SnippetId)
	if err != nil {
		if errors.Is(err, sharedrepo.ErrNoRows) {
			if _, snippetErr := s.snippetSvc.GetMineByID(ctx, userID, req.SnippetId); snippetErr != nil {
				return nil, grpcerrs.ToStatusError(snippetErr)
			}

			return &notepb.SnippetAIMetadataResponse{
				SnippetId:     req.SnippetId,
				SuggestedTags: []string{},
				Todos:         []*notepb.AITodoItem{},
			}, nil
		}
		commonlogger.Ctx(ctx, s.log).Debug("GetSnippetAIMetadata", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	if md.OwnerID != userID {
		return nil, status.Error(codes.PermissionDenied, "无权访问该片段的 AI 数据")
	}

	todos := make([]*notepb.AITodoItem, 0, len(md.ExtractedTodos))
	for _, raw := range md.ExtractedTodos {
		todos = append(todos, &notepb.AITodoItem{
			Text:     stringFromMap(raw, "text"),
			Priority: stringFromMap(raw, "priority"),
			Done:     boolFromMap(raw, "done"),
		})
	}

	tags := md.SuggestedTags
	if tags == nil {
		tags = []string{}
	}

	return &notepb.SnippetAIMetadataResponse{
		SnippetId:     md.SnippetID,
		Summary:       md.Summary,
		SuggestedTags: tags,
		Todos:         todos,
		Model:         md.Model,
		PromptVersion: md.PromptVersion,
		UpdatedAt:     md.UpdatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolFromMap(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func toProtoTag(r *tagcontract.TagResult) *notepb.TagResponse {
	return &notepb.TagResponse{
		Id:        r.ID,
		Name:      r.Name,
		OwnerId:   r.OwnerID,
		Color:     r.Color,
		CreatedAt: r.CreatedAt,
	}
}

func toProtoTemplate(r *templatecontract.TemplateResult) *notepb.TemplateResponse {
	return &notepb.TemplateResponse{
		Id:          formatInt64(r.ID),
		Name:        r.Name,
		Language:    r.Language,
		Content:     r.Content,
		Description: r.Description,
		Category:    r.Category,
		IsSystem:    r.IsSystem,
		OwnerId:     r.OwnerID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toProtoShare(r *sharecontract.ShareResult) *notepb.ShareResponse {
	if r == nil {
		return nil
	}

	return &notepb.ShareResponse{
		Id:          r.ID,
		Token:       r.Token,
		Kind:        r.Kind,
		SnippetId:   r.SnippetID,
		OwnerId:     r.OwnerID,
		HasPassword: r.HasPassword,
		ExpiresAt:   r.ExpiresAt,
		ViewCount:   int32(r.ViewCount),
		ForkCount:   int32(r.ForkCount),
		CreatedAt:   r.CreatedAt,
	}
}

func optionalInt32ToInt(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func formatInt64(id int64) string {
	return strconv.FormatInt(id, 10)
}

func parseTemplateID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

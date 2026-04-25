package grpcserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	notepb "github.com/loqbit/ownforge/pkg/proto/note"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	groupcontract "github.com/loqbit/ownforge/services/notes/internal/service/group/contract"
	lineagecontract "github.com/loqbit/ownforge/services/notes/internal/service/lineage/contract"
	sharecontract "github.com/loqbit/ownforge/services/notes/internal/service/share/contract"
	snippetcontract "github.com/loqbit/ownforge/services/notes/internal/service/snippet/contract"
	tagcontract "github.com/loqbit/ownforge/services/notes/internal/service/tag/contract"
	templatecontract "github.com/loqbit/ownforge/services/notes/internal/service/template/contract"
	uploadcontract "github.com/loqbit/ownforge/services/notes/internal/service/upload/contract"
	grpcerrs "github.com/loqbit/ownforge/services/notes/internal/transport/grpc/codec/errs"
	"github.com/loqbit/ownforge/services/notes/internal/transport/grpc/interceptor"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =========================================================================
// Snippet extensions
// =========================================================================

// DeleteSnippet removes the specified snippet.
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

// RestoreSnippet restores a snippet from the trash.
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

// MoveSnippet moves a snippet to the target group and can optionally set its sort order.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC MoveSnippet failed", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProto(result), nil
}

// SearchSnippets searches snippets by query criteria.
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

// GetPublicSnippet is deprecated. Public access now goes through share short links.
func (s *NoteServer) GetPublicSnippet(ctx context.Context, req *notepb.GetPublicSnippetRequest) (*notepb.SnippetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "public snippet access has been removed, use share tokens instead")
}

// FavoriteSnippet marks the specified snippet as a favorite.
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

// UnfavoriteSnippet removes the specified snippet from favorites.
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

// CreateSnippetFromTemplate creates a new snippet from an existing template.
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
		commonlogger.Ctx(ctx, s.log).Warn("failed to record template source", zap.Int64("snippet_id", result.ID), zap.Error(err))
	}

	return toProto(result), nil
}

// CreateSnippetFromShare imports a shared snippet into the current user's library.
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

	// File snippets must copy the object in storage to avoid sharing the same key.
	fileURL := source.Snippet.FileURL
	if source.Snippet.Type == "file" && fileURL != "" {
		copiedURL, err := s.uploadSvc.CopyObject(ctx, fileURL, userID)
		if err != nil {
			commonlogger.Ctx(ctx, s.log).Warn("fork file copy failed, falling back to shared URL",
				zap.String("src_url", fileURL), zap.Error(err))
			// Fallback: keep using the original URL if the copy fails so the main flow can continue.
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
		commonlogger.Ctx(ctx, s.log).Warn("failed to record share import source", zap.Int64("snippet_id", result.ID), zap.Error(err))
	}

	if source.Share.Kind == "template" {
		if err := s.shareSvc.IncrementForkCount(ctx, source.Share.ID); err != nil {
			commonlogger.Ctx(ctx, s.log).Warn("failed to increment share fork_count", zap.Int64("share_id", source.Share.ID), zap.Error(err))
		}
	}

	return toProto(result), nil
}

// =========================================================================
// Workspace lists
// =========================================================================

// ListRecentSnippets returns the user's recently accessed snippets.
func (s *NoteServer) ListRecentSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	return s.ListSnippets(ctx, req)
}

// ListSharedSnippets returns snippets shared with the current user.
func (s *NoteServer) ListSharedSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	return &notepb.ListSnippetsResponse{Snippets: []*notepb.SnippetResponse{}}, nil
}

// ListFavoriteSnippets returns snippets already favorited by the current user.
func (s *NoteServer) ListFavoriteSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	req.OnlyFavorites = true
	return s.ListSnippets(ctx, req)
}

// =========================================================================
// Resource management: groups
// =========================================================================

// ListGroups returns all groups owned by the current user.
func (s *NoteServer) ListGroups(ctx context.Context, req *notepb.ListGroupsRequest) (*notepb.ListGroupsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.groupSvc.List(ctx, userID, nil)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListGroups failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	groups := make([]*notepb.GroupResponse, 0, len(results))
	for i := range results {
		groups = append(groups, toProtoGroup(&results[i]))
	}

	return &notepb.ListGroupsResponse{Groups: groups}, nil
}

// GetGroup returns details for a single group.
func (s *NoteServer) GetGroup(ctx context.Context, req *notepb.GetGroupRequest) (*notepb.GroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.groupSvc.GetByID(ctx, userID, req.GroupId)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC GetGroup failed", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// CreateGroup creates a new group.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateGroup failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// UpdateGroup updates an existing group, for example by renaming it.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateGroup failed", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoGroup(result), nil
}

// DeleteGroup removes the specified group.
func (s *NoteServer) DeleteGroup(ctx context.Context, req *notepb.DeleteGroupRequest) (*notepb.DeleteGroupResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.groupSvc.Delete(ctx, userID, req.GroupId); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteGroup failed", zap.Int64("group_id", req.GroupId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteGroupResponse{Id: req.GroupId}, nil
}

// =========================================================================
// Resource management: tags
// =========================================================================

// ListTags returns all tags created by the current user.
func (s *NoteServer) ListTags(ctx context.Context, req *notepb.ListTagsRequest) (*notepb.ListTagsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.tagSvc.List(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListTags failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	tags := make([]*notepb.TagResponse, 0, len(results))
	for i := range results {
		tags = append(tags, toProtoTag(&results[i]))
	}

	return &notepb.ListTagsResponse{Tags: tags}, nil
}

// CreateTag creates a new personal tag.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateTag failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTag(result), nil
}

// UpdateTag updates a tag.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateTag failed", zap.Int64("tag_id", req.TagId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTag(result), nil
}

// DeleteTag removes a tag.
func (s *NoteServer) DeleteTag(ctx context.Context, req *notepb.DeleteTagRequest) (*notepb.DeleteTagResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.tagSvc.Delete(ctx, userID, req.TagId); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteTag failed", zap.Int64("tag_id", req.TagId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteTagResponse{Id: req.TagId}, nil
}

// =========================================================================
// Templates and attachment uploads
// =========================================================================

// ListTemplates returns system and personal templates available to the user.
func (s *NoteServer) ListTemplates(ctx context.Context, req *notepb.ListTemplatesRequest) (*notepb.ListTemplatesResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	results, err := s.templateSvc.List(ctx, userID, req.Category)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListTemplates failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	templates := make([]*notepb.TemplateResponse, 0, len(results))
	for i := range results {
		templates = append(templates, toProtoTemplate(&results[i]))
	}
	return &notepb.ListTemplatesResponse{Templates: templates}, nil
}

// GetTemplate returns the content and details of a single template.
func (s *NoteServer) GetTemplate(ctx context.Context, req *notepb.GetTemplateRequest) (*notepb.TemplateResponse, error) {
	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template ID")
	}

	result, err := s.templateSvc.GetByID(ctx, id)
	if err != nil {
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// CreateTemplate creates a personal template.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateTemplate failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// UpdateTemplate updates a personal template.
func (s *NoteServer) UpdateTemplate(ctx context.Context, req *notepb.UpdateTemplateRequest) (*notepb.TemplateResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template ID")
	}

	result, err := s.templateSvc.Update(ctx, userID, id, &templatecontract.UpdateTemplateCommand{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Language:    req.Language,
		Category:    req.Category,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateTemplate failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoTemplate(result), nil
}

// DeleteTemplate removes a personal template.
func (s *NoteServer) DeleteTemplate(ctx context.Context, req *notepb.DeleteTemplateRequest) (*notepb.DeleteTemplateResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parseTemplateID(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template ID")
	}

	if err := s.templateSvc.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC DeleteTemplate failed", zap.Int64("id", id), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.DeleteTemplateResponse{Id: req.TemplateId}, nil
}

// CreateShare creates a new share link.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateShare failed", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return toProtoShare(result), nil
}

// ListMyShares returns shares created by the current user.
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

// DeleteShare removes a share.
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

// GetPublicShareByToken returns a public share anonymously by token.
func (s *NoteServer) GetPublicShareByToken(ctx context.Context, req *notepb.GetPublicShareByTokenRequest) (*notepb.PublicShareResponse, error) {
	result, err := s.shareSvc.GetPublicByToken(ctx, req.Token, req.Password)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Warn("gRPC GetPublicShareByToken failed", zap.String("token", req.Token), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.PublicShareResponse{
		Share:   toProtoShare(result.Share),
		Snippet: toProto(result.Snippet),
	}, nil
}

// PresignUpload generates a presigned URL for direct browser uploads to MinIO.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC PresignUpload failed", zap.Error(err))
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

// CompleteUpload confirms that a direct browser upload has finished.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC CompleteUpload failed", zap.Error(err))
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

// UploadFile uploads file bytes over gRPC.
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
		commonlogger.Ctx(ctx, s.log).Error("gRPC UploadFile failed", zap.Error(err))
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
		return nil, status.Error(codes.PermissionDenied, "no permission to access AI data for this snippet")
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

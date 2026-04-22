package grpcserver

import (
	"context"

	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	notepb "github.com/ownforge/ownforge/pkg/proto/note"
	aimetadatarepo "github.com/ownforge/ownforge/services/notes/internal/repository/aimetadata"
	groupsvc "github.com/ownforge/ownforge/services/notes/internal/service/group"
	lineagesvc "github.com/ownforge/ownforge/services/notes/internal/service/lineage"
	sharesvc "github.com/ownforge/ownforge/services/notes/internal/service/share"
	snippetsvc "github.com/ownforge/ownforge/services/notes/internal/service/snippet"
	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"
	tagsvc "github.com/ownforge/ownforge/services/notes/internal/service/tag"
	templatesvc "github.com/ownforge/ownforge/services/notes/internal/service/template"
	uploadsvc "github.com/ownforge/ownforge/services/notes/internal/service/upload"
	grpcerrs "github.com/ownforge/ownforge/services/notes/internal/transport/grpc/codec/errs"
	"github.com/ownforge/ownforge/services/notes/internal/transport/grpc/interceptor"
	"go.uber.org/zap"
)

// NoteServer implements the NoteServiceServer interface.
type NoteServer struct {
	notepb.UnimplementedNoteServiceServer
	snippetSvc     snippetsvc.SnippetService
	groupSvc       groupsvc.GroupService
	tagSvc         tagsvc.TagService
	templateSvc    templatesvc.TemplateService
	lineageSvc     lineagesvc.Service
	shareSvc       sharesvc.Service
	uploadSvc      uploadsvc.UploadService
	aimetadataRepo aimetadatarepo.Repository
	log            *zap.Logger
}

// NewNoteServer creates a gRPC implementation of NoteService.
func NewNoteServer(
	snippetSvc snippetsvc.SnippetService,
	groupSvc groupsvc.GroupService,
	tagSvc tagsvc.TagService,
	templateSvc templatesvc.TemplateService,
	lineageSvc lineagesvc.Service,
	shareSvc sharesvc.Service,
	uploadSvc uploadsvc.UploadService,
	aimetadataRepo aimetadatarepo.Repository,
	log *zap.Logger,
) *NoteServer {
	return &NoteServer{
		snippetSvc:     snippetSvc,
		groupSvc:       groupSvc,
		tagSvc:         tagSvc,
		templateSvc:    templateSvc,
		lineageSvc:     lineageSvc,
		shareSvc:       shareSvc,
		uploadSvc:      uploadSvc,
		aimetadataRepo: aimetadataRepo,
		log:            log,
	}
}

func (s *NoteServer) CreateSnippet(ctx context.Context, req *notepb.CreateSnippetRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cmd := &contract.CreateSnippetCommand{
		Type:     "code",
		Title:    req.Title,
		Content:  req.Content,
		Language: req.Language,
	}
	if req.GroupId != nil {
		cmd.GroupID = req.GroupId
	}

	result, err := s.snippetSvc.Create(ctx, userID, cmd)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC CreateSnippet failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}
	return toProto(result), nil
}

func (s *NoteServer) GetSnippet(ctx context.Context, req *notepb.GetSnippetRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.snippetSvc.GetMineByID(ctx, userID, req.SnippetId)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC GetSnippet failed", zap.Int64("snippet_id", req.SnippetId), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}
	return toProto(result), nil
}

func (s *NoteServer) ListSnippets(ctx context.Context, req *notepb.ListSnippetsRequest) (*notepb.ListSnippetsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	query := &contract.ListQuery{
		Keyword:       req.Keyword,
		Cursor:        req.Cursor,
		Limit:         int(req.Limit),
		SortBy:        req.SortBy,
		Type:          req.Type,
		Status:        req.Status,
		OnlyFavorites: req.OnlyFavorites,
	}
	if req.GroupId != nil {
		query.GroupID = req.GroupId
	}
	if req.TagId != nil {
		query.TagID = req.TagId
	}

	result, err := s.snippetSvc.ListMineFiltered(ctx, userID, query)
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC ListSnippets failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	snippets := make([]*notepb.SnippetResponse, 0, len(result.Items))
	for i := range result.Items {
		snippets = append(snippets, toProto(&result.Items[i]))
	}
	return &notepb.ListSnippetsResponse{
		Snippets:   snippets,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

func (s *NoteServer) UpdateSnippet(ctx context.Context, req *notepb.UpdateSnippetRequest) (*notepb.SnippetResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.snippetSvc.Update(ctx, userID, req.SnippetId, &contract.UpdateSnippetCommand{
		Title:    req.Title,
		Content:  req.Content,
		Language: req.Language,
	})
	if err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC UpdateSnippet failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}
	return toProto(result), nil
}

// toProto converts a service-layer result into a protobuf response.
func toProto(r *contract.SnippetResult) *notepb.SnippetResponse {
	resp := &notepb.SnippetResponse{
		Id:         r.ID,
		OwnerId:    r.OwnerID,
		Title:      r.Title,
		Type:       r.Type,
		Content:    r.Content,
		FileUrl:    r.FileURL,
		FileSize:   r.FileSize,
		MimeType:   r.MimeType,
		Language:   r.Language,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		TagIds:     r.TagIDs,
		SortOrder:  int32(r.SortOrder),
		IsFavorite: r.IsFavorite,
		DeletedAt:  r.DeletedAt,
	}
	if r.GroupID != nil {
		resp.GroupId = *r.GroupID
	}
	return resp
}

func (s *NoteServer) SetSnippetTags(ctx context.Context, req *notepb.SetSnippetTagsRequest) (*notepb.SetSnippetTagsResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.snippetSvc.SetTags(ctx, userID, req.SnippetId, req.TagIds); err != nil {
		commonlogger.Ctx(ctx, s.log).Error("gRPC SetSnippetTags failed", zap.Error(err))
		return nil, grpcerrs.ToStatusError(err)
	}

	return &notepb.SetSnippetTagsResponse{}, nil
}

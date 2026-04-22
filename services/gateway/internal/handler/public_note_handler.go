package handler

import (
	"io"
	"strconv"

	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	notepb "github.com/ownforge/ownforge/pkg/proto/note"
	"go.uber.org/zap"
)

// PublicNoteHandler handles public note endpoints that do not require authentication.
// These endpoints do not go through the JWT middleware group, so they cannot be uniformly proxied via gRPC-Gateway,
// and therefore remain standalone Gin handlers.
type PublicNoteHandler struct {
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

// NewPublicNoteHandler creates a public note handler.
func NewPublicNoteHandler(noteClient notepb.NoteServiceClient, log *zap.Logger) *PublicNoteHandler {
	return &PublicNoteHandler{noteClient: noteClient, log: log}
}

// GetPublic fetches a public snippet without login.
// GET /api/v1/notes/public/snippets/:id
func (h *PublicNoteHandler) GetPublic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid note ID")
		return
	}

	res, err := h.noteClient.GetPublicSnippet(c.Request.Context(), &notepb.GetPublicSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("failed to fetch public note", zap.Int64("snippetID", id), zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetPublicShare fetches a public share without login.
// GET /api/v1/notes/public/shares/:token
func (h *PublicNoteHandler) GetPublicShare(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")
	if password == "" {
		password = c.GetHeader("X-Share-Password")
	}

	res, err := h.noteClient.GetPublicShareByToken(c.Request.Context(), &notepb.GetPublicShareByTokenRequest{
		Token:    token,
		Password: password,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("failed to fetch public share", zap.String("token", token), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, res)
}

// UploadHandler handles file-upload endpoints.
// Binary-stream uploads do not fit JSON-based gRPC-Gateway well, so they remain handwritten handlers.
type UploadHandler struct {
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

// NewUploadHandler creates a file-upload handler.
func NewUploadHandler(noteClient notepb.NoteServiceClient, log *zap.Logger) *UploadHandler {
	return &UploadHandler{noteClient: noteClient, log: log}
}

// Upload receives multipart/form-data files from the browser,
// forwards them to the go-note microservice via gRPC UploadFile for MinIO storage,
// and then returns the file URL and related metadata to the frontend.
// POST /api/v1/notes/uploads
func (h *UploadHandler) Upload(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "unauthorized")
		return
	}
	userID := val.(int64)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "missing uploaded file")
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("failed to read uploaded file", zap.Error(err))
		response.BadRequest(c, "failed to read file")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.noteClient.UploadFile(grpcCtx, &notepb.UploadFileRequest{
		FileData: fileData,
		Filename: header.Filename,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("gRPC UploadFile failed",
			zap.Int64("userID", userID),
			zap.String("filename", header.Filename),
			zap.Error(err),
		)
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, gin.H{
		"url":           resp.Url,
		"filename":      resp.Filename,
		"size":          resp.Size,
		"mime_type":     resp.MimeType,
		"thumbnail_url": resp.ThumbnailUrl,
	})
}

package handler

import (
	uploadsvc "github.com/ownforge/ownforge/services/notes/internal/service/upload"
	"github.com/ownforge/ownforge/services/notes/internal/service/upload/contract"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/codec/response"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/server/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UploadHandler handles attachment upload HTTP requests.
type UploadHandler struct {
	svc    uploadsvc.UploadService
	logger *zap.Logger
}

// NewUploadHandler creates an UploadHandler instance.
func NewUploadHandler(svc uploadsvc.UploadService, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{svc: svc, logger: logger}
}

// Upload accepts a single file, stores it in object storage, and returns its access URL.
// POST /api/v1/notes/uploads
func (h *UploadHandler) Upload(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "not logged in")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "missing uploaded file")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	cmd := &contract.UploadCommand{
		OwnerID:     userID,
		Filename:    header.Filename,
		Size:        header.Size,
		ContentType: contentType,
	}

	result, err := h.svc.Upload(c.Request.Context(), cmd, file)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{
		"url":           result.URL,
		"filename":      result.Filename,
		"size":          result.Size,
		"mime_type":     result.MimeType,
		"thumbnail_url": result.ThumbnailURL,
	})
}

// Presign generates a presigned URL for direct browser uploads to object storage.
func (h *UploadHandler) Presign(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "not logged in")
		return
	}

	var req struct {
		Filename string `json:"filename"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid upload-signature parameters")
		return
	}

	result, err := h.svc.Presign(c.Request.Context(), &contract.PresignCommand{
		OwnerID:     userID,
		Filename:    req.Filename,
		Size:        req.Size,
		ContentType: req.MimeType,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{
		"url":        result.URL,
		"object_key": result.ObjectKey,
		"expires_at": result.ExpiresAt,
		"headers":    result.Headers,
		"public_url": result.PublicURL,
	})
}

// Complete confirms that a direct frontend upload has finished.
func (h *UploadHandler) Complete(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Unauthorized(c, "not logged in")
		return
	}

	var req struct {
		ObjectKey string `json:"object_key"`
		Filename  string `json:"filename"`
		Size      int64  `json:"size"`
		MimeType  string `json:"mime_type"`
		SnippetID *int64 `json:"snippet_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid upload-complete parameters")
		return
	}

	result, err := h.svc.Complete(c.Request.Context(), &contract.CompleteUploadCommand{
		OwnerID:     userID,
		ObjectKey:   req.ObjectKey,
		Filename:    req.Filename,
		Size:        req.Size,
		ContentType: req.MimeType,
		SnippetID:   req.SnippetID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, gin.H{
		"url":           result.URL,
		"object_key":    result.ObjectKey,
		"filename":      result.Filename,
		"size":          result.Size,
		"mime_type":     result.MimeType,
		"thumbnail_url": result.ThumbnailURL,
	})
}

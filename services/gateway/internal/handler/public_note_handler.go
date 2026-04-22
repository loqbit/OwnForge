package handler

import (
	"io"
	"strconv"

	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
	notepb "github.com/luckysxx/common/proto/note"
	"go.uber.org/zap"
)

// PublicNoteHandler 处理无需鉴权的笔记公开端点。
// 此类端点不走 JWT 中间件组，因此不能通过 gRPC-Gateway 统一代理，
// 需保留为独立的 Gin handler。
type PublicNoteHandler struct {
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

// NewPublicNoteHandler 创建一个公开笔记 handler。
func NewPublicNoteHandler(noteClient notepb.NoteServiceClient, log *zap.Logger) *PublicNoteHandler {
	return &PublicNoteHandler{noteClient: noteClient, log: log}
}

// GetPublic 获取公开片段（不需要登录）。
// GET /api/v1/notes/public/snippets/:id
func (h *PublicNoteHandler) GetPublic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的笔记ID")
		return
	}

	res, err := h.noteClient.GetPublicSnippet(c.Request.Context(), &notepb.GetPublicSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取公开笔记失败", zap.Int64("snippetID", id), zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetPublicShare 获取公开分享（不需要登录）。
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
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("获取公开分享失败", zap.String("token", token), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, res)
}

// UploadHandler 处理文件上传端点。
// 二进制流上传不适合 JSON-based gRPC-Gateway，保留为手写 handler。
type UploadHandler struct {
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

// NewUploadHandler 创建一个文件上传 handler。
func NewUploadHandler(noteClient notepb.NoteServiceClient, log *zap.Logger) *UploadHandler {
	return &UploadHandler{noteClient: noteClient, log: log}
}

// Upload 接收浏览器的 multipart/form-data 文件，
// 通过 gRPC UploadFile 转发给 go-note 微服务写入 MinIO，
// 然后将文件访问 URL 等信息返回给前端。
// POST /api/v1/notes/uploads
func (h *UploadHandler) Upload(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少上传文件")
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("读取上传文件失败", zap.Error(err))
		response.BadRequest(c, "读取文件失败")
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.noteClient.UploadFile(grpcCtx, &notepb.UploadFileRequest{
		FileData: fileData,
		Filename: header.Filename,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("gRPC UploadFile 失败",
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

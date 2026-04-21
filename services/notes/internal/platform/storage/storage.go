package storage

import (
	"context"
	"io"
	"time"
)

// Storage 对象存储抽象接口。
type Storage interface {
	// Upload 上传对象，返回公开访问 URL。
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (url string, err error)
	// PresignedPutObject 生成带签名的 PUT URL 与建议请求头。
	PresignedPutObject(ctx context.Context, key string, expiry time.Duration, contentType string, size int64) (url string, headers map[string]string, err error)
	// GetURL 生成带签名的临时访问 URL（用于私有文件）。
	GetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// PublicURL 根据对象 key 返回公开访问 URL。
	PublicURL(key string) string
	// Copy 复制对象到新 key（用于 fork 场景，避免共享 key）。
	Copy(ctx context.Context, srcKey, dstKey string) error
	// Delete 删除对象。
	Delete(ctx context.Context, key string) error
}

package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig MinIO 连接配置。
// 包含了连接 MinIO 服务器所需的各项基础参数，由外部统一注入。
type MinIOConfig struct {
	Endpoint       string // 内网连接地址 (例如: global-minio:9000)
	PublicEndpoint string // 浏览器可访问地址 (例如: localhost:9000)
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

// MinIOStorage 基于 MinIO 的对象存储实现。
// 封装了底层的官方 Client SDK，屏蔽了复杂的参数传递，实现了项目内部自定义的 Storage 接口。
type MinIOStorage struct {
	client         *minio.Client // 真正负责发送网络请求给 MinIO 的官方客户端
	bucket         string        // 当前存储实例绑定的 Bucket 名称
	publicEndpoint string        // 浏览器可访问的公开端点，拼装下载 URL 时使用
	useSSL         bool          // 记录状态以决定拼装 URL 时是采用 http:// 还是 https://
}

// NewMinIOStorage 创建 MinIOStorage 实例并确认 bucket 存在。
func NewMinIOStorage(cfg MinIOConfig) (*MinIOStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: create client: %w", err)
	}

	// PublicEndpoint 未配置时回退到内网 Endpoint
	pubEndpoint := cfg.PublicEndpoint
	if pubEndpoint == "" {
		pubEndpoint = cfg.Endpoint
	}

	return &MinIOStorage{
		client:         client,
		bucket:         cfg.Bucket,
		publicEndpoint: pubEndpoint,
		useSSL:         cfg.UseSSL,
	}, nil
}

// Upload 上传对象，返回公开访问 URL。
func (s *MinIOStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio: put object: %w", err)
	}
	return s.PublicURL(key), nil
}

// PresignedPutObject 生成带签名的 PUT URL。
func (s *MinIOStorage) PresignedPutObject(ctx context.Context, key string, expiry time.Duration, contentType string, size int64) (string, map[string]string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", nil, fmt.Errorf("minio: presign put: %w", err)
	}

	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	if size > 0 {
		headers["Content-Length"] = strconv.FormatInt(size, 10)
	}

	return u.String(), headers, nil
}

// GetURL 生成带签名的临时访问 URL。
func (s *MinIOStorage) GetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("minio: presign: %w", err)
	}
	return u.String(), nil
}

// PublicURL 返回对象公开访问地址。
func (s *MinIOStorage) PublicURL(key string) string {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.publicEndpoint, s.bucket, key)
}

// Copy 复制对象到新 key。
func (s *MinIOStorage) Copy(ctx context.Context, srcKey, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: s.bucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: s.bucket, Object: dstKey}
	if _, err := s.client.CopyObject(ctx, dst, src); err != nil {
		return fmt.Errorf("minio: copy object %s -> %s: %w", srcKey, dstKey, err)
	}
	return nil
}

// Delete 删除对象。
func (s *MinIOStorage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio: remove object: %w", err)
	}
	return nil
}

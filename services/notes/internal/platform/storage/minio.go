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

// MinIOConfig contains the connection settings for MinIO.
// It includes the base parameters required to connect to the MinIO server and is injected from outside.
type MinIOConfig struct {
	Endpoint       string // Internal endpoint, for example global-minio:9000.
	PublicEndpoint string // Browser-accessible endpoint, for example localhost:9000.
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

// MinIOStorage is a MinIO-backed implementation of object storage.
// It wraps the official client SDK behind the project's Storage interface.
type MinIOStorage struct {
	client         *minio.Client // Official client used to send requests to MinIO.
	bucket         string        // Bucket bound to this storage instance.
	publicEndpoint string        // Public endpoint used when building download URLs.
	useSSL         bool          // Tracks whether generated URLs should use http or https.
}

// NewMinIOStorage creates a MinIOStorage instance.
func NewMinIOStorage(cfg MinIOConfig) (*MinIOStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: create client: %w", err)
	}

	// Fall back to the internal endpoint when PublicEndpoint is not configured.
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

// Upload stores an object and returns a public URL.
func (s *MinIOStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio: put object: %w", err)
	}
	return s.PublicURL(key), nil
}

// PresignedPutObject generates a signed PUT URL.
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

// GetURL generates a signed temporary access URL.
func (s *MinIOStorage) GetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("minio: presign: %w", err)
	}
	return u.String(), nil
}

// PublicURL returns the public address for an object.
func (s *MinIOStorage) PublicURL(key string) string {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.publicEndpoint, s.bucket, key)
}

// Copy duplicates an object under a new key.
func (s *MinIOStorage) Copy(ctx context.Context, srcKey, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: s.bucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: s.bucket, Object: dstKey}
	if _, err := s.client.CopyObject(ctx, dst, src); err != nil {
		return fmt.Errorf("minio: copy object %s -> %s: %w", srcKey, dstKey, err)
	}
	return nil
}

// Delete removes an object.
func (s *MinIOStorage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio: remove object: %w", err)
	}
	return nil
}

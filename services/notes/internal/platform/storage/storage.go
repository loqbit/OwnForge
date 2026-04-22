package storage

import (
	"context"
	"io"
	"time"
)

// Storage abstracts object storage operations.
type Storage interface {
	// Upload stores an object and returns a public URL.
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (url string, err error)
	// PresignedPutObject generates a signed PUT URL and suggested request headers.
	PresignedPutObject(ctx context.Context, key string, expiry time.Duration, contentType string, size int64) (url string, headers map[string]string, err error)
	// GetURL generates a signed temporary access URL for private files.
	GetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// PublicURL returns the public URL for an object key.
	PublicURL(key string) string
	// Copy duplicates an object under a new key, for example during fork flows to avoid shared keys.
	Copy(ctx context.Context, srcKey, dstKey string) error
	// Delete removes an object.
	Delete(ctx context.Context, key string) error
}

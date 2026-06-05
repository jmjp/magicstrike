package ports

import (
	"context"
	"io"
	"time"
)

// ObjectInfo contains metadata about a stored object.
type ObjectInfo struct {
	Key         string
	Size        int64
	MD5Hash     string
	ContentType string
}

// StorageService defines the output port for S3-compatible storage operations.
type StorageService interface {
	// GeneratePresignedUploadURL creates a time-limited URL that allows
	// a client to upload a file directly to storage without credentials.
	// Returns the URL string.
	// Errors: ErrStorageUnavailable if the storage backend cannot be reached.
	GeneratePresignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error)

	// Download retrieves a stored object by key. The caller MUST close the
	// returned io.ReadCloser when done.
	// Errors: ErrObjectNotFound if the key does not exist, ErrStorageUnavailable
	// if the storage backend cannot be reached.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// GetObjectInfo retrieves metadata about a stored object without downloading it.
	// Errors: ErrObjectNotFound if the key does not exist, ErrStorageUnavailable
	// if the storage backend cannot be reached.
	GetObjectInfo(ctx context.Context, key string) (*ObjectInfo, error)
}

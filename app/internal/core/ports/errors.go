package ports

import "errors"

// Storage errors
var (
	ErrStorageUnavailable = errors.New("storage service is unavailable")
	ErrObjectNotFound     = errors.New("object not found in storage")
	ErrInvalidKey         = errors.New("invalid storage key")
)

// Queue errors
var (
	ErrQueueUnavailable = errors.New("message queue is unavailable")
	ErrInvalidJob       = errors.New("invalid job message")
)

// Upload errors
var (
	ErrInvalidUploadRequest = errors.New("invalid upload request")
	ErrMatchNotFound        = errors.New("match not found")
	ErrDuplicateUpload      = errors.New("a demo with this MD5 hash has already been uploaded")
)

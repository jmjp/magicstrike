package ports

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var md5HexRegex = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

// UploadRequest represents the initial upload request from a user.
// UserID is set by the auth middleware from JWT claims — not from the JSON body.
type UploadRequest struct {
	UserID   string  `json:"-"`       // Set by middleware from JWT claims
	MatchID  string  `json:"match_id"` // Client-specified unique ID for the match
	Filename string  `json:"filename"` // Original filename (must end with .dem)
	MD5Hash  *string `json:"md5_hash,omitempty"`
	TeamA    string  `json:"team_a"`
	TeamB    string  `json:"team_b"`
	MapName  string  `json:"map_name"`
}

// Validate checks the UploadRequest fields.
func (r *UploadRequest) Validate() error {
	if r.UserID == "" {
		return errors.New("user ID is required")
	}
	if r.MatchID != "" && len(r.MatchID) > 64 {
		return errors.New("match ID must be at most 64 characters")
	}
	if r.Filename == "" {
		return errors.New("filename is required")
	}
	if !strings.HasSuffix(strings.ToLower(r.Filename), ".dem") {
		return errors.New("filename must have .dem extension")
	}
	if r.TeamA != "" && len(r.TeamA) > 100 {
		return errors.New("team A name must be at most 100 characters")
	}
	if r.TeamB != "" && len(r.TeamB) > 100 {
		return errors.New("team B name must be at most 100 characters")
	}
	if r.TeamA != "" && r.TeamB != "" && strings.EqualFold(r.TeamA, r.TeamB) {
		return errors.New("team A and team B cannot be the same")
	}
	if r.MD5Hash != nil {
		if !md5HexRegex.MatchString(*r.MD5Hash) {
			return errors.New("md5_hash must be a 32-character hex string")
		}
	}
	return nil
}

// UploadResponse is returned from RequestUpload with the presigned URL and metadata.
type UploadResponse struct {
	UploadURL string    `json:"upload_url"`
	BucketKey string    `json:"bucket_key"`
	ExpiresAt time.Time `json:"expires_at"`
	MatchID   string    `json:"match_id"`
}

// ConfirmUploadRequest is sent by the client after uploading the file to storage.
// UserID is set by the auth middleware from JWT claims — not from JSON body.
type ConfirmUploadRequest struct {
	UserID    string `json:"-"`
	MatchID   string `json:"match_id"`
	BucketKey string `json:"bucket_key"`
}

// Validate checks the ConfirmUploadRequest fields.
func (r *ConfirmUploadRequest) Validate() error {
	if r.MatchID == "" {
		return errors.New("match ID is required")
	}
	if r.BucketKey == "" {
		return errors.New("bucket key is required")
	}
	// Basic path traversal prevention
	if strings.Contains(r.BucketKey, "..") {
		return errors.New("bucket key contains invalid path segments")
	}
	return nil
}

// UploadUseCase defines the input port for demo file upload operations.
type UploadUseCase interface {
	// RequestUpload initiates an upload flow:
	// 1. Validates the UploadRequest.
	// 2. Creates a Match entity in "waiting" status.
	// 3. Generates a presigned upload URL via StorageService.
	// 4. Returns the URL and metadata to the caller.
	// Errors: ErrInvalidUploadRequest if validation fails.
	//         ErrStorageUnavailable if presigned URL generation fails.
	RequestUpload(ctx context.Context, input *UploadRequest) (*UploadResponse, error)

	// ConfirmUpload finalizes the upload flow after the user has uploaded the file:
	// 1. Validates the ConfirmUploadRequest.
	// 2. Calls StorageService.GetObjectInfo to verify the object exists.
	// 3. Updates the Match entity with DemoMD5 from object metadata.
	// 4. Publishes a DemoJobMessage to the queue.
	// Errors: ErrMatchNotFound if MatchID does not exist.
	//         ErrObjectNotFound if the uploaded file is missing.
	//         ErrDuplicateUpload if the MD5 matches an already-processed file.
	//         ErrQueueUnavailable if publishing the job fails.
	ConfirmUpload(ctx context.Context, input *ConfirmUploadRequest) error
}

package entities

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrEmptyMatchID      = errors.New("match ID cannot be empty")
	ErrEmptyBucketPath   = errors.New("bucket path cannot be empty")
	ErrInvalidMD5Hash    = errors.New("invalid MD5 hash format")
	ErrZeroTimestamp     = errors.New("upload timestamp cannot be zero")
)

var hexMD5Regex = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

// DemoJobMessage represents the value object for a demo processing job message.
// It is serialized to JSON for transport over AMQP.
type DemoJobMessage struct {
	MatchID    string `json:"match_id"`
	BucketPath string `json:"bucket_path"`
	MD5Hash    string `json:"md5_hash"`
	UploadedAt int64  `json:"uploaded_at"` // Unix timestamp in nanoseconds
}

// NewDemoJobMessage creates a new DemoJobMessage with validation.
func NewDemoJobMessage(matchID, bucketPath, md5Hash string, uploadedAt time.Time) (*DemoJobMessage, error) {
	job := &DemoJobMessage{
		MatchID:    strings.TrimSpace(matchID),
		BucketPath: strings.TrimSpace(bucketPath),
		MD5Hash:    strings.TrimSpace(md5Hash),
		UploadedAt: uploadedAt.UnixNano(),
	}
	if err := job.Valid(); err != nil {
		return nil, err
	}
	return job, nil
}

// Valid validates the DemoJobMessage fields.
func (j *DemoJobMessage) Valid() error {
	if j.MatchID == "" {
		return ErrEmptyMatchID
	}
	if j.BucketPath == "" {
		return ErrEmptyBucketPath
	}
	// MD5 hash is required for queue messages; validate format
	if j.MD5Hash == "" {
		return fmt.Errorf("%w: MD5 hash is required for queue messages", ErrInvalidMD5Hash)
	}
	if !hexMD5Regex.MatchString(j.MD5Hash) {
		return fmt.Errorf("%w: %s", ErrInvalidMD5Hash, j.MD5Hash)
	}
	if j.UploadedAt <= 0 {
		return ErrZeroTimestamp
	}
	return nil
}

// UploadTime returns the upload timestamp as time.Time.
func (j *DemoJobMessage) UploadTime() time.Time {
	return time.Unix(0, j.UploadedAt)
}

// ComputeMD5 computes the MD5 hash of the provided data and validates
// it against the stored hash. Returns an error if the hashes don't match.
func (j *DemoJobMessage) ComputeMD5(data []byte) (string, error) {
	hash := md5.Sum(data)
	computed := hex.EncodeToString(hash[:])
	if !strings.EqualFold(j.MD5Hash, computed) {
		return computed, fmt.Errorf("MD5 mismatch: expected %s, computed %s", j.MD5Hash, computed)
	}
	return computed, nil
}

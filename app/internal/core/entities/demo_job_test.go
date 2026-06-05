package entities

import (
	"strings"
	"testing"
	"time"
)

func TestNewDemoJobMessage(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		matchID    string
		bucketPath string
		md5Hash    string
		uploadedAt time.Time
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid",
			matchID:    "test-match-001",
			bucketPath: "uploads/user1/match1/test.dem",
			md5Hash:    "d41d8cd98f00b204e9800998ecf8427e",
			uploadedAt: now,
		},
		{
			name:       "empty match ID",
			matchID:    "",
			bucketPath: "uploads/test.dem",
			md5Hash:    "d41d8cd98f00b204e9800998ecf8427e",
			uploadedAt: now,
			wantErr:    true,
			errMsg:     "match ID cannot be empty",
		},
		{
			name:       "empty bucket path",
			matchID:    "test-001",
			bucketPath: "",
			md5Hash:    "d41d8cd98f00b204e9800998ecf8427e",
			uploadedAt: now,
			wantErr:    true,
			errMsg:     "bucket path cannot be empty",
		},
		{
			name:       "invalid MD5 (wrong length)",
			matchID:    "test-001",
			bucketPath: "uploads/test.dem",
			md5Hash:    "abc123",
			uploadedAt: now,
			wantErr:    true,
			errMsg:     "invalid MD5 hash format",
		},
		{
			name:       "invalid MD5 (non-hex chars)",
			matchID:    "test-001",
			bucketPath: "uploads/test.dem",
			md5Hash:    "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			uploadedAt: now,
			wantErr:    true,
			errMsg:     "invalid MD5 hash format",
		},
		{
			name:       "empty MD5",
			matchID:    "test-001",
			bucketPath: "uploads/test.dem",
			md5Hash:    "",
			uploadedAt: now,
			wantErr:    true,
			errMsg:     "MD5 hash is required",
		},
		{
			name:       "zero timestamp",
			matchID:    "test-001",
			bucketPath: "uploads/test.dem",
			md5Hash:    "d41d8cd98f00b204e9800998ecf8427e",
			uploadedAt: time.Time{},
			wantErr:    true,
			errMsg:     "upload timestamp cannot be zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := NewDemoJobMessage(tt.matchID, tt.bucketPath, tt.md5Hash, tt.uploadedAt)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if job.MatchID != tt.matchID {
					t.Errorf("MatchID: want %q, got %q", tt.matchID, job.MatchID)
				}
				if job.UploadedAt != tt.uploadedAt.UnixNano() {
					t.Errorf("UploadedAt: want %d, got %d", tt.uploadedAt.UnixNano(), job.UploadedAt)
				}
			}
		})
	}
}

func TestDemoJobMessage_UploadTime(t *testing.T) {
	now := time.Now()
	job, err := NewDemoJobMessage("test", "path/test.dem", "d41d8cd98f00b204e9800998ecf8427e", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	uploadTime := job.UploadTime()
	// Compare with second precision (nanoseconds may lose precision)
	if uploadTime.Unix() != now.Unix() {
		t.Errorf("UploadTime: want %v, got %v", now.Unix(), uploadTime.Unix())
	}
}

func TestDemoJobMessage_ComputeMD5(t *testing.T) {
	data := []byte("test data")
	expected := "eb733a00c0c9d336e65691a37ab54293" // MD5 of "test data"

	t.Run("match", func(t *testing.T) {
		job, _ := NewDemoJobMessage("test", "path/test.dem", expected, time.Now())
		computed, err := job.ComputeMD5(data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if computed != expected {
			t.Errorf("ComputeMD5: want %s, got %s", expected, computed)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		job, _ := NewDemoJobMessage("test", "path/test.dem", "00000000000000000000000000000000", time.Now())
		computed, err := job.ComputeMD5(data)
		if err == nil {
			t.Fatal("expected error on MD5 mismatch")
		}
		if computed != expected {
			t.Errorf("ComputeMD5 should return computed hash even on mismatch: want %s, got %s", expected, computed)
		}
		if !strings.Contains(err.Error(), "MD5 mismatch") {
			t.Errorf("expected 'MD5 mismatch' in error, got: %v", err)
		}
	})
}

func TestDemoJobMessage_WhitespaceTrimming(t *testing.T) {
	now := time.Now()
	job, err := NewDemoJobMessage("  test-001  ", "  uploads/test.dem  ", "  d41d8cd98f00b204e9800998ecf8427e  ", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.MatchID != "test-001" {
		t.Errorf("MatchID not trimmed: want 'test-001', got %q", job.MatchID)
	}
	if job.BucketPath != "uploads/test.dem" {
		t.Errorf("BucketPath not trimmed: want 'uploads/test.dem', got %q", job.BucketPath)
	}
}

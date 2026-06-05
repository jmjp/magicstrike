package ports

import (
	"strings"
	"testing"
)

func TestNewEmailAddress(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"valid", "test@example.com", nil},
		{"empty", "", ErrInvalidEmailFormat},
		{"too long", strings.Repeat("a", 255) + "@example.com", ErrEmailTooLong},
		{"no at", "example.com", ErrInvalidEmailFormat},
		{"no dot in domain", "test@domain", ErrInvalidEmailFormat},
		{"empty local part", "@example.com", ErrInvalidEmailFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := NewEmailAddress(tt.raw)
			if err != tt.wantErr {
				t.Errorf("NewEmailAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && addr.String() != strings.TrimSpace(tt.raw) {
				t.Errorf("expected string representation to match raw trimmed, got %s", addr.String())
			}
		})
	}
}

func TestUploadRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     UploadRequest
		wantErr string
	}{
		{
			name: "valid",
			req: UploadRequest{
				UserID:   "u123",
				MatchID:  "m123",
				Filename: "test.dem",
				TeamA:    "NaVi",
				TeamB:    "Vitality",
			},
			wantErr: "",
		},
		{
			name: "empty UserID",
			req: UploadRequest{
				UserID:   "",
				Filename: "test.dem",
			},
			wantErr: "user ID is required",
		},
		{
			name: "MatchID too long",
			req: UploadRequest{
				UserID:   "u123",
				MatchID:  strings.Repeat("a", 65),
				Filename: "test.dem",
			},
			wantErr: "must be at most 64 characters",
		},
		{
			name: "empty filename",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "",
			},
			wantErr: "filename is required",
		},
		{
			name: "invalid extension",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "test.txt",
			},
			wantErr: "must have .dem extension",
		},
		{
			name: "Team A too long",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "test.dem",
				TeamA:    strings.Repeat("a", 101),
			},
			wantErr: "at most 100 characters",
		},
		{
			name: "Team B too long",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "test.dem",
				TeamB:    strings.Repeat("a", 101),
			},
			wantErr: "at most 100 characters",
		},
		{
			name: "same teams",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "test.dem",
				TeamA:    "NaVi",
				TeamB:    "NaVi",
			},
			wantErr: "cannot be the same",
		},
		{
			name: "invalid md5",
			req: UploadRequest{
				UserID:   "u123",
				Filename: "test.dem",
				MD5Hash:  ptr("invalid"),
			},
			wantErr: "must be a 32-character hex string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			}
		})
	}
}

func ptr(s string) *string {
	return &s
}

func TestConfirmUploadRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ConfirmUploadRequest
		wantErr string
	}{
		{"valid", ConfirmUploadRequest{MatchID: "m123", BucketKey: "uploads/test.dem"}, ""},
		{"empty match id", ConfirmUploadRequest{MatchID: "", BucketKey: "uploads/test.dem"}, "match ID is required"},
		{"empty bucket key", ConfirmUploadRequest{MatchID: "m123", BucketKey: ""}, "bucket key is required"},
		{"path traversal", ConfirmUploadRequest{MatchID: "m123", BucketKey: "uploads/../../test.dem"}, "contains invalid path segments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			}
		})
	}
}

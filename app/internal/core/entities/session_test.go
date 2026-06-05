package entities

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func ptr(s string) *string {
	return &s
}

func TestNewSession(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	pastTime := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name      string
		userID    string
		userAgent *string
		ipAddress *string
		device    *string
		expiresAt time.Time
		wantErr   error
	}{
		{
			name:      "valid session",
			userID:    "user_123",
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr("MacBook Pro"),
			expiresAt: futureTime,
			wantErr:   nil,
		},
		{
			name:      "empty userID",
			userID:    "",
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr("MacBook Pro"),
			expiresAt: futureTime,
			wantErr:   ErrUserIDRequired,
		},
		{
			name:      "userID too long",
			userID:    strings.Repeat("u", 65),
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr("MacBook Pro"),
			expiresAt: futureTime,
			wantErr:   ErrUserIDTooLong,
		},
		{
			name:      "user agent too long",
			userID:    "user_123",
			userAgent: ptr(strings.Repeat("a", 513)),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr("MacBook Pro"),
			expiresAt: futureTime,
			wantErr:   ErrUserAgentTooLong,
		},
		{
			name:      "ip address too long",
			userID:    "user_123",
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr(strings.Repeat("1", 46)),
			device:    ptr("MacBook Pro"),
			expiresAt: futureTime,
			wantErr:   ErrIPAddressTooLong,
		},
		{
			name:      "device too long",
			userID:    "user_123",
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr(strings.Repeat("d", 257)),
			expiresAt: futureTime,
			wantErr:   ErrDeviceTooLong,
		},
		{
			name:      "expired session creation",
			userID:    "user_123",
			userAgent: ptr("Mozilla/5.0"),
			ipAddress: ptr("192.168.1.1"),
			device:    ptr("MacBook Pro"),
			expiresAt: pastTime,
			wantErr:   ErrSessionExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSession(tt.userID, tt.userAgent, tt.ipAddress, tt.device, tt.expiresAt)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil && got == nil {
				t.Error("NewSession() returned nil for valid inputs")
			}
		})
	}
}

func TestSession_Valid(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name    string
		session Session
		wantErr error
	}{
		{
			name: "valid session",
			session: Session{
				UserID:    "user_123",
				ExpiresAt: futureTime,
			},
			wantErr: nil,
		},
		{
			name: "session ID too long",
			session: Session{
				ID:        strings.Repeat("s", 65),
				UserID:    "user_123",
				ExpiresAt: futureTime,
			},
			wantErr: ErrSessionIDTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Valid()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkSession_Valid(b *testing.B) {
	futureTime := time.Now().Add(1 * time.Hour)
	ua := "Mozilla/5.0"
	ip := "192.168.1.1"
	dev := "MacBook Pro"

	s := Session{
		UserID:    "user_123",
		UserAgent: &ua,
		IPAddress: &ip,
		Device:    &dev,
		ExpiresAt: futureTime,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Valid()
	}
}

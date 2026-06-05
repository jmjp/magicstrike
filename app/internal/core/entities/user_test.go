package entities

import (
	"errors"
	"strings"
	"testing"
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		username string
		avatar   string
		wantErr  error
	}{
		{
			name:     "valid user creation",
			email:    "test@example.com",
			username: "testuser",
			avatar:   "avatar.png",
			wantErr:  nil,
		},
		{
			name:     "invalid email",
			email:    "invalid-email",
			username: "testuser",
			avatar:   "avatar.png",
			wantErr:  ErrInvalidEmail,
		},
		{
			name:     "empty username",
			email:    "test@example.com",
			username: "",
			avatar:   "avatar.png",
			wantErr:  ErrUsernameRequired,
		},
		{
			name:     "username too short",
			email:    "test@example.com",
			username: "ab",
			avatar:   "avatar.png",
			wantErr:  ErrUsernameTooShort,
		},
		{
			name:     "username too long",
			email:    "test@example.com",
			username: strings.Repeat("a", 51),
			avatar:   "avatar.png",
			wantErr:  ErrUsernameTooLong,
		},
		{
			name:     "email too long",
			email:    strings.Repeat("a", 246) + "@example.com",
			username: "testuser",
			avatar:   "avatar.png",
			wantErr:  ErrEmailTooLong,
		},
		{
			name:     "avatar too long",
			email:    "test@example.com",
			username: "testuser",
			avatar:   strings.Repeat("a", 2049),
			wantErr:  ErrAvatarTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUser(tt.email, tt.username, tt.avatar)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil && got == nil {
				t.Error("NewUser() returned nil for valid inputs")
			}
		})
	}
}

func TestUser_Valid(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr error
	}{
		{
			name: "valid user",
			user: User{
				Email:    "valid@example.com",
				Username: "user",
				Points:   10,
				Blocked:  false,
			},
			wantErr: nil,
		},
		{
			name: "negative points",
			user: User{
				Email:    "valid@example.com",
				Username: "user",
				Points:   -1,
				Blocked:  false,
			},
			wantErr: ErrNegativePoints,
		},
		{
			name: "blocked user validation",
			user: User{
				Email:    "valid@example.com",
				Username: "user",
				Points:   10,
				Blocked:  true,
			},
			wantErr: ErrUserBlocked,
		},
		{
			name: "ID too long",
			user: User{
				ID:       strings.Repeat("a", 65),
				Email:    "valid@example.com",
				Username: "user",
				Points:   10,
				Blocked:  false,
			},
			wantErr: ErrIDTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Valid()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkUser_Valid(b *testing.B) {
	u := User{
		Email:    "test@example.com",
		Username: "testuser",
		Points:   10,
		Blocked:  false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = u.Valid()
	}
}

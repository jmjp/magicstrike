package ports

import (
	"context"
	"time"

	"magicstrike/internal/core/entities"
)

// SessionResult holds the result of a successful authentication or session refresh.
type SessionResult struct {
	AccessToken string         `json:"access_token"`
	SessionID   string         `json:"session_id,omitempty"`
	User        *entities.User `json:"user"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// MagicLinkData holds the metadata associated with a magic link token.
type MagicLinkData struct {
	Email     string
	ExpiresAt time.Time
}

// AuthUseCase defines the input port for passwordless authentication operations.
type AuthUseCase interface {
	RequestMagicLink(ctx context.Context, email string) error
	ValidateToken(ctx context.Context, token string) (*SessionResult, error)
	RefreshSession(ctx context.Context, sessionID string) (*SessionResult, error)
	Logout(ctx context.Context, sessionID string) error
}

// MagicLinkStore defines the output port for magic link token persistence.
type MagicLinkStore interface {
	Store(ctx context.Context, token string, data MagicLinkData) error
	Retrieve(ctx context.Context, token string) (*MagicLinkData, error)
	Delete(ctx context.Context, token string) error
}

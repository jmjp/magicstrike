package entities

import (
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	// ErrSessionExpired is returned when the session has expired.
	ErrSessionExpired = errors.New("session has expired")
	// ErrUserIDRequired is returned when the user ID is empty.
	ErrUserIDRequired = errors.New("user ID is required")
	// ErrUserIDTooLong is returned when the user ID exceeds length limits.
	ErrUserIDTooLong = errors.New("user ID is too long")
	// ErrSessionIDTooLong is returned when the session ID exceeds length limits.
	ErrSessionIDTooLong = errors.New("session ID is too long")
	// ErrUserAgentTooLong is returned when the user agent string exceeds limits.
	ErrUserAgentTooLong = errors.New("user agent is too long")
	// ErrIPAddressTooLong is returned when the IP address string exceeds limits.
	ErrIPAddressTooLong = errors.New("IP address is too long")
	// ErrDeviceTooLong is returned when the device string exceeds limits.
	ErrDeviceTooLong = errors.New("device description is too long")
)

const (
	maxSessionIDLen = 64
	maxUserIDLen    = 64
	maxIPAddressLen = 45 // Fits IPv6 addresses
	maxUserAgentLen = 512
	maxDeviceLen    = 256
)

// Session represents a user session optimized for memory alignment (fields ordered by size).
type Session struct {
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserAgent *string   `json:"user_agent"`
	IPAddress *string   `json:"ip_address"`
	Device    *string   `json:"device"`
}

// NewSession creates a new Session instance with basic properties.
func NewSession(userID string, userAgent, ipAddress, device *string, expiresAt time.Time) (*Session, error) {
	session := &Session{
		ID:        ulid.Make().String(),
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		UserID:    userID,
		UserAgent: userAgent,
		IPAddress: ipAddress,
		Device:    device,
	}

	if err := session.Valid(); err != nil {
		return nil, err
	}

	return session, nil
}

// Valid checks if the session properties and lifetime constraints are met.
func (s *Session) Valid() error {
	if len(s.ID) > maxSessionIDLen {
		return ErrSessionIDTooLong
	}

	if s.UserID == "" {
		return ErrUserIDRequired
	}
	if len(s.UserID) > maxUserIDLen {
		return ErrUserIDTooLong
	}

	if s.UserAgent != nil && len(*s.UserAgent) > maxUserAgentLen {
		return ErrUserAgentTooLong
	}

	if s.IPAddress != nil && len(*s.IPAddress) > maxIPAddressLen {
		return ErrIPAddressTooLong
	}

	if s.Device != nil && len(*s.Device) > maxDeviceLen {
		return ErrDeviceTooLong
	}

	if time.Now().After(s.ExpiresAt) {
		return ErrSessionExpired
	}

	return nil
}

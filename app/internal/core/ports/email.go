package ports

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidEmailFormat = errors.New("invalid email format")
	ErrEmailTooLong       = errors.New("email is too long")
)

const maxEmailLen = 254

// EmailAddress represents a validated email address string.
type EmailAddress string

// NewEmailAddress creates a validated EmailAddress value object.
func NewEmailAddress(raw string) (EmailAddress, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidEmailFormat
	}
	if len(trimmed) > maxEmailLen {
		return "", ErrEmailTooLong
	}
	if !strings.Contains(trimmed, "@") {
		return "", ErrInvalidEmailFormat
	}
	parts := strings.SplitN(trimmed, "@", 2)
	if len(parts) != 2 || parts[0] == "" || !strings.Contains(parts[1], ".") {
		return "", ErrInvalidEmailFormat
	}
	return EmailAddress(trimmed), nil
}

// String returns the raw email address as a string.
func (e EmailAddress) String() string {
	return string(e)
}

// EmailSender defines the output port for sending emails (magic links, notifications).
type EmailSender interface {
	SendMagicLink(ctx context.Context, to EmailAddress, token string) error
}

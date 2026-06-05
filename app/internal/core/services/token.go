package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// TokenGenerator is a domain service that generates cryptographically secure tokens.
type TokenGenerator struct{}

// NewTokenGenerator creates a new TokenGenerator instance.
func NewTokenGenerator() *TokenGenerator {
	return &TokenGenerator{}
}

// GenerateToken generates a cryptographically secure 32-byte (256-bit) random token
// encoded as a 64-character hexadecimal string.
func (s *TokenGenerator) GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

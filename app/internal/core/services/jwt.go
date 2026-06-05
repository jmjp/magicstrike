package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultJWTSecret   = "magicstrike-dev-secret-change-in-production"
	defaultAccessTokenTTL = 7 * 24 * time.Hour
)

// JWTService is a domain service for signing and verifying JWT access tokens.
type JWTService struct {
	secret []byte
}

// JWTClaims carries the custom claims embedded in access tokens.
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

// NewJWTService creates a new JWTService with the given secret.
// If secret is empty, a default development secret is used.
func NewJWTService(secret string) *JWTService {
	if secret == "" {
		secret = defaultJWTSecret
	}
	return &JWTService{secret: []byte(secret)}
}

// Sign creates a signed JWT access token for the given user and session.
func (s *JWTService) Sign(userID, sessionID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = defaultAccessTokenTTL
	}

	now := time.Now()
	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "magicstrike",
			Subject:   userID,
			ID:        sessionID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, nil
}

// Verify parses and validates a JWT access token, returning the claims if valid.
func (s *JWTService) Verify(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid JWT token")
	}

	return claims, nil
}

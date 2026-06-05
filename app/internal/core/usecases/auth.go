package usecases

import (
	"context"
	"fmt"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
)

const sessionTTL = 7 * 24 * time.Hour

// AuthUseCase implements ports.AuthUseCase for passwordless authentication.
type AuthUseCase struct {
	userRepo    ports.UserRepository
	sessionRepo ports.SessionRepository
	magicStore  ports.MagicLinkStore
	emailSender ports.EmailSender
	tokenGen    *services.TokenGenerator
	rateLimiter *services.RateLimiter
	jwtSvc      *services.JWTService
}

// NewAuthUseCase creates a new AuthUseCase instance.
func NewAuthUseCase(
	userRepo ports.UserRepository,
	sessionRepo ports.SessionRepository,
	magicStore ports.MagicLinkStore,
	emailSender ports.EmailSender,
	tokenGen *services.TokenGenerator,
	rateLimiter *services.RateLimiter,
	jwtSvc *services.JWTService,
) ports.AuthUseCase {
	return &AuthUseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		magicStore:  magicStore,
		emailSender: emailSender,
		tokenGen:    tokenGen,
		rateLimiter: rateLimiter,
		jwtSvc:      jwtSvc,
	}
}

// RequestMagicLink generates a magic link token and "sends" it via the email sender.
// It always returns nil (202 Accepted) to prevent email enumeration.
func (uc *AuthUseCase) RequestMagicLink(ctx context.Context, email string) error {
	emailAddr, err := ports.NewEmailAddress(email)
	if err != nil {
		return nil
	}

	if !uc.rateLimiter.Allow(emailAddr.String()) {
		return nil
	}

	token, err := uc.tokenGen.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	err = uc.magicStore.Store(ctx, token, ports.MagicLinkData{
		Email:     emailAddr.String(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("failed to store magic link: %w", err)
	}

	if err := uc.emailSender.SendMagicLink(ctx, emailAddr, token); err != nil {
		return fmt.Errorf("failed to send magic link: %w", err)
	}

	return nil
}

// ValidateToken validates a magic link token, creates or retrieves the user,
// creates a session, and returns a signed JWT access token.
func (uc *AuthUseCase) ValidateToken(ctx context.Context, token string) (*ports.SessionResult, error) {
	data, err := uc.magicStore.Retrieve(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	if time.Now().After(data.ExpiresAt) {
		_ = uc.magicStore.Delete(ctx, token)
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Single-use: delete token immediately
	if err := uc.magicStore.Delete(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to delete token: %w", err)
	}

	// Find or create user
	user, err := uc.userRepo.FindByEmail(ctx, data.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if user == nil {
		username := deriveUsername(data.Email)
		user, err = entities.NewUser(data.Email, username, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		if err := uc.userRepo.Save(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to save user: %w", err)
		}
	}

	// Create session in DB (for logout/revocation tracking)
	session, err := entities.NewSession(user.ID, nil, nil, nil, time.Now().Add(sessionTTL))
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	if err := uc.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Issue JWT access token (self-contained, no DB lookup for auth middleware)
	accessToken, err := uc.jwtSvc.Sign(user.ID, session.ID, sessionTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	return &ports.SessionResult{
		AccessToken: accessToken,
		SessionID:   session.ID,
		User:        user,
		ExpiresAt:   session.ExpiresAt,
	}, nil
}

// RefreshSession validates an existing session, extends it, and issues a new JWT.
func (uc *AuthUseCase) RefreshSession(ctx context.Context, sessionID string) (*ports.SessionResult, error) {
	session, err := uc.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("invalid or expired session")
	}

	// Extend session expiration
	session.ExpiresAt = time.Now().Add(sessionTTL)
	if err := uc.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to refresh session: %w", err)
	}

	user, err := uc.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found for session")
	}

	// Issue new JWT with extended expiry
	accessToken, err := uc.jwtSvc.Sign(user.ID, session.ID, sessionTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	return &ports.SessionResult{
		AccessToken: accessToken,
		SessionID:   session.ID,
		User:        user,
		ExpiresAt:   session.ExpiresAt,
	}, nil
}

// Logout deletes a session, effectively logging the user out.
func (uc *AuthUseCase) Logout(ctx context.Context, sessionID string) error {
	if err := uc.sessionRepo.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// deriveUsername creates a username from the local part of an email address.
func deriveUsername(email string) string {
	for i, c := range email {
		if c == '@' {
			name := email[:i]
			if len(name) >= 3 {
				return name
			}
			for len(name) < 3 {
				name += "x"
			}
			return name
		}
	}
	if len(email) >= 3 {
		return email
	}
	return email + "xxx"[:3-len(email)]
}

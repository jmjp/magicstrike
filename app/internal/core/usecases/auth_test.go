package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/adapters/out/email"
	"magicstrike/internal/adapters/out/memory"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
	"magicstrike/internal/core/usecases"
)

func setupAuthUseCase() ports.AuthUseCase {
	userRepo := memory.NewUserRepo()
	sessionRepo := memory.NewSessionRepo()
	magicStore := memory.NewMagicLinkStore()
	emailSender := email.NewMockSender()
	tokenGen := services.NewTokenGenerator()
	rateLimiter := services.NewRateLimiter()
	jwtSvc := services.NewJWTService("")

	return usecases.NewAuthUseCase(userRepo, sessionRepo, magicStore, emailSender, tokenGen, rateLimiter, jwtSvc)
}

func TestAuthUseCase_RequestMagicLink(t *testing.T) {
	uc := setupAuthUseCase()
	ctx := context.Background()

	t.Run("returns nil for valid email (always 202)", func(t *testing.T) {
		err := uc.RequestMagicLink(ctx, "user@example.com")
		require.NoError(t, err)
	})

	t.Run("returns nil for invalid email (don't leak info)", func(t *testing.T) {
		err := uc.RequestMagicLink(ctx, "not-an-email")
		require.NoError(t, err)
	})

	t.Run("returns nil for empty email (don't leak info)", func(t *testing.T) {
		err := uc.RequestMagicLink(ctx, "")
		require.NoError(t, err)
	})
}

func TestAuthUseCase_ValidateToken(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error for non-existent token", func(t *testing.T) {
		uc := setupAuthUseCase()
		_, err := uc.ValidateToken(ctx, "nonexistent-token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("returns error for expired token", func(t *testing.T) {
		magicStore := memory.NewMagicLinkStore()
		userRepo := memory.NewUserRepo()
		sessionRepo := memory.NewSessionRepo()
		emailSender := email.NewMockSender()
		tokenGen := services.NewTokenGenerator()
		rateLimiter := services.NewRateLimiter()
		jwtSvc := services.NewJWTService("")

		uc := usecases.NewAuthUseCase(userRepo, sessionRepo, magicStore, emailSender, tokenGen, rateLimiter, jwtSvc)

		token, err := tokenGen.GenerateToken()
		require.NoError(t, err)

		err = magicStore.Store(ctx, token, ports.MagicLinkData{
			Email:     "test@example.com",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		})
		require.NoError(t, err)

		_, err = uc.ValidateToken(ctx, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})
}

func TestAuthUseCase_FullFlow(t *testing.T) {
	ctx := context.Background()

	t.Run("full magic link flow with JWT: create user, session, refresh, logout", func(t *testing.T) {
		magicStore := memory.NewMagicLinkStore()
		userRepo := memory.NewUserRepo()
		sessionRepo := memory.NewSessionRepo()
		emailSender := email.NewMockSender()
		tokenGen := services.NewTokenGenerator()
		rateLimiter := services.NewRateLimiter()
		jwtSvc := services.NewJWTService("test-secret")

		uc := usecases.NewAuthUseCase(userRepo, sessionRepo, magicStore, emailSender, tokenGen, rateLimiter, jwtSvc)

		token, err := tokenGen.GenerateToken()
		require.NoError(t, err)

		// Store a valid magic link token
		err = magicStore.Store(ctx, token, ports.MagicLinkData{
			Email:     "player@example.com",
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})
		require.NoError(t, err)

		// Step 1: Validate token → creates user + session + JWT access token
		result, err := uc.ValidateToken(ctx, token)
		require.NoError(t, err)
		assert.NotEmpty(t, result.AccessToken, "should return a JWT access token")
		assert.NotEmpty(t, result.SessionID)
		assert.Equal(t, "player@example.com", result.User.Email)
		assert.True(t, result.ExpiresAt.After(time.Now()))

		// Verify the JWT is valid
		claims, err := jwtSvc.Verify(result.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, result.User.ID, claims.UserID)
		assert.Equal(t, result.SessionID, claims.ID)

		// Step 2: Token should now be single-use (deleted)
		_, err = uc.ValidateToken(ctx, token)
		require.Error(t, err, "token should be single-use and now deleted")

		// Step 3: Refresh session → new JWT issued with extended expiry
		refreshed, err := uc.RefreshSession(ctx, result.SessionID)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshed.AccessToken)
		assert.True(t, refreshed.ExpiresAt.After(result.ExpiresAt), "refreshed expiry should be later than original")

		// Verify refreshed JWT
		claims2, err := jwtSvc.Verify(refreshed.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, refreshed.User.ID, claims2.UserID)

		// Step 4: Logout
		err = uc.Logout(ctx, result.SessionID)
		require.NoError(t, err)

		// Session should be gone from DB
		_, err = uc.RefreshSession(ctx, result.SessionID)
		require.Error(t, err)
	})
}

func TestAuthUseCase_RefreshSession_Invalid(t *testing.T) {
	uc := setupAuthUseCase()
	ctx := context.Background()

	t.Run("non-existent session ID returns error", func(t *testing.T) {
		_, err := uc.RefreshSession(ctx, "nonexistent-session")
		require.Error(t, err)
	})
}

func TestAuthUseCase_Logout(t *testing.T) {
	uc := setupAuthUseCase()
	ctx := context.Background()

	t.Run("logout non-existent session is no-op", func(t *testing.T) {
		err := uc.Logout(ctx, "nonexistent-session")
		require.NoError(t, err)
	})
}

func TestAuthUseCase_JWTClaims(t *testing.T) {
	jwtSvc := services.NewJWTService("secret")
	ctx := context.Background()

	t.Run("JWT contains user_id in claims", func(t *testing.T) {
		magicStore := memory.NewMagicLinkStore()
		userRepo := memory.NewUserRepo()
		sessionRepo := memory.NewSessionRepo()
		emailSender := email.NewMockSender()
		tokenGen := services.NewTokenGenerator()
		rateLimiter := services.NewRateLimiter()

		uc := usecases.NewAuthUseCase(userRepo, sessionRepo, magicStore, emailSender, tokenGen, rateLimiter, jwtSvc)

		tok, _ := tokenGen.GenerateToken()
		_ = magicStore.Store(ctx, tok, ports.MagicLinkData{
			Email:     "jwt-test@example.com",
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})

		result, err := uc.ValidateToken(ctx, tok)
		require.NoError(t, err)

		// Verify JWT structure
		claims, err := jwtSvc.Verify(result.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, "magicstrike", claims.Issuer)
		assert.Equal(t, result.User.ID, claims.Subject)
		assert.Equal(t, result.User.ID, claims.UserID)
		assert.Equal(t, result.SessionID, claims.ID)
		assert.NotNil(t, claims.IssuedAt)
		assert.NotNil(t, claims.ExpiresAt)
	})
}

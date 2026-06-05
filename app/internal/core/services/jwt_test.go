package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/core/services"
)

func TestJWTService_SignAndVerify(t *testing.T) {
	svc := services.NewJWTService("my-secret-key")

	t.Run("sign and verify valid token", func(t *testing.T) {
		token, err := svc.Sign("user-123", "session-abc", 1*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := svc.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "user-123", claims.Subject)
		assert.Equal(t, "session-abc", claims.ID)
		assert.Equal(t, "magicstrike", claims.Issuer)
	})

	t.Run("verify fails with wrong secret", func(t *testing.T) {
		svc2 := services.NewJWTService("different-secret")
		token, err := svc.Sign("user-123", "session-abc", 1*time.Hour)
		require.NoError(t, err)

		_, err = svc2.Verify(token)
		require.Error(t, err)
	})

	t.Run("verify fails with tampered token", func(t *testing.T) {
		token, err := svc.Sign("user-123", "session-abc", 1*time.Hour)
		require.NoError(t, err)

		// Tamper with the token
		tampered := token + "x"
		_, err = svc.Verify(tampered)
		require.Error(t, err)
	})

	t.Run("verify fails with expired token", func(t *testing.T) {
		token, err := svc.Sign("user-123", "session-abc", 1*time.Millisecond)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond) // let it expire
		_, err = svc.Verify(token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWT")
	})

	t.Run("sign uses default TTL when zero", func(t *testing.T) {
		token, err := svc.Sign("user-456", "session-def", 0)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Should still verify (uses 7-day default)
		claims, err := svc.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, "user-456", claims.UserID)
	})

	t.Run("different tokens for different sessions", func(t *testing.T) {
		t1, _ := svc.Sign("user-1", "session-1", 1*time.Hour)
		t2, _ := svc.Sign("user-1", "session-2", 1*time.Hour)
		assert.NotEqual(t, t1, t2)
	})

	t.Run("verify fails with empty token", func(t *testing.T) {
		_, err := svc.Verify("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWT")
	})

	t.Run("verify fails with malformed token", func(t *testing.T) {
		_, err := svc.Verify("not.a.jwt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWT")
	})

	t.Run("verify fails with wrong signing method header", func(t *testing.T) {
		// Create a token with a valid structure but manipulated header
		// ECDSA header instead of HMAC
		_, err := svc.Verify("eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEyMyIsImp0aSI6InNlc3Npb24tYWJjIn0.invalidsignature")
		require.Error(t, err)
	})

	t.Run("default secret when empty", func(t *testing.T) {
		svcDefault := services.NewJWTService("")
		token, err := svcDefault.Sign("user-789", "session-ghi", 1*time.Hour)
		require.NoError(t, err)

		claims, err := svcDefault.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, "user-789", claims.UserID)
	})
}

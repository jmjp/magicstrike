package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/core/services"
)

func TestTokenGenerator_GenerateToken(t *testing.T) {
	gen := services.NewTokenGenerator()

	t.Run("generates 64-character hex token", func(t *testing.T) {
		token, err := gen.GenerateToken()
		require.NoError(t, err)
		assert.Len(t, token, 64)
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := gen.GenerateToken()
			require.NoError(t, err)
			assert.False(t, tokens[token], "duplicate token generated")
			tokens[token] = true
		}
	})

	t.Run("token contains only hex characters", func(t *testing.T) {
		token, err := gen.GenerateToken()
		require.NoError(t, err)
		for _, c := range token {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "non-hex character: %c", c)
		}
	})
}

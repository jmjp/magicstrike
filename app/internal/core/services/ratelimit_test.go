package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"magicstrike/internal/core/services"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := services.NewRateLimiter()
	key := "test@example.com"

	t.Run("allows up to 5 requests", func(t *testing.T) {
		rl.Reset()
		for i := 0; i < 5; i++ {
			assert.True(t, rl.Allow(key), "request %d should be allowed", i+1)
		}
	})

	t.Run("blocks after 5 requests", func(t *testing.T) {
		rl.Reset()
		for i := 0; i < 5; i++ {
			rl.Allow(key)
		}
		assert.False(t, rl.Allow(key), "6th request should be blocked")
	})

	t.Run("different keys are independent", func(t *testing.T) {
		rl.Reset()
		for i := 0; i < 5; i++ {
			rl.Allow("a@example.com")
		}
		assert.True(t, rl.Allow("b@example.com"), "different key should not be rate-limited")
	})
}

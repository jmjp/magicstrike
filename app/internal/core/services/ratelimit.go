package services

import (
	"sync"
	"time"
)

const (
	defaultMaxRequests = 5
	defaultWindow      = 1 * time.Minute
)

// RateLimiter is a domain service that provides in-memory sliding-window rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
	}
}

// Allow reports whether a request identified by key is allowed under the rate limit.
// Returns true if the request is allowed, false if the rate limit has been exceeded.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-defaultWindow)

	// Filter out timestamps outside the current window
	var recent []time.Time
	for _, t := range r.attempts[key] {
		if t.After(windowStart) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= defaultMaxRequests {
		r.attempts[key] = recent
		return false
	}

	recent = append(recent, now)
	r.attempts[key] = recent
	return true
}

// Reset clears all rate limit state. Useful for testing.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = make(map[string][]time.Time)
}

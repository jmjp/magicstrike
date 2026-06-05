package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"magicstrike/internal/core/services"
)

func TestGetUserIDAndSessionID(t *testing.T) {
	ctx := context.Background()
	if id := GetUserID(ctx); id != "" {
		t.Errorf("expected empty user ID, got %q", id)
	}
	if id := GetSessionID(ctx); id != "" {
		t.Errorf("expected empty session ID, got %q", id)
	}

	ctx = context.WithValue(ctx, userIDKey, "user-1")
	ctx = context.WithValue(ctx, sessionIDKey, "sess-1")

	if id := GetUserID(ctx); id != "user-1" {
		t.Errorf("expected user-1, got %q", id)
	}
	if id := GetSessionID(ctx); id != "sess-1" {
		t.Errorf("expected sess-1, got %q", id)
	}
}

func TestTokenBlocklist(t *testing.T) {
	bl := &TokenBlocklist{
		revoked: make(map[string]time.Time),
	}

	if bl.IsRevoked("sess-1") {
		t.Error("expected sess-1 not to be revoked")
	}

	now := time.Now()
	bl.Revoke("sess-1", now.Add(1*time.Second))

	if !bl.IsRevoked("sess-1") {
		t.Error("expected sess-1 to be revoked")
	}

	// Manual test of cleanupLoop functionality without waiting a full minute:
	// We insert one expired and one non-expired token, then perform a quick synchronous cleanup step.
	bl.mu.Lock()
	bl.revoked["expired-sess"] = now.Add(-1 * time.Second)
	bl.revoked["valid-sess"] = now.Add(10 * time.Second)
	bl.mu.Unlock()

	// Mimic cleanup logic:
	bl.mu.Lock()
	for id, exp := range bl.revoked {
		if time.Now().After(exp) {
			delete(bl.revoked, id)
		}
	}
	bl.mu.Unlock()

	if bl.IsRevoked("expired-sess") {
		t.Error("expected expired-sess to be deleted by cleanup logic")
	}
	if !bl.IsRevoked("valid-sess") {
		t.Error("expected valid-sess to still exist")
	}
}

func TestAuthMiddleware(t *testing.T) {
	jwtSvc := services.NewJWTService("my-test-secret-key-1234567890")
	blocklist := &TokenBlocklist{
		revoked: make(map[string]time.Time),
	}

	middleware := AuthMiddleware(jwtSvc, blocklist)
	nextCalled := false
	var ctxUser, ctxSess string

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		ctxUser = GetUserID(r.Context())
		ctxSess = GetSessionID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("missing auth header", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
		if nextCalled {
			t.Error("next handler should not be called")
		}
	})

	t.Run("invalid auth header format", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token signature", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("revoked token", func(t *testing.T) {
		token, err := jwtSvc.Sign("user-123", "session-revoked", 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		blocklist.Revoke("session-revoked", time.Now().Add(1*time.Hour))

		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("successful auth", func(t *testing.T) {
		token, err := jwtSvc.Sign("user-123", "session-ok", 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !nextCalled {
			t.Error("expected next handler to be called")
		}
		if ctxUser != "user-123" || ctxSess != "session-ok" {
			t.Errorf("expected user-123 and session-ok in context, got user=%q sess=%q", ctxUser, ctxSess)
		}
	})
}

func TestTokenBlocklist_CleanupLoop(t *testing.T) {
	bl := &TokenBlocklist{
		revoked:         make(map[string]time.Time),
		cleanupInterval: 1 * time.Millisecond,
		stopChan:        make(chan struct{}),
	}
	defer bl.Close()

	now := time.Now()
	bl.Revoke("expired-sess", now.Add(-1*time.Second))
	bl.Revoke("valid-sess", now.Add(5*time.Second))

	go bl.cleanupLoop()

	// Wait briefly for the 1ms ticker to fire and clean up the expired session
	time.Sleep(10 * time.Millisecond)

	if bl.IsRevoked("expired-sess") {
		t.Error("expected expired-sess to be cleaned up by the background loop")
	}
	if !bl.IsRevoked("valid-sess") {
		t.Error("expected valid-sess to still be present in the blocklist")
	}
}

func TestCorsMiddleware(t *testing.T) {
	allowed := []string{"http://localhost:5173", "https://magicstrike.com"}
	middleware := CorsMiddleware(allowed)

	nextCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("OPTIONS preflight request allowed origin", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/magic-link", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 NoContent, got %d", rec.Code)
		}
		if nextCalled {
			t.Error("next handler should not be called for OPTIONS preflight")
		}
		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
			t.Errorf("expected Access-Control-Allow-Origin to be %q, got %q", "http://localhost:5173", origin)
		}
	})

	t.Run("POST request allowed origin", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
		if !nextCalled {
			t.Error("expected next handler to be called")
		}
		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:5173" {
			t.Errorf("expected Access-Control-Allow-Origin to be %q, got %q", "http://localhost:5173", origin)
		}
	})

	t.Run("POST request disallowed origin", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", nil)
		req.Header.Set("Origin", "http://malicious.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
		if !nextCalled {
			t.Error("expected next handler to be called")
		}
		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "" {
			t.Errorf("expected Access-Control-Allow-Origin to be empty for disallowed origin, got %q", origin)
		}
	})
}


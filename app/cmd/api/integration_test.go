package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"magicstrike/internal/core/services"
)

// setupTestEnv configures the environment for in-memory-only testing.
func setupTestEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("POSTGRES_HOST") == "" {
		os.Unsetenv("POSTGRES_HOST")
	}
	if os.Getenv("CLICKHOUSE_ADDR") == "" {
		os.Unsetenv("CLICKHOUSE_ADDR")
	}
	if os.Getenv("MINIO_ENDPOINT") == "" {
		os.Unsetenv("MINIO_ENDPOINT")
	}
	if os.Getenv("RABBITMQ_URL") == "" {
		os.Unsetenv("RABBITMQ_URL")
	}
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("VOYAGE_API_KEY")
	os.Setenv("JWT_SECRET", "test-secret-for-integration-min-32bytes!")
}

// setupTestEnvNoClickHouse forces ClickHouse to be unreachable.
func setupTestEnvNoClickHouse(t *testing.T) {
	t.Helper()
	setupTestEnv(t)
	os.Setenv("CLICKHOUSE_ADDR", "255.255.255.255:1")
}

func cleanupTestEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("CLICKHOUSE_ADDR")
}

func TestBuildMux_PublicRoutes(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	mux, cleanup := buildMux()
	if cleanup != nil {
		defer cleanup()
	}

	t.Run("POST /api/v1/auth/magic-link with valid email", func(t *testing.T) {
		body := map[string]string{"email": "test@example.com"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// Magic link returns 202 Accepted
		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/auth/magic-link returns 400 with empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/auth/callback returns 400 with empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/callback", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}

func TestBuildMux_ProtectedRoutes(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	mux, cleanup := buildMux()
	if cleanup != nil {
		defer cleanup()
	}

	jwtSvc := services.NewJWTService("test-secret-for-integration-min-32bytes!")

	t.Run("protected routes require auth", func(t *testing.T) {
		routes := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/v1/matches"},
			{http.MethodGet, "/api/v1/matches/some-id"},
			{http.MethodPost, "/api/v1/auth/refresh"},
			{http.MethodDelete, "/api/v1/auth/session"},
		}

		for _, route := range routes {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: expected 401, got %d", route.method, route.path, rec.Code)
			}
		}
	})

	t.Run("GET /api/v1/matches with valid JWT returns 200", func(t *testing.T) {
		token, err := jwtSvc.Sign("user-123", "session-test", time.Hour)
		if err != nil {
			t.Fatalf("failed to sign JWT: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET /api/v1/matches/{id} with valid JWT returns 404", func(t *testing.T) {
		token, err := jwtSvc.Sign("user-123", "session-test-2", time.Hour)
		if err != nil {
			t.Fatalf("failed to sign JWT: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestBuildMux_DisabledEndpoints(t *testing.T) {
	setupTestEnvNoClickHouse(t)
	defer cleanupTestEnv(t)

	mux, cleanup := buildMux()
	if cleanup != nil {
		defer cleanup()
	}

	jwtSvc := services.NewJWTService("test-secret-for-integration-min-32bytes!")
	token, err := jwtSvc.Sign("user-123", "session-disabled", time.Hour)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	t.Run("POST /api/v1/chat returns 503 without ClickHouse", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBufferString("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/demos/upload-request returns 501 without Minio/RabbitMQ", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewBufferString("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Errorf("expected 501, got %d", rec.Code)
		}
	})
}

func TestBuildMux_RouterMethodNotAllowed(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	mux, cleanup := buildMux()
	if cleanup != nil {
		defer cleanup()
	}

	t.Run("GET on POST-only route returns 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/magic-link", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})
}

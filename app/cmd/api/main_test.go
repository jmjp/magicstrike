package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEnvOr(t *testing.T) {
	os.Setenv("TEST_ENV_OR_KEY", "value")
	defer os.Unsetenv("TEST_ENV_OR_KEY")

	if val := envOr("TEST_ENV_OR_KEY", "fallback"); val != "value" {
		t.Errorf("expected 'value', got %q", val)
	}

	if val := envOr("NON_EXISTENT_KEY", "fallback"); val != "fallback" {
		t.Errorf("expected 'fallback', got %q", val)
	}
}

func TestEnvOrInt(t *testing.T) {
	os.Setenv("TEST_ENV_INT_KEY", "42")
	defer os.Unsetenv("TEST_ENV_INT_KEY")

	if val := envOrInt("TEST_ENV_INT_KEY", 100); val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	if val := envOrInt("NON_EXISTENT_KEY", 100); val != 100 {
		t.Errorf("expected 100, got %d", val)
	}

	os.Setenv("TEST_ENV_INT_INVALID", "not-an-int")
	defer os.Unsetenv("TEST_ENV_INT_INVALID")

	if val := envOrInt("TEST_ENV_INT_INVALID", 100); val != 100 {
		t.Errorf("expected 100 for invalid int, got %d", val)
	}
}

func TestLoggerMiddleware(t *testing.T) {
	// Redirect logs to a buffer
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := loggerMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "→ GET /test-path") {
		t.Errorf("log output does not contain request start: %q", logOutput)
	}
	if !strings.Contains(logOutput, "← GET /test-path") {
		t.Errorf("log output does not contain request end: %q", logOutput)
	}
}

func TestRecovererMiddleware(t *testing.T) {
	// Redirect logs to prevent spamming
	log.SetOutput(os.NewFile(0, os.DevNull))
	defer log.SetOutput(os.Stderr)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went terribly wrong")
	})

	handler := recovererMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/panic-path", nil)
	rec := httptest.NewRecorder()

	// Should recover instead of crashing
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Errorf("expected error message in body, got %q", body)
	}
}

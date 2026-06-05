package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

type stubAuthUseCase struct {
	requestMagicLinkFn func(ctx context.Context, email string) error
	validateTokenFn    func(ctx context.Context, token string) (*ports.SessionResult, error)
	refreshSessionFn   func(ctx context.Context, sessionID string) (*ports.SessionResult, error)
	logoutFn           func(ctx context.Context, sessionID string) error
}

func (s *stubAuthUseCase) RequestMagicLink(ctx context.Context, email string) error {
	if s.requestMagicLinkFn != nil {
		return s.requestMagicLinkFn(ctx, email)
	}
	return nil
}

func (s *stubAuthUseCase) ValidateToken(ctx context.Context, token string) (*ports.SessionResult, error) {
	if s.validateTokenFn != nil {
		return s.validateTokenFn(ctx, token)
	}
	return &ports.SessionResult{
		AccessToken: "access-token-123",
		SessionID:   "session-123",
		User:        &entities.User{ID: "user-123", Email: "test@example.com"},
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}, nil
}

func (s *stubAuthUseCase) RefreshSession(ctx context.Context, sessionID string) (*ports.SessionResult, error) {
	if s.refreshSessionFn != nil {
		return s.refreshSessionFn(ctx, sessionID)
	}
	return &ports.SessionResult{
		AccessToken: "new-access-token-123",
		SessionID:   sessionID,
		User:        &entities.User{ID: "user-123", Email: "test@example.com"},
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}, nil
}

func (s *stubAuthUseCase) Logout(ctx context.Context, sessionID string) error {
	if s.logoutFn != nil {
		return s.logoutFn(ctx, sessionID)
	}
	return nil
}

func TestHandleRequestMagicLink(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewBufferString("{invalid"))
		rec := httptest.NewRecorder()

		h.HandleRequestMagicLink(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty email", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(requestMagicLinkRequest{Email: ""})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleRequestMagicLink(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("usecase error is non-fatal", func(t *testing.T) {
		uc := &stubAuthUseCase{
			requestMagicLinkFn: func(ctx context.Context, email string) error {
				return errors.New("something went wrong")
			},
		}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(requestMagicLinkRequest{Email: "test@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleRequestMagicLink(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(requestMagicLinkRequest{Email: "test@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleRequestMagicLink(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
	})
}

func TestHandleCallback(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/callback", bytes.NewBufferString("{invalid"))
		rec := httptest.NewRecorder()

		h.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(callbackRequest{Token: ""})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/callback", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleCallback(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("usecase error (unauthorized)", func(t *testing.T) {
		uc := &stubAuthUseCase{
			validateTokenFn: func(ctx context.Context, token string) (*ports.SessionResult, error) {
				return nil, errors.New("invalid token")
			},
		}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(callbackRequest{Token: "bad-token"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/callback", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleCallback(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		reqBody, _ := json.Marshal(callbackRequest{Token: "good-token"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/callback", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()

		h.HandleCallback(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["success"] != true {
			t.Errorf("expected success: true, got %v", resp["success"])
		}
	})
}

func TestHandleRefresh(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		rec := httptest.NewRecorder()

		h.HandleRefresh(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("usecase error", func(t *testing.T) {
		uc := &stubAuthUseCase{
			refreshSessionFn: func(ctx context.Context, sessionID string) (*ports.SessionResult, error) {
				return nil, errors.New("refresh failed")
			},
		}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		ctx := context.WithValue(req.Context(), sessionIDKey, "session-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRefresh(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		ctx := context.WithValue(req.Context(), sessionIDKey, "session-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRefresh(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestHandleLogout(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/session", nil)
		rec := httptest.NewRecorder()

		h.HandleLogout(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("usecase error is non-fatal", func(t *testing.T) {
		uc := &stubAuthUseCase{
			logoutFn: func(ctx context.Context, sessionID string) error {
				return errors.New("logout failed")
			},
		}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/session", nil)
		ctx := context.WithValue(req.Context(), sessionIDKey, "session-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleLogout(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubAuthUseCase{}
		bl := NewTokenBlocklist()
		h := NewAuthHandler(uc, bl)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/session", nil)
		ctx := context.WithValue(req.Context(), sessionIDKey, "session-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleLogout(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rec.Code)
		}

		if !bl.IsRevoked("session-123") {
			t.Errorf("expected session-123 to be revoked in blocklist")
		}
	})
}

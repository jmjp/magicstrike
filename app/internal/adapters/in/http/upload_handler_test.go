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

	"magicstrike/internal/core/ports"
)

type stubUploadUseCase struct {
	requestUploadFn func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error)
	confirmUploadFn func(ctx context.Context, req *ports.ConfirmUploadRequest) error
}

func (s *stubUploadUseCase) RequestUpload(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
	if s.requestUploadFn != nil {
		return s.requestUploadFn(ctx, req)
	}
	return &ports.UploadResponse{
		UploadURL: "http://minio/upload-url",
		BucketKey: "demos/key.dem",
		ExpiresAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		MatchID:   "match-123",
	}, nil
}

func (s *stubUploadUseCase) ConfirmUpload(ctx context.Context, req *ports.ConfirmUploadRequest) error {
	if s.confirmUploadFn != nil {
		return s.confirmUploadFn(ctx, req)
	}
	return nil
}

func TestUploadHandler_HandleRequestUpload(t *testing.T) {
	t.Run("missing user id in context", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewBufferString("{invalid"))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("usecase validation error", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("validation failed for filename")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "invalid.txt"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("usecase user not found", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("user not found in db")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("usecase match not found", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("match not found")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("usecase duplicate conflict", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("already uses this MD5 hash")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("usecase storage unavailable", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("storage service is unavailable")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("usecase default internal error", func(t *testing.T) {
		uc := &stubUploadUseCase{
			requestUploadFn: func(ctx context.Context, req *ports.UploadRequest) (*ports.UploadResponse, error) {
				return nil, errors.New("some DB error")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(uploadRequestBody{
			MatchID:  "match-123",
			Filename: "demo.dem",
			TeamA:    "NaVi",
			TeamB:    "G2",
			MapName:  "de_dust2",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleRequestUpload(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		var resp uploadResponseBody
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.UploadURL != "http://minio/upload-url" {
			t.Errorf("expected upload URL, got %q", resp.UploadURL)
		}
		if resp.ExpiresAt != "2026-06-04T12:00:00Z" {
			t.Errorf("expected formatted expiry, got %q", resp.ExpiresAt)
		}
	})
}

func TestUploadHandler_HandleConfirmUpload(t *testing.T) {
	t.Run("missing user id in context", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-confirm", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		h.HandleConfirmUpload(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-confirm", bytes.NewBufferString("{invalid"))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleConfirmUpload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("usecase confirm error", func(t *testing.T) {
		uc := &stubUploadUseCase{
			confirmUploadFn: func(ctx context.Context, req *ports.ConfirmUploadRequest) error {
				return errors.New("object not found in storage")
			},
		}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(confirmUploadBody{MatchID: "match-123", BucketKey: "key.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-confirm", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleConfirmUpload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		uc := &stubUploadUseCase{}
		h := NewUploadHandler(uc)

		reqBody, _ := json.Marshal(confirmUploadBody{MatchID: "match-123", BucketKey: "key.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-confirm", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleConfirmUpload(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rec.Code)
		}
	})
}

func TestUploadHandler_ServeHTTP(t *testing.T) {
	uc := &stubUploadUseCase{}
	h := NewUploadHandler(uc)

	t.Run("upload-request method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/demos/upload-request", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("upload-request POST success", func(t *testing.T) {
		reqBody, _ := json.Marshal(uploadRequestBody{Filename: "demo.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-request", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("upload-confirm method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/demos/upload-confirm", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("upload-confirm POST success", func(t *testing.T) {
		reqBody, _ := json.Marshal(confirmUploadBody{MatchID: "match-123", BucketKey: "key.dem"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/upload-confirm", bytes.NewReader(reqBody))
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rec.Code)
		}
	})

	t.Run("not found route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/demos/something-else", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})
}

func TestMapUploadError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, http.StatusOK},
		{"validation", errors.New("validation failed"), http.StatusBadRequest},
		{"invalid upload", errors.New("invalid upload request"), http.StatusBadRequest},
		{"user not found", errors.New("user not found"), http.StatusUnauthorized},
		{"match not found", errors.New("match not found"), http.StatusNotFound},
		{"object not found", errors.New("object not found"), http.StatusNotFound},
		{"duplicate", errors.New("duplicate hash"), http.StatusConflict},
		{"already uses MD5", errors.New("already uses this MD5"), http.StatusConflict},
		{"storage unavailable", errors.New("storage service is unavailable"), http.StatusServiceUnavailable},
		{"queue unavailable", errors.New("message queue is unavailable"), http.StatusServiceUnavailable},
		{"unknown", errors.New("random error"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		code := mapUploadError(tt.err)
		if code != tt.expected {
			t.Errorf("mapUploadError(%s) = %d, want %d", tt.name, code, tt.expected)
		}
	}
}

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
)

func TestHandleContinueChat_EdgeCases(t *testing.T) {
	t.Run("empty session ID", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/", bytes.NewReader([]byte(`{"question":"ok"}`)))
		req.SetPathValue("id", "")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sess-1", bytes.NewReader([]byte(`{invalid`)))
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("question too long", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		longQuestion := strings.Repeat("a", 501)
		reqBody, _ := json.Marshal(map[string]string{"question": longQuestion})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sess-1", bytes.NewReader(reqBody))
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("usecase general internal error", func(t *testing.T) {
		uc := &stubChatUseCase{
			continueSessionFn: func(ctx context.Context, userID, sessionID, question string) (*ports.ChatResponse, error) {
				return nil, errors.New("database connection lost")
			},
		}
		handler := NewChatHandler(uc, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sess-1", bytes.NewReader([]byte(`{"question":"hello"}`)))
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestHandleListSessions_EdgeCases(t *testing.T) {
	t.Run("invalid limit parameter", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=abc", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("limit too low", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=0", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("limit too high", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=51", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid offset parameter", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?offset=abc", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("offset negative", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?offset=-1", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("usecase list error", func(t *testing.T) {
		suc := &stubChatSessionUseCase{
			listFn: func(ctx context.Context, userID string, limit, offset int) (*ports.SessionListResult, error) {
				return nil, errors.New("failed to load database")
			},
		}
		handler := NewChatHandler(nil, suc, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("session summary with no messages", func(t *testing.T) {
		now := time.Now()
		suc := &stubChatSessionUseCase{
			listFn: func(ctx context.Context, userID string, limit, offset int) (*ports.SessionListResult, error) {
				return &ports.SessionListResult{
					Sessions: []*entities.ChatSession{
						{
							ID:        "sess-empty",
							UserID:    userID,
							Messages:  nil,
							CreatedAt: now,
							UpdatedAt: now,
						},
					},
					Total:  1,
					Limit:  limit,
					Offset: offset,
				}, nil
			},
		}
		handler := NewChatHandler(nil, suc, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat", nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var payload map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &payload)
		data, _ := payload["data"].(map[string]any)
		sessions, _ := data["sessions"].([]any)
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session summary, got %d", len(sessions))
		}
		summary, _ := sessions[0].(map[string]any)
		if summary["last_question"] != "" {
			t.Errorf("expected last_question to be empty, got %q", summary["last_question"])
		}
	})
}

func TestHandleGetSession_EdgeCases(t *testing.T) {
	t.Run("empty session ID", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/", nil)
		req.SetPathValue("id", "")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("usecase get error", func(t *testing.T) {
		suc := &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, errors.New("db error")
			},
		}
		handler := NewChatHandler(nil, suc, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sess-1", nil)
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("session messages greater than 10", func(t *testing.T) {
		now := time.Now()
		var messages []entities.Message
		for i := 0; i < 15; i++ {
			messages = append(messages, entities.Message{
				Question:  "q",
				Answer:    "a",
				Source:    "mock",
				CreatedAt: now,
			})
		}
		suc := &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return &entities.ChatSession{
					ID:        "sess-1",
					UserID:    userID,
					Messages:  messages,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
		}
		handler := NewChatHandler(nil, suc, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sess-1", nil)
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var payload map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &payload)
		data, _ := payload["data"].(map[string]any)
		respMsgs, _ := data["messages"].([]any)
		if len(respMsgs) != 10 {
			t.Errorf("expected messages count to be truncated to 10, got %d", len(respMsgs))
		}
	})
}

func TestHandleDeleteSession_EdgeCases(t *testing.T) {
	t.Run("empty session ID", func(t *testing.T) {
		handler := NewChatHandler(nil, nil, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/", nil)
		req.SetPathValue("id", "")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("usecase delete error", func(t *testing.T) {
		suc := &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return &entities.ChatSession{ID: "sess-1", UserID: userID}, nil
			},
			deleteFn: func(ctx context.Context, userID, id string) error {
				return errors.New("cannot delete from database")
			},
		}
		handler := NewChatHandler(nil, suc, services.NewRateLimiter())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/sess-1", nil)
		req.SetPathValue("id", "sess-1")
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

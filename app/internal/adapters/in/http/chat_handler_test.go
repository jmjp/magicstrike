package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
)

// Sentinel errors used by stubs to simulate specific failure modes.
var (
	assertAnError     = errors.New("test error")
	errSessionNotFound = errors.New("session not found")
	errSessionExpired  = errors.New("session has expired")
)

// --- Stubs ---

type stubChatUseCase struct {
	newSessionFn            func(ctx context.Context, userID string, matchIDs []string, question string) (*ports.ChatResponse, error)
	continueSessionFn       func(ctx context.Context, userID string, sessionID string, question string) (*ports.ChatResponse, error)
	newSessionStreamFn      func(ctx context.Context, userID string, matchIDs []string, question string) (*ports.StreamResponse, error)
	continueSessionStreamFn func(ctx context.Context, userID string, sessionID string, question string) (*ports.StreamResponse, error)
}

func (s *stubChatUseCase) NewSession(ctx context.Context, userID string, matchIDs []string, question string) (*ports.ChatResponse, error) {
	if s.newSessionFn != nil {
		return s.newSessionFn(ctx, userID, matchIDs, question)
	}
	return &ports.ChatResponse{SessionID: "session-1", Answer: "test answer", Source: "clickhouse", MatchesUsed: matchIDs}, nil
}

func (s *stubChatUseCase) ContinueSession(ctx context.Context, userID string, sessionID string, question string) (*ports.ChatResponse, error) {
	if s.continueSessionFn != nil {
		return s.continueSessionFn(ctx, userID, sessionID, question)
	}
	return &ports.ChatResponse{SessionID: sessionID, Answer: "follow-up answer", Source: "clickhouse", MatchesUsed: []string{"match-1"}}, nil
}

func (s *stubChatUseCase) NewSessionStream(ctx context.Context, userID string, matchIDs []string, question string) (*ports.StreamResponse, error) {
	if s.newSessionStreamFn != nil {
		return s.newSessionStreamFn(ctx, userID, matchIDs, question)
	}
	outChan := make(chan string, 1)
	outChan <- "mock stream content"
	close(outChan)
	errChan := make(chan error, 1)
	close(errChan)
	return &ports.StreamResponse{
		SessionID:   "session-1",
		Source:      "clickhouse",
		MatchesUsed: matchIDs,
		Stream:      outChan,
		ErrChan:     errChan,
	}, nil
}

func (s *stubChatUseCase) ContinueSessionStream(ctx context.Context, userID string, sessionID string, question string) (*ports.StreamResponse, error) {
	if s.continueSessionStreamFn != nil {
		return s.continueSessionStreamFn(ctx, userID, sessionID, question)
	}
	outChan := make(chan string, 1)
	outChan <- "mock continue stream content"
	close(outChan)
	errChan := make(chan error, 1)
	close(errChan)
	return &ports.StreamResponse{
		SessionID:   sessionID,
		Source:      "clickhouse",
		MatchesUsed: []string{"match-1"},
		Stream:      outChan,
		ErrChan:     errChan,
	}, nil
}

type stubChatSessionUseCase struct {
	listFn   func(ctx context.Context, userID string, limit, offset int) (*ports.SessionListResult, error)
	getFn    func(ctx context.Context, userID, id string) (*entities.ChatSession, error)
	deleteFn func(ctx context.Context, userID, id string) error
}

func (s *stubChatSessionUseCase) List(ctx context.Context, userID string, limit, offset int) (*ports.SessionListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, userID, limit, offset)
	}
	return &ports.SessionListResult{Sessions: nil, Total: 0, Limit: limit, Offset: offset}, nil
}

func (s *stubChatSessionUseCase) Get(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
	if s.getFn != nil {
		return s.getFn(ctx, userID, id)
	}
	return nil, nil
}

func (s *stubChatSessionUseCase) Delete(ctx context.Context, userID, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, userID, id)
	}
	return nil
}

// --- Helpers ---

func contextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func newChatHandler(chatUseCase ports.ChatUseCase, sessionUseCase ports.ChatSessionUseCase) *ChatHandler {
	if chatUseCase == nil {
		chatUseCase = &stubChatUseCase{}
	}
	if sessionUseCase == nil {
		sessionUseCase = &stubChatSessionUseCase{}
	}
	return NewChatHandler(chatUseCase, sessionUseCase, services.NewRateLimiter())
}

// --- Tests ---

func TestHandleNewChat(t *testing.T) {
	t.Run("success 201", func(t *testing.T) {
		handler := newChatHandler(&stubChatUseCase{
			newSessionFn: func(ctx context.Context, userID string, matchIDs []string, question string) (*ports.ChatResponse, error) {
				return &ports.ChatResponse{
					SessionID: "session-abc", Answer: "Team analysis complete.",
					Source: "clickhouse", MatchesUsed: matchIDs,
					DataPoints: []ports.ChatDataPoint{{Label: "Team A", Value: "16"}},
				}, nil
			},
		}, nil)

		body := `{"match_ids":["match-1"],"question":"who won?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected 201, got %d", resp.StatusCode)
		}
		var payload map[string]any
		json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()

		data, _ := payload["data"].(map[string]any)
		if data == nil || data["session_id"] != "session-abc" {
			t.Errorf("expected session_id=session-abc, got %v", data)
		}
		if data["answer"] != "Team analysis complete." {
			t.Errorf("unexpected answer: %v", data["answer"])
		}
	})

	t.Run("match_ids empty 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		body := `{"match_ids":[],"question":"who won?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("match_ids missing 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		body := `{"question":"who won?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("question empty 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		body := `{"match_ids":["match-1"],"question":"   "}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("question too long 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		longQ := strings.Repeat("a", 501)
		body := `{"match_ids":["match-1"],"question":"` + longQ + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("no auth 401", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		body := `{"match_ids":["match-1"],"question":"who won?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		// No user ID in context
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("invalid JSON body 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		body := `not json`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("too many match IDs 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)
		ids := make([]string, 21)
		for i := range ids {
			ids[i] = fmt.Sprintf("m-%d", i)
		}
		body := `{"match_ids":["` + strings.Join(ids, `","`) + `"],"question":"who?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("use case error 500", func(t *testing.T) {
		handler := newChatHandler(&stubChatUseCase{
			newSessionFn: func(ctx context.Context, userID string, matchIDs []string, question string) (*ports.ChatResponse, error) {
				return nil, assertAnError
			},
		}, nil)

		body := `{"match_ids":["match-1"],"question":"who won?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestHandleContinueChat(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		handler := newChatHandler(&stubChatUseCase{
			continueSessionFn: func(ctx context.Context, userID string, sessionID string, question string) (*ports.ChatResponse, error) {
				return &ports.ChatResponse{
					SessionID: sessionID, Answer: "follow-up answer",
					Source: "clickhouse", MatchesUsed: []string{"match-1"},
				}, nil
			},
		}, nil)

		body := `{"question":"tell me more"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/session-1", strings.NewReader(body))
		req.SetPathValue("id", "session-1")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var payload map[string]any
		json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()

		data, _ := payload["data"].(map[string]any)
		if data == nil || data["session_id"] != "session-1" {
			t.Errorf("expected session_id=session-1, got %v", data)
		}
	})

	t.Run("session not found 404", func(t *testing.T) {
		handler := newChatHandler(&stubChatUseCase{
			continueSessionFn: func(ctx context.Context, userID string, sessionID string, question string) (*ports.ChatResponse, error) {
				return nil, errSessionNotFound
			},
		}, nil)

		body := `{"question":"tell me more"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/session-nonexistent", strings.NewReader(body))
		req.SetPathValue("id", "session-nonexistent")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("question empty 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		body := `{"question":"  "}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/session-1", strings.NewReader(body))
		req.SetPathValue("id", "session-1")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("no auth 401", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		body := `{"question":"tell me more"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/session-1", strings.NewReader(body))
		req.SetPathValue("id", "session-1")
		// No user ID
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("session expired 404", func(t *testing.T) {
		handler := newChatHandler(&stubChatUseCase{
			continueSessionFn: func(ctx context.Context, userID string, sessionID string, question string) (*ports.ChatResponse, error) {
				return nil, errSessionExpired
			},
		}, nil)

		body := `{"question":"tell me more"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/session-expired", strings.NewReader(body))
		req.SetPathValue("id", "session-expired")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChat(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestHandleListSessions(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		now := time.Now()
		session := &entities.ChatSession{
			ID: "session-1", UserID: "user-1", MatchIDs: []string{"match-1"},
			Messages: []entities.Message{
				{Question: "first q", Answer: "first a", Source: "clickhouse", CreatedAt: now.Add(-time.Hour)},
				{Question: "last q", Answer: "last a", Source: "clickhouse", CreatedAt: now},
			},
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
		}

		handler := newChatHandler(nil, &stubChatSessionUseCase{
			listFn: func(ctx context.Context, userID string, limit, offset int) (*ports.SessionListResult, error) {
				return &ports.SessionListResult{
					Sessions: []*entities.ChatSession{session},
					Total:    1, Limit: limit, Offset: offset,
				}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=10&offset=0", nil)
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var payload map[string]any
		json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()

		data, _ := payload["data"].(map[string]any)
		if data == nil {
			t.Fatal("missing data in response")
		}
		sessions, _ := data["sessions"].([]any)
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}
		total, _ := data["total"].(float64)
		if total != 1 {
			t.Errorf("expected total=1, got %f", total)
		}
	})

	t.Run("invalid limit 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=0", nil)
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("limit exceeds max 50", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?limit=51", nil)
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("invalid offset 400", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat?offset=-1", nil)
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("no auth 401", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat", nil)
		// No user ID
		w := httptest.NewRecorder()

		handler.HandleListSessions(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestHandleGetSession(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		now := time.Now()
		session := &entities.ChatSession{
			ID: "session-1", UserID: "user-1", MatchIDs: []string{"match-1"},
			Messages: []entities.Message{
				{Question: "q1", Answer: "a1", Source: "clickhouse", DataPoints: []entities.DataPoint{{Label: "kills", Value: "10"}}, CreatedAt: now.Add(-2 * time.Hour)},
				{Question: "q2", Answer: "a2", Source: "clickhouse", CreatedAt: now.Add(-1 * time.Hour)},
			},
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
		}

		handler := newChatHandler(nil, &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return session, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/session-1", nil)
		req.SetPathValue("id", "session-1")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var payload map[string]any
		json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()

		data, _ := payload["data"].(map[string]any)
		if data == nil || data["id"] != "session-1" {
			t.Errorf("expected id=session-1, got %v", data)
		}
	})

	t.Run("not found 404", func(t *testing.T) {
		handler := newChatHandler(nil, &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/session-nonexistent", nil)
		req.SetPathValue("id", "session-nonexistent")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("no auth 401", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/session-1", nil)
		req.SetPathValue("id", "session-1")
		// No user ID
		w := httptest.NewRecorder()

		handler.HandleGetSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestHandleDeleteSession(t *testing.T) {
	t.Run("success 204", func(t *testing.T) {
		session := &entities.ChatSession{
			ID: "session-1", UserID: "user-1", MatchIDs: []string{"match-1"},
			Messages: []entities.Message{{Question: "q", Answer: "a", Source: "clickhouse", CreatedAt: time.Now()}},
			CreatedAt: time.Now(), UpdatedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		}

		handler := newChatHandler(nil, &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return session, nil
			},
			deleteFn: func(ctx context.Context, userID, id string) error {
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/session-1", nil)
		req.SetPathValue("id", "session-1")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("not found 404", func(t *testing.T) {
		handler := newChatHandler(nil, &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, nil
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/session-nonexistent", nil)
		req.SetPathValue("id", "session-nonexistent")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("no auth 401", func(t *testing.T) {
		handler := newChatHandler(nil, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/session-1", nil)
		req.SetPathValue("id", "session-1")
		// No user ID
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("get error 500", func(t *testing.T) {
		handler := newChatHandler(nil, &stubChatSessionUseCase{
			getFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, assertAnError
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/session-1", nil)
		req.SetPathValue("id", "session-1")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleDeleteSession(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestChatHandler_HandleNewChatStream(t *testing.T) {
	t.Run("success streaming 200", func(t *testing.T) {
		outChan := make(chan string, 2)
		outChan <- "chunk1 "
		outChan <- "chunk2"
		close(outChan)
		errChan := make(chan error, 1)
		close(errChan)

		uc := &stubChatUseCase{
			newSessionStreamFn: func(ctx context.Context, userID string, matchIDs []string, question string) (*ports.StreamResponse, error) {
				return &ports.StreamResponse{
					SessionID:   "session-abc",
					Source:      "clickhouse",
					MatchesUsed: matchIDs,
					Stream:      outChan,
					ErrChan:     errChan,
				}, nil
			},
		}

		handler := newChatHandler(uc, nil)
		body := `{"match_ids":["match-1"],"question":"how many kills?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChatStream(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Errorf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
		}

		bodyStr := w.Body.String()
		if !strings.Contains(bodyStr, `data: "chunk1 "`) {
			t.Errorf("missing chunk1")
		}
		if !strings.Contains(bodyStr, `data: "chunk2"`) {
			t.Errorf("missing chunk2")
		}
		if !strings.Contains(bodyStr, "event: done") {
			t.Errorf("missing done event")
		}
	})

	t.Run("rate limit exceeded 429", func(t *testing.T) {
		uc := &stubChatUseCase{}
		limiter := services.NewRateLimiter()
		// Exceed limit (default max requests is 5 in services/ratelimit.go)
		key := "chat:stream:user:user-1"
		for i := 0; i < 5; i++ {
			limiter.Allow(key)
		}
		// next one should be blocked
		handler := NewChatHandler(uc, nil, limiter)

		body := `{"match_ids":["match-1"],"question":"how many kills?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", strings.NewReader(body))
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleNewChatStream(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("expected 429, got %d", resp.StatusCode)
		}
	})
}

func TestChatHandler_HandleContinueChatStream(t *testing.T) {
	t.Run("success streaming 200", func(t *testing.T) {
		outChan := make(chan string, 2)
		outChan <- "chunk1 "
		outChan <- "chunk2"
		close(outChan)
		errChan := make(chan error, 1)
		close(errChan)

		uc := &stubChatUseCase{
			continueSessionStreamFn: func(ctx context.Context, userID string, sessionID string, question string) (*ports.StreamResponse, error) {
				return &ports.StreamResponse{
					SessionID:   sessionID,
					Source:      "clickhouse",
					MatchesUsed: []string{"match-1"},
					Stream:      outChan,
					ErrChan:     errChan,
				}, nil
			},
		}

		handler := newChatHandler(uc, nil)
		body := `{"question":"how many kills?"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream/session-abc", strings.NewReader(body))
		req.SetPathValue("id", "session-abc")
		req = req.WithContext(contextWithUserID(req.Context(), "user-1"))
		w := httptest.NewRecorder()

		handler.HandleContinueChatStream(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		bodyStr := w.Body.String()
		if !strings.Contains(bodyStr, `data: "chunk1 "`) {
			t.Errorf("missing chunk1")
		}
		if !strings.Contains(bodyStr, `data: "chunk2"`) {
			t.Errorf("missing chunk2")
		}
		if !strings.Contains(bodyStr, "event: done") {
			t.Errorf("missing done event")
		}
	})
}


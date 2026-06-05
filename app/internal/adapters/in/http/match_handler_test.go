package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"magicstrike/internal/core/entities"
)

// stubMatchRepo implements ports.MatchRepository for testing.
type stubMatchRepo struct {
	saveFn          func(ctx context.Context, match *entities.Match) error
	findByIDFn      func(ctx context.Context, id string) (*entities.Match, error)
	findByDemoMD5Fn func(ctx context.Context, md5Hash string) (*entities.Match, error)
	updateFn        func(ctx context.Context, match *entities.Match) error
	listFn          func(ctx context.Context, limit, offset int) ([]*entities.Match, error)
	listByUserIDFn  func(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error)
}

func (s *stubMatchRepo) Save(ctx context.Context, match *entities.Match) error {
	if s.saveFn != nil {
		return s.saveFn(ctx, match)
	}
	return nil
}

func (s *stubMatchRepo) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *stubMatchRepo) FindByDemoMD5(ctx context.Context, md5Hash string) (*entities.Match, error) {
	if s.findByDemoMD5Fn != nil {
		return s.findByDemoMD5Fn(ctx, md5Hash)
	}
	return nil, nil
}

func (s *stubMatchRepo) Update(ctx context.Context, match *entities.Match) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, match)
	}
	return nil
}

func (s *stubMatchRepo) List(ctx context.Context, limit, offset int) ([]*entities.Match, error) {
	if s.listFn != nil {
		return s.listFn(ctx, limit, offset)
	}
	return nil, nil
}

func (s *stubMatchRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
	if s.listByUserIDFn != nil {
		return s.listByUserIDFn(ctx, userID, limit, offset)
	}
	return nil, nil
}

func TestMatchHandler_HandleListMatches(t *testing.T) {
	t.Run("missing user id in context", func(t *testing.T) {
		repo := &stubMatchRepo{}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid limit", func(t *testing.T) {
		repo := &stubMatchRepo{}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=100", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for limit > 50, got %d", rec.Code)
		}
	})

	t.Run("invalid offset", func(t *testing.T) {
		repo := &stubMatchRepo{}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?offset=-1", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for negative offset, got %d", rec.Code)
		}
	})

	t.Run("successful list with default pagination", func(t *testing.T) {
		teamA := "Team A"
		repo := &stubMatchRepo{
			listByUserIDFn: func(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
				if userID != "user-123" {
					t.Errorf("expected userID 'user-123', got '%s'", userID)
				}
				if limit != 20 {
					t.Errorf("expected default limit 20, got %d", limit)
				}
				return []*entities.Match{
					{
						ID:     "match-1",
						UserID: "user-123",
						Status: entities.MatchStatusWaiting,
						TeamA:  &teamA,
					},
				}, nil
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("successful list with custom pagination", func(t *testing.T) {
		repo := &stubMatchRepo{
			listByUserIDFn: func(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
				if limit != 10 || offset != 5 {
					t.Errorf("expected limit=10 offset=5, got limit=%d offset=%d", limit, offset)
				}
				return []*entities.Match{}, nil
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches?limit=10&offset=5", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("repo returns error", func(t *testing.T) {
		repo := &stubMatchRepo{
			listByUserIDFn: func(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
				return nil, fmt.Errorf("database timeout")
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("nil matches returns empty array", func(t *testing.T) {
		repo := &stubMatchRepo{
			listByUserIDFn: func(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
				return nil, nil // nil matches
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleListMatches(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestMatchHandler_HandleGetMatch(t *testing.T) {
	t.Run("missing user id in context", func(t *testing.T) {
		repo := &stubMatchRepo{}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/match-1", nil)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing match id", func(t *testing.T) {
		repo := &stubMatchRepo{}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/", nil)
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("match not found", func(t *testing.T) {
		repo := &stubMatchRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return nil, nil // not found
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("match belongs to another user", func(t *testing.T) {
		repo := &stubMatchRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return &entities.Match{
					ID:     id,
					UserID: "other-user",
					Status: entities.MatchStatusWaiting,
				}, nil
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/match-1", nil)
		req.SetPathValue("id", "match-1")
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for ownership violation, got %d", rec.Code)
		}
	})

	t.Run("successful get", func(t *testing.T) {
		teamA := "NIP"
		repo := &stubMatchRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return &entities.Match{
					ID:     id,
					UserID: "user-123",
					Status: entities.MatchStatusWaiting,
					TeamA:  &teamA,
				}, nil
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/match-1", nil)
		req.SetPathValue("id", "match-1")
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("repo returns error", func(t *testing.T) {
		repo := &stubMatchRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return nil, fmt.Errorf("database connection lost")
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/match-1", nil)
		req.SetPathValue("id", "match-1")
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("match with all fields populated", func(t *testing.T) {
		teamA := "NaVi"
		teamB := "FaZe"
		demoMD5 := "0123456789abcdef0123456789abcdef"
		scoreA := 13
		scoreB := 10
		totalRounds := 23
		mapName := entities.MapNameDust2

		repo := &stubMatchRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return &entities.Match{
					ID:          id,
					UserID:      "user-123",
					Status:      entities.MatchStatusFinished,
					TeamA:       &teamA,
					TeamB:       &teamB,
					DemoMD5:     &demoMD5,
					ScoreA:      &scoreA,
					ScoreB:      &scoreB,
					TotalRounds: &totalRounds,
					MapName:     &mapName,
				}, nil
			},
		}
		h := NewMatchHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/matches/match-1", nil)
		req.SetPathValue("id", "match-1")
		ctx := context.WithValue(req.Context(), userIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		h.HandleGetMatch(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}


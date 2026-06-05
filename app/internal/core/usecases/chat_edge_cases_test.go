package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chatEdgeMockEventRepo struct {
	ports.EventRepository
	findByMatchIDFn func(ctx context.Context, matchID string) ([]*entities.Event, error)
}

func (m *chatEdgeMockEventRepo) FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error) {
	if m.findByMatchIDFn != nil {
		return m.findByMatchIDFn(ctx, matchID)
	}
	return nil, nil
}

type chatEdgeMockVectorRepo struct {
	ports.VectorRepository
	searchFn func(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error)
}

func (m *chatEdgeMockVectorRepo) Search(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, vector, limit)
	}
	return nil, nil
}

type chatEdgeMockEmbedder struct {
	ports.EmbeddingService
	embedFn func(ctx context.Context, text string) ([]float32, error)
}

func (m *chatEdgeMockEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, text)
	}
	return []float32{0.1, 0.2}, nil
}

type chatEdgeMockLLM struct {
	ports.LLMService
	generateFn func(ctx context.Context, prompt string) (string, error)
}

func (m *chatEdgeMockLLM) GenerateText(ctx context.Context, prompt string) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, prompt)
	}
	return `{"target":"clickhouse","query_type":"top_players_by_metric","metric":"kills","limit":5}`, nil
}

type chatEdgeMockChatSessionRepo struct {
	ports.ChatSessionRepository
	saveFn       func(ctx context.Context, session *entities.ChatSession) error
	findByIDFn   func(ctx context.Context, userID, id string) (*entities.ChatSession, error)
	addMessageFn func(ctx context.Context, userID, sessionID string, question, answer, source string, points []ports.ChatDataPoint) (*entities.ChatSession, error)
}

func (m *chatEdgeMockChatSessionRepo) Save(ctx context.Context, session *entities.ChatSession) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, session)
	}
	return nil
}

func (m *chatEdgeMockChatSessionRepo) FindByID(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID, id)
	}
	return nil, nil
}

func (m *chatEdgeMockChatSessionRepo) AddMessage(ctx context.Context, userID, sessionID string, question, answer, source string, points []ports.ChatDataPoint) (*entities.ChatSession, error) {
	if m.addMessageFn != nil {
		return m.addMessageFn(ctx, userID, sessionID, question, answer, source, points)
	}
	return nil, nil
}

func TestChatUseCase_NewSession_EdgeCases(t *testing.T) {
	ctx := context.Background()
	llm := &chatEdgeMockLLM{}
	eventRepo := &chatEdgeMockEventRepo{}

	t.Run("empty match IDs", func(t *testing.T) {
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, nil, 7).(*ChatUseCase)
		_, err := uc.NewSession(ctx, "user-1", []string{}, "hello")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one match_id is required")
	})

	t.Run("session save fails is logged but returns success", func(t *testing.T) {
		sessionRepo := &chatEdgeMockChatSessionRepo{
			saveFn: func(ctx context.Context, session *entities.ChatSession) error {
				return errors.New("save failed")
			},
		}
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, sessionRepo, 7).(*ChatUseCase)
		res, err := uc.NewSession(ctx, "user-1", []string{"match-1"}, "hello")
		require.NoError(t, err)
		assert.NotNil(t, res)
	})
}

func TestChatUseCase_ContinueSession_EdgeCases(t *testing.T) {
	ctx := context.Background()
	llm := &chatEdgeMockLLM{}
	eventRepo := &chatEdgeMockEventRepo{}

	t.Run("session get error", func(t *testing.T) {
		sessionRepo := &chatEdgeMockChatSessionRepo{
			findByIDFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, errors.New("db error")
			},
		}
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, sessionRepo, 7).(*ChatUseCase)
		_, err := uc.ContinueSession(ctx, "user-1", "sess-1", "hello")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load session")
	})

	t.Run("session not found", func(t *testing.T) {
		sessionRepo := &chatEdgeMockChatSessionRepo{
			findByIDFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return nil, nil
			},
		}
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, sessionRepo, 7).(*ChatUseCase)
		_, err := uc.ContinueSession(ctx, "user-1", "sess-1", "hello")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("session expired", func(t *testing.T) {
		sessionRepo := &chatEdgeMockChatSessionRepo{
			findByIDFn: func(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
				return &entities.ChatSession{
					ID:        id,
					ExpiresAt: time.Now().Add(-1 * time.Hour),
				}, nil
			},
		}
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, sessionRepo, 7).(*ChatUseCase)
		_, err := uc.ContinueSession(ctx, "user-1", "sess-1", "hello")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session has expired")
	})
}

func TestChatUseCase_QueryByIntent_EdgeCases(t *testing.T) {
	ctx := context.Background()
	llm := &chatEdgeMockLLM{}
	eventRepo := &chatEdgeMockEventRepo{}

	t.Run("unknown target target type", func(t *testing.T) {
		uc := NewChatUseCase(eventRepo, nil, nil, nil, llm, nil, 7).(*ChatUseCase)
		intent := &services.QueryIntent{Target: "mysql"}
		_, _, err := uc.queryByIntent(ctx, []string{"match-1"}, intent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown target: mysql")
	})

	t.Run("event repo find error", func(t *testing.T) {
		errEventRepo := &chatEdgeMockEventRepo{
			findByMatchIDFn: func(ctx context.Context, matchID string) ([]*entities.Event, error) {
				return nil, errors.New("db error")
			},
		}
		uc := NewChatUseCase(errEventRepo, nil, nil, nil, llm, nil, 7).(*ChatUseCase)
		intent := &services.QueryIntent{Target: "clickhouse"}
		_, _, err := uc.queryByIntent(ctx, []string{"match-1"}, intent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch events for match")
	})

	t.Run("queryQdrant search error", func(t *testing.T) {
		embedder := &chatEdgeMockEmbedder{}
		vectorRepo := &chatEdgeMockVectorRepo{
			searchFn: func(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error) {
				return nil, errors.New("search failed")
			},
		}
		uc := NewChatUseCase(eventRepo, nil, vectorRepo, embedder, llm, nil, 7).(*ChatUseCase)
		intent := &services.QueryIntent{Target: "qdrant", SearchQuery: "who is the mvp"}
		_, _, err := uc.queryByIntent(ctx, []string{"match-1"}, intent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "qdrant search failed")
	})

	t.Run("queryQdrant search returns empty", func(t *testing.T) {
		embedder := &chatEdgeMockEmbedder{}
		vectorRepo := &chatEdgeMockVectorRepo{
			searchFn: func(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error) {
				return []*ports.VectorRecord{}, nil
			},
		}
		uc := NewChatUseCase(eventRepo, nil, vectorRepo, embedder, llm, nil, 7).(*ChatUseCase)
		intent := &services.QueryIntent{Target: "qdrant", SearchQuery: "who is the mvp"}
		ans, _, err := uc.queryByIntent(ctx, []string{"match-1"}, intent)
		require.NoError(t, err)
		assert.Contains(t, ans, "No similar round narratives found")
	})
}

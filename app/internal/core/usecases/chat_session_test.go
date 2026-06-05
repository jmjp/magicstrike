package usecases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/usecases"
)

// chatSessionMockRepo implements ports.ChatSessionRepository for ChatSessionUseCase tests.
type chatSessionMockRepo struct {
	sessions map[string]*entities.ChatSession
	count    int
	listErr  error
	countErr error
	findErr  error
	deleteErr error
}

func (m *chatSessionMockRepo) Save(ctx context.Context, session *entities.ChatSession) error {
	return nil
}

func (m *chatSessionMockRepo) AddMessage(
	ctx context.Context, userID, sessionID string,
	question, answer, source string,
	dataPoints []ports.ChatDataPoint,
) (*entities.ChatSession, error) {
	return nil, nil
}

func (m *chatSessionMockRepo) FindByID(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.sessions == nil {
		return nil, nil
	}
	s, ok := m.sessions[id]
	if !ok {
		return nil, nil
	}
	if s.UserID != userID {
		return nil, nil
	}
	return s, nil
}

func (m *chatSessionMockRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.ChatSession, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.sessions == nil {
		return []*entities.ChatSession{}, nil
	}
	var result []*entities.ChatSession
	for _, s := range m.sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	// Apply offset and limit
	if offset >= len(result) {
		return []*entities.ChatSession{}, nil
	}
	result = result[offset:]
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (m *chatSessionMockRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.count, nil
}

func (m *chatSessionMockRepo) DeleteByID(ctx context.Context, userID, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.sessions != nil {
		delete(m.sessions, id)
	}
	return nil
}

func (m *chatSessionMockRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func TestChatSessionUseCase_List_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		count:    0,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.List(ctx, "user-1", 0, 0)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 20, result.Limit) // default limit
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Sessions)
}

func TestChatSessionUseCase_List_ValidLimit(t *testing.T) {
	ctx := context.Background()
	s1, _ := entities.NewChatSession("user-1", []string{"match-1"}, "q1", "a1", "clickhouse", nil, 7)
	s2, _ := entities.NewChatSession("user-1", []string{"match-2"}, "q2", "a2", "clickhouse", nil, 7)
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{
			s1.ID: s1,
			s2.ID: s2,
		},
		count: 2,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.List(ctx, "user-1", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 10, result.Limit)
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Sessions, 2)
}

func TestChatSessionUseCase_List_ClampsLimitToMax50(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		count:    0,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.List(ctx, "user-1", 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 20, result.Limit) // out of range, fallback to default 20
}

func TestChatSessionUseCase_List_NegativeOffset(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		count:    0,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.List(ctx, "user-1", 20, -5)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Offset) // clamped to 0
}

func TestChatSessionUseCase_List_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		listErr: assert.AnError,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	_, err := uc.List(ctx, "user-1", 20, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list sessions")
}

func TestChatSessionUseCase_List_CountError(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		countErr: assert.AnError,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	_, err := uc.List(ctx, "user-1", 20, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to count sessions")
}

func TestChatSessionUseCase_Get_Found(t *testing.T) {
	ctx := context.Background()
	session, _ := entities.NewChatSession("user-1", []string{"match-1"}, "q1", "a1", "clickhouse", nil, 7)
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{
			session.ID: session,
		},
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.Get(ctx, "user-1", session.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, session.ID, result.ID)
	assert.Equal(t, "user-1", result.UserID)
}

func TestChatSessionUseCase_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.Get(ctx, "user-1", "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestChatSessionUseCase_Get_DifferentUser(t *testing.T) {
	ctx := context.Background()
	session, _ := entities.NewChatSession("user-1", []string{"match-1"}, "q1", "a1", "clickhouse", nil, 7)
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{
			session.ID: session,
		},
	}
	uc := usecases.NewChatSessionUseCase(repo)

	result, err := uc.Get(ctx, "user-2", session.ID)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestChatSessionUseCase_Get_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		findErr:  assert.AnError,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	_, err := uc.Get(ctx, "user-1", "any-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get session")
}

func TestChatSessionUseCase_Delete_Found(t *testing.T) {
	ctx := context.Background()
	session, _ := entities.NewChatSession("user-1", []string{"match-1"}, "q1", "a1", "clickhouse", nil, 7)
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{
			session.ID: session,
		},
	}
	uc := usecases.NewChatSessionUseCase(repo)

	err := uc.Delete(ctx, "user-1", session.ID)
	require.NoError(t, err)
	// Verify it was actually deleted
	result, _ := uc.Get(ctx, "user-1", session.ID)
	assert.Nil(t, result)
}

func TestChatSessionUseCase_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{}
	uc := usecases.NewChatSessionUseCase(repo)

	err := uc.Delete(ctx, "user-1", "nonexistent-id")
	require.NoError(t, err)
}

func TestChatSessionUseCase_Delete_DifferentUser(t *testing.T) {
	ctx := context.Background()
	session, _ := entities.NewChatSession("user-1", []string{"match-1"}, "q1", "a1", "clickhouse", nil, 7)
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{
			session.ID: session,
		},
	}
	uc := usecases.NewChatSessionUseCase(repo)

	err := uc.Delete(ctx, "user-2", session.ID)
	require.NoError(t, err) // FindByID returns nil for wrong user, Delete treats that as "not found"
	// But the session should still exist for user-1
	result, _ := uc.Get(ctx, "user-1", session.ID)
	require.NotNil(t, result)
	assert.Equal(t, session.ID, result.ID)
}

func TestChatSessionUseCase_Delete_FindError(t *testing.T) {
	ctx := context.Background()
	repo := &chatSessionMockRepo{
		sessions: map[string]*entities.ChatSession{},
		findErr:  assert.AnError,
	}
	uc := usecases.NewChatSessionUseCase(repo)

	err := uc.Delete(ctx, "user-1", "any-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find session")
}

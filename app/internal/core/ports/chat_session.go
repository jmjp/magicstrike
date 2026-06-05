package ports

import (
	"context"

	"magicstrike/internal/core/entities"
)

// ChatSessionRepository defines the persistence interface for ChatSession entities.
type ChatSessionRepository interface {
	// Save persists a new chat session.
	Save(ctx context.Context, session *entities.ChatSession) error

	// AddMessage appends a message to an existing session and returns the updated session.
	AddMessage(
		ctx context.Context, userID, sessionID string,
		question, answer, source string,
		dataPoints []ChatDataPoint,
	) (*entities.ChatSession, error)

	// FindByID retrieves a session by ID for a specific user.
	// Returns nil, nil if not found.
	FindByID(ctx context.Context, userID, id string) (*entities.ChatSession, error)

	// ListByUserID retrieves paginated session summaries for a user, newest first.
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.ChatSession, error)

	// CountByUserID returns the total number of chat sessions for a user.
	CountByUserID(ctx context.Context, userID string) (int, error)

	// DeleteByID removes a session by ID for a specific user.
	DeleteByID(ctx context.Context, userID, id string) error

	// DeleteExpired removes all expired sessions. Returns count of deleted records.
	DeleteExpired(ctx context.Context) (int64, error)
}

// ChatSessionUseCase defines the input port for managing chat sessions (CRUD).
type ChatSessionUseCase interface {
	// List returns paginated session summaries for the given user.
	List(ctx context.Context, userID string, limit, offset int) (*SessionListResult, error)

	// Get retrieves a single session with messages for the given user.
	Get(ctx context.Context, userID, id string) (*entities.ChatSession, error)

	// Delete removes an entire session for the given user.
	Delete(ctx context.Context, userID, id string) error
}

// SessionListResult carries paginated session list results.
type SessionListResult struct {
	Sessions []*entities.ChatSession `json:"sessions"`
	Total    int                     `json:"total"`
	Limit    int                     `json:"limit"`
	Offset   int                     `json:"offset"`
}

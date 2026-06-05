package usecases

import (
	"context"
	"fmt"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// ChatSessionUseCase implements ports.ChatSessionUseCase.
type ChatSessionUseCase struct {
	repo ports.ChatSessionRepository
}

// NewChatSessionUseCase creates a new ChatSessionUseCase.
func NewChatSessionUseCase(repo ports.ChatSessionRepository) ports.ChatSessionUseCase {
	return &ChatSessionUseCase{repo: repo}
}

// List returns paginated session summaries for the given user.
func (uc *ChatSessionUseCase) List(
	ctx context.Context, userID string, limit, offset int,
) (*ports.SessionListResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	sessions, err := uc.repo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	total, err := uc.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	return &ports.SessionListResult{
		Sessions: sessions,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

// Get retrieves a single session with messages for the given user.
func (uc *ChatSessionUseCase) Get(
	ctx context.Context, userID, id string,
) (*entities.ChatSession, error) {
	session, err := uc.repo.FindByID(ctx, userID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, nil
	}
	return session, nil
}

// Delete removes an entire session for the given user.
func (uc *ChatSessionUseCase) Delete(
	ctx context.Context, userID, id string,
) error {
	// Verify session exists and belongs to user
	session, err := uc.repo.FindByID(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("failed to find session: %w", err)
	}
	if session == nil {
		return nil // not found -- handler will check and return 404
	}

	if err := uc.repo.DeleteByID(ctx, userID, id); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

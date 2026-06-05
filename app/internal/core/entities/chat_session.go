package entities

import (
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
)

// Sentinela errors for ChatSession validation.
var (
	// ErrSessionUserIDRequired is returned when the user ID is empty.
	ErrSessionUserIDRequired = errors.New("user ID is required")
	// ErrSessionNoMatches is returned when no match IDs are provided.
	ErrSessionNoMatches = errors.New("at least one match ID is required")
	// ErrSessionQuestionRequired is returned when the question is empty.
	ErrSessionQuestionRequired = errors.New("question is required")
	// ErrSessionQuestionTooLong is returned when the question exceeds the maximum length.
	ErrSessionQuestionTooLong = errors.New("question must be at most 500 characters")
	// ErrSessionAnswerRequired is returned when the answer is empty.
	ErrSessionAnswerRequired = errors.New("answer is required")
	// ErrInvalidSource is returned when the source is not a valid data source identifier.
	ErrInvalidSource = errors.New("source must be 'clickhouse', 'qdrant', or 'clickhouse + qdrant'")
	// ErrMaxMessagesReached is returned when the session has reached the maximum number of messages.
	ErrMaxMessagesReached = errors.New("session has reached the maximum number of messages")
)

// Constants for validation limits.
const (
	MaxQuestionLen        = 500
	MaxAnswerLen          = 50000
	MaxMessagesPerSession = 50  // safety cap
	MaxMessagesInContext  = 10  // for LLM context window
)

// DataPoint represents a single row of supporting data in a chat response.
type DataPoint struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Message represents a single question-answer pair within a chat session.
type Message struct {
	Question   string      `json:"question"`
	Answer     string      `json:"answer"`
	Source     string      `json:"source"`
	DataPoints []DataPoint `json:"data_points,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}

// ChatSession groups a conversation between a user and the AI about a specific set of matches.
// ExpiresAt has no json tag -- it is never sent to the client directly.
type ChatSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	MatchIDs  []string  `json:"match_ids"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"-"`
}

// NewChatSession creates a new ChatSession with the first Q&A pair.
func NewChatSession(
	userID string,
	matchIDs []string,
	question, answer, source string,
	dataPoints []DataPoint,
	ttlDays int,
) (*ChatSession, error) {
	now := time.Now()
	s := &ChatSession{
		ID:       ulid.Make().String(),
		UserID:   userID,
		MatchIDs: matchIDs,
		Messages: []Message{
			{
				Question:   question,
				Answer:     answer,
				Source:     source,
				DataPoints: dataPoints,
				CreatedAt:  now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Duration(ttlDays) * 24 * time.Hour),
	}
	if err := s.Valid(); err != nil {
		return nil, err
	}
	return s, nil
}

// AddMessage appends a new Q&A pair to the session.
func (s *ChatSession) AddMessage(
	question, answer, source string,
	dataPoints []DataPoint,
) error {
	if len(s.Messages) >= MaxMessagesPerSession {
		return ErrMaxMessagesReached
	}
	s.Messages = append(s.Messages, Message{
		Question:   question,
		Answer:     answer,
		Source:     source,
		DataPoints: dataPoints,
		CreatedAt:  time.Now(),
	})
	s.UpdatedAt = time.Now()
	return s.Valid()
}

// Valid checks all validation constraints on the session and its messages.
func (s *ChatSession) Valid() error {
	if s.UserID == "" {
		return ErrSessionUserIDRequired
	}
	if len(s.MatchIDs) == 0 {
		return ErrSessionNoMatches
	}
	if len(s.Messages) == 0 {
		return ErrSessionQuestionRequired
	}
	for _, m := range s.Messages {
		if m.Question == "" {
			return ErrSessionQuestionRequired
		}
		if len(m.Question) > MaxQuestionLen {
			return ErrSessionQuestionTooLong
		}
		if m.Answer == "" {
			return ErrSessionAnswerRequired
		}
		if m.Source != "clickhouse" && m.Source != "qdrant" && m.Source != "clickhouse + qdrant" {
			return ErrInvalidSource
		}
	}
	return nil
}

// LastNMessages returns the last N messages in chronological order (oldest first),
// suitable for use as LLM context. If the session has fewer than N messages,
// returns all messages.
func (s *ChatSession) LastNMessages(n int) []Message {
	if n <= 0 {
		return nil
	}
	if len(s.Messages) <= n {
		result := make([]Message, len(s.Messages))
		copy(result, s.Messages)
		return result
	}
	return s.Messages[len(s.Messages)-n:]
}

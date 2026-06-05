package ports

import "context"

// ChatResponse carries the synthesized answer, session ID, and metadata about how it was produced.
type ChatResponse struct {
	SessionID   string          `json:"session_id"`
	Answer      string          `json:"answer"`
	Source      string          `json:"source"` // "clickhouse", "qdrant", or "clickhouse + qdrant"
	MatchesUsed []string        `json:"matches_used,omitempty"`
	DataPoints  []ChatDataPoint `json:"data_points,omitempty"`
}

// ChatDataPoint represents a single row of supporting data.
type ChatDataPoint struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ChatUseCase defines the input port for conversational analytics over ingested demos.
// It classifies the user's question, routes to the appropriate database (ClickHouse for
// quantitative queries, Qdrant for semantic queries), and synthesizes a natural-language
// answer backed by real data from one or more matches.
type ChatUseCase interface {
	// NewSession creates a chat session with the first question.
	NewSession(ctx context.Context, userID string, matchIDs []string, question string) (*ChatResponse, error)

	// ContinueSession adds a follow-up question to an existing session.
	// The LLM receives previous messages as conversational context.
	ContinueSession(ctx context.Context, userID, sessionID, question string) (*ChatResponse, error)

	// NewSessionStream creates a chat session streaming the response.
	NewSessionStream(ctx context.Context, userID string, matchIDs []string, question string) (*StreamResponse, error)

	// ContinueSessionStream adds a follow-up question streaming the response.
	ContinueSessionStream(ctx context.Context, userID string, sessionID string, question string) (*StreamResponse, error)
}

// StreamResponse carries the stream channels and metadata about how the stream was produced.
type StreamResponse struct {
	SessionID   string          `json:"session_id"`
	Source      string          `json:"source"`
	MatchesUsed []string        `json:"matches_used,omitempty"`
	DataPoints  []ChatDataPoint `json:"data_points,omitempty"`
	Stream      <-chan string   `json:"-"`
	ErrChan     <-chan error    `json:"-"`
}


package ports

import (
	"context"
)

// LLMService defines the interface to interact with Large Language Models (e.g. for generating match summaries or AI analysis).
type LLMService interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}

// StreamingLLMService extends LLMService to support response streaming.
type StreamingLLMService interface {
	LLMService
	GenerateTextStream(ctx context.Context, prompt string) (<-chan string, <-chan error)
}


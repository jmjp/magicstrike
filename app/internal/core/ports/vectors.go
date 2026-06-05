package ports

import (
	"context"
)

// VectorRecord represents an embedded vector record to store or retrieve.
type VectorRecord struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
}

// VectorRepository defines the interface for interacting with a vector database.
type VectorRepository interface {
	Upsert(ctx context.Context, record *VectorRecord) error
	Search(ctx context.Context, vector []float32, limit int) ([]*VectorRecord, error)
	Delete(ctx context.Context, id string) error
}

// EmbeddingService defines the interface for generating vector embeddings from text.
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

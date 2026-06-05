package ports

import (
	"context"
	"io"
)

// IngestionUseCase defines the input port for the end-to-end CS2 demo ingestion pipeline.
// It orchestrates: parse .dem → persist events → generate narratives → create embeddings → index vectors.
type IngestionUseCase interface {
	IngestDemo(ctx context.Context, matchID string, reader io.Reader) error
}

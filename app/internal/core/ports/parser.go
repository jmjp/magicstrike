package ports

import (
	"context"
	"io"
)

type ParseResult struct {
	MapName     string
	TotalRounds int
	TeamA       string
	TeamB       string
	ScoreA      int
	ScoreB      int
}

// Parser defines the input port to parse CS2 demo files from a stream.
type Parser interface {
	ParseStream(ctx context.Context, matchID string, r io.Reader) (*ParseResult, error)
}

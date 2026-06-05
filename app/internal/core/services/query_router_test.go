package services

import (
	"context"
	"errors"
	"testing"
)

type mockLLMService struct {
	text string
	err  error
}

func (m *mockLLMService) GenerateText(ctx context.Context, prompt string) (string, error) {
	return m.text, m.err
}

func TestQueryRouter_Classify(t *testing.T) {
	tests := []struct {
		name      string
		mockText  string
		mockErr   error
		wantErr   bool
		wantTarget string
		wantLimit int
	}{
		{
			name:      "valid clickhouse classification",
			mockText:  `{"target":"clickhouse","query_type":"top_players_by_metric","metric":"thru_smoke","limit":5}`,
			wantErr:   false,
			wantTarget: "clickhouse",
			wantLimit:  5,
		},
		{
			name:      "valid classification wrapped in markdown json block",
			mockText:  "```json\n" + `{"target":"qdrant","search_query":"clutch situation","limit":0}` + "\n```",
			wantErr:   false,
			wantTarget: "qdrant",
			wantLimit:  5, // defaults to 5 if limit <= 0
		},
		{
			name:      "llm error",
			mockErr:   errors.New("llm down"),
			wantErr:   true,
		},
		{
			name:      "invalid json returned",
			mockText:  `invalid json`,
			wantErr:   true,
		},
		{
			name:      "unknown target",
			mockText:  `{"target":"unknown"}`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := &mockLLMService{text: tt.mockText, err: tt.mockErr}
			qr := NewQueryRouter(llm)

			intent, err := qr.Classify(context.Background(), "test question", nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Classify() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && intent != nil {
				if intent.Target != tt.wantTarget {
					t.Errorf("Target: want %s, got %s", tt.wantTarget, intent.Target)
				}
				if intent.Limit != tt.wantLimit {
					t.Errorf("Limit: want %d, got %d", tt.wantLimit, intent.Limit)
				}
			}
		})
	}
}

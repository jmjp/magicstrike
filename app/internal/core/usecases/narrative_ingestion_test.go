package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/usecases"
)

type mockEventRepo struct {
	events []*entities.Event
	err    error
}

func (m *mockEventRepo) Save(ctx context.Context, event *entities.Event) error { return nil }
func (m *mockEventRepo) SaveBatch(ctx context.Context, events []*entities.Event) error { return nil }
func (m *mockEventRepo) FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error) {
	return m.events, m.err
}
func (m *mockEventRepo) DeleteByMatchID(ctx context.Context, matchID string) error { return nil }

type mockLLM struct {
	text           string
	err            error
	receivedPrompt string
}

func (m *mockLLM) GenerateText(ctx context.Context, prompt string) (string, error) {
	m.receivedPrompt = prompt
	return m.text, m.err
}

type mockEmbedder struct {
	vector []float32
	err    error
}

func (m *mockEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.vector, m.err
}

type mockVectorRepo struct {
	records []*ports.VectorRecord
	err     error
}

func (m *mockVectorRepo) Upsert(ctx context.Context, record *ports.VectorRecord) error {
	m.records = append(m.records, record)
	return m.err
}

func (m *mockVectorRepo) Search(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error) {
	return m.records, m.err
}

func (m *mockVectorRepo) Delete(ctx context.Context, id string) error { return nil }

func TestNarrativeService_ProcessMatch_Success(t *testing.T) {
	ctx := context.Background()

	// 1. Setup events for round 1
	matchID := "match-test-1"
	t1 := 0.0

	evStart, _ := entities.NewRoundStartEvent("ev-1", matchID, 1, t1)
	evKill, _ := entities.NewKillEvent("ev-2", matchID, 1, 10.0,
		"111", "Fallen", "CT",
		"222", "s1mple", "T",
		"", "", "",
		"awp", true, 0,
		false, false, false, false,
	)
	evKill.BuyType = "Full Buy"
	evEnd, _ := entities.NewRoundEndEvent("ev-3", matchID, 1, 30.0, "CT", "TargetSaved", 0, 1)
	evMVP, _ := entities.NewRoundMVPEvent("ev-4", matchID, 1, 31.0, "111", "MostEliminations")

	eventsList := []*entities.Event{evStart, evKill, evEnd, evMVP}

	eventRepo := &mockEventRepo{events: eventsList}
	llmSvc := &mockLLM{text: "Fallen made an amazing AWP headshot on s1mple, saving the round for CT."}
	embedSvc := &mockEmbedder{vector: []float32{0.1, 0.2, 0.3}}
	vectorRepo := &mockVectorRepo{}

	svc := usecases.NewNarrativeService(eventRepo, nil, llmSvc, embedSvc, vectorRepo)

	err := svc.ProcessMatch(ctx, matchID)
	require.NoError(t, err)

	// Verify LLM received prompt
	assert.Contains(t, llmSvc.receivedPrompt, "Macroeconomic Context:")
	assert.Contains(t, llmSvc.receivedPrompt, "- Buy Type CT: Full Buy (Avg Equipment Value: >= $4000)")
	assert.Contains(t, llmSvc.receivedPrompt, "- Buy Type T: Eco (Avg Equipment Value: < $1500)")
	assert.Contains(t, llmSvc.receivedPrompt, "Tactical Utility & Combat Performance:")

	// Verify upserted record
	require.Len(t, vectorRepo.records, 1)
	rec := vectorRepo.records[0]

	assert.Equal(t, []float32{0.1, 0.2, 0.3}, rec.Vector)
	assert.Equal(t, matchID, rec.Metadata["match_id"])
	assert.Equal(t, 1, rec.Metadata["round"])
	assert.Equal(t, "CT", rec.Metadata["winner_team"])
	assert.Equal(t, "TargetSaved", rec.Metadata["win_reason"])
	assert.Equal(t, "111", rec.Metadata["mvp_player_id"])
	assert.Equal(t, "Fallen made an amazing AWP headshot on s1mple, saving the round for CT.", rec.Metadata["narrative"])
}

func TestNarrativeService_ProcessMatch_Fallback(t *testing.T) {
	ctx := context.Background()

	matchID := "match-test-2"
	t1 := 0.0

	evStart, _ := entities.NewRoundStartEvent("ev-1", matchID, 1, t1)
	evEnd, _ := entities.NewRoundEndEvent("ev-2", matchID, 1, 30.0, "T", "TargetBombed", 1, 0)
	evMVP, _ := entities.NewRoundMVPEvent("ev-3", matchID, 1, 31.0, "333", "BombPlanted")

	eventsList := []*entities.Event{evStart, evEnd, evMVP}

	eventRepo := &mockEventRepo{events: eventsList}
	// LLM returns error to trigger fallback
	llmSvc := &mockLLM{err: errors.New("deepseek timeout")}
	embedSvc := &mockEmbedder{vector: []float32{0.5, 0.6}}
	vectorRepo := &mockVectorRepo{}

	svc := usecases.NewNarrativeService(eventRepo, nil, llmSvc, embedSvc, vectorRepo)

	err := svc.ProcessMatch(ctx, matchID)
	require.NoError(t, err)

	require.Len(t, vectorRepo.records, 1)
	rec := vectorRepo.records[0]

	// Narrative should be the static rule-based fallback summary
	expectedFallback := "Round 1 won by T. Reason: TargetBombed. MVP: 333 (Reason: BombPlanted)."
	assert.Equal(t, expectedFallback, rec.Metadata["narrative"])
	assert.Equal(t, "T", rec.Metadata["winner_team"])
	assert.Equal(t, "TargetBombed", rec.Metadata["win_reason"])
	assert.Equal(t, "333", rec.Metadata["mvp_player_id"])
}

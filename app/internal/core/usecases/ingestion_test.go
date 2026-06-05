package usecases

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	dispatch "github.com/markus-wa/godispatch"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// Define mock services specifically for Ingestion tests

type mockLLMService struct {
	text string
	err  error
}

func (m *mockLLMService) GenerateText(ctx context.Context, prompt string) (string, error) {
	return m.text, m.err
}

type mockEmbedderService struct {
	vector []float32
	err    error
}

func (m *mockEmbedderService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
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

func (m *mockVectorRepo) Delete(ctx context.Context, id string) error {
	return m.err
}

// custom mockIngestionEventRepo that implements all ports.EventRepository methods without embedding
type mockIngestionEventRepo struct {
	events       []*entities.Event
	deleteCalled bool
	deleteErr    error
}

func (m *mockIngestionEventRepo) Save(ctx context.Context, event *entities.Event) error {
	return nil
}

func (m *mockIngestionEventRepo) SaveBatch(ctx context.Context, events []*entities.Event) error {
	return nil
}

func (m *mockIngestionEventRepo) FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error) {
	return m.events, nil
}

func (m *mockIngestionEventRepo) DeleteByMatchID(ctx context.Context, matchID string) error {
	m.deleteCalled = true
	return m.deleteErr
}

// mockParserThatFails simulates a parser that returns error
type mockParserThatFails struct {
	demoinfocs.Parser
}

func (m *mockParserThatFails) RegisterEventHandler(handler interface{}) dispatch.HandlerIdentifier {
	return nil
}

func (m *mockParserThatFails) RegisterNetMessageHandler(handler interface{}) dispatch.HandlerIdentifier {
	return nil
}

func (m *mockParserThatFails) ParseNextFrame() (bool, error) {
	return false, errors.New("parse error")
}

func (m *mockParserThatFails) Close() error {
	return nil
}

type mockIngestionMatchRepository struct {
	matches map[string]*entities.Match
	saveErr   error
	findErr   error
	updateErr error
}

func (m *mockIngestionMatchRepository) Save(ctx context.Context, match *entities.Match) error {
	m.matches[match.ID] = match
	return m.saveErr
}

func (m *mockIngestionMatchRepository) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.matches[id], nil
}

func (m *mockIngestionMatchRepository) FindByDemoMD5(ctx context.Context, md5Hash string) (*entities.Match, error) {
	return nil, nil
}

func (m *mockIngestionMatchRepository) Update(ctx context.Context, match *entities.Match) error {
	m.matches[match.ID] = match
	return m.updateErr
}

func (m *mockIngestionMatchRepository) List(ctx context.Context, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

func (m *mockIngestionMatchRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

func TestIngestionUseCase_IngestDemo_Success(t *testing.T) {
	eventRepo := &mockIngestionEventRepo{}
	matchRepo := &mockIngestionMatchRepository{
		matches: make(map[string]*entities.Match),
	}

	// Create and register a match first to test status transition
	m, err := entities.NewMatch("user-123", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	m.ID = "match-success"
	matchRepo.matches[m.ID] = m

	// Construct ParserService with a mocked parser creation func
	parserSvc := NewParserService(eventRepo)
	parserSvc.newParser = func(r io.Reader) demoinfocs.Parser {
		return &mockParser{
			handlers: make(map[string]interface{}),
			mockGS: &mockGameState{
				roundsPlayed: 30,
				tTeam:        makeTeamState(16),
				ctTeam:       makeTeamState(14),
			},
		}
	}

	// Construct NarrativeService
	llm := &mockLLMService{text: "round summary"}
	embedder := &mockEmbedderService{vector: make([]float32, 1024)}
	vectorRepo := &mockVectorRepo{}
	narrativeSvc := NewNarrativeService(eventRepo, matchRepo, llm, embedder, vectorRepo)

	// Simulate narrative service reading events
	eventRepo.events = []*entities.Event{
		{ID: "e1", MatchID: "match-success", Round: 1, Type: entities.EventTypeRoundStart},
		{ID: "e2", MatchID: "match-success", Round: 1, Type: entities.EventTypeRoundEnd, WinnerTeam: "T", WinReason: "Elimination"},
	}

	uc := NewIngestionUseCase(parserSvc, narrativeSvc, eventRepo, matchRepo)

	r := strings.NewReader("dummy data")
	err = uc.IngestDemo(context.Background(), "match-success", r)
	if err != nil {
		t.Fatalf("IngestDemo failed: %v", err)
	}

	// Verify match updates in database
	updatedMatch := matchRepo.matches["match-success"]
	if updatedMatch.Status != entities.MatchStatusFinished {
		t.Errorf("expected status %s, got %s", entities.MatchStatusFinished, updatedMatch.Status)
	}
	if updatedMatch.TeamA == nil || *updatedMatch.TeamA != "Team A" {
		val := "nil"
		if updatedMatch.TeamA != nil {
			val = *updatedMatch.TeamA
		}
		t.Errorf("expected team A to be 'Team A', got %q", val)
	}
	if updatedMatch.ScoreA == nil || *updatedMatch.ScoreA != 14 {
		val := -1
		if updatedMatch.ScoreA != nil {
			val = *updatedMatch.ScoreA
		}
		t.Errorf("expected score A to be 14, got %d", val)
	}
}

func TestIngestionUseCase_IngestDemo_ParseFailure(t *testing.T) {
	repo := &mockIngestionEventRepo{}
	matchRepo := &mockIngestionMatchRepository{
		matches: make(map[string]*entities.Match),
	}

	m, err := entities.NewMatch("user-123", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	m.ID = "match-fail"
	matchRepo.matches[m.ID] = m

	parserSvc := NewParserService(repo)
	parserSvc.newParser = func(r io.Reader) demoinfocs.Parser {
		return &mockParserThatFails{}
	}

	llm := &mockLLMService{}
	embedder := &mockEmbedderService{}
	vectorRepo := &mockVectorRepo{}
	narrativeSvc := NewNarrativeService(repo, matchRepo, llm, embedder, vectorRepo)

	uc := NewIngestionUseCase(parserSvc, narrativeSvc, repo, matchRepo)

	r := strings.NewReader("dummy data")
	err = uc.IngestDemo(context.Background(), "match-fail", r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !repo.deleteCalled {
		t.Error("expected DeleteByMatchID to be called for rollback")
	}

	// Verify match status transitioned to failed
	updatedMatch := matchRepo.matches["match-fail"]
	if updatedMatch.Status != entities.MatchStatusFailed {
		t.Errorf("expected status %s, got %s", entities.MatchStatusFailed, updatedMatch.Status)
	}
}

func TestIngestionUseCase_IngestDemo_NarrativeFailure(t *testing.T) {
	repo := &mockIngestionEventRepo{}
	matchRepo := &mockIngestionMatchRepository{
		matches: make(map[string]*entities.Match),
	}

	m, err := entities.NewMatch("user-123", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to create match: %v", err)
	}
	m.ID = "match-narrative-fail"
	matchRepo.matches[m.ID] = m

	parserSvc := NewParserService(repo)
	parserSvc.newParser = func(r io.Reader) demoinfocs.Parser {
		return &mockParser{
			handlers: make(map[string]interface{}),
			mockGS: &mockGameState{
				roundsPlayed: 1,
				tTeam:        makeTeamState(16),
				ctTeam:       makeTeamState(14),
			},
		}
	}

	// Make narrative service fail by returning error from VectorRepo
	llm := &mockLLMService{text: "summary"}
	embedder := &mockEmbedderService{vector: make([]float32, 1024)}
	vectorRepo := &mockVectorRepo{err: errors.New("vector db error")}
	
	// Create narrative service
	narrativeSvc := NewNarrativeService(repo, matchRepo, llm, embedder, vectorRepo)

	repo.events = []*entities.Event{
		{ID: "e1", MatchID: "match-narrative-fail", Round: 1, Type: entities.EventTypeRoundStart},
		{ID: "e2", MatchID: "match-narrative-fail", Round: 1, Type: entities.EventTypeRoundEnd, WinnerTeam: "T", WinReason: "Elimination"},
	}

	uc := NewIngestionUseCase(parserSvc, narrativeSvc, repo, matchRepo)

	r := strings.NewReader("dummy data")
	err = uc.IngestDemo(context.Background(), "match-narrative-fail", r)
	if err == nil {
		t.Fatal("expected narrative error, got nil")
	}

	// Verify match status transitioned to failed
	updatedMatch := matchRepo.matches["match-narrative-fail"]
	if updatedMatch.Status != entities.MatchStatusFailed {
		t.Errorf("expected status %s, got %s", entities.MatchStatusFailed, updatedMatch.Status)
	}
}

func TestMapToMapName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		isNil    bool
	}{
		{input: "de_dust2", expected: "de_dust2", isNil: false},
		{input: "de_mirage", expected: "de_mirage", isNil: false},
		{input: "de_unknown", expected: "", isNil: true},
	}

	for _, tc := range tests {
		res := mapToMapName(tc.input)
		if tc.isNil {
			if res != nil {
				t.Errorf("expected nil for input %s, got %v", tc.input, *res)
			}
		} else {
			if res == nil {
				t.Errorf("expected non-nil for input %s, got nil", tc.input)
			} else if string(*res) != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, string(*res))
			}
		}
	}
}

package usecases_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/usecases"
)

// --- Mocks ---

type chatMockEventRepo struct {
	eventsByMatch map[string][]*entities.Event
	allEvents     []*entities.Event // fallback for single-match tests
	err           error
}

func (m *chatMockEventRepo) Save(ctx context.Context, event *entities.Event) error     { return nil }
func (m *chatMockEventRepo) SaveBatch(ctx context.Context, events []*entities.Event) error { return nil }
func (m *chatMockEventRepo) FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error) {
	if m.eventsByMatch != nil {
		return m.eventsByMatch[matchID], m.err
	}
	return m.allEvents, m.err
}
func (m *chatMockEventRepo) DeleteByMatchID(ctx context.Context, matchID string) error { return nil }

type chatMockVectorRepo struct {
	results []*ports.VectorRecord
	err     error
}

func (m *chatMockVectorRepo) Upsert(ctx context.Context, record *ports.VectorRecord) error { return nil }
func (m *chatMockVectorRepo) Search(ctx context.Context, vector []float32, limit int) ([]*ports.VectorRecord, error) {
	return m.results, m.err
}
func (m *chatMockVectorRepo) Delete(ctx context.Context, id string) error { return nil }

type chatMockEmbedder struct {
	vector []float32
	err    error
}

func (m *chatMockEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.vector, m.err
}

type chatMockLLM struct {
	responses []string
	callCount int
	err       error
}

func (m *chatMockLLM) GenerateText(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return resp, nil
	}
	return "Synthesized answer based on data.", nil
}

func (m *chatMockLLM) GenerateTextStream(ctx context.Context, prompt string) (<-chan string, <-chan error) {
	outChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		if m.err != nil {
			errChan <- m.err
			return
		}

		var text string
		if m.callCount < len(m.responses) {
			text = m.responses[m.callCount]
			m.callCount++
		} else {
			text = "Synthesized answer based on data."
		}

		// Split by spaces or just send the text in chunks
		words := strings.Split(text, " ")
		for i, word := range words {
			if i > 0 {
				outChan <- " "
			}
			outChan <- word
		}
	}()

	return outChan, errChan
}

// chatMockSessionRepo implements ports.ChatSessionRepository for testing.
type chatMockSessionRepo struct {
	sessions map[string]*entities.ChatSession
	err      error
}

func (m *chatMockSessionRepo) Save(ctx context.Context, session *entities.ChatSession) error {
	if m.err != nil {
		return m.err
	}
	if m.sessions == nil {
		m.sessions = make(map[string]*entities.ChatSession)
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *chatMockSessionRepo) AddMessage(
	ctx context.Context, userID, sessionID string,
	question, answer, source string,
	dataPoints []ports.ChatDataPoint,
) (*entities.ChatSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	chatPoints := make([]entities.DataPoint, len(dataPoints))
	for i, dp := range dataPoints {
		chatPoints[i] = entities.DataPoint{Label: dp.Label, Value: dp.Value}
	}
	if err := session.AddMessage(question, answer, source, chatPoints); err != nil {
		return nil, err
	}
	return session, nil
}

func (m *chatMockSessionRepo) FindByID(ctx context.Context, userID, id string) (*entities.ChatSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.sessions == nil {
		return nil, nil
	}
	s, ok := m.sessions[id]
	if !ok {
		return nil, nil
	}
	if s.UserID != userID {
		return nil, nil
	}
	return s, nil
}

func (m *chatMockSessionRepo) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.ChatSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
}

func (m *chatMockSessionRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 0, nil
}

func (m *chatMockSessionRepo) DeleteByID(ctx context.Context, userID, id string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.sessions, id)
	return nil
}

func (m *chatMockSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

type mockMatchRepo struct {
	ports.MatchRepository
	match *entities.Match
	err   error
}

func (m *mockMatchRepo) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	return m.match, m.err
}

// --- Helper to create a ChatUseCase with all dependencies ---

func newTestChatUseCase(
	eventRepo ports.EventRepository,
	vectorRepo ports.VectorRepository,
	embedder ports.EmbeddingService,
	llm ports.LLMService,
	sessionRepo ports.ChatSessionRepository,
) ports.ChatUseCase {
	ttlDays := 7
	return usecases.NewChatUseCase(eventRepo, &mockMatchRepo{}, vectorRepo, embedder, llm, sessionRepo, ttlDays)
}

func newTestChatUseCaseWithMatch(
	eventRepo ports.EventRepository,
	vectorRepo ports.VectorRepository,
	embedder ports.EmbeddingService,
	llm ports.LLMService,
	sessionRepo ports.ChatSessionRepository,
	matchRepo ports.MatchRepository,
) ports.ChatUseCase {
	ttlDays := 7
	return usecases.NewChatUseCase(eventRepo, matchRepo, vectorRepo, embedder, llm, sessionRepo, ttlDays)
}


// --- Tests ---

func TestChatUseCase_ClickHouse_TopPlayersByMetric(t *testing.T) {
	ctx := context.Background()
	matchID := "match-chat-1"
	t1 := 0.0

	// Build events: Fallen has 3 smoke kills, s1mple has 1, coldzera has 2
	events := []*entities.Event{}
	addKill := func(attacker string, thruSmoke bool) {
		ev, _ := entities.NewKillEvent("ev-"+attacker, matchID, 1, t1,
			"sid-"+attacker, attacker, "CT",
			"victim", "Victim", "T",
			"", "", "",
			"awp", false, 0,
			thruSmoke, false, false, false,
		)
		events = append(events, ev)
	}
	addKill("Fallen", true)
	addKill("Fallen", true)
	addKill("Fallen", true)
	addKill("s1mple", true)
	addKill("coldzera", true)
	addKill("coldzera", true)

	eventRepo := &chatMockEventRepo{allEvents: events}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"top_players_by_metric","metric":"thru_smoke","limit":3}`,
			"Fallen lidera com 3 smoke kills, seguido por coldzera com 2 e s1mple com 1.",
		},
	}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "quem fez mais smokes?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Contains(t, resp.Answer, "Fallen")
	assert.Len(t, resp.DataPoints, 3)
	assert.NotEmpty(t, resp.SessionID)
	assert.Equal(t, "Fallen", resp.DataPoints[0].Label)
	assert.Equal(t, "3", resp.DataPoints[0].Value)
}

func TestChatUseCase_ClickHouse_PlayerStat(t *testing.T) {
	ctx := context.Background()
	matchID := "match-chat-2"
	t1 := 0.0

	events := []*entities.Event{}
	for i := 0; i < 5; i++ {
		ev, _ := entities.NewKillEvent("ev-"+string(rune('a'+i)), matchID, 1, t1,
			"sid-s1mple", "s1mple", "T",
			"victim", "Victim", "CT",
			"", "", "",
			"ak47", i%2 == 0, 0,
			false, false, false, false,
		)
		events = append(events, ev)
	}

	eventRepo := &chatMockEventRepo{allEvents: events}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"player_stat","metric":"is_headshot","player_name":"s1mple"}`,
			"s1mple acertou 3 headshots nesta partida.",
		},
	}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "quantos headshots o s1mple acertou?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Contains(t, resp.Answer, "s1mple")
	assert.Len(t, resp.DataPoints, 1)
	assert.Equal(t, "s1mple", resp.DataPoints[0].Label)
	assert.Equal(t, "3", resp.DataPoints[0].Value)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_Qdrant(t *testing.T) {
	ctx := context.Background()
	matchID := "match-chat-3"

	events := []*entities.Event{}
	eventRepo := &chatMockEventRepo{allEvents: events}

	vectorRepo := &chatMockVectorRepo{
		results: []*ports.VectorRecord{
			{
				ID:     "uuid-round-15",
				Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id":    matchID,
					"round":       float64(15),
					"winner_team": "CT",
					"win_reason":  "TargetSaved",
					"narrative":   "Fallen pushed through smoke on A site, catching the T players off guard. His aggressive smoke play won the round for CT.",
				},
			},
		},
	}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"player pushing through smoke against CT defensive setup"}`,
			"Fallen fez uma jogada agressiva através da smoke no site A, pegando os Ts desprevenidos e vencendo o round para os CT.",
		},
	}
	embedder := &chatMockEmbedder{vector: []float32{0.1, 0.2, 0.3}}
	sessionRepo := &chatMockSessionRepo{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "o que fez o jogador smokar o CT?")
	require.NoError(t, err)

	assert.Equal(t, "qdrant", resp.Source)
	assert.Contains(t, resp.Answer, "Fallen")
	assert.Contains(t, resp.Answer, "smoke")
	assert.GreaterOrEqual(t, len(resp.DataPoints), 1)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_ClassificationError(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}

	llm := &chatMockLLM{
		responses: []string{"not valid json"},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.NewSession(ctx, "user-1", []string{"match-x"}, "pergunta qualquer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "classification")
}

func TestChatUseCase_SynthesisWithEmptyAnswer(t *testing.T) {
	ctx := context.Background()
	matchID := "match-chat-5"
	t1 := 0.0

	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT",
		"sid-y", "PlayerY", "T",
		"", "", "",
		"deagle", true, 0,
		false, false, false, false,
	)
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{ev}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"match_summary"}`,
			"Sumario: 1 kill, 0 headshots.", // entity requires non-empty answer
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "resumo da partida?")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.DataPoints)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_EmptyEvents(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"top_players_by_metric","metric":"kills","limit":5}`,
			"Nenhum dado encontrado para esta partida.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"match-empty"}, "quem fez mais kills?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Len(t, resp.DataPoints, 0)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_MatchSummary(t *testing.T) {
	ctx := context.Background()
	matchID := "match-chat-summary"
	t1 := 0.0

	events := []*entities.Event{}
	ev1, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-a", "PlayerA", "T", "sid-b", "PlayerB", "CT",
		"", "", "", "ak47", true, 0, true, false, false, false,
	)
	ev2, _ := entities.NewBombPlantedEvent("ev-2", matchID, 1, 10.0, "A", "p1")
	ev3, _ := entities.NewBombExplodedEvent("ev-3", matchID, 1, 40.0, "A")
	events = append(events, ev1, ev2, ev3)

	ev4, _ := entities.NewKillEvent("ev-4", matchID, 2, 60.0,
		"sid-c", "PlayerC", "CT", "sid-d", "PlayerD", "T",
		"", "", "", "m4a4", false, 0, false, false, false, false,
	)
	ev5, _ := entities.NewBombDefusedEvent("ev-5", matchID, 2, 90.0, "B", "d1")
	events = append(events, ev4, ev5)

	eventRepo := &chatMockEventRepo{allEvents: events}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"match_summary"}`,
			"A partida teve 2 rounds com 2 kills, 1 headshot, 1 smoke kill, 1 bomba plantada, 1 explodida e 1 defusada.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "resumo da partida?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.GreaterOrEqual(t, len(resp.DataPoints), 5)
	assert.Equal(t, []string{matchID}, resp.MatchesUsed)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_Qdrant_EmptyResults(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{results: []*ports.VectorRecord{}}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"clutch round 1v3"}`,
			"Nenhuma narrativa similar encontrada.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"match-x"}, "teve clutch nessa partida?")
	require.NoError(t, err)

	assert.Equal(t, "qdrant", resp.Source)
	assert.Len(t, resp.DataPoints, 0)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_QuestionValidation(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"player using smoke against CT"}`,
			"Resposta sintetizada.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)

	resp, err := uc.NewSession(ctx, "user-1", []string{"match-x"}, "o que fez o jogador smokar o CT?")
	require.NoError(t, err)
	assert.Equal(t, "qdrant", resp.Source)
	assert.NotEmpty(t, resp.SessionID)

	llm.callCount = 0
	resp, err = uc.NewSession(ctx, "user-1", []string{"match-x"}, "who had the most headshots?")
	require.NoError(t, err)
	assert.Equal(t, "qdrant", resp.Source)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_MultiMatch_ClickHouse(t *testing.T) {
	ctx := context.Background()
	t1 := 0.0

	eventsByMatch := make(map[string][]*entities.Event)
	for i := 0; i < 3; i++ {
		ev, _ := entities.NewKillEvent("ev-a-"+string(rune('a'+i)), "match-A", 1, t1,
			"sid-fallen", "Fallen", "CT",
			"victim", "Victim", "T",
			"", "", "", "awp", false, 0,
			true, false, false, false,
		)
		eventsByMatch["match-A"] = append(eventsByMatch["match-A"], ev)
	}
	for i := 0; i < 2; i++ {
		ev, _ := entities.NewKillEvent("ev-a-s"+string(rune('a'+i)), "match-A", 2, t1,
			"sid-s1mple", "s1mple", "T",
			"victim", "Victim", "CT",
			"", "", "", "ak47", false, 0,
			true, false, false, false,
		)
		eventsByMatch["match-A"] = append(eventsByMatch["match-A"], ev)
	}
	for i := 0; i < 4; i++ {
		ev, _ := entities.NewKillEvent("ev-b-"+string(rune('a'+i)), "match-B", 1, t1,
			"sid-coldzera", "coldzera", "CT",
			"victim", "Victim", "T",
			"", "", "", "m4a4", false, 0,
			true, false, false, false,
		)
		eventsByMatch["match-B"] = append(eventsByMatch["match-B"], ev)
	}

	eventRepo := &chatMockEventRepo{eventsByMatch: eventsByMatch}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"top_players_by_metric","metric":"thru_smoke","limit":3}`,
			"coldzera lidera com 4 smoke kills (match-B), seguido por Fallen com 3 (match-A) e s1mple com 2 (match-A).",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"match-A", "match-B"}, "quem fez mais smokes?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Contains(t, resp.Answer, "coldzera")
	assert.Len(t, resp.DataPoints, 3)
	assert.Equal(t, "coldzera", resp.DataPoints[0].Label)
	assert.Equal(t, "4", resp.DataPoints[0].Value)
	assert.Equal(t, []string{"match-A", "match-B"}, resp.MatchesUsed)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_MultiMatch_Qdrant(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: []float32{0.1, 0.2, 0.3}}

	vectorRepo := &chatMockVectorRepo{
		results: []*ports.VectorRecord{
			{
				ID: "uuid-1", Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id": "match-A", "round": float64(15), "winner_team": "CT",
					"win_reason": "TargetSaved",
					"narrative": "Clutch 1v3 by Fallen on A site.",
				},
			},
			{
				ID: "uuid-2", Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id": "match-B", "round": float64(8), "winner_team": "T",
					"win_reason": "TargetBombed",
					"narrative": "Coldzera ninja defuse fake into plant.",
				},
			},
			{
				ID: "uuid-3", Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id": "match-C", "round": float64(3), "winner_team": "T",
					"win_reason": "CTSurrender",
					"narrative": "Should be filtered out.",
				},
			},
		},
	}

	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"clutch plays"}`,
			"Encontradas 2 jogadas de clutch: Fallen 1v3 na match-A e coldzera fake defuse na match-B.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"match-A", "match-B"}, "teve clutch?")
	require.NoError(t, err)

	assert.Equal(t, "qdrant", resp.Source)
	assert.Contains(t, resp.Answer, "Fallen")
	assert.Contains(t, resp.Answer, "coldzera")
	for _, dp := range resp.DataPoints {
		assert.NotContains(t, dp.Label, "match-C")
	}
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_EmptyMatchIDs(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.NewSession(ctx, "user-1", []string{}, "alguma pergunta?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one match_id")
}

func TestChatUseCase_ContinueSession_Success(t *testing.T) {
	ctx := context.Background()
	matchID := "match-cont-1"
	t1 := 0.0

	

	events := []*entities.Event{}
	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT",
		"sid-y", "PlayerY", "T",
		"", "", "",
		"ak47", true, 0, false, false, false, false,
	)
	events = append(events, ev)

	eventRepo := &chatMockEventRepo{allEvents: events}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}

	// Create a session by running NewSession first
	sessionRepo := &chatMockSessionRepo{}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"match_summary"}`,
			"Resumo inicial da partida.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	firstResp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "resumo da partida?")
	require.NoError(t, err)
	require.NotEmpty(t, firstResp.SessionID)

	// Now continue the session with a follow-up
	llm.callCount = 0 // reset
	llm.responses = []string{
		`{"target":"clickhouse","query_type":"player_stat","metric":"is_headshot","player_name":"PlayerX"}`,
		"PlayerX acertou 1 headshot.",
	}

	resp, err := uc.ContinueSession(ctx, "user-1", firstResp.SessionID, "quantos headshots o PlayerX fez?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Contains(t, resp.Answer, "PlayerX")
	assert.Equal(t, firstResp.SessionID, resp.SessionID)
	assert.Len(t, resp.DataPoints, 1)
}

func TestChatUseCase_ContinueSession_NotFound(t *testing.T) {
	ctx := context.Background()

	eventRepo := &chatMockEventRepo{}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.ContinueSession(ctx, "user-1", "nonexistent-session-id", "follow up?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestChatUseCase_ContinueSession_Expired(t *testing.T) {
	ctx := context.Background()

	// Create an expired session directly in the mock repo
	expiredSession, err := entities.NewChatSession(
		"user-1", []string{"match-1"}, "first question",
		"first answer", "clickhouse", nil, 0, // ttlDays=0 → expires immediately
	)
	require.NoError(t, err)

	// Manually set ExpiresAt to the past
	expiredSession.ExpiresAt = time.Now().Add(-1 * time.Hour)

	eventRepo := &chatMockEventRepo{}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{
		sessions: map[string]*entities.ChatSession{
			expiredSession.ID: expiredSession,
		},
	}
	llm := &chatMockLLM{}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err = uc.ContinueSession(ctx, "user-1", expiredSession.ID, "follow up?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session has expired")
}

// errAddMessageSessionRepo errors only on AddMessage, not on FindByID.
type errAddMessageSessionRepo struct {
	chatMockSessionRepo
}

func (m *errAddMessageSessionRepo) AddMessage(
	ctx context.Context, userID, sessionID string,
	question, answer, source string,
	dataPoints []ports.ChatDataPoint,
) (*entities.ChatSession, error) {
	return nil, assert.AnError
}

func TestChatUseCase_ContinueSession_BestEffortSave(t *testing.T) {
	ctx := context.Background()
	matchID := "match-best-effort"
	t1 := 0.0

	events := []*entities.Event{}
	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT",
		"sid-y", "PlayerY", "T",
		"", "", "",
		"deagle", true, 0, false, false, false, false,
	)
	events = append(events, ev)

	eventRepo := &chatMockEventRepo{allEvents: events}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"match_summary"}`,
			"Resumo inicial.",
		},
	}

	// Create session first via NewSession
	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	firstResp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "resumo da partida?")
	require.NoError(t, err)
	require.NotEmpty(t, firstResp.SessionID)

	// Use a repo that only errors on AddMessage — should still succeed gracefully
	ucWithErr := newTestChatUseCase(eventRepo, vectorRepo, embedder,
		&chatMockLLM{
			responses: []string{
				`{"target":"clickhouse","query_type":"match_summary"}`,
				"Resposta de follow-up.",
			},
		},
		&errAddMessageSessionRepo{
			chatMockSessionRepo: chatMockSessionRepo{
				sessions: sessionRepo.sessions, // copy sessions from original
			},
		},
	)

	resp, err := ucWithErr.ContinueSession(ctx, "user-1", firstResp.SessionID, "mais detalhes?")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, firstResp.SessionID, resp.SessionID)
}

func TestChatUseCase_NewSession_BestEffortSave(t *testing.T) {
	ctx := context.Background()
	matchID := "match-best-effort-new"
	t1 := 0.0

	events := []*entities.Event{}
	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT",
		"sid-y", "PlayerY", "T",
		"", "", "",
		"deagle", true, 0, false, false, false, false,
	)
	events = append(events, ev)

	eventRepo := &chatMockEventRepo{allEvents: events}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{err: assert.AnError} // Save fails
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"match_summary"}`,
			"Resumo mesmo com erro de persistencia.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "resumo da partida?")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.SessionID) // session ID is still generated even if save fails
	assert.Equal(t, "clickhouse", resp.Source)
}

func TestChatUseCase_QueryByIntent_UnknownTarget(t *testing.T) {
	ctx := context.Background()

	// A classification response with an unsupported target will be parsed
	// and routed through queryByIntent, hitting the default case.
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"unsupported_db","query_type":"match_summary"}`,
			"irrelevant",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.NewSession(ctx, "user-1", []string{"match-x"}, "test?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown target")
}

func TestChatUseCase_ConvertDataPoints(t *testing.T) {
	// Test with data points from a query
	t.Run("multiple data points", func(t *testing.T) {
		ctx := context.Background()
		matchID := "match-dp"
		t1 := 0.0

		events := []*entities.Event{}
		ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
			"sid-x", "PlayerX", "CT",
			"sid-y", "PlayerY", "T",
			"", "", "", "ak47", true, 0, false, false, false, false,
		)
		events = append(events, ev)

		eventRepo := &chatMockEventRepo{allEvents: events}
		embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
		vectorRepo := &chatMockVectorRepo{}
		sessionRepo := &chatMockSessionRepo{}
		llm := &chatMockLLM{
			responses: []string{
				`{"target":"clickhouse","query_type":"top_players_by_metric","metric":"kills","limit":5}`,
				"PlayerX: 1 kill",
			},
		}

		uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
		resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "quem fez mais kills?")
		require.NoError(t, err)
		assert.Len(t, resp.DataPoints, 1)
		assert.Equal(t, "PlayerX", resp.DataPoints[0].Label)
		assert.Equal(t, "1", resp.DataPoints[0].Value)
	})

	// Test with empty data points (no events)
	t.Run("empty data points", func(t *testing.T) {
		ctx := context.Background()

		eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
		embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
		vectorRepo := &chatMockVectorRepo{}
		sessionRepo := &chatMockSessionRepo{}
		llm := &chatMockLLM{
			responses: []string{
				`{"target":"clickhouse","query_type":"top_players_by_metric","metric":"kills","limit":5}`,
				"No data found.",
			},
		}

		uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
		resp, err := uc.NewSession(ctx, "user-1", []string{"match-empty"}, "quem fez mais kills?")
		require.NoError(t, err)
		assert.Empty(t, resp.DataPoints)
	})
}

func TestChatUseCase_TruncateFunctions(t *testing.T) {
	ctx := context.Background()

	// Test truncateMatchID and truncate edge cases through Qdrant flow.
	// These are called from queryQdrant when building data points.

	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: []float32{0.1, 0.2, 0.3}}
	vectorRepo := &chatMockVectorRepo{
		results: []*ports.VectorRecord{
			{
				ID: "uuid-1", Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id":    "a-very-long-match-id-that-exceeds-twelve-chars",
					"round":       float64(5),
					"winner_team": "CT",
					"win_reason":  "TargetSaved",
					"narrative":   "This is a very long narrative that should be truncated to 120 characters maximum by the truncate function in chat.go. It has words that go on and on without any real point.",
				},
			},
		},
	}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"test truncation"}`,
			"Result.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"a-very-long-match-id-that-exceeds-twelve-chars"}, "test?")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.DataPoints)
	// The match ID should be truncated in the label
	assert.Contains(t, resp.DataPoints[0].Label, "...")
	// The narrative should be truncated in the value
	require.Len(t, resp.DataPoints, 1)
	assert.LessOrEqual(t, len(resp.DataPoints[0].Value), 123) // 120 + "..."
}

func TestChatUseCase_EmptyMatchIDs_Qdrant(t *testing.T) {
	ctx := context.Background()

	// Test Qdrant with empty search results and no match IDs returned
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{}}
	embedder := &chatMockEmbedder{vector: []float32{0.1}}
	vectorRepo := &chatMockVectorRepo{
		results: []*ports.VectorRecord{},
	}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"qdrant","search_query":"something not found"}`,
			"No results.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{"match-x"}, "test qdrant empty?")
	require.NoError(t, err)
	assert.Equal(t, "qdrant", resp.Source)
	assert.Empty(t, resp.DataPoints)
}

func TestChatUseCase_ClickHouse_RoundAggregate(t *testing.T) {
	ctx := context.Background()
	matchID := "match-round-aggr"
	t1 := 0.0

	// Create events across rounds for round aggregate
	events := []*entities.Event{}
	for r := 1; r <= 3; r++ {
		for i := 0; i < r; i++ { // round 1 has 1 kill, round 2 has 2, round 3 has 3
			ev, _ := entities.NewKillEvent("ev-r", matchID, r, t1,
				"sid-x", "PlayerX", "CT",
				"sid-y", "PlayerY", "T",
				"", "", "", "ak47", i%2 == 0, 0, false, false, false, false,
			)
			events = append(events, ev)
		}
	}

	eventRepo := &chatMockEventRepo{allEvents: events}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"round_aggregate","metric":"kills","limit":3}`,
			"Round 3 had the most kills with 3, followed by round 2 with 2 and round 1 with 1.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "quais rounds tiveram mais kills?")
	require.NoError(t, err)

	assert.Equal(t, "clickhouse", resp.Source)
	assert.Len(t, resp.DataPoints, 3)
	assert.NotEmpty(t, resp.SessionID)
}

func TestChatUseCase_ClickHouse_PlayerStatMissingName(t *testing.T) {
	ctx := context.Background()
	matchID := "match-ps-missing"
	t1 := 0.0

	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT", "sid-y", "PlayerY", "T",
		"", "", "", "ak47", true, 0, false, false, false, false,
	)
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{ev}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"player_stat","metric":"kills"}`,
			"irrelevant",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.NewSession(ctx, "user-1", []string{matchID}, "quantas kills?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "player_name is required")
}

func TestChatUseCase_QueryClickHouse_UnsupportedQueryType(t *testing.T) {
	ctx := context.Background()
	matchID := "match-unsupported"
	t1 := 0.0

	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT", "sid-y", "PlayerY", "T",
		"", "", "", "ak47", true, 0, false, false, false, false,
	)
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{ev}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"invalid_type","metric":"kills"}`,
			"irrelevant",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)
	_, err := uc.NewSession(ctx, "user-1", []string{matchID}, "alguma coisa?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported query_type")
}

func TestChatUseCase_Streaming(t *testing.T) {
	ctx := context.Background()
	matchID := "match-stream-test"
	t1 := 0.0

	ev, _ := entities.NewKillEvent("ev-1", matchID, 1, t1,
		"sid-x", "PlayerX", "CT", "sid-y", "PlayerY", "T",
		"", "", "", "ak47", true, 0, false, false, false, false,
	)
	eventRepo := &chatMockEventRepo{allEvents: []*entities.Event{ev}}
	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}
	vectorRepo := &chatMockVectorRepo{}
	sessionRepo := &chatMockSessionRepo{}
	llm := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"round_aggregate","metric":"kills","limit":3}`,
			"Generated summary response from streaming LLM.",
		},
	}

	uc := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm, sessionRepo)

	// Test NewSessionStream
	resp, err := uc.NewSessionStream(ctx, "user-1", []string{matchID}, "quais rounds tiveram mais kills?")
	require.NoError(t, err)
	assert.Equal(t, "clickhouse", resp.Source)
	assert.NotEmpty(t, resp.SessionID)

	var answerBuilder strings.Builder
	for chunk := range resp.Stream {
		answerBuilder.WriteString(chunk)
	}

	// Check if there was any streaming error
	require.NoError(t, <-resp.ErrChan)

	assert.Equal(t, "Generated summary response from streaming LLM.", answerBuilder.String())

	// Verify the session was persisted in mock database
	persisted, err := sessionRepo.FindByID(ctx, "user-1", resp.SessionID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, "Generated summary response from streaming LLM.", persisted.Messages[0].Answer)

	// Test ContinueSessionStream
	llm2 := &chatMockLLM{
		responses: []string{
			`{"target":"clickhouse","query_type":"round_aggregate","metric":"kills","limit":3}`,
			"Follow up stream answer.",
		},
	}
	uc2 := newTestChatUseCase(eventRepo, vectorRepo, embedder, llm2, sessionRepo)

	resp2, err := uc2.ContinueSessionStream(ctx, "user-1", resp.SessionID, "outra pergunta?")
	require.NoError(t, err)
	assert.Equal(t, "clickhouse", resp2.Source)

	var answerBuilder2 strings.Builder
	for chunk := range resp2.Stream {
		answerBuilder2.WriteString(chunk)
	}
	require.NoError(t, <-resp2.ErrChan)
	assert.Equal(t, "Follow up stream answer.", answerBuilder2.String())

	// Verify follow-up message was persisted
	persisted2, err := sessionRepo.FindByID(ctx, "user-1", resp.SessionID)
	require.NoError(t, err)
	require.NotNil(t, persisted2)
	assert.Len(t, persisted2.Messages, 2)
	assert.Equal(t, "Follow up stream answer.", persisted2.Messages[1].Answer)
}

func TestChatUseCase_HybridAndMatchContext(t *testing.T) {
	ctx := context.Background()
	matchID := "match-hybrid-1"

	mapName := entities.MapNameDust2
	teamA := "NaVi"
	teamB := "FaZe"
	scoreA := 13
	scoreB := 11
	totalRounds := 24
	m := &entities.Match{
		ID:          matchID,
		UserID:      "user-1",
		Status:      entities.MatchStatusFinished,
		MapName:     &mapName,
		TeamA:       &teamA,
		TeamB:       &teamB,
		ScoreA:      &scoreA,
		ScoreB:      &scoreB,
		TotalRounds: &totalRounds,
	}

	matchRepo := &mockMatchRepo{match: m}

	// Create a mock kill event to verify buildMatchContext player extraction
	ev, err := entities.NewKillEvent("kill-1", matchID, 1, 15.5,
		"sid-s1mple", "s1mple", "T",
		"sid-karrigan", "karrigan", "CT",
		"", "", "",
		"ak47", true, 0,
		false, false, false, false,
	)
	require.NoError(t, err)
	events := []*entities.Event{ev}
	eventRepo := &chatMockEventRepo{allEvents: events}

	vectorRepo := &chatMockVectorRepo{
		results: []*ports.VectorRecord{
			{
				ID:     "rec-1",
				Vector: make([]float32, 1024),
				Metadata: map[string]any{
					"match_id":  matchID,
					"round":     float64(1),
					"narrative": "s1mple clutched the first round.",
				},
			},
		},
	}

	embedder := &chatMockEmbedder{vector: make([]float32, 1024)}

	llm := &chatMockLLM{
		responses: []string{
			`{"target":"hybrid","query_type":"custom_query"}`,
			"NaVi venciou o jogo com s1mple se destacando.",
		},
	}
	sessionRepo := &chatMockSessionRepo{}

	uc := newTestChatUseCaseWithMatch(eventRepo, vectorRepo, embedder, llm, sessionRepo, matchRepo)

	resp, err := uc.NewSession(ctx, "user-1", []string{matchID}, "como foi o jogo da NaVi?")
	require.NoError(t, err)
	assert.Equal(t, "clickhouse + qdrant", resp.Source)
	assert.Contains(t, resp.Answer, "NaVi venciou o jogo")
}



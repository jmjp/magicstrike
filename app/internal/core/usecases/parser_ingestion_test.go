package usecases

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	msg "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
	dispatch "github.com/markus-wa/godispatch"

	"magicstrike/internal/core/entities"
)

type mockEventRepo struct {
	saveBatchFunc func(ctx context.Context, events []*entities.Event) error
}

func (m *mockEventRepo) Save(ctx context.Context, event *entities.Event) error {
	return nil
}

func (m *mockEventRepo) SaveBatch(ctx context.Context, events []*entities.Event) error {
	if m.saveBatchFunc != nil {
		return m.saveBatchFunc(ctx, events)
	}
	return nil
}

func (m *mockEventRepo) FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error) {
	return nil, nil
}

func (m *mockEventRepo) DeleteByMatchID(ctx context.Context, matchID string) error {
	return nil
}

type mockDemoInfoProvider struct{}

func (m mockDemoInfoProvider) IngameTick() int                                     { return 0 }
func (m mockDemoInfoProvider) TickRate() float64                                   { return 128.0 }
func (m mockDemoInfoProvider) FindPlayerByHandle(handle uint64) *common.Player     { return nil }
func (m mockDemoInfoProvider) FindPlayerByPawnHandle(handle uint64) *common.Player { return nil }
func (m mockDemoInfoProvider) PlayerResourceEntity() st.Entity                     { return nil }
func (m mockDemoInfoProvider) FindWeaponByEntityID(id int) *common.Equipment       { return nil }
func (m mockDemoInfoProvider) FindEntityByHandle(handle uint64) st.Entity          { return nil }
func (m mockDemoInfoProvider) IsSource2() bool                                     { return false }

type mockEntity struct {
	st.Entity
	score    int32
	clanName string
}

func (m mockEntity) PropertyValueMust(name string) st.PropertyValue {
	if strings.HasPrefix(name, "m_sz") {
		return st.PropertyValue{Any: m.clanName}
	}
	return st.PropertyValue{Any: m.score}
}

func makeTeamState(score int32) *common.TeamState {
	return makeTeamStateWithName(score, "")
}

func makeTeamStateWithName(score int32, name string) *common.TeamState {
	ts := &common.TeamState{
		Entity: mockEntity{score: score, clanName: name},
	}
	val := reflect.ValueOf(ts).Elem()
	f := val.FieldByName("demoInfoProvider")
	ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	ptr.Set(reflect.ValueOf(mockDemoInfoProvider{}))
	return ts
}

// mockParser implements demoinfocs.Parser.
type mockParser struct {
	demoinfocs.Parser
	handlers           map[string]interface{}
	currTime           time.Duration
	mockGS             *mockGameState
	parseCalled        bool
	parseNextFrameFunc func() (bool, error)
}

func (mp *mockParser) RegisterEventHandler(handler interface{}) dispatch.HandlerIdentifier {
	t := reflect.TypeOf(handler)
	if t.Kind() == reflect.Func && t.NumIn() == 1 {
		argType := t.In(0).String()
		mp.handlers[argType] = handler
	}
	return nil
}

func (mp *mockParser) RegisterNetMessageHandler(handler interface{}) dispatch.HandlerIdentifier {
	t := reflect.TypeOf(handler)
	if t.Kind() == reflect.Func && t.NumIn() == 1 {
		argType := t.In(0).String()
		mp.handlers[argType] = handler
	}
	return nil
}

func (mp *mockParser) ParseNextFrame() (bool, error) {
	if mp.parseNextFrameFunc != nil {
		return mp.parseNextFrameFunc()
	}
	if !mp.parseCalled {
		mp.parseCalled = true
		mp.Trigger(events.MatchStart{})
		mp.Trigger(events.AnnouncementWinPanelMatch{})
		mp.Trigger(events.RoundStart{})
		mp.Trigger(events.RoundEnd{
			Winner: common.TeamTerrorists,
			Reason: 9,
		})
		mp.Trigger(events.RoundMVPAnnouncement{
			Player: &common.Player{SteamID64: 999, Name: "MVPPlayer"},
			Reason: events.MVPReasonMostEliminations,
		})
		mp.Trigger(events.Kill{
			Killer: &common.Player{SteamID64: 111, Name: "Killer", Team: common.TeamCounterTerrorists},
			Victim: &common.Player{SteamID64: 222, Name: "Victim", Team: common.TeamTerrorists},
		})
		mp.Trigger(events.BombPlanted{
			BombEvent: events.BombEvent{
				Player: &common.Player{SteamID64: 444},
				Site:   events.BombsiteA,
			},
		})
		mp.Trigger(events.BombDefused{
			BombEvent: events.BombEvent{
				Player: &common.Player{SteamID64: 555},
				Site:   events.BombsiteB,
			},
		})
		mp.Trigger(events.BombExplode{
			BombEvent: events.BombEvent{
				Site: events.BombsiteA,
			},
		})
		return true, nil // process one frame
	}
	return false, nil
}

func (mp *mockParser) Close() error {
	return nil
}

func (mp *mockParser) CurrentTime() time.Duration {
	return mp.currTime
}

func (mp *mockParser) GameState() demoinfocs.GameState {
	return mp.mockGS
}

func (mp *mockParser) Cancel() {}

func (mp *mockParser) Trigger(ev interface{}) {
	t := reflect.TypeOf(ev)
	argType := t.String()
	if handler, ok := mp.handlers[argType]; ok {
		reflect.ValueOf(handler).Call([]reflect.Value{reflect.ValueOf(ev)})
	}
}

// mockGameState implements demoinfocs.GameState interface.
type mockParticipants struct {
	demoinfocs.Participants
	playing []*common.Player
}

func (mp mockParticipants) Playing() []*common.Player {
	return mp.playing
}

type mockGameState struct {
	demoinfocs.GameState
	roundsPlayed int
	tTeam        *common.TeamState
	ctTeam       *common.TeamState
	sidesSwapped bool
}

func (mgs *mockGameState) Participants() demoinfocs.Participants {
	p1 := &common.Player{SteamID64: 111, Name: "Killer", Team: common.TeamCounterTerrorists}
	p2 := &common.Player{SteamID64: 222, Name: "Victim", Team: common.TeamTerrorists}
	p3 := &common.Player{SteamID64: 444, Name: "Planter", Team: common.TeamTerrorists}
	p4 := &common.Player{SteamID64: 555, Name: "Defuser", Team: common.TeamCounterTerrorists}
	return mockParticipants{playing: []*common.Player{p1, p2, p3, p4}}
}


func (mgs *mockGameState) TotalRoundsPlayed() int {
	return mgs.roundsPlayed
}

func (mgs *mockGameState) TeamTerrorists() *common.TeamState {
	if mgs.sidesSwapped {
		return mgs.ctTeam
	}
	return mgs.tTeam
}

func (mgs *mockGameState) TeamCounterTerrorists() *common.TeamState {
	if mgs.sidesSwapped {
		return mgs.tTeam
	}
	return mgs.ctTeam
}

func (mgs *mockGameState) Team(team common.Team) *common.TeamState {
	t := team
	if mgs.sidesSwapped {
		switch team {
		case common.TeamTerrorists:
			t = common.TeamCounterTerrorists
		case common.TeamCounterTerrorists:
			t = common.TeamTerrorists
		}
	}
	switch t {
	case common.TeamTerrorists:
		return mgs.tTeam
	case common.TeamCounterTerrorists:
		return mgs.ctTeam
	default:
		return nil
	}
}

func TestParserService_ParseStream_InvalidHeader(t *testing.T) {
	repo := &mockEventRepo{}
	svc := NewParserService(repo)

	r := strings.NewReader("invalid demo stream content")
	ctx := context.Background()

	_, err := svc.ParseStream(ctx, "match_123", r)
	if err == nil {
		t.Fatal("expected error parsing invalid header, got nil")
	}
}

func TestParserService_ParseStream_DemoHeader(t *testing.T) {
	repo := &mockEventRepo{}
	svc := NewParserService(repo)

	headerBytes := make([]byte, 1072)
	copy(headerBytes, "HL2DEMO\x00")

	r := bytes.NewReader(headerBytes)
	ctx := context.Background()

	_, err := svc.ParseStream(ctx, "match_123", r)
	t.Logf("ParseStream returned error: %v", err)
}

func TestParserService_ParseStream_MockedPipeline(t *testing.T) {
	repo := &mockEventRepo{}
	var savedEvents []*entities.Event
	repo.saveBatchFunc = func(ctx context.Context, events []*entities.Event) error {
		savedEvents = append(savedEvents, events...)
		return nil
	}

	svc := NewParserService(repo)

	mp := &mockParser{
		handlers: make(map[string]interface{}),
		currTime: 10 * time.Second,
		mockGS: &mockGameState{
			roundsPlayed: 5,
			tTeam:        makeTeamState(3),
			ctTeam:       makeTeamState(2),
		},
	}

	// Override parser creation with our mock parser
	svc.newParser = func(r io.Reader) demoinfocs.Parser {
		return mp
	}

	// Run ParseStream in a goroutine because we want to trigger events while ParseStream is running
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := strings.NewReader("dummy stream")

	_, err := svc.ParseStream(ctx, "match_123", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert events were saved
	if len(savedEvents) == 0 {
		t.Fatalf("expected saved events, got none")
	}

	eventTypes := make(map[entities.EventType]bool)
	for _, e := range savedEvents {
		eventTypes[e.Type] = true
	}

	expectedTypes := []entities.EventType{
		entities.EventTypeMatchStart,
		entities.EventTypeMatchEnd,
		entities.EventTypeRoundStart,
		entities.EventTypeRoundEnd,
		entities.EventTypeRoundMVP,
		entities.EventTypeKill,
		entities.EventTypeBombPlanted,
		entities.EventTypeBombDefused,
		entities.EventTypeBombExploded,
	}

	for _, et := range expectedTypes {
		if !eventTypes[et] {
			t.Errorf("expected event type %s, but was not found", et)
		}
	}
}

func TestHandleMatchStart(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	t.Run("Valid", func(t *testing.T) {
		evt := svc.HandleMatchStart("match_1", 10*time.Second, 0)
		if evt == nil || evt.MatchID != "match_1" || evt.Type != entities.EventTypeMatchStart || evt.Round != 0 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Invalid (Empty MatchID)", func(t *testing.T) {
		evt := svc.HandleMatchStart("", 10*time.Second, 0)
		if evt != nil {
			t.Errorf("expected nil for invalid event, got: %+v", evt)
		}
	})
}

func TestHandleMatchEnd(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	t.Run("T Wins", func(t *testing.T) {
		evt := svc.HandleMatchEnd("match_1", 20*time.Second, 15, 16, 14)
		if evt == nil || evt.WinnerTeam != "T" || evt.ScoreT != 16 || evt.ScoreCT != 14 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("CT Wins", func(t *testing.T) {
		evt := svc.HandleMatchEnd("match_1", 20*time.Second, 15, 12, 16)
		if evt == nil || evt.WinnerTeam != "CT" || evt.ScoreT != 12 || evt.ScoreCT != 16 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Draw", func(t *testing.T) {
		evt := svc.HandleMatchEnd("match_1", 20*time.Second, 15, 15, 15)
		if evt == nil || evt.WinnerTeam != "Draw" || evt.ScoreT != 15 || evt.ScoreCT != 15 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Invalid (Negative score)", func(t *testing.T) {
		evt := svc.HandleMatchEnd("match_1", 20*time.Second, 15, -1, 15)
		if evt != nil {
			t.Errorf("expected nil for negative score, got: %+v", evt)
		}
	})
}

func TestHandleRoundStart(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	t.Run("Valid", func(t *testing.T) {
		evt := svc.HandleRoundStart("match_1", 30*time.Second, 1)
		if evt == nil || evt.MatchID != "match_1" || evt.Type != entities.EventTypeRoundStart || evt.Round != 1 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Invalid (Negative round)", func(t *testing.T) {
		evt := svc.HandleRoundStart("match_1", 30*time.Second, -1)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})
}

func TestHandleRoundEnd(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	ev := events.RoundEnd{
		Winner: common.TeamTerrorists,
		Reason: 9, // TerroristsWin
	}

	t.Run("Valid T Win", func(t *testing.T) {
		evt := svc.HandleRoundEnd("match_1", 40*time.Second, 2, ev, 2, 0)
		if evt == nil || evt.WinnerTeam != "T" || evt.WinReason != "TerroristsWin" || evt.ScoreT != 2 || evt.ScoreCT != 0 {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Valid CT Win", func(t *testing.T) {
		evCT := events.RoundEnd{
			Winner: common.TeamCounterTerrorists,
			Reason: 8, // CTWin
		}
		evtCT := svc.HandleRoundEnd("match_1", 40*time.Second, 2, evCT, 1, 1)
		if evtCT == nil || evtCT.WinnerTeam != "CT" || evtCT.WinReason != "CTWin" {
			t.Errorf("unexpected event details: %+v", evtCT)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		evt := svc.HandleRoundEnd("match_1", 40*time.Second, -2, ev, 2, 0)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})
}

func TestHandleRoundMVP(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	player := &common.Player{
		SteamID64: 76561198000000001,
		Name:      "MVPPlayer",
	}
	ev := events.RoundMVPAnnouncement{
		Player: player,
		Reason: events.MVPReasonMostEliminations,
	}

	t.Run("Valid", func(t *testing.T) {
		evt := svc.HandleRoundMVP("match_1", 50*time.Second, 3, ev)
		if evt == nil || evt.MVPPlayerID != "76561198000000001" || evt.MVPReason != "MostEliminations" {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Invalid MVP Player ID Too Long", func(t *testing.T) {
		longPlayer := &common.Player{
			SteamID64: 999999999999999999,
			Name:      strings.Repeat("P", 100),
		}
		evInvalid := events.RoundMVPAnnouncement{
			Player: longPlayer,
			Reason: events.MVPReasonMostEliminations,
		}
		evt := svc.HandleRoundMVP("match_1", 50*time.Second, -3, evInvalid)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})
}

func TestHandleKill(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})

	killer := &common.Player{SteamID64: 111, Name: "Killer", Team: common.TeamCounterTerrorists}
	victim := &common.Player{SteamID64: 222, Name: "Victim", Team: common.TeamTerrorists}
	assister := &common.Player{SteamID64: 333, Name: "Assister", Team: common.TeamCounterTerrorists}

	ev := events.Kill{
		Killer:            killer,
		Victim:            victim,
		Assister:          assister,
		IsHeadshot:        true,
		PenetratedObjects: 1,
		ThroughSmoke:      true,
		AssistedFlash:     true,
		AttackerBlind:     true,
		NoScope:           true,
	}

	t.Run("Valid", func(t *testing.T) {
		evt := svc.HandleKill("match_1", 60*time.Second, 4, ev)
		if evt == nil || evt.AttackerID != "111" || evt.VictimID != "222" || evt.AssisterID != "333" ||
			!evt.IsHeadshot || evt.WallbangCount != 1 || !evt.ThruSmoke ||
			!evt.AssistedFlash || !evt.AttackerBlind || !evt.NoScope {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		evInvalid := ev
		evInvalid.PenetratedObjects = -1
		evt := svc.HandleKill("match_1", 60*time.Second, 4, evInvalid)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})
}

func TestHandleBombEvents(t *testing.T) {
	svc := NewParserService(&mockEventRepo{})
	player := &common.Player{SteamID64: 444, Name: "BombGuy"}

	t.Run("Planted - Valid", func(t *testing.T) {
		ev := events.BombPlanted{
			BombEvent: events.BombEvent{
				Player: player,
				Site:   events.BombsiteA,
			},
		}
		evt := svc.HandleBombPlanted("match_1", 70*time.Second, 5, ev)
		if evt == nil || evt.PlanterID != "444" || evt.BombSite != "A" || evt.Type != entities.EventTypeBombPlanted {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Planted - Invalid site", func(t *testing.T) {
		ev := events.BombPlanted{
			BombEvent: events.BombEvent{
				Player: player,
				Site:   events.Bombsite(0),
			},
		}
		evt := svc.HandleBombPlanted("match_1", 70*time.Second, 5, ev)
		if evt == nil || evt.BombSite != "" {
			t.Errorf("expected empty string or nil-like site representation, got: %+v", evt)
		}
	})

	t.Run("Defused - Valid", func(t *testing.T) {
		ev := events.BombDefused{
			BombEvent: events.BombEvent{
				Player: player,
				Site:   events.BombsiteB,
			},
		}
		evt := svc.HandleBombDefused("match_1", 80*time.Second, 5, ev)
		if evt == nil || evt.DefuserID != "444" || evt.BombSite != "B" || evt.Type != entities.EventTypeBombDefused {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Defused - Invalid", func(t *testing.T) {
		ev := events.BombDefused{
			BombEvent: events.BombEvent{
				Player: player,
				Site:   events.BombsiteB,
			},
		}
		evt := svc.HandleBombDefused("match_1", 80*time.Second, -5, ev)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})

	t.Run("Explode - Valid", func(t *testing.T) {
		ev := events.BombExplode{
			BombEvent: events.BombEvent{
				Site: events.BombsiteA,
			},
		}
		evt := svc.HandleBombExplode("match_1", 90*time.Second, 5, ev)
		if evt == nil || evt.BombSite != "A" || evt.Type != entities.EventTypeBombExploded {
			t.Errorf("unexpected event details: %+v", evt)
		}
	})

	t.Run("Explode - Invalid", func(t *testing.T) {
		ev := events.BombExplode{
			BombEvent: events.BombEvent{
				Site: events.BombsiteA,
			},
		}
		evt := svc.HandleBombExplode("match_1", 90*time.Second, -1, ev)
		if evt != nil {
			t.Errorf("expected nil, got: %+v", evt)
		}
	})
}

func TestParserHelperFunctions(t *testing.T) {
	// Test getTeamName
	if getTeamName(common.TeamSpectators) != "Spectators" {
		t.Errorf("expected Spectators, got %s", getTeamName(common.TeamSpectators))
	}
	if getTeamName(common.Team(0)) != "Unassigned" {
		t.Errorf("expected Unassigned, got %s", getTeamName(common.Team(0)))
	}

	// Test parseBombsite
	if parseBombsite(events.Bombsite(100)) != "" {
		t.Errorf("expected empty, got %s", parseBombsite(events.Bombsite(100)))
	}
	if parseBombsite(events.Bombsite('A')) != "A" {
		t.Errorf("expected A, got %s", parseBombsite(events.Bombsite('A')))
	}

	// Test mvpReasonStr
	if mvpReasonStr(events.RoundMVPReason(99)) != "Reason 99" {
		t.Errorf("expected Reason 99, got %s", mvpReasonStr(events.RoundMVPReason(99)))
	}
	if mvpReasonStr(events.MVPReasonBombDefused) != "BombDefused" {
		t.Errorf("expected BombDefused, got %s", mvpReasonStr(events.MVPReasonBombDefused))
	}
	if mvpReasonStr(events.MVPReasonBombPlanted) != "BombPlanted" {
		t.Errorf("expected BombPlanted, got %s", mvpReasonStr(events.MVPReasonBombPlanted))
	}

	// Test roundEndReasonStr
	for i := 1; i <= 18; i++ {
		res := roundEndReasonStr(events.RoundEndReason(i))
		if strings.HasPrefix(res, "Reason ") {
			t.Errorf("expected named reason for %d, got %s", i, res)
		}
	}
	if roundEndReasonStr(events.RoundEndReason(99)) != "Reason 99" {
		t.Errorf("expected Reason 99, got %s", roundEndReasonStr(events.RoundEndReason(99)))
	}
}

func TestParserService_ParseStream_MapAndStats(t *testing.T) {
	repo := &mockEventRepo{}
	svc := NewParserService(repo)

	mp := &mockParser{
		handlers: make(map[string]interface{}),
		currTime: 120 * time.Second,
		mockGS: &mockGameState{
			roundsPlayed: 26,
			ctTeam:       makeTeamStateWithName(16, "Natus Vincere"),
			tTeam:        makeTeamStateWithName(10, "FaZe Clan"),
		},
	}

	svc.newParser = func(r io.Reader) demoinfocs.Parser {
		return mp
	}

	mp.parseNextFrameFunc = func() (bool, error) {
		if !mp.parseCalled {
			mp.parseCalled = true

			// 1. Map name net message
			mapPath := "/workshop/12345/de_dust2"
			mp.Trigger(&msg.CSVCMsg_ServerInfo{
				MapName: &mapPath,
			})

			// 2. Start match
			mp.Trigger(events.MatchStart{})

			// 3. Side switch
			mp.Trigger(events.TeamSideSwitch{})
			mp.mockGS.sidesSwapped = true

			// 4. Half ended
			mp.Trigger(events.GameHalfEnded{})

			// 5. Overtime change
			mp.Trigger(events.OvertimeNumberChanged{NewCount: 1})

			return true, nil
		}
		return false, nil
	}

	ctx := context.Background()
	r := strings.NewReader("dummy stream")
	res, err := svc.ParseStream(ctx, "match_stats", r)
	if err != nil {
		t.Fatalf("ParseStream failed: %v", err)
	}

	if res.MapName != "/workshop/12345/de_dust2" {
		t.Errorf("expected original map name to be '/workshop/12345/de_dust2', got %q", res.MapName)
	}

	if res.TeamA != "Natus Vincere" {
		t.Errorf("expected team A name 'Natus Vincere', got %q", res.TeamA)
	}
	if res.TeamB != "FaZe Clan" {
		t.Errorf("expected team B name 'FaZe Clan', got %q", res.TeamB)
	}
	if res.ScoreA != 16 {
		t.Errorf("expected score A to be 16, got %d", res.ScoreA)
	}
	if res.ScoreB != 10 {
		t.Errorf("expected score B to be 10, got %d", res.ScoreB)
	}
	if res.TotalRounds != 26 {
		t.Errorf("expected total rounds to be 26, got %d", res.TotalRounds)
	}
}

func TestParserService_RichDataCalculations(t *testing.T) {
	repo := &mockEventRepo{}
	var savedEvents []*entities.Event
	repo.saveBatchFunc = func(ctx context.Context, events []*entities.Event) error {
		savedEvents = append(savedEvents, events...)
		return nil
	}

	svc := NewParserService(repo)

	mp := &mockParser{
		handlers: make(map[string]interface{}),
		currTime: 10 * time.Second,
		mockGS: &mockGameState{
			roundsPlayed: 5,
			tTeam:        makeTeamState(3),
			ctTeam:       makeTeamState(2),
		},
	}

	svc.newParser = func(r io.Reader) demoinfocs.Parser {
		return mp
	}

	mp.parseNextFrameFunc = func() (bool, error) {
		if !mp.parseCalled {
			mp.parseCalled = true

			// Trigger RoundStart
			mp.Trigger(events.RoundStart{})

			// Trigger FreezetimeEnd
			mp.Trigger(events.RoundFreezetimeEnd{})

			// Trigger PlayerHurt to start duel/TTK tracking
			mp.Trigger(events.PlayerHurt{
				Attacker:     &common.Player{SteamID64: 111, Team: common.TeamCounterTerrorists},
				Player:       &common.Player{SteamID64: 222, Team: common.TeamTerrorists},
				Weapon:       &common.Equipment{Type: common.EqAK47},
				HealthDamage: 50,
			})

			// 1 second later
			mp.currTime = 11 * time.Second

			// Trigger Kill (this finishes TTK and reaction time tracking)
			mp.Trigger(events.Kill{
				Killer: &common.Player{SteamID64: 111, Name: "Killer", Team: common.TeamCounterTerrorists},
				Victim: &common.Player{SteamID64: 222, Name: "Victim", Team: common.TeamTerrorists},
				Weapon: &common.Equipment{Type: common.EqAK47},
			})

			return true, nil
		}
		return false, nil
	}

	ctx := context.Background()
	_, err := svc.ParseStream(ctx, "match_123", strings.NewReader("dummy"))
	if err != nil {
		t.Fatalf("ParseStream failed: %v", err)
	}

	var killEvt *entities.Event
	for _, e := range savedEvents {
		if e.Type == entities.EventTypeKill {
			killEvt = e
			break
		}
	}

	if killEvt == nil {
		t.Fatal("expected kill event to be captured and saved")
	}

	if killEvt.TTK != 1.0 {
		t.Errorf("expected TTK of 1.0s, got %v", killEvt.TTK)
	}

	if killEvt.ReactionTime != 0.45 {
		t.Errorf("expected ReactionTime of 0.45s, got %v", killEvt.ReactionTime)
	}
}


package entities

import (
	"errors"
	"strings"
	"testing"
)

func TestNewMatchStartEvent(t *testing.T) {
	now := 0.0
	e, err := NewMatchStartEvent("evt_1", "match_123", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.ID != "evt_1" || e.MatchID != "match_123" || e.Type != EventTypeMatchStart {
		t.Errorf("incorrect fields set: %+v", e)
	}

	// Validation checks
	_, err = NewMatchStartEvent("evt_1", "", now)
	if !errors.Is(err, ErrMatchIDRequired) {
		t.Errorf("expected ErrMatchIDRequired, got %v", err)
	}

	_, err = NewMatchStartEvent(strings.Repeat("e", 65), "match_123", now)
	if !errors.Is(err, ErrEventIDTooLong) {
		t.Errorf("expected ErrEventIDTooLong, got %v", err)
	}
}

func TestNewKillEvent(t *testing.T) {
	now := 0.0
	e, err := NewKillEvent(
		"evt_kill", "match_123", 5, now,
		"atk_1", "Attacker", "CT",
		"vic_1", "Victim", "T",
		"ast_1", "Assister", "T",
		"ak47", true, 1,
		false, false, false, false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.Round != 5 || e.Weapon != "ak47" || !e.IsHeadshot || e.WallbangCount != 1 {
		t.Errorf("incorrect kill fields set: %+v", e)
	}

	// Negative checks
	_, err = NewKillEvent(
		"evt_kill", "match_123", -1, now,
		"atk_1", "Attacker", "CT",
		"vic_1", "Victim", "T",
		"", "", "",
		"ak47", true, 0,
		false, false, false, false,
	)
	if !errors.Is(err, ErrNegativeRound) {
		t.Errorf("expected ErrNegativeRound, got %v", err)
	}

	_, err = NewKillEvent(
		"evt_kill", "match_123", 5, now,
		"atk_1", "Attacker", "CT",
		"vic_1", "Victim", "T",
		"", "", "",
		"ak47", true, -2,
		false, false, false, false,
	)
	if !errors.Is(err, ErrNegativeWallbang) {
		t.Errorf("expected ErrNegativeWallbang, got %v", err)
	}
}

func TestNewBombEvent(t *testing.T) {
	now := 0.0
	e, err := NewBombPlantedEvent("evt_bomb", "match_123", 12, now, "A", "planter_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.BombSite != "A" || e.PlanterID != "planter_1" {
		t.Errorf("incorrect bomb fields set: %+v", e)
	}

	// Invalid site
	_, err = NewBombPlantedEvent("evt_bomb", "match_123", 12, now, "C", "planter_1")
	if !errors.Is(err, ErrInvalidBombSite) {
		t.Errorf("expected ErrInvalidBombSite, got %v", err)
	}
}

func TestNewRoundEndEvent(t *testing.T) {
	now := 0.0
	e, err := NewRoundEndEvent("evt_end", "match_123", 15, now, "CT", "bomb_defused", 9, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.WinnerTeam != "CT" || e.WinReason != "bomb_defused" || e.ScoreT != 9 || e.ScoreCT != 6 {
		t.Errorf("incorrect round end fields: %+v", e)
	}

	// Negative scores
	_, err = NewRoundEndEvent("evt_end", "match_123", 15, now, "CT", "bomb_defused", -1, 6)
	if !errors.Is(err, ErrEventNegativeScore) {
		t.Errorf("expected ErrEventNegativeScore, got %v", err)
	}
}

func TestNewMatchEndEvent(t *testing.T) {
	now := 0.0
	e, err := NewMatchEndEvent("evt_mend", "match_123", now, "T", 16, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.WinnerTeam != "T" || e.ScoreT != 16 || e.ScoreCT != 14 {
		t.Errorf("incorrect fields: %+v", e)
	}
}

func TestNewRoundStartEvent(t *testing.T) {
	now := 0.0
	e, err := NewRoundStartEvent("evt_rstart", "match_123", 2, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Round != 2 {
		t.Errorf("incorrect round: %d", e.Round)
	}
}

func TestNewRoundMVPEvent(t *testing.T) {
	now := 0.0
	e, err := NewRoundMVPEvent("evt_mvp", "match_123", 3, now, "player1", "clutch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.MVPPlayerID != "player1" || e.MVPReason != "clutch" {
		t.Errorf("incorrect MVP fields: %+v", e)
	}
}

func TestNewBombDefusedEvent(t *testing.T) {
	now := 0.0
	e, err := NewBombDefusedEvent("evt_defuse", "match_123", 4, now, "B", "player2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.BombSite != "B" || e.DefuserID != "player2" {
		t.Errorf("incorrect bomb defused fields: %+v", e)
	}
}

func TestNewBombExplodedEvent(t *testing.T) {
	now := 0.0
	e, err := NewBombExplodedEvent("evt_explode", "match_123", 5, now, "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.BombSite != "A" {
		t.Errorf("incorrect bomb exploded fields: %+v", e)
	}
}

func TestEventValidationLimits(t *testing.T) {
	now := 0.0
	// Invalid type
	evt := &Event{Type: "invalid_type", MatchID: "match1", ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrInvalidEventType) {
		t.Errorf("expected ErrInvalidEventType, got %v", err)
	}

	// Match ID too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: strings.Repeat("m", 65), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrMatchIDTooLong) {
		t.Errorf("expected ErrMatchIDTooLong, got %v", err)
	}

	// Player IDs too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", AttackerID: strings.Repeat("p", 65), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrPlayerIDTooLong) {
		t.Errorf("expected ErrPlayerIDTooLong, got %v", err)
	}

	// Player names too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", AttackerName: strings.Repeat("n", 101), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrPlayerNameTooLong) {
		t.Errorf("expected ErrPlayerNameTooLong, got %v", err)
	}

	// Team names too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", AttackerTeam: strings.Repeat("t", 51), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrEventTeamNameTooLong) {
		t.Errorf("expected ErrEventTeamNameTooLong, got %v", err)
	}

	// Weapon name too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", Weapon: strings.Repeat("w", 51), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrWeaponTooLong) {
		t.Errorf("expected ErrWeaponTooLong, got %v", err)
	}

	// Win reason too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", WinReason: strings.Repeat("r", 101), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrWinReasonTooLong) {
		t.Errorf("expected ErrWinReasonTooLong, got %v", err)
	}

	// MVP reason too long
	evt = &Event{Type: EventTypeMatchStart, MatchID: "match1", MVPReason: strings.Repeat("r", 101), ElapsedSeconds: now}
	if err := evt.Valid(); !errors.Is(err, ErrMVPReasonTooLong) {
		t.Errorf("expected ErrMVPReasonTooLong, got %v", err)
	}
}


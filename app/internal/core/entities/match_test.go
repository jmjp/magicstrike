package entities

import (
	"errors"
	"strings"
	"testing"
)

func intPtr(val int) *int {
	return &val
}

func mapNamePtr(val MapName) *MapName {
	return &val
}

func TestNewMatch(t *testing.T) {
	validMD5 := "098f6bcd4621d373cade4e832627b4f6" // "test" md5

	tests := []struct {
		name    string
		userID  string
		teamA   *string
		teamB   *string
		demoMD5 *string
		mapName *MapName
		wantErr error
	}{
		{
			name:    "valid match creation",
			userID:  "user1",
			teamA:   ptr("NaVi"),
			teamB:   ptr("Vitality"),
			demoMD5: &validMD5,
			mapName: mapNamePtr(MapNameDust2),
			wantErr: nil,
		},
		{
			name:    "valid match creation with nil map name",
			userID:  "user1",
			teamA:   ptr("NaVi"),
			teamB:   ptr("Vitality"),
			demoMD5: &validMD5,
			mapName: nil,
			wantErr: nil,
		},
		{
			name:    "nil team A",
			userID:  "user1",
			teamA:   nil,
			teamB:   ptr("Vitality"),
			demoMD5: &validMD5,
			mapName: mapNamePtr(MapNameDust2),
			wantErr: nil,
		},
		{
			name:    "nil team A, team B, and map name",
			userID:  "user1",
			teamA:   nil,
			teamB:   nil,
			demoMD5: &validMD5,
			mapName: nil,
			wantErr: nil,
		},
		{
			name:    "same teams",
			userID:  "user1",
			teamA:   ptr("NaVi"),
			teamB:   ptr("NaVi"),
			demoMD5: &validMD5,
			mapName: mapNamePtr(MapNameDust2),
			wantErr: ErrTeamsCannotBeSame,
		},
		{
			name:    "invalid md5 hash",
			userID:  "user1",
			teamA:   ptr("NaVi"),
			teamB:   ptr("Vitality"),
			demoMD5: ptr("invalid-md5-hash"),
			mapName: mapNamePtr(MapNameDust2),
			wantErr: ErrInvalidDemoMD5,
		},
		{
			name:    "invalid map name",
			userID:  "user1",
			teamA:   ptr("NaVi"),
			teamB:   ptr("Vitality"),
			demoMD5: &validMD5,
			mapName: mapNamePtr(MapName("invalid_map")),
			wantErr: ErrInvalidMapName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMatch(tt.userID, tt.teamA, tt.teamB, tt.demoMD5, tt.mapName)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewMatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil && got == nil {
				t.Error("NewMatch() returned nil for valid inputs")
			}
		})
	}
}

func TestMatch_TransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		initial   MatchStatus
		target    MatchStatus
		expectErr bool
	}{
		{
			name:      "waiting to started is valid",
			initial:   MatchStatusWaiting,
			target:    MatchStatusStarted,
			expectErr: false,
		},
		{
			name:      "waiting to aborted is valid",
			initial:   MatchStatusWaiting,
			target:    MatchStatusAborted,
			expectErr: false,
		},
		{
			name:      "started to finished is valid",
			initial:   MatchStatusStarted,
			target:    MatchStatusFinished,
			expectErr: false,
		},
		{
			name:      "started to aborted is invalid",
			initial:   MatchStatusStarted,
			target:    MatchStatusAborted,
			expectErr: true,
		},
		{
			name:      "finished to started is invalid",
			initial:   MatchStatusFinished,
			target:    MatchStatusStarted,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &Match{
				Status: tt.initial,
				TeamA:  ptr("NaVi"),
				TeamB:  ptr("Vitality"),
			}
			err := match.TransitionTo(tt.target)
			if (err != nil) != tt.expectErr {
				t.Errorf("TransitionTo() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestMatch_WinnerLoser(t *testing.T) {
	match := &Match{
		TeamA: ptr("NaVi"),
		TeamB: ptr("Vitality"),
	}

	// Scores missing
	if w := match.Winner(); w != nil {
		t.Errorf("expected winner to be nil when scores are missing, got %s", *w)
	}
	if l := match.Loser(); l != nil {
		t.Errorf("expected loser to be nil when scores are missing, got %s", *l)
	}

	match.ScoreA = intPtr(16)
	match.ScoreB = intPtr(14)

	// Team A wins
	if w := match.Winner(); w == nil || *w != "NaVi" {
		t.Errorf("expected winner to be NaVi, got %v", w)
	}
	if l := match.Loser(); l == nil || *l != "Vitality" {
		t.Errorf("expected loser to be Vitality, got %v", l)
	}

	// Team B wins
	match.ScoreA = intPtr(10)
	match.ScoreB = intPtr(16)
	if w := match.Winner(); w == nil || *w != "Vitality" {
		t.Errorf("expected winner to be Vitality, got %v", w)
	}
	if l := match.Loser(); l == nil || *l != "NaVi" {
		t.Errorf("expected loser to be NaVi, got %v", l)
	}

	// Tie/Draw
	match.ScoreB = intPtr(10)
	if w := match.Winner(); w != nil {
		t.Errorf("expected winner to be nil on tie, got %s", *w)
	}
	if l := match.Loser(); l != nil {
		t.Errorf("expected loser to be nil on tie, got %s", *l)
	}
}

func TestMatch_Valid(t *testing.T) {
	// 1. Invalid status
	match := &Match{
		UserID: "user1",
		Status: MatchStatus("invalid-status"),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrInvalidMatchStatus) {
		t.Errorf("expected ErrInvalidMatchStatus, got %v", err)
	}

	// 2. Empty UserID
	match = &Match{
		UserID: "",
		Status: MatchStatusWaiting,
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrUserIDRequired) {
		t.Errorf("expected ErrUserIDRequired, got %v", err)
	}

	// 3. Status Finished validation
	// 3a. TeamA nil/empty
	match = &Match{
		UserID: "user1",
		Status: MatchStatusFinished,
		TeamB:  ptr("Vitality"),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamARequired) {
		t.Errorf("expected ErrTeamARequired, got %v", err)
	}
	match.TeamA = ptr("")
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamARequired) {
		t.Errorf("expected ErrTeamARequired, got %v", err)
	}
	match.TeamA = ptr(strings.Repeat("a", 101))
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamARequired) {
		t.Errorf("expected ErrTeamARequired, got %v", err)
	}

	// 3b. TeamB nil/empty
	match = &Match{
		UserID: "user1",
		Status: MatchStatusFinished,
		TeamA:  ptr("NaVi"),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamBRequired) {
		t.Errorf("expected ErrTeamBRequired, got %v", err)
	}
	match.TeamB = ptr("")
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamBRequired) {
		t.Errorf("expected ErrTeamBRequired, got %v", err)
	}
	match.TeamB = ptr(strings.Repeat("b", 101))
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamBRequired) {
		t.Errorf("expected ErrTeamBRequired, got %v", err)
	}

	// 3c. Same teams
	match = &Match{
		UserID: "user1",
		Status: MatchStatusFinished,
		TeamA:  ptr("NaVi"),
		TeamB:  ptr("NaVi"),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamsCannotBeSame) {
		t.Errorf("expected ErrTeamsCannotBeSame, got %v", err)
	}

	// 3d. Started status does not require teams
	match = &Match{
		UserID: "user1",
		Status: MatchStatusStarted,
		TeamA:  nil,
		TeamB:  nil,
	}
	if err := match.Valid(); err != nil {
		t.Errorf("expected no error for Started status without teams, got %v", err)
	}

	// 4. Other status (e.g. Waiting) team validation
	// 4a. Team A too long
	match = &Match{
		UserID: "user1",
		Status: MatchStatusWaiting,
		TeamA:  ptr(strings.Repeat("a", 101)),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamARequired) {
		t.Errorf("expected ErrTeamARequired, got %v", err)
	}

	// 4b. Team B too long
	match = &Match{
		UserID: "user1",
		Status: MatchStatusWaiting,
		TeamB:  ptr(strings.Repeat("b", 101)),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamBRequired) {
		t.Errorf("expected ErrTeamBRequired, got %v", err)
	}

	// 4c. Same teams in Waiting status
	match = &Match{
		UserID: "user1",
		Status: MatchStatusWaiting,
		TeamA:  ptr("NaVi"),
		TeamB:  ptr("NaVi"),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrTeamsCannotBeSame) {
		t.Errorf("expected ErrTeamsCannotBeSame, got %v", err)
	}

	// 5. Negative values
	match = &Match{
		UserID: "user1",
		Status: MatchStatusWaiting,
		ScoreA: intPtr(-1),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrNegativeScore) {
		t.Errorf("expected ErrNegativeScore, got %v", err)
	}

	match = &Match{
		UserID: "user1",
		Status: MatchStatusWaiting,
		ScoreB: intPtr(-1),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrNegativeScore) {
		t.Errorf("expected ErrNegativeScore, got %v", err)
	}

	match = &Match{
		UserID:      "user1",
		Status:      MatchStatusWaiting,
		TotalRounds: intPtr(-1),
	}
	if err := match.Valid(); err == nil || !errors.Is(err, ErrNegativeTotalRounds) {
		t.Errorf("expected ErrNegativeTotalRounds, got %v", err)
	}
}

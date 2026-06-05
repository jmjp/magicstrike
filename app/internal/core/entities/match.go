package entities

import (
	"errors"
	"regexp"
	"time"

	"github.com/oklog/ulid/v2"
)

type MatchStatus string

const (
	MatchStatusWaiting  MatchStatus = "waiting"
	MatchStatusStarted  MatchStatus = "started"
	MatchStatusFinished MatchStatus = "finished"
	MatchStatusAborted  MatchStatus = "aborted"
	MatchStatusFailed   MatchStatus = "failed"
)

type MapName string

const (
	MapNameDust2       MapName = "de_dust2"
	MapNameInferno     MapName = "de_inferno"
	MapNameNuke        MapName = "de_nuke"
	MapNameMirage      MapName = "de_mirage"
	MapNameOverpass    MapName = "de_overpass"
	MapNameVertigo     MapName = "de_vertigo"
	MapNameAncient     MapName = "de_ancient"
	MapNameTrain       MapName = "de_train"
	MapNameCobblestone MapName = "de_cobblestone"
	MapNameCache       MapName = "de_cache"
	MapNameAnubis      MapName = "de_anubis"
	MapNameItaly       MapName = "de_italy"
)

var (
	ErrInvalidMatchStatus  = errors.New("invalid match status")
	ErrInvalidTransition   = errors.New("invalid match status transition")
	ErrTeamARequired       = errors.New("team A is required")
	ErrTeamBRequired       = errors.New("team B is required")
	ErrTeamsCannotBeSame   = errors.New("team A and team B cannot be the same")
	ErrNegativeScore       = errors.New("scores cannot be negative")
	ErrNegativeTotalRounds = errors.New("total rounds cannot be negative")
	ErrInvalidDemoMD5      = errors.New("invalid demo MD5 hash format")
	ErrInvalidMapName      = errors.New("invalid map name")
)

const (
	maxTeamNameLen = 100
	maxDemoMD5Len  = 32
)

var md5Regex = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

type Match struct {
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	Status      MatchStatus `json:"status"`
	TeamA       *string     `json:"team_a,omitempty"`
	TeamB       *string     `json:"team_b,omitempty"`
	DemoMD5     *string     `json:"demo_md5,omitempty"`
	ScoreA      *int        `json:"score_a,omitempty"`
	ScoreB      *int        `json:"score_b,omitempty"`
	TotalRounds *int        `json:"total_rounds,omitempty"`
	MapName     *MapName    `json:"map_name"`
}

// NewMatch creates a new Match entity with ULID generation.
// userID identifies the owner of this match (set from JWT claims).
func NewMatch(userID string, teamA, teamB *string, demoMD5 *string, mapName *MapName) (*Match, error) {
	match := &Match{
		ID:        ulid.Make().String(),
		UserID:    userID,
		Status:    MatchStatusWaiting,
		TeamA:     teamA,
		TeamB:     teamB,
		DemoMD5:   demoMD5,
		MapName:   mapName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := match.Valid(); err != nil {
		return nil, err
	}

	return match, nil
}

// TransitionTo attempts to change the match status respecting transition constraints.
func (m *Match) TransitionTo(newStatus MatchStatus) error {
	if !m.CanTransitionTo(newStatus) {
		return ErrInvalidTransition
	}
	m.Status = newStatus
	m.UpdatedAt = time.Now()
	return nil
}

// CanTransitionTo checks if a transition to newStatus is allowed.
func (m *Match) CanTransitionTo(newStatus MatchStatus) bool {
	switch m.Status {
	case MatchStatusWaiting:
		return newStatus == MatchStatusStarted || newStatus == MatchStatusAborted || newStatus == MatchStatusFailed
	case MatchStatusStarted:
		// Cannot transition to MatchStatusAborted once started
		return newStatus == MatchStatusFinished || newStatus == MatchStatusFailed
	default:
		// Finished, aborted, and failed are terminal states
		return false
	}
}

// Winner returns a pointer to the winning team name, or nil if there's a tie or scores are missing.
func (m *Match) Winner() *string {
	if m.ScoreA == nil || m.ScoreB == nil {
		return nil
	}
	if *m.ScoreA > *m.ScoreB {
		return m.TeamA
	}
	if *m.ScoreB > *m.ScoreA {
		return m.TeamB
	}
	return nil
}

// Loser returns a pointer to the losing team name, or nil if there's a tie or scores are missing.
func (m *Match) Loser() *string {
	if m.ScoreA == nil || m.ScoreB == nil {
		return nil
	}
	if *m.ScoreA < *m.ScoreB {
		return m.TeamA
	}
	if *m.ScoreB < *m.ScoreA {
		return m.TeamB
	}
	return nil
}

// Valid checks if the match entity fields satisfy constraints.
func (m *Match) Valid() error {
	switch m.Status {
	case MatchStatusWaiting, MatchStatusStarted, MatchStatusFinished, MatchStatusAborted, MatchStatusFailed:
	default:
		return ErrInvalidMatchStatus
	}

	if m.UserID == "" {
		return ErrUserIDRequired
	}
	if m.Status == MatchStatusFinished {
		if m.TeamA == nil || *m.TeamA == "" || len(*m.TeamA) > maxTeamNameLen {
			return ErrTeamARequired
		}
		if m.TeamB == nil || *m.TeamB == "" || len(*m.TeamB) > maxTeamNameLen {
			return ErrTeamBRequired
		}
		if *m.TeamA == *m.TeamB {
			return ErrTeamsCannotBeSame
		}
	} else {
		if m.TeamA != nil && *m.TeamA != "" && len(*m.TeamA) > maxTeamNameLen {
			return ErrTeamARequired
		}
		if m.TeamB != nil && *m.TeamB != "" && len(*m.TeamB) > maxTeamNameLen {
			return ErrTeamBRequired
		}
		if m.TeamA != nil && m.TeamB != nil && *m.TeamA != "" && *m.TeamB != "" && *m.TeamA == *m.TeamB {
			return ErrTeamsCannotBeSame
		}
	}

	if m.DemoMD5 != nil && !md5Regex.MatchString(*m.DemoMD5) {
		return ErrInvalidDemoMD5
	}

	if m.MapName != nil {
		switch *m.MapName {
		case MapNameDust2, MapNameInferno, MapNameNuke, MapNameMirage, MapNameOverpass,
			MapNameVertigo, MapNameAncient, MapNameTrain, MapNameCobblestone, MapNameCache,
			MapNameAnubis, MapNameItaly:
		default:
			return ErrInvalidMapName
		}
	}

	if m.ScoreA != nil && *m.ScoreA < 0 {
		return ErrNegativeScore
	}
	if m.ScoreB != nil && *m.ScoreB < 0 {
		return ErrNegativeScore
	}
	if m.TotalRounds != nil && *m.TotalRounds < 0 {
		return ErrNegativeTotalRounds
	}

	return nil
}

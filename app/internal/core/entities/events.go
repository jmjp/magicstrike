package entities

import (
	"errors"
	"time"
)

type EventType string

const (
	EventTypeMatchStart   EventType = "match_start"
	EventTypeMatchEnd     EventType = "match_end"
	EventTypeRoundStart   EventType = "round_start"
	EventTypeRoundEnd     EventType = "round_end"
	EventTypeRoundMVP     EventType = "round_mvp"
	EventTypeBombPlanted  EventType = "bomb_planted"
	EventTypeBombExploded EventType = "bomb_exploded"
	EventTypeBombDefused  EventType = "bomb_defused"
	EventTypeKill         EventType = "kill"
)

var (
	ErrInvalidEventType     = errors.New("invalid event type")
	ErrNegativeRound        = errors.New("round cannot be negative")
	ErrEventNegativeScore   = errors.New("score cannot be negative")
	ErrNegativeWallbang     = errors.New("wallbang count cannot be negative")
	ErrMatchIDRequired      = errors.New("match ID is required")
	ErrMatchIDTooLong       = errors.New("match ID is too long")
	ErrEventIDTooLong       = errors.New("event ID is too long")
	ErrWeaponTooLong        = errors.New("weapon name is too long")
	ErrPlayerNameTooLong    = errors.New("player name is too long")
	ErrPlayerIDTooLong      = errors.New("player ID is too long")
	ErrEventTeamNameTooLong = errors.New("team name is too long")
	ErrInvalidBombSite      = errors.New("invalid bomb site, must be A or B")
	ErrWinReasonTooLong     = errors.New("win reason is too long")
	ErrMVPReasonTooLong     = errors.New("MVP reason is too long")
)

const (
	maxEventIDLen       = 64
	maxMatchIDLen       = 64
	maxPlayerIDLen      = 64
	maxPlayerNameLen    = 100
	maxEventTeamNameLen = 50
	maxWeaponNameLen    = 50
	maxReasonLen        = 100
)

// Event represents a parsed game demo event optimized for memory alignment (grouped by field size).
type Event struct {
	// --- Floats (8 bytes) e time.Time (24 bytes) ---
	ElapsedSeconds        float64   `json:"elapsed_seconds"`
	AttackerPosX          float64   `json:"attacker_pos_x,omitempty"`
	AttackerPosY          float64   `json:"attacker_pos_y,omitempty"`
	AttackerPosZ          float64   `json:"attacker_pos_z,omitempty"`
	VictimPosX            float64   `json:"victim_pos_x,omitempty"`
	VictimPosY            float64   `json:"victim_pos_y,omitempty"`
	VictimPosZ            float64   `json:"victim_pos_z,omitempty"`
	RotationTime          float64   `json:"rotation_time,omitempty"`
	DistanceTraveled      float64   `json:"distance_traveled,omitempty"`
	ViewAngleX            float64   `json:"view_angle_x,omitempty"`
	ViewAngleY            float64   `json:"view_angle_y,omitempty"`
	TimeRemaining         float64   `json:"time_remaining,omitempty"`
	PlantTime             float64   `json:"plant_time,omitempty"`
	DefuseTime            float64   `json:"defuse_time,omitempty"`
	RoundDuration         float64   `json:"round_duration,omitempty"`
	TradeTime             float64   `json:"trade_time,omitempty"`
	TeammateDistance      float64   `json:"teammate_distance,omitempty"`
	ShotDistance          float64   `json:"shot_distance,omitempty"`
	GeneralAccuracy       float64   `json:"general_accuracy,omitempty"`
	TTK                   float64   `json:"ttk,omitempty"`
	ReactionTime          float64   `json:"reaction_time,omitempty"`
	GrenadeEffectDuration float64   `json:"grenade_effect_duration,omitempty"`
	CreatedAt             time.Time `json:"created_at"`

	// --- Strings e LowCardinality Enums (16 bytes na stack) ---
	ID           string    `json:"id"`
	MatchID      string    `json:"match_id"`
	Type         EventType `json:"type"`
	AttackerID   string    `json:"attacker_id,omitempty"`
	AttackerName string    `json:"attacker_name,omitempty"`
	AttackerTeam string    `json:"attacker_team,omitempty"`
	VictimID     string    `json:"victim_id,omitempty"`
	VictimName   string    `json:"victim_name,omitempty"`
	VictimTeam   string    `json:"victim_team,omitempty"`
	AssisterID   string    `json:"assister_id,omitempty"`
	AssisterName string    `json:"assister_name,omitempty"`
	AssisterTeam string    `json:"assister_team,omitempty"`
	Weapon       string    `json:"weapon,omitempty"`
	BombSite     string    `json:"bomb_site,omitempty"`
	PlanterID    string    `json:"planter_id,omitempty"`
	DefuserID    string    `json:"defuser_id,omitempty"`
	WinnerTeam   string    `json:"winner_team,omitempty"`
	WinReason    string    `json:"win_reason,omitempty"`
	MVPPlayerID  string    `json:"mvp_player_id,omitempty"`
	MVPReason    string    `json:"mvp_reason,omitempty"`
	BuyType      string    `json:"buy_type,omitempty"`
	FiringType   string    `json:"firing_type,omitempty"`

	// --- Inteiros (8 bytes / 4 bytes) ---
	Round              int `json:"round"`
	WallbangCount      int `json:"wallbang_count,omitempty"`
	ScoreT             int `json:"score_t,omitempty"`
	ScoreCT            int `json:"score_ct,omitempty"`
	MoneyStart         int `json:"money_start,omitempty"`
	UtilitySpent       int `json:"utility_spent,omitempty"`
	MoneyRemaining     int `json:"money_remaining,omitempty"`
	GrenadeDamage      int `json:"grenade_damage,omitempty"`
	BulletsFired       int `json:"bullets_fired,omitempty"`
	ClutchOpponents    int `json:"clutch_opponents,omitempty"`
	ClutchHpStart      int `json:"clutch_hp_start,omitempty"`
	ClutchUtilityCount int `json:"clutch_utility_count,omitempty"`
	AliveTAtPlant      int `json:"alive_t_at_plant,omitempty"`
	AliveCtAtPlant     int `json:"alive_ct_at_plant,omitempty"`

	// --- Booleans (1 byte) ---
	IsHeadshot    bool `json:"is_headshot,omitempty"`
	ThruSmoke     bool `json:"thru_smoke,omitempty"`
	AssistedFlash bool `json:"assisted_flash,omitempty"`
	AttackerBlind bool `json:"attacker_blind,omitempty"`
	NoScope       bool `json:"no_scope,omitempty"`
	IsTrade       bool `json:"is_trade,omitempty"`
	HasDefuseKit  bool `json:"has_defuse_kit,omitempty"`
}

// NewMatchStartEvent creates a base match start event.
func NewMatchStartEvent(id, matchID string, elapsedSeconds float64) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeMatchStart,
		ElapsedSeconds: elapsedSeconds,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewMatchEndEvent creates a match end event.
func NewMatchEndEvent(id, matchID string, elapsedSeconds float64, winnerTeam string, scoreT, scoreCT int) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeMatchEnd,
		ElapsedSeconds: elapsedSeconds,
		WinnerTeam:     winnerTeam,
		ScoreT:         scoreT,
		ScoreCT:        scoreCT,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewRoundStartEvent creates a round start event.
func NewRoundStartEvent(id, matchID string, round int, elapsedSeconds float64) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeRoundStart,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewRoundEndEvent creates a round end event with score updates.
func NewRoundEndEvent(id, matchID string, round int, elapsedSeconds float64, winnerTeam, winReason string, scoreT, scoreCT int) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeRoundEnd,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		WinnerTeam:     winnerTeam,
		WinReason:      winReason,
		ScoreT:         scoreT,
		ScoreCT:        scoreCT,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewRoundMVPEvent creates a round MVP reward event.
func NewRoundMVPEvent(id, matchID string, round int, elapsedSeconds float64, mvpPlayerID, mvpReason string) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeRoundMVP,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		MVPPlayerID:    mvpPlayerID,
		MVPReason:      mvpReason,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewKillEvent creates a detailed player kill event.
func NewKillEvent(id, matchID string, round int, elapsedSeconds float64,
	attackerID, attackerName, attackerTeam,
	victimID, victimName, victimTeam,
	assisterID, assisterName, assisterTeam,
	weapon string, isHeadshot bool, wallbangCount int,
	thruSmoke, assistedFlash, attackerBlind, noScope bool) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeKill,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		AttackerID:     attackerID,
		AttackerName:   attackerName,
		AttackerTeam:   attackerTeam,
		VictimID:       victimID,
		VictimName:     victimName,
		VictimTeam:     victimTeam,
		AssisterID:     assisterID,
		AssisterName:   assisterName,
		AssisterTeam:   assisterTeam,
		Weapon:         weapon,
		IsHeadshot:     isHeadshot,
		WallbangCount:  wallbangCount,
		ThruSmoke:      thruSmoke,
		AssistedFlash:  assistedFlash,
		AttackerBlind:  attackerBlind,
		NoScope:        noScope,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewBombPlantedEvent creates a bomb planted event.
func NewBombPlantedEvent(id, matchID string, round int, elapsedSeconds float64, bombSite, planterID string) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeBombPlanted,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		BombSite:       bombSite,
		PlanterID:      planterID,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewBombDefusedEvent creates a bomb defused event.
func NewBombDefusedEvent(id, matchID string, round int, elapsedSeconds float64, bombSite, defuserID string) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeBombDefused,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		BombSite:       bombSite,
		DefuserID:      defuserID,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// NewBombExplodedEvent creates a bomb exploded event.
func NewBombExplodedEvent(id, matchID string, round int, elapsedSeconds float64, bombSite string) (*Event, error) {
	e := &Event{
		ID:             id,
		MatchID:        matchID,
		Type:           EventTypeBombExploded,
		Round:          round,
		ElapsedSeconds: elapsedSeconds,
		BombSite:       bombSite,
		CreatedAt:      time.Now(),
	}
	if err := e.Valid(); err != nil {
		return nil, err
	}
	return e, nil
}

// Valid validates constraints across common and specific fields.
func (e *Event) Valid() error {
	switch e.Type {
	case EventTypeMatchStart, EventTypeMatchEnd, EventTypeRoundStart, EventTypeRoundEnd,
		EventTypeRoundMVP, EventTypeBombPlanted, EventTypeBombExploded, EventTypeBombDefused, EventTypeKill:
	default:
		return ErrInvalidEventType
	}

	if len(e.ID) > maxEventIDLen {
		return ErrEventIDTooLong
	}
	if e.MatchID == "" {
		return ErrMatchIDRequired
	}
	if len(e.MatchID) > maxMatchIDLen {
		return ErrMatchIDTooLong
	}

	if e.Round < 0 {
		return ErrNegativeRound
	}
	if e.WallbangCount < 0 {
		return ErrNegativeWallbang
	}
	if e.ScoreT < 0 || e.ScoreCT < 0 {
		return ErrEventNegativeScore
	}

	// Validate Player and Team names
	if len(e.AttackerID) > maxPlayerIDLen || len(e.VictimID) > maxPlayerIDLen || len(e.AssisterID) > maxPlayerIDLen ||
		len(e.PlanterID) > maxPlayerIDLen || len(e.DefuserID) > maxPlayerIDLen || len(e.MVPPlayerID) > maxPlayerIDLen {
		return ErrPlayerIDTooLong
	}

	if len(e.AttackerName) > maxPlayerNameLen || len(e.VictimName) > maxPlayerNameLen || len(e.AssisterName) > maxPlayerNameLen {
		return ErrPlayerNameTooLong
	}

	if len(e.AttackerTeam) > maxEventTeamNameLen || len(e.VictimTeam) > maxEventTeamNameLen || len(e.AssisterTeam) > maxEventTeamNameLen || len(e.WinnerTeam) > maxEventTeamNameLen {
		return ErrEventTeamNameTooLong
	}

	if len(e.Weapon) > maxWeaponNameLen {
		return ErrWeaponTooLong
	}

	if len(e.WinReason) > maxReasonLen {
		return ErrWinReasonTooLong
	}
	if len(e.MVPReason) > maxReasonLen {
		return ErrMVPReasonTooLong
	}

	if e.BombSite != "" && e.BombSite != "A" && e.BombSite != "B" {
		return ErrInvalidBombSite
	}

	return nil
}

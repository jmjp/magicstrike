package services

import (
	"sort"

	"magicstrike/internal/core/entities"
)

// AggregatedRow holds a label-value pair for a single aggregation result.
type AggregatedRow struct {
	Label string
	Value int
}

// EventAggregator provides safe, in-memory aggregation operations over game events.
// It avoids SQL generation — all filtering and grouping is done in Go.
type EventAggregator struct{}

// NewEventAggregator creates a new EventAggregator.
func NewEventAggregator() *EventAggregator {
	return &EventAggregator{}
}

// TopPlayersByMetric returns the top N players ranked by the occurrence of a
// specific boolean metric or event type across all events.
// Supported metrics: thru_smoke, is_headshot, assisted_flash, attacker_blind, no_scope, kills, bomb_planted, bomb_defused, bomb_exploded, wallbang_count
func (ea *EventAggregator) TopPlayersByMetric(events []*entities.Event, metric string, limit int) []AggregatedRow {
	counts := make(map[string]int)

	for _, ev := range events {
		name, matched := ea.matchPlayerMetric(ev, metric)
		if matched && name != "" {
			counts[name]++
		}
	}

	return rankAndLimit(counts, limit)
}

// PlayerStat returns the total count of a metric for a specific player.
func (ea *EventAggregator) PlayerStat(events []*entities.Event, metric, playerName string) int {
	count := 0
	for _, ev := range events {
		name, matched := ea.matchPlayerMetric(ev, metric)
		if matched && equalsFold(name, playerName) {
			count++
		}
	}
	return count
}

// RoundAggregate returns rounds ranked by the occurrence of a metric.
func (ea *EventAggregator) RoundAggregate(events []*entities.Event, metric string, limit int) []AggregatedRow {
	counts := make(map[int]int)
	for _, ev := range events {
		if ev.Round <= 0 {
			continue
		}
		if ea.matchRoundMetric(ev, metric) {
			counts[ev.Round]++
		}
	}

	var rows []AggregatedRow
	for r, c := range counts {
		rows = append(rows, AggregatedRow{Label: formatRound(r), Value: c})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Value > rows[j].Value })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// MatchSummary returns a high-level summary of the match.
func (ea *EventAggregator) MatchSummary(events []*entities.Event) map[string]int {
	summary := map[string]int{
		"total_kills":       0,
		"total_headshots":   0,
		"total_smokes":      0,
		"total_bomb_plants": 0,
		"total_bomb_defuses": 0,
		"total_bomb_explodes": 0,
		"total_rounds":      0,
	}
	rounds := make(map[int]bool)
	for _, ev := range events {
		switch ev.Type {
		case entities.EventTypeKill:
			summary["total_kills"]++
			if ev.IsHeadshot {
				summary["total_headshots"]++
			}
			if ev.ThruSmoke {
				summary["total_smokes"]++
			}
		case entities.EventTypeBombPlanted:
			summary["total_bomb_plants"]++
		case entities.EventTypeBombDefused:
			summary["total_bomb_defuses"]++
		case entities.EventTypeBombExploded:
			summary["total_bomb_explodes"]++
		}
		if ev.Round > 0 {
			rounds[ev.Round] = true
		}
	}
	summary["total_rounds"] = len(rounds)
	return summary
}

// matchPlayerMetric checks if an event matches a metric and returns the associated player name.
func (ea *EventAggregator) matchPlayerMetric(ev *entities.Event, metric string) (string, bool) {
	switch metric {
	case "kills":
		if ev.Type == entities.EventTypeKill {
			return ev.AttackerName, true
		}
	case "thru_smoke":
		if ev.Type == entities.EventTypeKill && ev.ThruSmoke {
			return ev.AttackerName, true
		}
	case "is_headshot":
		if ev.Type == entities.EventTypeKill && ev.IsHeadshot {
			return ev.AttackerName, true
		}
	case "assisted_flash":
		if ev.Type == entities.EventTypeKill && ev.AssistedFlash {
			return ev.AttackerName, true
		}
	case "attacker_blind":
		if ev.Type == entities.EventTypeKill && ev.AttackerBlind {
			return ev.AttackerName, true
		}
	case "no_scope":
		if ev.Type == entities.EventTypeKill && ev.NoScope {
			return ev.AttackerName, true
		}
	case "wallbang_count":
		if ev.Type == entities.EventTypeKill && ev.WallbangCount > 0 {
			return ev.AttackerName, true
		}
	case "bomb_planted":
		if ev.Type == entities.EventTypeBombPlanted {
			return ev.PlanterID, true
		}
	case "bomb_defused":
		if ev.Type == entities.EventTypeBombDefused {
			return ev.DefuserID, true
		}
	case "bomb_exploded":
		if ev.Type == entities.EventTypeBombExploded {
			return "Bomb", true
		}
	}
	return "", false
}

// matchRoundMetric checks if an event matches a metric regardless of player.
func (ea *EventAggregator) matchRoundMetric(ev *entities.Event, metric string) bool {
	switch metric {
	case "kills":
		return ev.Type == entities.EventTypeKill
	case "thru_smoke":
		return ev.Type == entities.EventTypeKill && ev.ThruSmoke
	case "is_headshot":
		return ev.Type == entities.EventTypeKill && ev.IsHeadshot
	case "assisted_flash":
		return ev.Type == entities.EventTypeKill && ev.AssistedFlash
	case "attacker_blind":
		return ev.Type == entities.EventTypeKill && ev.AttackerBlind
	case "no_scope":
		return ev.Type == entities.EventTypeKill && ev.NoScope
	case "wallbang_count":
		return ev.Type == entities.EventTypeKill && ev.WallbangCount > 0
	case "bomb_planted":
		return ev.Type == entities.EventTypeBombPlanted
	case "bomb_defused":
		return ev.Type == entities.EventTypeBombDefused
	case "bomb_exploded":
		return ev.Type == entities.EventTypeBombExploded
	}
	return false
}

func rankAndLimit(counts map[string]int, limit int) []AggregatedRow {
	var rows []AggregatedRow
	for k, v := range counts {
		rows = append(rows, AggregatedRow{Label: k, Value: v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Value > rows[j].Value })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func equalsFold(a, b string) bool {
	// Simple case-insensitive comparison
	return len(a) == len(b) && foldRune(a) == foldRune(b)
}

func foldRune(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func formatRound(r int) string {
	return "Round " + itoa(r)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

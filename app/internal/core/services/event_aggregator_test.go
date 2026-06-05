package services

import (
	"testing"

	"magicstrike/internal/core/entities"
)

func TestEventAggregator_TopPlayersByMetric(t *testing.T) {
	ea := NewEventAggregator()

	events := []*entities.Event{
		{Type: entities.EventTypeKill, AttackerName: "Fallen", IsHeadshot: true, ThruSmoke: true, AssistedFlash: true, AttackerBlind: true, NoScope: true, WallbangCount: 1},
		{Type: entities.EventTypeKill, AttackerName: "Fallen", IsHeadshot: true},
		{Type: entities.EventTypeKill, AttackerName: "coldzera", IsHeadshot: true},
		{Type: entities.EventTypeBombPlanted, PlanterID: "coldzera"},
		{Type: entities.EventTypeBombDefused, DefuserID: "Fallen"},
		{Type: entities.EventTypeBombExploded},
	}

	metrics := []string{
		"kills", "thru_smoke", "is_headshot", "assisted_flash", "attacker_blind",
		"no_scope", "wallbang_count", "bomb_planted", "bomb_defused", "bomb_exploded",
		"invalid_metric",
	}

	for _, m := range metrics {
		res := ea.TopPlayersByMetric(events, m, 2)
		if m == "kills" {
			if len(res) != 2 || res[0].Label != "Fallen" || res[0].Value != 2 {
				t.Errorf("TopPlayersByMetric kills failed: %v", res)
			}
		}
	}
}

func TestEventAggregator_PlayerStat(t *testing.T) {
	ea := NewEventAggregator()

	events := []*entities.Event{
		{Type: entities.EventTypeKill, AttackerName: "Fallen", IsHeadshot: true},
		{Type: entities.EventTypeKill, AttackerName: "Fallen", IsHeadshot: false},
	}

	stat := ea.PlayerStat(events, "is_headshot", "FALLEN")
	if stat != 1 {
		t.Errorf("expected 1 headshot for fallen, got %d", stat)
	}
}

func TestEventAggregator_RoundAggregate(t *testing.T) {
	ea := NewEventAggregator()

	events := []*entities.Event{
		{Round: 1, Type: entities.EventTypeKill},
		{Round: 1, Type: entities.EventTypeKill},
		{Round: 2, Type: entities.EventTypeKill},
		{Round: -1, Type: entities.EventTypeKill}, // should be ignored
	}

	agg := ea.RoundAggregate(events, "kills", 1)
	if len(agg) != 1 || agg[0].Label != "Round 1" || agg[0].Value != 2 {
		t.Errorf("RoundAggregate failed: %v", agg)
	}

	aggAll := ea.RoundAggregate(events, "kills", 0)
	if len(aggAll) != 2 {
		t.Errorf("RoundAggregate with 0 limit failed: %v", aggAll)
	}

	metrics := []string{
		"kills", "thru_smoke", "is_headshot", "assisted_flash", "attacker_blind",
		"no_scope", "wallbang_count", "bomb_planted", "bomb_defused", "bomb_exploded",
		"invalid_metric",
	}
	for _, m := range metrics {
		ea.RoundAggregate(events, m, 1)
	}
}

func TestEventAggregator_MatchSummary(t *testing.T) {
	ea := NewEventAggregator()

	events := []*entities.Event{
		{Round: 1, Type: entities.EventTypeKill, IsHeadshot: true, ThruSmoke: true},
		{Round: 1, Type: entities.EventTypeBombPlanted},
		{Round: 1, Type: entities.EventTypeBombDefused},
		{Round: 2, Type: entities.EventTypeBombExploded},
	}

	summary := ea.MatchSummary(events)
	if summary["total_kills"] != 1 ||
		summary["total_headshots"] != 1 ||
		summary["total_smokes"] != 1 ||
		summary["total_bomb_plants"] != 1 ||
		summary["total_bomb_defuses"] != 1 ||
		summary["total_bomb_explodes"] != 1 ||
		summary["total_rounds"] != 2 {
		t.Errorf("MatchSummary failed: %v", summary)
	}
}

func TestItoa_Negative(t *testing.T) {
	s := itoa(-42)
	if s != "-42" {
		t.Errorf("expected -42, got %s", s)
	}
	s0 := itoa(0)
	if s0 != "0" {
		t.Errorf("expected 0, got %s", s0)
	}
}

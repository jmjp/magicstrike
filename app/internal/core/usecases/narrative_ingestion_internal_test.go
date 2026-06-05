package usecases

import (
	"testing"

	"magicstrike/internal/core/entities"
)

func TestFormatEventText(t *testing.T) {
	now := 0.0

	// 1. Match Start
	evStart, _ := entities.NewMatchStartEvent("ev1", "match1", now)
	res := formatEventText(evStart, "Team A", "Team B")
	if res != "Match started" {
		t.Errorf("expected 'Match started', got '%s'", res)
	}

	// 2. Match End
	evEnd, _ := entities.NewMatchEndEvent("ev2", "match1", now, "T", 16, 14)
	res = formatEventText(evEnd, "Team A", "Team B")
	expectedEnd := "Match ended. Winner: T (Score T: 16, CT: 14)"
	if res != expectedEnd {
		t.Errorf("expected '%s', got '%s'", expectedEnd, res)
	}

	// 3. Round Start
	evRStart, _ := entities.NewRoundStartEvent("ev3", "match1", 2, now)
	res = formatEventText(evRStart, "Team A", "Team B")
	expectedRStart := "Round 2 started"
	if res != expectedRStart {
		t.Errorf("expected '%s', got '%s'", expectedRStart, res)
	}

	// 4. Round End
	evREnd, _ := entities.NewRoundEndEvent("ev4", "match1", 2, now, "T", "TerroristsWin", 1, 0)
	evREnd.MVPPlayerID = "mvp1"
	evREnd.MVPReason = "MostEliminations"
	res = formatEventText(evREnd, "Team A", "Team B")
	expectedREnd := "Team B (T) won. Reason: TerroristsWin. MVP: mvp1 (Reason: MostEliminations)"
	if res != expectedREnd {
		t.Errorf("expected '%s', got '%s'", expectedREnd, res)
	}

	// 5. Round MVP
	evMVP, _ := entities.NewRoundMVPEvent("ev5", "match1", 2, now, "mvp1", "clutch")
	res = formatEventText(evMVP, "Team A", "Team B")
	expectedMVP := "Round MVP: mvp1 (Reason: clutch)"
	if res != expectedMVP {
		t.Errorf("expected '%s', got '%s'", expectedMVP, res)
	}

	// 6. Bomb Planted
	evPlanted, _ := entities.NewBombPlantedEvent("ev6", "match1", 2, now, "A", "planter1")
	evPlanted.AliveTAtPlant = 3
	evPlanted.AliveCtAtPlant = 2
	evPlanted.TimeRemaining = 35.0
	res = formatEventText(evPlanted, "Team A", "Team B")
	expectedPlanted := "Bomb planted at Site A (3v2 situation, 35s remaining)"
	if res != expectedPlanted {
		t.Errorf("expected '%s', got '%s'", expectedPlanted, res)
	}

	// 7. Bomb Exploded
	evExploded, _ := entities.NewBombExplodedEvent("ev7", "match1", 2, now, "A")
	res = formatEventText(evExploded, "Team A", "Team B")
	expectedExploded := "Bomb exploded at Site A"
	if res != expectedExploded {
		t.Errorf("expected '%s', got '%s'", expectedExploded, res)
	}

	// 8. Bomb Defused
	evDefused, _ := entities.NewBombDefusedEvent("ev8", "match1", 2, now, "B", "defuser1")
	res = formatEventText(evDefused, "Team A", "Team B")
	expectedDefused := "defuser1 (Team A - CT) defused the bomb at Site B"
	if res != expectedDefused {
		t.Errorf("expected '%s', got '%s'", expectedDefused, res)
	}

	// 9. Kill with flags
	evKill, _ := entities.NewKillEvent("ev9", "match1", 2, now,
		"atk1", "Attacker", "CT",
		"vic1", "Victim", "T",
		"ast1", "Assister", "CT",
		"m4a4", true, 2,
		true, true, true, true,
	)
	res = formatEventText(evKill, "Team A", "Team B")
	expectedKill := "Attacker (Team A - CT) killed Victim (Team B - T) with m4a4 [Flags: Headshot=true, ThruSmoke=true, NoScope=true, AttackerBlind=true, AssistedFlash=true, WallbangCount=2]"
	if res != expectedKill {
		t.Errorf("expected '%s', got '%s'", expectedKill, res)
	}

	// 10. Kill without flags and no assister
	evKillSimple, _ := entities.NewKillEvent("ev10", "match1", 2, now,
		"atk1", "Attacker", "CT",
		"vic1", "Victim", "T",
		"", "", "",
		"usp", false, 0,
		false, false, false, false,
	)
	res = formatEventText(evKillSimple, "Team A", "Team B")
	expectedKillSimple := "Attacker (Team A - CT) killed Victim (Team B - T) with usp"
	if res != expectedKillSimple {
		t.Errorf("expected '%s', got '%s'", expectedKillSimple, res)
	}

	// 11. Kill with unnamed attacker and unnamed victim
	evKillNoName, _ := entities.NewKillEvent("ev11", "match1", 2, now,
		"", "", "",
		"", "", "",
		"", "", "",
		"world", false, 0,
		false, false, false, false,
	)
	res = formatEventText(evKillNoName, "Team A", "Team B")
	expectedKillNoName := "Unknown Attacker killed Unknown Victim with world"
	if res != expectedKillNoName {
		t.Errorf("expected '%s', got '%s'", expectedKillNoName, res)
	}

	// 12. Trade Kill
	evTrade, _ := entities.NewKillEvent("ev12", "match1", 2, now,
		"atk1", "venomzera", "CT",
		"vic1", "Victim", "T",
		"", "", "",
		"ak47", false, 0,
		false, false, false, false,
	)
	evTrade.IsTrade = true
	evTrade.TradeTime = 1.1
	evTrade.TeammateDistance = 4.2
	res = formatEventText(evTrade, "Team A", "Team B")
	expectedTrade := "venomzera executed trade in 1.1s (Distance to Teammate: 4.2m)"
	if res != expectedTrade {
		t.Errorf("expected '%s', got '%s'", expectedTrade, res)
	}

	// 13. Clutch Kill
	evClutch, _ := entities.NewKillEvent("ev13", "match1", 2, now,
		"atk1", "LNZ", "CT",
		"vic1", "Victim", "T",
		"", "", "",
		"m4a4", false, 0,
		false, false, false, false,
	)
	evClutch.ClutchOpponents = 2
	evClutch.ClutchHpStart = 35
	evClutch.ClutchUtilityCount = 0
	res = formatEventText(evClutch, "Team A", "Team B")
	expectedClutch := "LNZ won 1v2 clutch starting with 35 HP, 0 utilities left"
	if res != expectedClutch {
		t.Errorf("expected '%s', got '%s'", expectedClutch, res)
	}

	// 14. Kill with new flags
	evKillNewFlags, _ := entities.NewKillEvent("ev14", "match1", 2, now,
		"atk1", "brnz4n", "CT",
		"vic1", "z4KR", "T",
		"", "", "",
		"AK-47", true, 0,
		false, false, false, false,
	)
	evKillNewFlags.ShotDistance = 24.5
	evKillNewFlags.TTK = 0.28
	evKillNewFlags.FiringType = "Tap"
	res = formatEventText(evKillNewFlags, "Team A", "Team B")
	expectedKillNew := "brnz4n (Team A - CT) killed z4KR (Team B - T) with AK-47 [Flags: Headshot=true, ShotDistance=24.5m, TTK=0.28s, FiringType=Tap]"
	if res != expectedKillNew {
		t.Errorf("expected '%s', got '%s'", expectedKillNew, res)
	}
}

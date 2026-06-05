package usecases

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"text/template"

	"github.com/google/uuid"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

const promptTemplateStr = `Analyze the following CS2 Match Round:
Match ID: {{.MatchID}}
Round: {{.RoundNumber}}
Initial Score: T {{.ScoreT}} - CT {{.ScoreCT}}
Winner: {{.WinnerTeam}} (Reason: {{.WinReason}})
MVP: {{.MVPPlayerID}} (Reason: {{.MVPReason}})

Macroeconomic Context:
- Buy Type T: {{.BuyTypeT}} (Avg Equipment Value: {{.AvgEqValT}})
- Buy Type CT: {{.BuyTypeCT}} (Avg Equipment Value: {{.AvgEqValCT}})

Tactical Utility & Combat Performance:
{{range .Events}}
- {{.FormattedText}}
{{end}}

Please write a highly descriptive tactical narrative summary incorporating spatial dynamics, economy, timings, trade efficiency, and clutch scenarios.`

var promptTemplate = template.Must(template.New("prompt").Parse(promptTemplateStr))

type RoundTemplateData struct {
	MatchID     string
	RoundNumber int
	ScoreT      int
	ScoreCT     int
	WinnerTeam  string
	WinReason   string
	MVPPlayerID string
	MVPReason   string
	BuyTypeT    string
	AvgEqValT   string
	BuyTypeCT   string
	AvgEqValCT  string
	Events      []EventTemplateData
}

type EventTemplateData struct {
	FormattedText string
}

// NarrativeService orchestrates the narrative generation pipeline for a match.
// It reads events from the EventRepository, groups them by round, generates
// tactical narratives via an LLM, produces vector embeddings, and upserts
// everything into the VectorRepository.
type NarrativeService struct {
	eventRepo  ports.EventRepository
	matchRepo  ports.MatchRepository
	llmSvc     ports.LLMService
	embedSvc   ports.EmbeddingService
	vectorRepo ports.VectorRepository
}

// NewNarrativeService creates a new NarrativeService instance.
func NewNarrativeService(
	eventRepo ports.EventRepository,
	matchRepo ports.MatchRepository,
	llmSvc ports.LLMService,
	embedSvc ports.EmbeddingService,
	vectorRepo ports.VectorRepository,
) *NarrativeService {
	return &NarrativeService{
		eventRepo:  eventRepo,
		matchRepo:  matchRepo,
		llmSvc:     llmSvc,
		embedSvc:   embedSvc,
		vectorRepo: vectorRepo,
	}
}

// ProcessMatch retrieves all events for a match, groups them by round,
// generates narratives, and upserts their embeddings to the vector database.
func (s *NarrativeService) ProcessMatch(ctx context.Context, matchID string) error {
	events, err := s.eventRepo.FindByMatchID(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	var teamA, teamB string
	if s.matchRepo != nil {
		match, err := s.matchRepo.FindByID(ctx, matchID)
		if err == nil && match != nil {
			if match.TeamA != nil {
				teamA = *match.TeamA
			}
			if match.TeamB != nil {
				teamB = *match.TeamB
			}
		}
	}
	if teamA == "" {
		teamA = "Team A"
	}
	if teamB == "" {
		teamB = "Team B"
	}

	// Group events by round
	roundsMap := make(map[int][]*entities.Event)
	for _, ev := range events {
		// Only group events that are assigned to a round
		if ev.Round > 0 {
			roundsMap[ev.Round] = append(roundsMap[ev.Round], ev)
		}
	}

	// Extract sorted list of rounds
	var roundNums []int
	for r := range roundsMap {
		roundNums = append(roundNums, r)
	}
	sort.Ints(roundNums)

	// Keep track of round end scores to compute initial scores
	roundEndScores := make(map[int][2]int) // round -> [scoreT, scoreCT]
	for _, rNum := range roundNums {
		for _, ev := range roundsMap[rNum] {
			if ev.Type == entities.EventTypeRoundEnd {
				roundEndScores[rNum] = [2]int{ev.ScoreT, ev.ScoreCT}
			}
		}
	}

	for _, rNum := range roundNums {
		rEvents := roundsMap[rNum]

		// Sort events chronologically within the round
		sort.Slice(rEvents, func(i, j int) bool {
			if rEvents[i].ElapsedSeconds == rEvents[j].ElapsedSeconds {
				return rEvents[i].ID < rEvents[j].ID
			}
			return rEvents[i].ElapsedSeconds < rEvents[j].ElapsedSeconds
		})

		// Build template data
		data := RoundTemplateData{
			MatchID:     matchID,
			RoundNumber: rNum,
		}

		// Initial score calculation
		if rNum > 1 {
			if prevScore, ok := roundEndScores[rNum-1]; ok {
				data.ScoreT = prevScore[0]
				data.ScoreCT = prevScore[1]
			}
		}

		// Extract outcome metadata, buy type info, and format events
		var buyTypeT, buyTypeCT string
		for _, ev := range rEvents {
			switch ev.Type {
			case entities.EventTypeRoundEnd:
				data.WinnerTeam = ev.WinnerTeam
				data.WinReason = ev.WinReason
			case entities.EventTypeRoundMVP:
				data.MVPPlayerID = ev.MVPPlayerID
				data.MVPReason = ev.MVPReason
			}

			if ev.BuyType != "" {
				if ev.Type == entities.EventTypeKill {
					if ev.AttackerTeam == "T" {
						buyTypeT = ev.BuyType
					} else if ev.AttackerTeam == "CT" {
						buyTypeCT = ev.BuyType
					}
				} else if ev.Type == entities.EventTypeBombPlanted {
					buyTypeT = ev.BuyType
				} else if ev.Type == entities.EventTypeBombDefused {
					buyTypeCT = ev.BuyType
				}
			}

			data.Events = append(data.Events, EventTemplateData{
				FormattedText: formatEventText(ev, teamA, teamB),
			})
		}

		if buyTypeT == "" {
			buyTypeT = "Eco"
		}
		if buyTypeCT == "" {
			buyTypeCT = "Eco"
		}

		data.BuyTypeT = buyTypeT
		data.AvgEqValT = getAvgEqVal(buyTypeT)
		data.BuyTypeCT = buyTypeCT
		data.AvgEqValCT = getAvgEqVal(buyTypeCT)

		// Fill prompt template
		var buf bytes.Buffer
		if err := promptTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to execute prompt template for round %d: %w", rNum, err)
		}
		prompt := buf.String()

		// Generate Narrative (with static fallback inside)
		narrative, err := s.llmSvc.GenerateText(ctx, prompt)
		if err != nil {
			// Static fallback narrative summary
			narrative = fmt.Sprintf("Round %d won by %s. Reason: %s. MVP: %s (Reason: %s).",
				rNum, data.WinnerTeam, data.WinReason, data.MVPPlayerID, data.MVPReason)
		}

		// Generate Narrative Embedding
		emb, err := s.embedSvc.GenerateEmbedding(ctx, narrative)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for round %d: %w", rNum, err)
		}

		// Generate deterministic UUID for Qdrant record to ensure idempotency
		recordID := deterministicUUID(matchID, rNum)

		record := &ports.VectorRecord{
			ID:     recordID,
			Vector: emb,
			Metadata: map[string]any{
				"match_id":      matchID,
				"round":         rNum,
				"winner_team":   data.WinnerTeam,
				"win_reason":    data.WinReason,
				"mvp_player_id": data.MVPPlayerID,
				"narrative":     narrative,
			},
		}

		// Upsert vector record
		if err := s.vectorRepo.Upsert(ctx, record); err != nil {
			return fmt.Errorf("failed to upsert narrative to Qdrant for round %d: %w", rNum, err)
		}
	}

	return nil
}

func deterministicUUID(matchID string, round int) string {
	ns := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	hash := md5.Sum([]byte(fmt.Sprintf("%s-round-%d", matchID, round)))
	return uuid.NewMD5(ns, hash[:]).String()
}

func getAvgEqVal(buyType string) string {
	switch buyType {
	case "Eco":
		return "< $1500"
	case "Semi-Buy":
		return "$1500 - $3000"
	case "Force-Buy":
		return "$1500 - $3500"
	case "Full Buy":
		return ">= $4000"
	default:
		return "Unknown"
	}
}

func formatEventText(ev *entities.Event, teamA, teamB string) string {
	switch ev.Type {
	case entities.EventTypeMatchStart:
		return "Match started"
	case entities.EventTypeMatchEnd:
		return fmt.Sprintf("Match ended. Winner: %s (Score T: %d, CT: %d)", ev.WinnerTeam, ev.ScoreT, ev.ScoreCT)
	case entities.EventTypeRoundStart:
		return fmt.Sprintf("Round %d started", ev.Round)
	case entities.EventTypeRoundEnd:
		winnerRealTeam := getTeamForSide(ev.Round, ev.WinnerTeam, teamA, teamB)
		winnerDesc := ev.WinnerTeam
		if winnerRealTeam != "" {
			winnerDesc = fmt.Sprintf("%s (%s)", winnerRealTeam, ev.WinnerTeam)
		}
		return fmt.Sprintf("%s won. Reason: %s. MVP: %s (Reason: %s)", winnerDesc, ev.WinReason, ev.MVPPlayerID, ev.MVPReason)
	case entities.EventTypeRoundMVP:
		return fmt.Sprintf("Round MVP: %s (Reason: %s)", ev.MVPPlayerID, ev.MVPReason)
	case entities.EventTypeBombPlanted:
		return fmt.Sprintf("Bomb planted at Site %s (%dv%d situation, %.0fs remaining)", ev.BombSite, ev.AliveTAtPlant, ev.AliveCtAtPlant, ev.TimeRemaining)
	case entities.EventTypeBombExploded:
		return fmt.Sprintf("Bomb exploded at Site %s", ev.BombSite)
	case entities.EventTypeBombDefused:
		defuserRealTeam := getTeamForSide(ev.Round, "CT", teamA, teamB)
		defuserDesc := ev.DefuserID
		if defuserRealTeam != "" {
			defuserDesc = fmt.Sprintf("%s (%s - CT)", ev.DefuserID, defuserRealTeam)
		}
		return fmt.Sprintf("%s defused the bomb at Site %s", defuserDesc, ev.BombSite)
	case entities.EventTypeKill:
		attackerName := ev.AttackerName
		if attackerName == "" {
			attackerName = "Unknown Attacker"
		}
		attacker := attackerName
		attackerTeamName := getTeamForSide(ev.Round, ev.AttackerTeam, teamA, teamB)
		if ev.AttackerTeam != "" {
			if attackerTeamName != "" {
				attacker = fmt.Sprintf("%s (%s - %s)", attacker, attackerTeamName, ev.AttackerTeam)
			} else {
				attacker = fmt.Sprintf("%s (%s)", attacker, ev.AttackerTeam)
			}
		}

		victimName := ev.VictimName
		if victimName == "" {
			victimName = "Unknown Victim"
		}
		victim := victimName
		victimTeamName := getTeamForSide(ev.Round, ev.VictimTeam, teamA, teamB)
		if ev.VictimTeam != "" {
			if victimTeamName != "" {
				victim = fmt.Sprintf("%s (%s - %s)", victim, victimTeamName, ev.VictimTeam)
			} else {
				victim = fmt.Sprintf("%s (%s)", victim, ev.VictimTeam)
			}
		}

		weapon := ev.Weapon
		if weapon == "" {
			weapon = "unknown weapon"
		}

		if ev.IsTrade {
			return fmt.Sprintf("%s executed trade in %.1fs (Distance to Teammate: %.1fm)", attackerName, ev.TradeTime, ev.TeammateDistance)
		}
		if ev.ClutchOpponents > 0 {
			return fmt.Sprintf("%s won 1v%d clutch starting with %d HP, %d utilities left", attackerName, ev.ClutchOpponents, ev.ClutchHpStart, ev.ClutchUtilityCount)
		}

		flags := ""
		var flagList []string
		if ev.IsHeadshot {
			flagList = append(flagList, "Headshot=true")
		}
		if ev.ThruSmoke {
			flagList = append(flagList, "ThruSmoke=true")
		}
		if ev.NoScope {
			flagList = append(flagList, "NoScope=true")
		}
		if ev.AttackerBlind {
			flagList = append(flagList, "AttackerBlind=true")
		}
		if ev.AssistedFlash {
			flagList = append(flagList, "AssistedFlash=true")
		}
		if ev.WallbangCount > 0 {
			flagList = append(flagList, fmt.Sprintf("WallbangCount=%d", ev.WallbangCount))
		}
		if ev.ShotDistance > 0 {
			flagList = append(flagList, fmt.Sprintf("ShotDistance=%.1fm", ev.ShotDistance))
		}
		if ev.TTK > 0 {
			flagList = append(flagList, fmt.Sprintf("TTK=%.2fs", ev.TTK))
		}
		if ev.FiringType != "" {
			flagList = append(flagList, fmt.Sprintf("FiringType=%s", ev.FiringType))
		}

		if len(flagList) > 0 {
			flags = " [Flags: "
			for idx, f := range flagList {
				if idx > 0 {
					flags += ", "
				}
				flags += f
			}
			flags += "]"
		}

		return fmt.Sprintf("%s killed %s with %s%s", attacker, victim, weapon, flags)
	default:
		return string(ev.Type)
	}
}

package usecases

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
)

// Prompt template used when there is NO previous conversation context (new session).
const NewSessionPrompt = `You are a professional CS2 tactical analyst and analytics assistant.
Use the data provided below to answer the user's question, construct tactical/anti-tactical plans, and perform calculations (e.g. ratios, headshot percentages, plant success rates, hold rates, weapon efficiency).

Here is the database schema of the game events saved in ClickHouse for your reference when explaining calculations:
- Event Types: match_start, match_end, round_start, round_end, round_mvp, bomb_planted, bomb_exploded, bomb_defused, kill
- Round: Round number
- AttackerName, AttackerTeam, VictimName, VictimTeam, AssisterName, AssisterTeam (CT or T sides mapping to their real teams)
- Weapon: Weapon name
- BombSite: A or B
- WinnerTeam (CT or T), WinReason (TargetBombed, BombDefused, CTWin, TerroristsWin, etc.)
- MVPPlayerID, MVPReason
- Flags: IsHeadshot, ThruSmoke, NoScope, AttackerBlind, AssistedFlash, WallbangCount

When requested, formulate tactical plans (e.g., how to play a site, execute a play) or anti-tactical plans (e.g., how to counter a specific opponent's pattern shown in the data) and explain your calculations clearly based on specific numbers from the data.

Match Context:
%s

User question: %s

Matches analyzed: %s

Data from %s:
%s

When both quantitative data and tactical narratives are provided, correlate them — ground your tactical analysis with specific numbers and use narratives to add context to the statistics.

Answer in the same language as the question (Portuguese or English). Include specific numbers when available.`

// Prompt template used when there IS previous conversation context (continue session).
const ContinueSessionPrompt = `You are a professional CS2 tactical analyst and analytics assistant.
Use the data provided below to answer the user's question, construct tactical/anti-tactical plans, and perform calculations (e.g. ratios, headshot percentages, plant success rates, hold rates, weapon efficiency).

Here is the database schema of the game events saved in ClickHouse for your reference when explaining calculations:
- Event Types: match_start, match_end, round_start, round_end, round_mvp, bomb_planted, bomb_exploded, bomb_defused, kill
- Round: Round number
- AttackerName, AttackerTeam, VictimName, VictimTeam, AssisterName, AssisterTeam (CT or T sides mapping to their real teams)
- Weapon: Weapon name
- BombSite: A or B
- WinnerTeam (CT or T), WinReason (TargetBombed, BombDefused, CTWin, TerroristsWin, etc.)
- MVPPlayerID, MVPReason
- Flags: IsHeadshot, ThruSmoke, NoScope, AttackerBlind, AssistedFlash, WallbangCount

When requested, formulate tactical plans (e.g., how to play a site, execute a play) or anti-tactical plans (e.g., how to counter a specific opponent's pattern shown in the data) and explain your calculations clearly based on specific numbers from the data.

Match Context:
%s

Previous conversation about matches %s:

%s

New question: %s

Data from %s:
%s

When both quantitative data and tactical narratives are provided, correlate them — ground your tactical analysis with specific numbers and use narratives to add context to the statistics.

Answer in the same language as the question (Portuguese or English). Include specific numbers when available.`

// ChatUseCase implements ports.ChatUseCase for conversational analytics over demos.
type ChatUseCase struct {
	eventRepo       ports.EventRepository
	matchRepo       ports.MatchRepository
	vectorRepo      ports.VectorRepository
	embedSvc        ports.EmbeddingService
	llmSvc          ports.LLMService
	router          *services.QueryRouter
	aggregator      *services.EventAggregator
	chatSessionRepo ports.ChatSessionRepository
	ttlDays         int
}

// NewChatUseCase creates a new ChatUseCase instance.
func NewChatUseCase(
	eventRepo ports.EventRepository,
	matchRepo ports.MatchRepository,
	vectorRepo ports.VectorRepository,
	embedSvc ports.EmbeddingService,
	llmSvc ports.LLMService,
	chatSessionRepo ports.ChatSessionRepository,
	ttlDays int,
) ports.ChatUseCase {
	return &ChatUseCase{
		eventRepo:       eventRepo,
		matchRepo:       matchRepo,
		vectorRepo:      vectorRepo,
		embedSvc:        embedSvc,
		llmSvc:          llmSvc,
		router:          services.NewQueryRouter(llmSvc),
		aggregator:      services.NewEventAggregator(),
		chatSessionRepo: chatSessionRepo,
		ttlDays:         ttlDays,
	}
}

// getTeamForSide determines the real team name for a side in a specific round.
func getTeamForSide(round int, side string, teamA, teamB string) string {
	if side != "CT" && side != "T" {
		return ""
	}
	if round <= 12 {
		if side == "CT" {
			return teamA
		}
		return teamB
	}
	if round <= 24 {
		if side == "CT" {
			return teamB
		}
		return teamA
	}
	otRound := round - 25
	relRound := otRound % 6
	if relRound < 3 {
		if side == "CT" {
			return teamA
		}
		return teamB
	}
	if side == "CT" {
		return teamB
	}
	return teamA
}

// buildMatchContext fetches match details and builds a rich metadata header for the LLM context.
func (uc *ChatUseCase) buildMatchContext(ctx context.Context, matchIDs []string) (string, error) {
	if uc.matchRepo == nil {
		return "No MatchRepository configured.", nil
	}

	var sb strings.Builder
	for _, matchID := range matchIDs {
		match, err := uc.matchRepo.FindByID(ctx, matchID)
		if err != nil {
			return "", fmt.Errorf("failed to fetch match details for %s: %w", matchID, err)
		}
		if match == nil {
			sb.WriteString(fmt.Sprintf("Match details not found for Match ID: %s\n\n", matchID))
			continue
		}

		mapName := "unknown"
		if match.MapName != nil {
			mapName = string(*match.MapName)
		}
		teamA := "Team A"
		if match.TeamA != nil {
			teamA = *match.TeamA
		}
		teamB := "Team B"
		if match.TeamB != nil {
			teamB = *match.TeamB
		}
		scoreA := 0
		if match.ScoreA != nil {
			scoreA = *match.ScoreA
		}
		scoreB := 0
		if match.ScoreB != nil {
			scoreB = *match.ScoreB
		}

		sb.WriteString(fmt.Sprintf("Match ID: %s\n", match.ID))
		sb.WriteString(fmt.Sprintf("- Map: %s\n", mapName))
		sb.WriteString(fmt.Sprintf("- Score: %s %d x %d %s\n", teamA, scoreA, scoreB, teamB))
		if match.Winner() != nil {
			sb.WriteString(fmt.Sprintf("- Winner: %s\n", *match.Winner()))
		}

		// Get players list from events
		_, events, err := uc.loadAllEvents(ctx, []string{matchID})
		if err == nil && len(events) > 0 {
			playersA := make(map[string]bool)
			playersB := make(map[string]bool)
			for _, ev := range events {
				if ev.AttackerName != "" && ev.AttackerTeam != "" {
					t := getTeamForSide(ev.Round, ev.AttackerTeam, teamA, teamB)
					if t == teamA {
						playersA[ev.AttackerName] = true
					} else if t == teamB {
						playersB[ev.AttackerName] = true
					}
				}
				if ev.VictimName != "" && ev.VictimTeam != "" {
					t := getTeamForSide(ev.Round, ev.VictimTeam, teamA, teamB)
					if t == teamA {
						playersA[ev.VictimName] = true
					} else if t == teamB {
						playersB[ev.VictimName] = true
					}
				}
			}

			var listA, listB []string
			for p := range playersA {
				listA = append(listA, p)
			}
			for p := range playersB {
				listB = append(listB, p)
			}
			sort.Strings(listA)
			sort.Strings(listB)

			sb.WriteString(fmt.Sprintf("- %s Players: %s\n", teamA, strings.Join(listA, ", ")))
			sb.WriteString(fmt.Sprintf("- %s Players: %s\n", teamB, strings.Join(listB, ", ")))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String()), nil
}

// NewSession creates a chat session with the first question.
func (uc *ChatUseCase) NewSession(
	ctx context.Context, userID string, matchIDs []string, question string,
) (*ports.ChatResponse, error) {
	if len(matchIDs) == 0 {
		return nil, fmt.Errorf("at least one match_id is required")
	}

	// Phase 1: Classify the question
	intent, err := uc.router.Classify(ctx, question, nil)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Phase 2: Route to appropriate database and fetch data
	dataText, dataPoints, err := uc.queryByIntent(ctx, matchIDs, intent)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Fetch match context
	matchCtx, err := uc.buildMatchContext(ctx, matchIDs)
	if err != nil {
		log.Printf("[Chat] Failed to build match context: %v", err)
	}

	// Phase 3: Synthesize answer via LLM (no previous context)
	matchesStr := strings.Join(matchIDs, ", ")
	src := sourceDisplay(intent.Target)
	prompt := fmt.Sprintf(NewSessionPrompt, matchCtx, question, matchesStr, src, dataText)
	answer, err := uc.llmSvc.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}
	answer = strings.TrimSpace(answer)

	// Phase 4: Create and persist session (best-effort save)
	chatPoints := convertDataPoints(dataPoints)
	session, err := entities.NewChatSession(
		userID, matchIDs, question, answer, src, chatPoints, uc.ttlDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session entity: %w", err)
	}

	if err = uc.chatSessionRepo.Save(ctx, session); err != nil {
		log.Printf("[Chat] Failed to persist new session: %v", err)
	}

	return &ports.ChatResponse{
		SessionID:   session.ID,
		Answer:      answer,
		Source:      src,
		MatchesUsed: matchIDs,
		DataPoints:  dataPoints,
	}, nil
}

// ContinueSession adds a follow-up question to an existing session.
func (uc *ChatUseCase) ContinueSession(
	ctx context.Context, userID, sessionID, question string,
) (*ports.ChatResponse, error) {
	// Phase 1: Load session and verify ownership
	session, err := uc.chatSessionRepo.FindByID(ctx, userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Phase 2: Check expiration
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session has expired")
	}

	// Fetch history for classification and prompt context (chronological order, oldest first)
	prevMessages := session.LastNMessages(entities.MaxMessagesInContext)

	// Phase 3: Classify the follow-up question using previous messages
	intent, err := uc.router.Classify(ctx, question, prevMessages)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Phase 4: Route to appropriate database using session's match_ids
	dataText, dataPoints, err := uc.queryByIntent(ctx, session.MatchIDs, intent)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Build context from previous messages
	var contextBuilder strings.Builder
	for _, msg := range prevMessages {
		contextBuilder.WriteString(fmt.Sprintf("User: %s\nAssistant: %s\n\n", msg.Question, msg.Answer))
	}

	// Fetch match context
	matchCtx, err := uc.buildMatchContext(ctx, session.MatchIDs)
	if err != nil {
		log.Printf("[Chat] Failed to build match context: %v", err)
	}

	// Phase 6: Synthesize answer with conversation context
	matchesStr := strings.Join(session.MatchIDs, ", ")
	src := sourceDisplay(intent.Target)
	prompt := fmt.Sprintf(
		ContinueSessionPrompt,
		matchCtx,
		matchesStr,
		strings.TrimSpace(contextBuilder.String()),
		question,
		src,
		dataText,
	)
	answer, err := uc.llmSvc.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}
	answer = strings.TrimSpace(answer)

	// Phase 7: Persist message (best-effort save).
	// repo.AddMessage also appends the message to the session entity in-memory, so we
	// do NOT call session.AddMessage() here -- doing so would duplicate the message.
	if _, err = uc.chatSessionRepo.AddMessage(
		ctx, userID, sessionID, question, answer, src, dataPoints,
	); err != nil {
		log.Printf("[Chat] Failed to persist message for session %s: %v", sessionID, err)
	}

	return &ports.ChatResponse{
		SessionID:   session.ID,
		Answer:      answer,
		Source:      src,
		MatchesUsed: session.MatchIDs,
		DataPoints:  dataPoints,
	}, nil
}

// NewSessionStream creates a chat session streaming the response.
// The session is persisted as a placeholder BEFORE streaming begins, so the
// session ID is immediately valid for follow-up requests — no race condition.
func (uc *ChatUseCase) NewSessionStream(
	ctx context.Context, userID string, matchIDs []string, question string,
) (*ports.StreamResponse, error) {
	if len(matchIDs) == 0 {
		return nil, fmt.Errorf("at least one match_id is required")
	}

	streamingLLM, ok := uc.llmSvc.(ports.StreamingLLMService)
	if !ok {
		return nil, fmt.Errorf("configured LLM service does not support streaming")
	}

	// Phase 1: Classify the question
	intent, err := uc.router.Classify(ctx, question, nil)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Phase 2: Route to appropriate database and fetch data
	dataText, dataPoints, err := uc.queryByIntent(ctx, matchIDs, intent)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Fetch match context
	matchCtx, err := uc.buildMatchContext(ctx, matchIDs)
	if err != nil {
		log.Printf("[Chat] Failed to build match context: %v", err)
	}

	// Synthesize prompt
	matchesStr := strings.Join(matchIDs, ", ")
	src := sourceDisplay(intent.Target)
	prompt := fmt.Sprintf(NewSessionPrompt, matchCtx, question, matchesStr, src, dataText)

	// Phase 3: Persist session placeholder BEFORE streaming starts.
	// This eliminates the race condition where the session ID is returned to the
	// client but the session doesn't exist in the database yet.
	chatPoints := convertDataPoints(dataPoints)
	session, err := entities.NewChatSession(
		userID, matchIDs, question, "[streaming...]", src, chatPoints, uc.ttlDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session entity: %w", err)
	}
	sessionID := session.ID

	if err := uc.chatSessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to persist session placeholder: %w", err)
	}

	// Call Streaming LLM
	llmStream, llmErrChan := streamingLLM.GenerateTextStream(ctx, prompt)

	// Create output channels
	outChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		var accumulatedAnswer strings.Builder

		for {
			select {
			case <-ctx.Done():
				log.Printf("[Chat] Streaming cancelled by client context: %v", ctx.Err())
				errChan <- ctx.Err()
				return
			case err, ok := <-llmErrChan:
				if ok && err != nil {
					log.Printf("[Chat] Streaming error from LLM: %v", err)
					errChan <- err
					return
				}
			case chunk, ok := <-llmStream:
				if !ok {
					// Stream finished — update the placeholder with the real answer.
					ans := strings.TrimSpace(accumulatedAnswer.String())

					// Re-fetch to avoid stale entity state, then update the answer.
					session, err := uc.chatSessionRepo.FindByID(ctx, userID, sessionID)
					if err != nil || session == nil {
						log.Printf("[Chat] Failed to find session %s for answer update: %v", sessionID, err)
						errChan <- fmt.Errorf("failed to find session for answer update")
						return
					}
					if len(session.Messages) > 0 {
						session.Messages[0].Answer = ans
						session.Messages[0].Source = src
					}
					session.UpdatedAt = time.Now()

					if err := uc.chatSessionRepo.Save(ctx, session); err != nil {
						log.Printf("[Chat] Failed to persist session answer: %v", err)
						errChan <- fmt.Errorf("failed to persist session answer: %w", err)
						return
					}
					return
				}
				accumulatedAnswer.WriteString(chunk)
				// Send chunk to client channel
				select {
				case <-ctx.Done():
					log.Printf("[Chat] Streaming cancelled by client context: %v", ctx.Err())
					errChan <- ctx.Err()
					return
				case outChan <- chunk:
				}
			}
		}
	}()

	return &ports.StreamResponse{
		SessionID:   sessionID,
		Source:      src,
		MatchesUsed: matchIDs,
		DataPoints:  dataPoints,
		Stream:      outChan,
		ErrChan:     errChan,
	}, nil
}

// ContinueSessionStream adds a follow-up question streaming the response.
func (uc *ChatUseCase) ContinueSessionStream(
	ctx context.Context, userID string, sessionID string, question string,
) (*ports.StreamResponse, error) {
	streamingLLM, ok := uc.llmSvc.(ports.StreamingLLMService)
	if !ok {
		return nil, fmt.Errorf("configured LLM service does not support streaming")
	}

	// Phase 1: Load session and verify ownership
	session, err := uc.chatSessionRepo.FindByID(ctx, userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Phase 2: Check expiration
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session has expired")
	}

	// Fetch history for classification and prompt context (chronological order, oldest first)
	prevMessages := session.LastNMessages(entities.MaxMessagesInContext)

	// Phase 3: Classify the follow-up question using previous messages
	intent, err := uc.router.Classify(ctx, question, prevMessages)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Phase 4: Route to appropriate database using session's match_ids
	dataText, dataPoints, err := uc.queryByIntent(ctx, session.MatchIDs, intent)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Build context from previous messages
	var contextBuilder strings.Builder
	for _, msg := range prevMessages {
		contextBuilder.WriteString(fmt.Sprintf("User: %s\nAssistant: %s\n\n", msg.Question, msg.Answer))
	}

	// Fetch match context
	matchCtx, err := uc.buildMatchContext(ctx, session.MatchIDs)
	if err != nil {
		log.Printf("[Chat] Failed to build match context: %v", err)
	}

	// Synthesize prompt
	matchesStr := strings.Join(session.MatchIDs, ", ")
	src := sourceDisplay(intent.Target)
	prompt := fmt.Sprintf(
		ContinueSessionPrompt,
		matchCtx,
		matchesStr,
		strings.TrimSpace(contextBuilder.String()),
		question,
		src,
		dataText,
	)

	// Call Streaming LLM
	llmStream, llmErrChan := streamingLLM.GenerateTextStream(ctx, prompt)

	// Create output channels
	outChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		var accumulatedAnswer strings.Builder

		for {
			select {
			case <-ctx.Done():
				log.Printf("[Chat] Streaming cancelled by client context: %v", ctx.Err())
				errChan <- ctx.Err()
				return
			case err, ok := <-llmErrChan:
				if ok && err != nil {
					log.Printf("[Chat] Streaming error from LLM: %v", err)
					errChan <- err
					return
				}
			case chunk, ok := <-llmStream:
				if !ok {
					// Stream finished successfully. Persist the message deferred.
					ans := strings.TrimSpace(accumulatedAnswer.String())
					if _, err = uc.chatSessionRepo.AddMessage(
						ctx, userID, sessionID, question, ans, src, dataPoints,
					); err != nil {
						log.Printf("[Chat] Failed to persist message for session %s on stream complete: %v", sessionID, err)
						errChan <- fmt.Errorf("failed to persist message: %w", err)
						return
					}
					return
				}
				accumulatedAnswer.WriteString(chunk)
				// Send chunk to client channel
				select {
				case <-ctx.Done():
					log.Printf("[Chat] Streaming cancelled by client context: %v", ctx.Err())
					errChan <- ctx.Err()
					return
				case outChan <- chunk:
				}
			}
		}
	}()

	return &ports.StreamResponse{
		SessionID:   session.ID,
		Source:      src,
		MatchesUsed: session.MatchIDs,
		DataPoints:  dataPoints,
		Stream:      outChan,
		ErrChan:     errChan,
	}, nil
}


// queryByIntent routes a query to ClickHouse, Qdrant, or both based on the classified intent.
func (uc *ChatUseCase) queryByIntent(
	ctx context.Context, matchIDs []string, intent *services.QueryIntent,
) (string, []ports.ChatDataPoint, error) {
	switch intent.Target {
	case "clickhouse":
		return uc.queryClickHouse(ctx, matchIDs, intent)
	case "qdrant":
		return uc.queryQdrant(ctx, matchIDs, intent)
	case "hybrid":
		return uc.queryHybrid(ctx, matchIDs, intent)
	default:
		return "", nil, fmt.Errorf("unknown target: %s", intent.Target)
	}
}

// queryHybrid queries both ClickHouse (quantitative) and Qdrant (semantic) and
// merges the results into a single enriched context for the LLM. If one source
// fails, the other is still returned — degradation is graceful.
func (uc *ChatUseCase) queryHybrid(
	ctx context.Context, matchIDs []string, intent *services.QueryIntent,
) (string, []ports.ChatDataPoint, error) {
	var sb strings.Builder
	var allPoints []ports.ChatDataPoint

	// Query ClickHouse for quantitative data
	chText, chPoints, chErr := uc.queryClickHouse(ctx, matchIDs, intent)
	if chErr == nil && chText != "" {
		sb.WriteString("=== Quantitative Data (ClickHouse) ===\n")
		sb.WriteString(chText)
		sb.WriteString("\n")
		allPoints = append(allPoints, chPoints...)
	} else if chErr != nil {
		sb.WriteString(fmt.Sprintf("Quantitative data temporarily unavailable: %v\n\n", chErr))
	}

	// Query Qdrant for semantic narratives
	qdText, qdPoints, qdErr := uc.queryQdrant(ctx, matchIDs, intent)
	if qdErr == nil && qdText != "" {
		sb.WriteString("=== Tactical Narratives (Qdrant) ===\n")
		sb.WriteString(qdText)
		allPoints = append(allPoints, qdPoints...)
	} else if qdErr != nil {
		sb.WriteString(fmt.Sprintf("Narrative data temporarily unavailable: %v\n\n", qdErr))
	}

	if sb.Len() == 0 {
		return "", nil, fmt.Errorf("both ClickHouse and Qdrant queries failed for matches: %s",
			strings.Join(matchIDs, ", "))
	}

	return sb.String(), allPoints, nil
}

// convertDataPoints converts []ports.ChatDataPoint to []entities.DataPoint.
func convertDataPoints(points []ports.ChatDataPoint) []entities.DataPoint {
	result := make([]entities.DataPoint, len(points))
	for i, p := range points {
		result[i] = entities.DataPoint{Label: p.Label, Value: p.Value}
	}
	return result
}

// loadAllEvents fetches events for all requested match IDs.
func (uc *ChatUseCase) loadAllEvents(ctx context.Context, matchIDs []string) ([]string, []*entities.Event, error) {
	var allEvents []*entities.Event
	var loadedMatches []string

	for _, matchID := range matchIDs {
		events, err := uc.eventRepo.FindByMatchID(ctx, matchID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch events for match %s: %w", matchID, err)
		}
		if len(events) > 0 {
			loadedMatches = append(loadedMatches, matchID)
			allEvents = append(allEvents, events...)
		}
	}

	return loadedMatches, allEvents, nil
}

func (uc *ChatUseCase) queryClickHouse(ctx context.Context, matchIDs []string, intent *services.QueryIntent) (string, []ports.ChatDataPoint, error) {
	loadedMatches, events, err := uc.loadAllEvents(ctx, matchIDs)
	if err != nil {
		return "", nil, err
	}

	if len(events) == 0 {
		return fmt.Sprintf("No events found for matches: %s", strings.Join(matchIDs, ", ")), nil, nil
	}

	var sb strings.Builder
	var points []ports.ChatDataPoint

	sb.WriteString(fmt.Sprintf("Data from %d match(es): %s\n\n", len(loadedMatches), strings.Join(loadedMatches, ", ")))

	switch intent.QueryType {
	case "top_players_by_metric":
		rows := uc.aggregator.TopPlayersByMetric(events, intent.Metric, intent.Limit)
		sb.WriteString(fmt.Sprintf("Top %d players by %s:\n", intent.Limit, intent.Metric))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", r.Label, r.Value))
			points = append(points, ports.ChatDataPoint{Label: r.Label, Value: fmt.Sprintf("%d", r.Value)})
		}
		if len(rows) == 0 {
			sb.WriteString("No matching data found.\n")
		}

	case "player_stat":
		playerName := intent.PlayerName
		if playerName == "" {
			return "", nil, fmt.Errorf("player_name is required for player_stat query")
		}
		count := uc.aggregator.PlayerStat(events, intent.Metric, playerName)
		sb.WriteString(fmt.Sprintf("%s has %d %s across %d match(es).\n", playerName, count, intent.Metric, len(loadedMatches)))
		points = append(points, ports.ChatDataPoint{Label: playerName, Value: fmt.Sprintf("%d", count)})

	case "round_aggregate":
		rows := uc.aggregator.RoundAggregate(events, intent.Metric, intent.Limit)
		sb.WriteString(fmt.Sprintf("Rounds ranked by %s:\n", intent.Metric))
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", r.Label, r.Value))
			points = append(points, ports.ChatDataPoint{Label: r.Label, Value: fmt.Sprintf("%d", r.Value)})
		}
		if len(rows) == 0 {
			sb.WriteString("No matching data found.\n")
		}

	case "match_summary":
		summary := uc.aggregator.MatchSummary(events)
		sb.WriteString(fmt.Sprintf("Aggregate Summary across %d match(es):\n", len(loadedMatches)))
		for k, v := range summary {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", k, v))
			points = append(points, ports.ChatDataPoint{Label: k, Value: fmt.Sprintf("%d", v)})
		}

	default:
		return "", nil, fmt.Errorf("unsupported query_type: %s", intent.QueryType)
	}

	return sb.String(), points, nil
}

func (uc *ChatUseCase) queryQdrant(ctx context.Context, matchIDs []string, intent *services.QueryIntent) (string, []ports.ChatDataPoint, error) {
	searchQuery := intent.SearchQuery
	if searchQuery == "" {
		return "", nil, fmt.Errorf("search_query is required for qdrant queries")
	}

	// Generate embedding for the search query
	vec, err := uc.embedSvc.GenerateEmbedding(ctx, searchQuery)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate search embedding: %w", err)
	}

	// Search Qdrant — request more results since we'll filter by match_id
	searchLimit := intent.Limit * len(matchIDs)
	if searchLimit < 10 {
		searchLimit = 10
	}

	results, err := uc.vectorRepo.Search(ctx, vec, searchLimit)
	if err != nil {
		return "", nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	// Build match_id set for fast lookup
	matchSet := make(map[string]bool, len(matchIDs))
	for _, mid := range matchIDs {
		matchSet[mid] = true
	}

	// Post-filter: keep only results belonging to requested matches
	var filtered []*ports.VectorRecord
	for _, rec := range results {
		mid, _ := rec.Metadata["match_id"].(string)
		if matchSet[mid] {
			filtered = append(filtered, rec)
		}
	}

	if len(filtered) == 0 {
		return fmt.Sprintf("No similar round narratives found for matches: %s", strings.Join(matchIDs, ", ")), nil, nil
	}

	topN := intent.Limit
	if len(filtered) < topN {
		topN = len(filtered)
	}

	var sb strings.Builder
	var points []ports.ChatDataPoint

	sb.WriteString(fmt.Sprintf("Relevant round narratives (top %d of %d results):\n", topN, len(filtered)))
	for i := 0; i < topN; i++ {
		rec := filtered[i]
		narrative, _ := rec.Metadata["narrative"].(string)
		round, _ := rec.Metadata["round"].(float64)
		winner, _ := rec.Metadata["winner_team"].(string)
		reason, _ := rec.Metadata["win_reason"].(string)
		mid, _ := rec.Metadata["match_id"].(string)

		sb.WriteString(fmt.Sprintf("\n--- Match %s / Round %d (Winner: %s, Reason: %s) ---\n", mid, int(round), winner, reason))
		sb.WriteString(narrative)
		sb.WriteString("\n")

		points = append(points, ports.ChatDataPoint{
			Label: fmt.Sprintf("%s / Round %d", truncateMatchID(mid, 12), int(round)),
			Value: truncate(narrative, 120),
		})
	}

	return sb.String(), points, nil
}

func truncateMatchID(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// sourceDisplay maps the internal target name to a human-readable source label.
// "hybrid" is displayed as "clickhouse + qdrant" to indicate blended data.
func sourceDisplay(target string) string {
	if target == "hybrid" {
		return "clickhouse + qdrant"
	}
	return target
}

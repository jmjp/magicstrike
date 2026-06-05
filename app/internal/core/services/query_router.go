package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// QueryIntent represents the structured classification of a user question.
type QueryIntent struct {
	Target      string `json:"target"`                // "clickhouse", "qdrant", or "hybrid" (both)
	QueryType   string `json:"query_type"`            // template key for clickhouse queries
	Metric      string `json:"metric,omitempty"`      // e.g. "thru_smoke", "is_headshot", "kills"
	PlayerName  string `json:"player_name,omitempty"` // extracted player name for filters
	Limit       int    `json:"limit,omitempty"`       // max results (default 5)
	SearchQuery string `json:"search_query,omitempty"` // for Qdrant — the semantic search text
}

const classifierPrompt = `You are a CS2 analytics query classifier. Analyze the user's question (in Portuguese or English) and output a JSON object that classifies what kind of data retrieval is needed.

Rules:
- "target": "clickhouse" for purely quantitative/statistical questions (counts, aggregates, who did most X, how many, which round had most Y). Use this when the question ONLY asks for numbers/stats without any "why" or "how".
- "target": "qdrant" for purely semantic/narrative questions (why did something happen, how was a round won, what led to a play, describe a tactical situation). Use this when the question ONLY asks for explanations without any quantitative component.
- "target": "hybrid" for questions that mix quantitative and semantic aspects — when the user asks "why" or "how" about something that also involves stats, counts, or metrics. Also use hybrid when the question asks for tactical analysis or patterns backed by data, or when the user asks about player performance including both stats and tactical reasoning. When in doubt, prefer hybrid — richer context never hurts.
- For clickhouse or hybrid, set "query_type" to one of: "top_players_by_metric", "player_stat", "round_aggregate", "match_summary".
- For clickhouse or hybrid, extract "metric" from the question: "thru_smoke", "is_headshot", "kills", "assisted_flash", "no_scope", "attacker_blind", "wallbang_count", "bomb_planted", "bomb_defused", "bomb_exploded".
- For clickhouse or hybrid, extract "player_name" if a specific player is mentioned.
- For qdrant or hybrid, set "search_query" to a concise English search phrase capturing the semantic intent.
- "limit" defaults to 5.

Output ONLY valid JSON, no markdown, no explanation.

Examples:
Q: "quem fez mais smokes?"
A: {"target":"clickhouse","query_type":"top_players_by_metric","metric":"thru_smoke","limit":5}

Q: "quantos headshots o Fallen acertou?"
A: {"target":"clickhouse","query_type":"player_stat","metric":"is_headshot","player_name":"Fallen"}

Q: "qual round teve mais kills?"
A: {"target":"clickhouse","query_type":"round_aggregate","metric":"kills","limit":1}

Q: "o que fez o jogador smokar o CT?"
A: {"target":"qdrant","search_query":"player using smoke against CT team tactical play"}

Q: "como o time T ganhou o round 15?"
A: {"target":"qdrant","search_query":"round 15 T team victory reason tactics"}

Q: "teve clutch nessa partida?"
A: {"target":"qdrant","search_query":"clutch situation 1vX round win"}

Q: "quantas kills o s1mple fez?"
A: {"target":"clickhouse","query_type":"player_stat","metric":"kills","player_name":"s1mple"}

Q: "como o Fallen performou nas clutches?"
A: {"target":"hybrid","query_type":"player_stat","metric":"kills","player_name":"Fallen","search_query":"Fallen clutch situation tactical play"}

Q: "por que os CTs perderam tantos rounds na dust2?"
A: {"target":"hybrid","query_type":"round_aggregate","metric":"bomb_exploded","search_query":"CT team losing rounds de_dust2 tactical analysis"}

Q: "quem foi o melhor jogador e por quê?"
A: {"target":"hybrid","query_type":"top_players_by_metric","metric":"kills","limit":5,"search_query":"best player performance MVP tactical impact"}

Q: "analisa a performance do time T nos rounds que teve planta"
A: {"target":"hybrid","query_type":"round_aggregate","metric":"bomb_planted","search_query":"T team bomb plant execution strategy"}`

// QueryRouter classifies natural-language questions into structured query intents
// using an LLM, enabling safe routing between ClickHouse and Qdrant.
type QueryRouter struct {
	llmSvc ports.LLMService
}

// NewQueryRouter creates a new QueryRouter.
func NewQueryRouter(llmSvc ports.LLMService) *QueryRouter {
	return &QueryRouter{llmSvc: llmSvc}
}

// Classify sends the user's question to the LLM with a few-shot prompt and
// returns a structured QueryIntent.
func (qr *QueryRouter) Classify(ctx context.Context, question string, prevMessages []entities.Message) (*QueryIntent, error) {
	var historyBuilder strings.Builder
	if len(prevMessages) > 0 {
		historyBuilder.WriteString("Previous conversation context:\n")
		for _, msg := range prevMessages {
			historyBuilder.WriteString(fmt.Sprintf("User: %s\nAssistant: %s\n", msg.Question, msg.Answer))
		}
		historyBuilder.WriteString("\n")
	}

	prompt := fmt.Sprintf("%s\n\n%sQ: %s\nA:", classifierPrompt, historyBuilder.String(), question)

	raw, err := qr.llmSvc.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Clean markdown fences if the model wraps the JSON
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var intent QueryIntent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse classifier response %q: %w", raw, err)
	}

	if intent.Target != "clickhouse" && intent.Target != "qdrant" && intent.Target != "hybrid" {
		return nil, fmt.Errorf("unknown target %q: expected clickhouse, qdrant, or hybrid", intent.Target)
	}

	if intent.Limit <= 0 {
		intent.Limit = 5
	}

	return &intent, nil
}

package http

import (
	"log"
	"net/http"
	"strconv"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// MatchHandler handles HTTP requests for match listing and retrieval.
// It depends on the MatchRepository output port and extracts user identity from the request context.
type MatchHandler struct {
	matchRepo ports.MatchRepository
}

// NewMatchHandler creates a new MatchHandler.
func NewMatchHandler(matchRepo ports.MatchRepository) *MatchHandler {
	return &MatchHandler{matchRepo: matchRepo}
}

// HandleListMatches handles GET /api/v1/matches — lists matches for the authenticated user.
func (h *MatchHandler) HandleListMatches(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > 50 {
			respondError(w, http.StatusBadRequest, "limit must be between 1 and 50")
			return
		}
		limit = v
	}
	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil || v < 0 {
			respondError(w, http.StatusBadRequest, "offset must be non-negative")
			return
		}
		offset = v
	}

	matches, err := h.matchRepo.ListByUserID(r.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("[Match] ListMatches error for user=%s: %v", userID, err)
		respondError(w, http.StatusInternalServerError, "Failed to list matches")
		return
	}

	if matches == nil {
		matches = []*entities.Match{}
	}

	items := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		items = append(items, matchToJSON(m))
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"matches": items,
			"limit":   limit,
			"offset":  offset,
			"count":   len(items),
		},
	})
}

// HandleGetMatch handles GET /api/v1/matches/{id} — retrieves a single match by ID.
// Ownership is enforced: users can only access their own matches.
func (h *MatchHandler) HandleGetMatch(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	matchID := r.PathValue("id")
	if matchID == "" {
		respondError(w, http.StatusBadRequest, "Match ID is required")
		return
	}

	match, err := h.matchRepo.FindByID(r.Context(), matchID)
	if err != nil {
		log.Printf("[Match] GetMatch error for id=%s: %v", matchID, err)
		respondError(w, http.StatusInternalServerError, "Failed to get match")
		return
	}
	if match == nil {
		respondError(w, http.StatusNotFound, "Match not found")
		return
	}

	// Ownership check: users can only access their own matches
	if match.UserID != userID {
		respondError(w, http.StatusNotFound, "Match not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    matchToJSON(match),
	})
}

// matchToJSON converts a Match entity to a JSON-safe map, handling nil pointer fields.
func matchToJSON(m *entities.Match) map[string]any {
	result := map[string]any{
		"id":         m.ID,
		"user_id":    m.UserID,
		"status":     string(m.Status),
		"created_at": m.CreatedAt,
		"updated_at": m.UpdatedAt,
	}

	if m.TeamA != nil {
		result["team_a"] = *m.TeamA
	}
	if m.TeamB != nil {
		result["team_b"] = *m.TeamB
	}
	if m.DemoMD5 != nil {
		result["demo_md5"] = *m.DemoMD5
	}
	if m.ScoreA != nil {
		result["score_a"] = *m.ScoreA
	}
	if m.ScoreB != nil {
		result["score_b"] = *m.ScoreB
	}
	if m.TotalRounds != nil {
		result["total_rounds"] = *m.TotalRounds
	}
	if m.MapName != nil {
		result["map_name"] = string(*m.MapName)
	}

	return result
}

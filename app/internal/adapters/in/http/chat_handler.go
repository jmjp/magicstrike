package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
)

// sseChunk encodes a string as a JSON-quoted value so that embedded newlines
// are represented as \n literals and do not break the SSE framing protocol.
func sseChunk(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ChatHandler handles HTTP requests for conversational analytics and session management.
type ChatHandler struct {
	chatUseCase        ports.ChatUseCase
	chatSessionUseCase ports.ChatSessionUseCase
	rateLimiter        *services.RateLimiter
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(
	chatUseCase ports.ChatUseCase,
	chatSessionUseCase ports.ChatSessionUseCase,
	rateLimiter *services.RateLimiter,
) *ChatHandler {
	return &ChatHandler{
		chatUseCase:        chatUseCase,
		chatSessionUseCase: chatSessionUseCase,
		rateLimiter:        rateLimiter,
	}
}

// --- Request types ---

type createSessionRequest struct {
	MatchIDs []string `json:"match_ids"`
	Question string   `json:"question"`
}

type continueSessionRequest struct {
	Question string `json:"question"`
}

// --- Handlers ---

// HandleNewChat handles POST /api/v1/chat -- creates a new chat session.
func (h *ChatHandler) HandleNewChat(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req createSessionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Question = strings.TrimSpace(req.Question)

	// Validate and deduplicate match IDs
	seen := make(map[string]bool)
	var cleanIDs []string
	for _, mid := range req.MatchIDs {
		mid = strings.TrimSpace(mid)
		if mid != "" && !seen[mid] {
			seen[mid] = true
			cleanIDs = append(cleanIDs, mid)
		}
	}

	if len(cleanIDs) == 0 {
		respondError(w, http.StatusBadRequest, "match_ids is required and must contain at least one non-empty match ID")
		return
	}
	if len(cleanIDs) > 20 {
		respondError(w, http.StatusBadRequest, "match_ids is limited to 20 matches per query")
		return
	}
	if req.Question == "" {
		respondError(w, http.StatusBadRequest, "question is required")
		return
	}
	if len(req.Question) > 500 {
		respondError(w, http.StatusBadRequest, "question must be at most 500 characters")
		return
	}

	result, err := h.chatUseCase.NewSession(r.Context(), userID, cleanIDs, req.Question)
	if err != nil {
		log.Printf("[Chat] NewSession error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to process your question. Please try again.")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data": map[string]any{
			"session_id":   result.SessionID,
			"answer":       result.Answer,
			"source":       result.Source,
			"matches_used": result.MatchesUsed,
			"data_points":  result.DataPoints,
		},
	})
}

// HandleContinueChat handles POST /api/v1/chat/{id} -- continues an existing session.
func (h *ChatHandler) HandleContinueChat(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	var req continueSessionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		respondError(w, http.StatusBadRequest, "question is required")
		return
	}
	if len(req.Question) > 500 {
		respondError(w, http.StatusBadRequest, "question must be at most 500 characters")
		return
	}

	result, err := h.chatUseCase.ContinueSession(r.Context(), userID, sessionID, req.Question)
	if err != nil {
		log.Printf("[Chat] ContinueSession error: %v", err)
		if strings.Contains(err.Error(), "session not found") ||
			strings.Contains(err.Error(), "session has expired") {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to process your question. Please try again.")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"session_id":   result.SessionID,
			"answer":       result.Answer,
			"source":       result.Source,
			"matches_used": result.MatchesUsed,
			"data_points":  result.DataPoints,
		},
	})
}

// HandleListSessions handles GET /api/v1/chat -- lists user sessions.
func (h *ChatHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.chatSessionUseCase.List(r.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("[Chat] ListSessions error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list sessions.")
		return
	}

	// Build summary response (no full messages)
	summaries := make([]map[string]any, 0, len(result.Sessions))
	for _, s := range result.Sessions {
		lastQuestion := ""
		if len(s.Messages) > 0 {
			lastQuestion = s.Messages[len(s.Messages)-1].Question
		}
		summaries = append(summaries, map[string]any{
			"id":            s.ID,
			"match_ids":     s.MatchIDs,
			"message_count": len(s.Messages),
			"last_question": lastQuestion,
			"created_at":    s.CreatedAt,
			"updated_at":    s.UpdatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"sessions": summaries,
			"total":    result.Total,
			"limit":    result.Limit,
			"offset":   result.Offset,
		},
	})
}

// HandleGetSession handles GET /api/v1/chat/{id} -- gets session detail with messages.
func (h *ChatHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.chatSessionUseCase.Get(r.Context(), userID, sessionID)
	if err != nil {
		log.Printf("[Chat] GetSession error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get session.")
		return
	}
	if session == nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Reverse messages to show newest first (client-facing order), limit to 10
	messages := session.Messages
	if len(messages) > 10 {
		messages = messages[len(messages)-10:]
	}
	reversed := make([]map[string]any, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		reversed = append(reversed, map[string]any{
			"question":    m.Question,
			"answer":      m.Answer,
			"source":      m.Source,
			"data_points": m.DataPoints,
			"created_at":  m.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id":         session.ID,
			"match_ids":  session.MatchIDs,
			"messages":   reversed,
			"created_at": session.CreatedAt,
			"updated_at": session.UpdatedAt,
			"expires_at": session.ExpiresAt,
		},
	})
}

// HandleDeleteSession handles DELETE /api/v1/chat/{id} -- deletes a session.
func (h *ChatHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.chatSessionUseCase.Get(r.Context(), userID, sessionID)
	if err != nil {
		log.Printf("[Chat] GetSession before delete error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to delete session.")
		return
	}
	if session == nil {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	if err := h.chatSessionUseCase.Delete(r.Context(), userID, sessionID); err != nil {
		log.Printf("[Chat] DeleteSession error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to delete session.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleNewChatStream handles POST /api/v1/chat/stream -- creates a new streaming chat session.
func (h *ChatHandler) HandleNewChatStream(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}
	clientIP = strings.TrimSpace(clientIP)

	var rateLimitKey string
	var userID string
	actualUserID := GetUserID(r.Context())
	if actualUserID != "" {
		rateLimitKey = "chat:stream:user:" + actualUserID
		userID = actualUserID
	} else {
		rateLimitKey = "chat:stream:ip:" + clientIP
		userID = "anonymous-stream-user"
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(rateLimitKey) {
		respondError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	var req createSessionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Question = strings.TrimSpace(req.Question)

	// Validate and deduplicate match IDs
	seen := make(map[string]bool)
	var cleanIDs []string
	for _, mid := range req.MatchIDs {
		mid = strings.TrimSpace(mid)
		if mid != "" && !seen[mid] {
			seen[mid] = true
			cleanIDs = append(cleanIDs, mid)
		}
	}

	if len(cleanIDs) == 0 {
		respondError(w, http.StatusBadRequest, "match_ids is required and must contain at least one non-empty match ID")
		return
	}
	if len(cleanIDs) > 20 {
		respondError(w, http.StatusBadRequest, "match_ids is limited to 20 matches per query")
		return
	}
	if req.Question == "" {
		respondError(w, http.StatusBadRequest, "question is required")
		return
	}
	if len(req.Question) > 500 {
		respondError(w, http.StatusBadRequest, "question must be at most 500 characters")
		return
	}

	streamResp, err := h.chatUseCase.NewSessionStream(r.Context(), userID, cleanIDs, req.Question)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
			log.Printf("[Chat] NewSessionStream error: %v", err)
		}
		respondError(w, http.StatusInternalServerError, "Failed to process your question. Please try again.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case err, ok := <-streamResp.ErrChan:
			if ok && err != nil {
				log.Printf("[Chat] SSE stream error: %v", err)
				return
			}
		case chunk, ok := <-streamResp.Stream:
			if !ok {
				// Stream finished — send terminal event and flush.
				fmt.Fprintf(w, "event: done\ndata: [DONE]\n\n")
				flusher.Flush()
				return
			}
			// JSON-encode the chunk so embedded newlines don't break SSE framing.
			fmt.Fprintf(w, "data: %s\n\n", sseChunk(chunk))
			flusher.Flush()
		}
	}
}

// HandleContinueChatStream handles POST /api/v1/chat/stream/{id} -- continues an existing streaming chat session.
func (h *ChatHandler) HandleContinueChatStream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}
	clientIP = strings.TrimSpace(clientIP)

	var rateLimitKey string
	var userID string
	actualUserID := GetUserID(r.Context())
	if actualUserID != "" {
		rateLimitKey = "chat:stream:user:" + actualUserID
		userID = actualUserID
	} else {
		rateLimitKey = "chat:stream:ip:" + clientIP
		userID = "anonymous-stream-user"
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(rateLimitKey) {
		respondError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	var req continueSessionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		respondError(w, http.StatusBadRequest, "question is required")
		return
	}
	if len(req.Question) > 500 {
		respondError(w, http.StatusBadRequest, "question must be at most 500 characters")
		return
	}

	streamResp, err := h.chatUseCase.ContinueSessionStream(r.Context(), userID, sessionID, req.Question)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
			log.Printf("[Chat] ContinueSessionStream error: %v", err)
		}
		if strings.Contains(err.Error(), "session not found") ||
			strings.Contains(err.Error(), "session has expired") {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to process your question. Please try again.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case err, ok := <-streamResp.ErrChan:
			if ok && err != nil {
				log.Printf("[Chat] SSE stream error: %v", err)
				return
			}
		case chunk, ok := <-streamResp.Stream:
			if !ok {
				// Stream finished — send terminal event and flush.
				fmt.Fprintf(w, "event: done\ndata: [DONE]\n\n")
				flusher.Flush()
				return
			}
			// JSON-encode the chunk so embedded newlines don't break SSE framing.
			fmt.Fprintf(w, "data: %s\n\n", sseChunk(chunk))
			flusher.Flush()
		}
	}
}


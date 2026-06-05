package http

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"magicstrike/internal/core/ports"
)

const sessionTTL = 7 * 24 * time.Hour

// AuthHandler handles HTTP requests for passwordless authentication.
type AuthHandler struct {
	useCase   ports.AuthUseCase
	blocklist *TokenBlocklist
}

// NewAuthHandler creates a new AuthHandler instance.
func NewAuthHandler(useCase ports.AuthUseCase, blocklist *TokenBlocklist) *AuthHandler {
	return &AuthHandler{useCase: useCase, blocklist: blocklist}
}

// --- Request types ---

type requestMagicLinkRequest struct {
	Email string `json:"email"`
}

type callbackRequest struct {
	Token string `json:"token"`
}

// --- Public handlers (no auth required) ---

// HandleRequestMagicLink handles POST /api/v1/auth/magic-link
func (h *AuthHandler) HandleRequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req requestMagicLinkRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Always return 202 to prevent email enumeration
	if err := h.useCase.RequestMagicLink(r.Context(), req.Email); err != nil {
		log.Printf("[Auth] RequestMagicLink error (non-fatal): %v", err)
	}

	respondJSON(w, http.StatusAccepted, map[string]any{
		"success": true,
		"message": "If the email exists, a magic link has been sent.",
	})
}

// HandleCallback handles POST /api/v1/auth/callback
func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var req callbackRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "Token is required")
		return
	}

	result, err := h.useCase.ValidateToken(r.Context(), req.Token)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid or expired token.")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"access_token": result.AccessToken,
			"session_id":   result.SessionID,
			"user":         result.User,
			"expires_at":   result.ExpiresAt,
		},
	})
}

// --- Protected handlers (JWT required via AuthMiddleware) ---

// HandleRefresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	sessionID := GetSessionID(r.Context())
	if sessionID == "" {
		respondError(w, http.StatusUnauthorized, "Missing session — auth middleware required")
		return
	}

	result, err := h.useCase.RefreshSession(r.Context(), sessionID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid or expired session.")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"access_token": result.AccessToken,
			"session_id":   result.SessionID,
			"user":         result.User,
			"expires_at":   result.ExpiresAt,
		},
	})
}

// HandleLogout handles DELETE /api/v1/auth/session
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := GetSessionID(r.Context())
	if sessionID == "" {
		respondError(w, http.StatusUnauthorized, "Missing session — auth middleware required")
		return
	}

	if err := h.useCase.Logout(r.Context(), sessionID); err != nil {
		log.Printf("[Auth] Logout error (non-fatal): %v", err)
	}

	// Revoke the JWT so it cannot be reused before natural expiry
	h.blocklist.Revoke(sessionID, time.Now().Add(sessionTTL))

	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

func respondJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("[HTTP] Failed to encode JSON response: %v", err)
	}
}

func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]any{
		"success": false,
		"message": message,
	})
}

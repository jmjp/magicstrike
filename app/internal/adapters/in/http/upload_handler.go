package http

import (
	"encoding/json"
	"log"
	"net/http"

	"magicstrike/internal/core/ports"
)

// UploadHandler handles HTTP requests for demo file upload operations.
// It depends on the UploadUseCase input port and extracts user identity from the request context.
type UploadHandler struct {
	uploadUC ports.UploadUseCase
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(uploadUC ports.UploadUseCase) *UploadHandler {
	return &UploadHandler{uploadUC: uploadUC}
}

// uploadRequest is the JSON body for POST /api/v1/demos/upload-request.
type uploadRequestBody struct {
	MatchID  string  `json:"match_id"`
	Filename string  `json:"filename"`
	MD5Hash  *string `json:"md5_hash,omitempty"`
	TeamA    string  `json:"team_a"`
	TeamB    string  `json:"team_b"`
	MapName  string  `json:"map_name"`
}

// uploadResponse is the JSON body returned from upload-request.
type uploadResponseBody struct {
	UploadURL string `json:"upload_url"`
	BucketKey string `json:"bucket_key"`
	ExpiresAt string `json:"expires_at"`
	MatchID   string `json:"match_id"`
}

// confirmUploadBody is the JSON body for POST /api/v1/demos/upload-confirm.
type confirmUploadBody struct {
	MatchID   string `json:"match_id"`
	BucketKey string `json:"bucket_key"`
}

// errorResponse follows RFC 7807 Problem Details format.
type errorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

// HandleRequestUpload handles POST /api/v1/demos/upload-request.
// It generates a presigned URL for direct upload to S3/Minio storage.
func (h *UploadHandler) HandleRequestUpload(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT context (set by auth middleware)
	userID := GetUserID(r.Context())
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, "Authentication required", "User ID not found in request context")
		return
	}

	// Parse request body
	var body uploadRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Build the port request (UserID comes from JWT, not from JSON body)
	req := &ports.UploadRequest{
		UserID:   userID,
		MatchID:  body.MatchID,
		Filename: body.Filename,
		MD5Hash:  body.MD5Hash,
		TeamA:    body.TeamA,
		TeamB:    body.TeamB,
		MapName:  body.MapName,
	}

	resp, err := h.uploadUC.RequestUpload(r.Context(), req)
	if err != nil {
		log.Printf("[upload] RequestUpload failed for user=%s match=%s: %v", userID, body.MatchID, err)
		writeError(w, r, mapUploadError(err), "Upload request failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, uploadResponseBody{
		UploadURL: resp.UploadURL,
		BucketKey: resp.BucketKey,
		ExpiresAt: resp.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		MatchID:   resp.MatchID,
	})
}

// HandleConfirmUpload handles POST /api/v1/demos/upload-confirm.
// It verifies the uploaded file and enqueues a processing job.
func (h *UploadHandler) HandleConfirmUpload(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT context
	userID := GetUserID(r.Context())
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, "Authentication required", "User ID not found in request context")
		return
	}

	// Parse request body
	var body confirmUploadBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	req := &ports.ConfirmUploadRequest{
		UserID:    userID,
		MatchID:   body.MatchID,
		BucketKey: body.BucketKey,
	}

	if err := h.uploadUC.ConfirmUpload(r.Context(), req); err != nil {
		log.Printf("[upload] ConfirmUpload failed for match=%s: %v", body.MatchID, err)
		writeError(w, r, mapUploadError(err), "Upload confirmation failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status":   "queued",
		"match_id": body.MatchID,
	})
}

// ServeHTTP implements http.Handler for routing upload requests.
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/demos/upload-request":
		if r.Method != http.MethodPost {
			writeError(w, r, http.StatusMethodNotAllowed, "Method not allowed", "Use POST")
			return
		}
		h.HandleRequestUpload(w, r)

	case "/api/v1/demos/upload-confirm":
		if r.Method != http.MethodPost {
			writeError(w, r, http.StatusMethodNotAllowed, "Method not allowed", "Use POST")
			return
		}
		h.HandleConfirmUpload(w, r)

	default:
		writeError(w, r, http.StatusNotFound, "Not found", "")
	}
}

// mapUploadError maps domain errors to appropriate HTTP status codes.
func mapUploadError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	errStr := err.Error()

	// Check for wrapped sentinel errors via string matching
	switch {
	case contains(errStr, "invalid upload request"),
		contains(errStr, "validation"):
		return http.StatusBadRequest

	case contains(errStr, "user not found"):
		return http.StatusUnauthorized

	case contains(errStr, "match not found"),
		contains(errStr, "object not found"):
		return http.StatusNotFound

	case contains(errStr, "duplicate"),
		contains(errStr, "already uses this MD5"):
		return http.StatusConflict

	case contains(errStr, "storage service is unavailable"),
		contains(errStr, "message queue is unavailable"):
		return http.StatusServiceUnavailable

	default:
		return http.StatusInternalServerError
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("[upload] Failed to encode JSON response: %v", err)
	}
}

// writeError writes an RFC 7807 error response.
func writeError(w http.ResponseWriter, _ *http.Request, status int, title, detail string) {
	writeJSON(w, status, errorResponse{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

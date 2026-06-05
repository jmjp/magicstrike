package http

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"magicstrike/internal/core/services"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	sessionIDKey contextKey = "session_id"
)

// GetUserID extracts the authenticated user ID from the request context.
func GetUserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// GetSessionID extracts the session ID from the request context.
func GetSessionID(ctx context.Context) string {
	id, _ := ctx.Value(sessionIDKey).(string)
	return id
}

// TokenBlocklist tracks revoked JWT session IDs until their natural expiry.
type TokenBlocklist struct {
	mu              sync.RWMutex
	revoked         map[string]time.Time // sessionID -> when the JWT expires (auto-cleanup)
	cleanupInterval time.Duration
	stopChan        chan struct{}
}

// NewTokenBlocklist creates a new TokenBlocklist.
func NewTokenBlocklist() *TokenBlocklist {
	bl := &TokenBlocklist{
		revoked:         make(map[string]time.Time),
		cleanupInterval: 1 * time.Minute,
		stopChan:        make(chan struct{}),
	}
	// Background cleanup of expired entries every minute
	go bl.cleanupLoop()
	return bl
}

// Revoke adds a session ID to the blocklist until its JWT would expire.
func (bl *TokenBlocklist) Revoke(sessionID string, jwtExpiresAt time.Time) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.revoked[sessionID] = jwtExpiresAt
}

// IsRevoked checks if a session ID is currently blocked.
func (bl *TokenBlocklist) IsRevoked(sessionID string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	_, revoked := bl.revoked[sessionID]
	return revoked
}

func (bl *TokenBlocklist) cleanupLoop() {
	interval := bl.cleanupInterval
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bl.mu.Lock()
			now := time.Now()
			for id, exp := range bl.revoked {
				if now.After(exp) {
					delete(bl.revoked, id)
				}
			}
			bl.mu.Unlock()
		case <-bl.stopChan:
			return
		}
	}
}

// Close stops the background cleanup loop.
func (bl *TokenBlocklist) Close() {
	if bl.stopChan != nil {
		select {
		case <-bl.stopChan:
			// Already closed
		default:
			close(bl.stopChan)
		}
	}
}

// AuthMiddleware creates HTTP middleware that verifies the JWT access token
// from the Authorization header and injects user_id + session_id into the context.
func AuthMiddleware(jwtSvc *services.JWTService, blocklist *TokenBlocklist) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "Missing Authorization header")
				return
			}

			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || token == "" {
				respondError(w, http.StatusUnauthorized, "Invalid Authorization header format. Expected: Bearer <token>")
				return
			}

			// Verify JWT signature and extract claims (zero DB calls)
			claims, err := jwtSvc.Verify(token)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			// Check if session was revoked (logged out)
			if blocklist.IsRevoked(claims.ID) {
				respondError(w, http.StatusUnauthorized, "Token has been revoked")
				return
			}

			// Inject user_id and session_id into context
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, sessionIDKey, claims.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CorsMiddleware handles Cross-Origin Resource Sharing (CORS) headers and preflight OPTIONS requests.
func CorsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				matched := false
				for _, o := range allowedOrigins {
					if o == "*" || o == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						matched = true
						break
					}
				}
				// If nothing matched but allowedOrigins is empty, default to allowing all in development
				if !matched && len(allowedOrigins) == 0 {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			} else if len(allowedOrigins) > 0 && allowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

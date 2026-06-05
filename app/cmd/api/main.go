package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	httpadapter "magicstrike/internal/adapters/in/http"
	"magicstrike/internal/adapters/out/clickhouse"
	"magicstrike/internal/adapters/out/deepseek"
	"magicstrike/internal/adapters/out/email"
	"magicstrike/internal/adapters/out/memory"
	"magicstrike/internal/adapters/out/minio"
	"magicstrike/internal/adapters/out/postgres"
	"magicstrike/internal/adapters/out/qdrant"
	"magicstrike/internal/adapters/out/rabbitmq"
	"magicstrike/internal/adapters/out/voyage"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"
	"magicstrike/internal/core/usecases"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file if it exists
	_ = godotenv.Load()

	port := envOr("PORT", "8080")

	mux, cleanup := buildMux()
	if cleanup != nil {
		defer cleanup()
	}

	// --- Global middleware ---
	var handler http.Handler = mux
	handler = recovererMiddleware(handler)
	handler = loggerMiddleware(handler)

	allowedOriginsStr := os.Getenv("CORS_ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsStr != "" {
		for _, s := range strings.Split(allowedOriginsStr, ",") {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(s))
		}
	} else {
		allowedOrigins = []string{"http://localhost:5173"}
	}
	handler = httpadapter.CorsMiddleware(allowedOrigins)(handler)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disable WriteTimeout to support long-lived SSE connections
		IdleTimeout:  60 * time.Second,
	}

	logServerEndpoints()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

// buildMux constructs the HTTP router with all routes wired to adapters.
// The cleanup function closes infrastructure connections (e.g., ClickHouse pool).
// In test scenarios where no external infra is configured, cleanup is nil.
func buildMux() (*http.ServeMux, func()) {
	jwtSecret := os.Getenv("JWT_SECRET")

	// --- Wiring: out adapters (auth & matches) ---
	var userRepo ports.UserRepository
	var sessionRepo ports.SessionRepository
	var magicStore ports.MagicLinkStore
	var matchRepo ports.MatchRepository

	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost != "" {
		pgPortStr := envOr("POSTGRES_PORT", "5432")
		pgPort, _ := strconv.Atoi(pgPortStr)
		pgUser := envOr("POSTGRES_USER", "postgres")
		pgPass := envOr("POSTGRES_PASSWORD", "postgres")
		pgDB := envOr("POSTGRES_DB", "magicstrike")
		pgSSL := envOr("POSTGRES_SSLMODE", "disable")

		pgPool, err := postgres.Connect(pgHost, pgPort, pgUser, pgPass, pgDB, pgSSL)
		if err != nil {
			log.Fatalf("[API] Failed to connect to Postgres: %v", err)
		}
		if err := postgres.MigrateSchema(context.Background(), pgPool); err != nil {
			log.Fatalf("[API] Failed to run Postgres migrations: %v", err)
		}
		userRepo = postgres.NewUserRepo(pgPool)
		sessionRepo = postgres.NewSessionRepo(pgPool)
		magicStore = postgres.NewMagicLinkStore(pgPool)
		matchRepo = postgres.NewMatchRepo(pgPool)
		log.Println("[API] Using Postgres database for Auth, Sessions & Matches")
	} else {
		userRepo = memory.NewUserRepo()
		sessionRepo = memory.NewSessionRepo()
		magicStore = memory.NewMagicLinkStore()
		matchRepo = memory.NewMatchRepo()
		log.Println("[API] Warning: POSTGRES_HOST not set. Using in-memory storage for Auth, Sessions & Matches")
	}
	emailSender := email.NewMockSender()

	// --- Wiring: domain services ---
	tokenGen := services.NewTokenGenerator()
	rateLimiter := services.NewRateLimiter()
	jwtSvc := services.NewJWTService(jwtSecret)

	// --- Wiring: auth infrastructure ---
	blocklist := httpadapter.NewTokenBlocklist()

	// --- Wiring: use cases ---
	authUseCase := usecases.NewAuthUseCase(userRepo, sessionRepo, magicStore, emailSender, tokenGen, rateLimiter, jwtSvc)

	// --- Wiring: Chat Use Case (analytics) ---
	chAddr := envOr("CLICKHOUSE_ADDR", "127.0.0.1:9000")
	chDB := envOr("CLICKHOUSE_DB", "magicstrike")
	chUser := envOr("CLICKHOUSE_USER", "default")
	chPass := envOr("CLICKHOUSE_PASSWORD", "test")
	qdURL := envOr("QDRANT_URL", "http://127.0.0.1:6333")
	dsKey := os.Getenv("DEEPSEEK_API_KEY")
	vyKey := os.Getenv("VOYAGE_API_KEY")

	ttlDays := envOrInt("CHAT_HISTORY_TTL_DAYS", 7)
	if ttlDays < 1 {
		ttlDays = 1
	} else if ttlDays > 365 {
		ttlDays = 365
	}

	var chatUseCase ports.ChatUseCase
	var chatSessionRepo ports.ChatSessionRepository
	var chatSessionUseCase ports.ChatSessionUseCase
	var cleanup func()

	chConn, err := clickhouse.Connect(chAddr, chDB, chUser, chPass)
	if err != nil {
		log.Printf("[API] Warning: ClickHouse unavailable — chat analytics will not work: %v", err)
	}
	if chConn != nil {
		if err := clickhouse.MigrateSchema(context.Background(), chConn); err != nil {
			log.Printf("[API] Warning: ClickHouse migration failed: %v", err)
		}
		eventRepo := clickhouse.NewEventRepository(chConn)
		qdClient := qdrant.NewClient(qdURL, "round_narratives")
		qdClient.CreateCollection(context.Background(), 1024)
		dsClient := deepseek.NewClient(dsKey, "", "")
		vyClient := voyage.NewClient(vyKey, "", "")

		// Chat Session Repository
		if err := clickhouse.MigrateChatSessionSchema(context.Background(), chConn); err != nil {
			log.Printf("[API] Warning: Chat session migration failed: %v", err)
		}
		chatSessionRepo = clickhouse.NewChatSessionRepo(chConn)
		log.Printf("[API] Chat history repository: ClickHouse (TTL: %d days)", ttlDays)

		// Chat Use Cases
		chatUseCase = usecases.NewChatUseCase(eventRepo, matchRepo, qdClient, vyClient, dsClient, chatSessionRepo, ttlDays)
		chatSessionUseCase = usecases.NewChatSessionUseCase(chatSessionRepo)
		log.Printf("[API] Chat analytics enabled.")

		cleanup = func() { chConn.Close() }
	} else {
		// Fallback in-memory for session management
		chatSessionRepo = memory.NewChatSessionRepo(ttlDays)
		chatSessionUseCase = usecases.NewChatSessionUseCase(chatSessionRepo)
		log.Printf("[API] Chat analytics using in-memory storage (TTL: %d days)", ttlDays)
	}

	// --- Wiring: Upload Use Case ---
	var uploadUseCase ports.UploadUseCase

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	rabbitmqURL := os.Getenv("RABBITMQ_URL")

	if minioEndpoint != "" && rabbitmqURL != "" {
		// Minio Storage
		minioCfg := minio.Config{
			Endpoint:        minioEndpoint,
			AccessKeyID:     envOr("MINIO_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: envOr("MINIO_SECRET_KEY", "minioadmin"),
			UseSSL:          os.Getenv("MINIO_USE_SSL") == "true",
			Region:          envOr("MINIO_REGION", "us-east-1"),
			UploadBucket:    envOr("MINIO_BUCKET", "magicstrike-demos"),
		}
		storageSvc, err := minio.NewStorage(minioCfg)
		if err != nil {
			log.Printf("[API] Warning: Failed to create Minio storage: %v", err)
		} else {
			if err := storageSvc.EnsureBucket(context.Background()); err != nil {
				log.Printf("[API] Warning: Failed to ensure Minio bucket: %v", err)
			} else {
				log.Printf("[API] Minio storage connected (bucket: %s)", minioCfg.UploadBucket)

				// RabbitMQ Publisher
				rmqCfg := rabbitmq.Config{
					URL:          rabbitmqURL,
					ExchangeName: envOr("RABBITMQ_EXCHANGE", "demo_uploads"),
					ExchangeType: "direct",
					QueueName:    envOr("RABBITMQ_QUEUE", "demo_processing"),
					RoutingKey:   envOr("RABBITMQ_ROUTING_KEY", "demo.upload"),
				}
				queuePub, err := rabbitmq.NewPublisher(rmqCfg)
				if err != nil {
					log.Printf("[API] Warning: Failed to create RabbitMQ publisher: %v", err)
				} else {
					log.Printf("[API] RabbitMQ publisher connected (exchange: %s, queue: %s)",
						rmqCfg.ExchangeName, rmqCfg.QueueName)

					// Wire UploadUseCase
					presignTTL := 15 * time.Minute
					if ttl := os.Getenv("PRESIGN_URL_TTL"); ttl != "" {
						if d, err := time.ParseDuration(ttl); err == nil {
							presignTTL = d
						}
					}
					uploadUseCase = usecases.NewUploadUseCase(
						matchRepo, userRepo, storageSvc, queuePub,
						minioCfg.UploadBucket, presignTTL,
					)
					log.Printf("[API] Upload endpoints enabled (presigned URL TTL: %v)", presignTTL)
				}
			}
		}
	} else {
		log.Printf("[API] Upload endpoints disabled — MINIO_ENDPOINT or RABBITMQ_URL not set")
	}

	// --- Wiring: in adapters ---
	authHandler := httpadapter.NewAuthHandler(authUseCase, blocklist)
	authMiddleware := httpadapter.AuthMiddleware(jwtSvc, blocklist)

	// --- Router ---
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.HandleFunc("POST /api/v1/auth/magic-link", authHandler.HandleRequestMagicLink)
	mux.HandleFunc("POST /api/v1/auth/callback", authHandler.HandleCallback)

	// Protected routes (JWT required)
	mux.Handle("POST /api/v1/auth/refresh", authMiddleware(http.HandlerFunc(authHandler.HandleRefresh)))
	mux.Handle("DELETE /api/v1/auth/session", authMiddleware(http.HandlerFunc(authHandler.HandleLogout)))

	// Match endpoints (JWT required, always available)
	matchHandler := httpadapter.NewMatchHandler(matchRepo)
	mux.Handle("GET /api/v1/matches", authMiddleware(http.HandlerFunc(matchHandler.HandleListMatches)))
	mux.Handle("GET /api/v1/matches/{id}", authMiddleware(http.HandlerFunc(matchHandler.HandleGetMatch)))

	// Chat analytics (JWT required + ClickHouse dependency)
	if chatUseCase != nil && chatSessionUseCase != nil {
		chatH := httpadapter.NewChatHandler(chatUseCase, chatSessionUseCase, rateLimiter)
		// Literal /stream routes registered before /{id} wildcard for clarity.
		// All chat routes use authMiddleware — streaming included.
		mux.Handle("POST /api/v1/chat/stream", authMiddleware(http.HandlerFunc(chatH.HandleNewChatStream)))
		mux.Handle("POST /api/v1/chat/stream/{id}", authMiddleware(http.HandlerFunc(chatH.HandleContinueChatStream)))
		mux.Handle("POST /api/v1/chat", authMiddleware(http.HandlerFunc(chatH.HandleNewChat)))
		mux.Handle("POST /api/v1/chat/{id}", authMiddleware(http.HandlerFunc(chatH.HandleContinueChat)))
		mux.Handle("GET /api/v1/chat", authMiddleware(http.HandlerFunc(chatH.HandleListSessions)))
		mux.Handle("GET /api/v1/chat/{id}", authMiddleware(http.HandlerFunc(chatH.HandleGetSession)))
		mux.Handle("DELETE /api/v1/chat/{id}", authMiddleware(http.HandlerFunc(chatH.HandleDeleteSession)))
	} else {
		serviceUnavailable := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"success":false,"message":"Chat analytics unavailable — ClickHouse is not connected"}`))
		}
		mux.HandleFunc("POST /api/v1/chat", serviceUnavailable)
		mux.HandleFunc("POST /api/v1/chat/{id}", serviceUnavailable)
		mux.HandleFunc("GET /api/v1/chat", serviceUnavailable)
		mux.HandleFunc("GET /api/v1/chat/{id}", serviceUnavailable)
		mux.HandleFunc("DELETE /api/v1/chat/{id}", serviceUnavailable)
		mux.HandleFunc("POST /api/v1/chat/stream", serviceUnavailable)
		mux.HandleFunc("POST /api/v1/chat/stream/{id}", serviceUnavailable)
	}

	// Upload endpoints (JWT required + Minio + RabbitMQ dependency)
	if uploadUseCase != nil {
		uploadH := httpadapter.NewUploadHandler(uploadUseCase)
		mux.Handle("POST /api/v1/demos/upload-request", authMiddleware(http.HandlerFunc(uploadH.HandleRequestUpload)))
		mux.Handle("POST /api/v1/demos/upload-confirm", authMiddleware(http.HandlerFunc(uploadH.HandleConfirmUpload)))
	} else {
		notImplemented := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			w.Write([]byte(`{"success":false,"message":"Upload endpoints unavailable — Minio or RabbitMQ not configured"}`))
		}
		mux.HandleFunc("POST /api/v1/demos/upload-request", notImplemented)
		mux.HandleFunc("POST /api/v1/demos/upload-confirm", notImplemented)
	}

	return mux, cleanup
}

func logServerEndpoints() {
	log.Printf("[API] Public endpoints:")
	log.Printf("[API]   POST /api/v1/auth/magic-link")
	log.Printf("[API]   POST /api/v1/auth/callback")
	log.Printf("[API] Protected endpoints (JWT required):")
	log.Printf("[API]   POST /api/v1/auth/refresh")
	log.Printf("[API]   DELETE /api/v1/auth/session")
	log.Printf("[API]   GET /api/v1/matches")
	log.Printf("[API]   GET /api/v1/matches/{id}")
	log.Printf("[API]   POST /api/v1/chat")
	log.Printf("[API]   POST /api/v1/chat/{id}")
	log.Printf("[API]   GET /api/v1/chat")
	log.Printf("[API]   GET /api/v1/chat/{id}")
	log.Printf("[API]   DELETE /api/v1/chat/{id}")
	log.Printf("[API]   POST /api/v1/demos/upload-request")
	log.Printf("[API]   POST /api/v1/demos/upload-confirm")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("← %s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func recovererMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, `{"success":false,"message":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

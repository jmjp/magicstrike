package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"magicstrike/internal/adapters/out/clickhouse"
	"magicstrike/internal/adapters/out/deepseek"
	"magicstrike/internal/adapters/out/memory"
	"magicstrike/internal/adapters/out/minio"
	"magicstrike/internal/adapters/out/postgres"
	"magicstrike/internal/adapters/out/qdrant"
	"magicstrike/internal/adapters/out/rabbitmq"
	"magicstrike/internal/adapters/out/voyage"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/usecases"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file if it exists
	_ = godotenv.Load()

	// Parse mode flag
	mode := flag.String("mode", "consumer", "Worker mode: 'cli' (single file) or 'consumer' (RabbitMQ)")
	demoPath := flag.String("demo", "", "Path to .dem file (CLI mode only)")
	flag.Parse()

	switch *mode {
	case "cli":
		runCLI(*demoPath)
	case "consumer":
		runConsumer()
	default:
		log.Fatalf("Unknown mode: %s. Use 'cli' or 'consumer'.", *mode)
	}
}

// runCLI processes a single .dem file from disk (original behavior).
func runCLI(demoPath string) {
	if demoPath == "" {
		log.Fatalf("Usage: %s -mode cli -demo <path-to-demo.dem>", os.Args[0])
	}

	file, err := os.Open(demoPath)
	if err != nil {
		log.Fatalf("Failed to open demo file: %v", err)
	}
	defer file.Close()

	baseName := filepath.Base(demoPath)
	matchID := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	if matchID == "" {
		matchID = "match-" + fmt.Sprintf("%d", time.Now().Unix())
	}
	log.Printf("[Worker][CLI] Match ID: %s", matchID)

	ctx := context.Background()
	eventRepo, qdClient := initInfrastructure(ctx)

	dsKey := os.Getenv("DEEPSEEK_API_KEY")
	vyKey := os.Getenv("VOYAGE_API_KEY")
	dsClient := deepseek.NewClient(dsKey, "", "")
	vyClient := voyage.NewClient(vyKey, "", "")

	mockMatchRepo := memory.NewMatchRepo()
	parserSvc := usecases.NewParserService(eventRepo)
	narrativeSvc := usecases.NewNarrativeService(eventRepo, mockMatchRepo, dsClient, vyClient, qdClient)
	ingestionUC := usecases.NewIngestionUseCase(parserSvc, narrativeSvc, eventRepo, mockMatchRepo)

	log.Printf("[Worker][CLI] Starting ingestion pipeline for match %s...", matchID)
	start := time.Now()

	if err := ingestionUC.IngestDemo(ctx, matchID, file); err != nil {
		log.Fatalf("[Worker][CLI] Ingestion pipeline failed: %v", err)
	}

	log.Printf("[Worker][CLI] Ingestion completed in %v for match %s!", time.Since(start), matchID)
}

// runConsumer runs the worker as a long-lived RabbitMQ consumer.
func runConsumer() {
	log.Printf("[Worker][Consumer] Starting RabbitMQ demo processing consumer...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Infrastructure ---
	eventRepo, qdClient := initInfrastructure(ctx)

	// --- External Services ---
	dsKey := os.Getenv("DEEPSEEK_API_KEY")
	vyKey := os.Getenv("VOYAGE_API_KEY")
	if dsKey == "" {
		log.Printf("[Worker][Consumer] Warning: DEEPSEEK_API_KEY not set — using rule-based fallback narratives.")
	}
	dsClient := deepseek.NewClient(dsKey, "", "")
	if vyKey == "" {
		log.Printf("[Worker][Consumer] Warning: VOYAGE_API_KEY not set — using zero-filled mock embeddings.")
	}
	vyClient := voyage.NewClient(vyKey, "", "")

	// --- Storage (Minio) ---
	minioCfg := minio.Config{
		Endpoint:        envOr("MINIO_ENDPOINT", "127.0.0.1:9000"),
		AccessKeyID:     envOr("MINIO_ACCESS_KEY", "minioadmin"),
		SecretAccessKey: envOr("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:          os.Getenv("MINIO_USE_SSL") == "true",
		Region:          envOr("MINIO_REGION", "us-east-1"),
		UploadBucket:    envOr("MINIO_BUCKET", "magicstrike-demos"),
	}
	storageSvc, err := minio.NewStorage(minioCfg)
	if err != nil {
		log.Fatalf("[Worker][Consumer] Failed to create Minio storage: %v", err)
	}
	if err := storageSvc.EnsureBucket(ctx); err != nil {
		log.Fatalf("[Worker][Consumer] Failed to ensure Minio bucket exists: %v", err)
	}
	log.Printf("[Worker][Consumer] Minio storage connected (bucket: %s)", minioCfg.UploadBucket)

	// --- RabbitMQ Subscriber ---
	rmqCfg := rabbitmq.Config{
		URL:          envOr("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		ExchangeName: envOr("RABBITMQ_EXCHANGE", "demo_uploads"),
		ExchangeType: "direct",
		QueueName:    envOr("RABBITMQ_QUEUE", "demo_processing"),
		RoutingKey:   envOr("RABBITMQ_ROUTING_KEY", "demo.upload"),
	}
	subscriber := rabbitmq.NewSubscriber(rmqCfg)

	// --- MatchRepository ---
	matchRepo := buildMatchRepo(ctx)

	// --- Use Cases ---
	parserSvc := usecases.NewParserService(eventRepo)
	narrativeSvc := usecases.NewNarrativeService(eventRepo, matchRepo, dsClient, vyClient, qdClient)
	ingestionUC := usecases.NewIngestionUseCase(parserSvc, narrativeSvc, eventRepo, matchRepo)

	// --- Signal Handling ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[Worker][Consumer] Received signal %v, initiating graceful shutdown...", sig)
		cancel()
		subscriber.Shutdown()
	}()

	// --- Consumer Loop ---
	log.Printf("[Worker][Consumer] Listening for demo processing jobs on queue %q...", rmqCfg.QueueName)

	if err := subscriber.SubscribeDemoJobs(ctx, func(handlerCtx context.Context, job *ports.DemoJobMessage) error {
		return processJob(handlerCtx, job, storageSvc, matchRepo, ingestionUC)
	}); err != nil {
		log.Fatalf("[Worker][Consumer] Fatal consumer error: %v", err)
	}

	log.Printf("[Worker][Consumer] Graceful shutdown complete.")
}

// processJob handles a single demo processing job from the queue.
func processJob(
	ctx context.Context,
	job *ports.DemoJobMessage,
	storageSvc ports.StorageService,
	matchRepo ports.MatchRepository,
	ingestionUC ports.IngestionUseCase,
) error {
	matchID := job.MatchID
	log.Printf("[Worker][Consumer] Processing job: match=%s bucket=%s", matchID, job.BucketPath)

	// 1. Dedup guard: check if match already exists with a terminal status
	existing, err := matchRepo.FindByDemoMD5(ctx, job.MD5Hash)
	if err != nil {
		return fmt.Errorf("dedup check failed: %w", err)
	}
	if existing != nil && existing.ID != matchID {
		// Same file already ingested under a different match ID — skip
		log.Printf("[Worker][Consumer] Match %s: duplicate of existing match %s (MD5 %s), skipping",
			matchID, existing.ID, job.MD5Hash)
		return nil
	}

	// Check if THIS match was already processed
	match, err := matchRepo.FindByID(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to find match: %w", err)
	}
	if match != nil {
		switch match.Status {
		case "finished", "failed":
			log.Printf("[Worker][Consumer] Match %s already in terminal state '%s', skipping", matchID, match.Status)
			return nil
		}
	}

	// 2. Download .dem from storage with size limit
	maxFileSize := getMaxFileSize()
	log.Printf("[Worker][Consumer] Downloading %s from bucket (max size: %d bytes)...", job.BucketPath, maxFileSize)
	reader, err := storageSvc.Download(ctx, job.BucketPath)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", job.BucketPath, err)
	}
	defer reader.Close()

	// Wrap reader with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(reader, maxFileSize+1) // +1 to detect truncation
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read downloaded file: %w", err)
	}
	if int64(len(data)) > maxFileSize {
		return fmt.Errorf("demo file exceeds maximum size of %d bytes (%d bytes read)",
			maxFileSize, len(data))
	}

	// 3. Verify MD5 hash
	hash := md5.Sum(data)
	computedMD5 := hex.EncodeToString(hash[:])
	if !strings.EqualFold(job.MD5Hash, computedMD5) {
		log.Printf("[Worker][Consumer] MD5 mismatch for match %s: expected %s, computed %s",
			matchID, job.MD5Hash, computedMD5)
		return fmt.Errorf("MD5 mismatch: expected %s, computed %s", job.MD5Hash, computedMD5)
	}
	log.Printf("[Worker][Consumer] MD5 verified: %s", computedMD5)

	// 4. Process via existing ingestion pipeline
	log.Printf("[Worker][Consumer] Starting ingestion for match %s...", matchID)
	start := time.Now()

	// Wrap data in a bytes.Reader for the ingestion pipeline
	pipeReader := bytes.NewReader(data)

	if err := ingestionUC.IngestDemo(ctx, matchID, pipeReader); err != nil {
		log.Printf("[Worker][Consumer] Ingestion failed for match %s: %v", matchID, err)
		return fmt.Errorf("ingestion failed for match %s: %w", matchID, err)
	}

	log.Printf("[Worker][Consumer] Ingestion completed for match %s in %v", matchID, time.Since(start))
	return nil
}

// buildMatchRepo wires the MatchRepository based on POSTGRES_HOST availability.
// Uses Postgres when available, falls back to in-memory storage.
func buildMatchRepo(ctx context.Context) ports.MatchRepository {
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
			log.Fatalf("[Worker] Failed to connect to Postgres: %v", err)
		}
		if err := postgres.MigrateSchema(ctx, pgPool); err != nil {
			log.Fatalf("[Worker] Failed to run Postgres migrations: %v", err)
		}
		matchRepo := postgres.NewMatchRepo(pgPool)
		log.Println("[Worker] Using Postgres database for Match deduplication & tracking")
		return matchRepo
	}
	log.Println("[Worker] Warning: POSTGRES_HOST not set. Using in-memory storage for Match deduplication & tracking")
	return memory.NewMatchRepo()
}

// initInfrastructure connects to ClickHouse and Qdrant with the standard env var configuration.
func initInfrastructure(ctx context.Context) (ports.EventRepository, ports.VectorRepository) {
	chAddr := envOr("CLICKHOUSE_ADDR", "127.0.0.1:9000")
	chDB := envOr("CLICKHOUSE_DB", "magicstrike")
	chUser := envOr("CLICKHOUSE_USER", "default")
	chPass := envOr("CLICKHOUSE_PASSWORD", "test")
	qdURL := envOr("QDRANT_URL", "http://127.0.0.1:6333")

	// ClickHouse
	log.Printf("[Worker] Connecting to ClickHouse at %s (DB: %s)...", chAddr, chDB)
	chConn, err := clickhouse.Connect(chAddr, chDB, chUser, chPass)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	if err := clickhouse.MigrateSchema(ctx, chConn); err != nil {
		log.Fatalf("Failed to migrate ClickHouse schema: %v", err)
	}
	log.Printf("[Worker] ClickHouse ready.")
	eventRepo := clickhouse.NewEventRepository(chConn)

	// Qdrant
	log.Printf("[Worker] Connecting to Qdrant at %s...", qdURL)
	qdClient := qdrant.NewClient(qdURL, "round_narratives")
	if err := qdClient.CreateCollection(ctx, 1024); err != nil {
		log.Fatalf("Failed to configure Qdrant collection: %v", err)
	}
	log.Printf("[Worker] Qdrant collection 'round_narratives' ready.")

	return eventRepo, qdClient
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getMaxFileSize reads the MAX_DEMO_FILE_SIZE env var (in bytes) or returns the default 900MB.
func getMaxFileSize() int64 {
	defaultSize := int64(900 * 1024 * 1024) // 900 MB
	if v := os.Getenv("MAX_DEMO_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return defaultSize
}

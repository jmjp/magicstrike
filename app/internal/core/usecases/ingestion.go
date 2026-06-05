package usecases

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// IngestionUseCase implements ports.IngestionUseCase and orchestrates the
// end-to-end CS2 demo ingestion pipeline:
//
//  1. Parse .dem file stream and persist structured events to ClickHouse.
//  2. On parse failure, rollback partial inserts via DeleteByMatchID.
//  3. Generate per-round tactical narratives via LLM (with static fallback).
//  4. Produce vector embeddings via Voyage AI (with zero-vector fallback).
//  5. Upsert vector records into Qdrant for semantic search.
type IngestionUseCase struct {
	parser    *ParserService
	narrative *NarrativeService
	eventRepo ports.EventRepository
	matchRepo ports.MatchRepository
}

// NewIngestionUseCase creates a new IngestionUseCase instance.
func NewIngestionUseCase(
	parser *ParserService,
	narrative *NarrativeService,
	eventRepo ports.EventRepository,
	matchRepo ports.MatchRepository,
) ports.IngestionUseCase {
	return &IngestionUseCase{
		parser:    parser,
		narrative: narrative,
		eventRepo: eventRepo,
		matchRepo: matchRepo,
	}
}

// IngestDemo runs the full ingestion pipeline for a single CS2 demo file.
func (uc *IngestionUseCase) IngestDemo(ctx context.Context, matchID string, reader io.Reader) error {
	log.Printf("[Ingestion] Phase 1/2: Parsing demo stream for match %s...", matchID)

	// Update match to started if it is in waiting state
	if err := uc.updateMatchStatus(ctx, matchID, entities.MatchStatusStarted); err != nil {
		log.Printf("[Ingestion] Failed to update match status to started for match %s: %v", matchID, err)
	}

	parseResult, err := uc.parser.ParseStream(ctx, matchID, reader)
	if err != nil {
		log.Printf("[Ingestion] Parse failed for match %s: %v", matchID, err)
		log.Printf("[Ingestion] Rolling back partial inserts for match %s...", matchID)

		if rbErr := uc.eventRepo.DeleteByMatchID(ctx, matchID); rbErr != nil {
			log.Printf("[Ingestion] Rollback failed for match %s: %v", matchID, rbErr)
		}

		// Update match status to failed in Postgres
		if mErr := uc.updateMatchStatus(ctx, matchID, entities.MatchStatusFailed); mErr != nil {
			log.Printf("[Ingestion] Failed to update match status to failed for match %s: %v", matchID, mErr)
		}

		return fmt.Errorf("ingestion phase 1 (parse) failed for match %s: %w", matchID, err)
	}

	log.Printf("[Ingestion] Phase 2/2: Generating narratives and embeddings for match %s...", matchID)

	if err := uc.narrative.ProcessMatch(ctx, matchID); err != nil {
		// In case of narrative failure, also mark match as failed
		if mErr := uc.updateMatchStatus(ctx, matchID, entities.MatchStatusFailed); mErr != nil {
			log.Printf("[Ingestion] Failed to update match status to failed on narrative error for match %s: %v", matchID, mErr)
		}
		return fmt.Errorf("ingestion phase 2 (narrative) failed for match %s: %w", matchID, err)
	}

	// Update match details on success
	if err := uc.finalizeMatch(ctx, matchID, parseResult); err != nil {
		return fmt.Errorf("failed to finalize match %s in Postgres: %w", matchID, err)
	}

	log.Printf("[Ingestion] Pipeline completed successfully for match %s.", matchID)
	return nil
}

func (uc *IngestionUseCase) updateMatchStatus(ctx context.Context, matchID string, status entities.MatchStatus) error {
	if uc.matchRepo == nil {
		return nil
	}
	match, err := uc.matchRepo.FindByID(ctx, matchID)
	if err != nil {
		return err
	}
	if match == nil {
		return nil // Match not found in Postgres, nothing to update (e.g. CLI mode without PG)
	}
	if match.Status == status {
		return nil
	}
	if err := match.TransitionTo(status); err != nil {
		return err
	}
	return uc.matchRepo.Update(ctx, match)
}

func (uc *IngestionUseCase) finalizeMatch(ctx context.Context, matchID string, result *ports.ParseResult) error {
	if uc.matchRepo == nil {
		return nil
	}
	match, err := uc.matchRepo.FindByID(ctx, matchID)
	if err != nil {
		return err
	}
	if match == nil {
		return nil // No match in PG, skip update (CLI mode fallback)
	}

	// Transition match from its current status (started or waiting) to finished.
	// Since TransitionTo only allows finished from started, we ensure it's started first.
	if match.Status == entities.MatchStatusWaiting {
		if err := match.TransitionTo(entities.MatchStatusStarted); err != nil {
			return err
		}
	}

	if err := match.TransitionTo(entities.MatchStatusFinished); err != nil {
		return err
	}

	match.TeamA = &result.TeamA
	match.TeamB = &result.TeamB
	match.ScoreA = &result.ScoreA
	match.ScoreB = &result.ScoreB
	match.TotalRounds = &result.TotalRounds
	match.UpdatedAt = time.Now()

	if result.MapName != "" {
		sanitizedMap := filepath.Base(result.MapName)
		match.MapName = mapToMapName(sanitizedMap)
	}

	if err := match.Valid(); err != nil {
		return fmt.Errorf("updated match is invalid: %w", err)
	}

	return uc.matchRepo.Update(ctx, match)
}

func mapToMapName(sanitized string) *entities.MapName {
	m := entities.MapName(sanitized)
	switch m {
	case entities.MapNameDust2, entities.MapNameInferno, entities.MapNameNuke, entities.MapNameMirage,
		entities.MapNameOverpass, entities.MapNameVertigo, entities.MapNameAncient, entities.MapNameTrain,
		entities.MapNameCobblestone, entities.MapNameCache, entities.MapNameAnubis, entities.MapNameItaly:
		return &m
	default:
		return nil
	}
}

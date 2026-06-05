# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MagicStrike is a CS2 (Counter-Strike 2) demo parser and AI-powered narrative analysis pipeline written in Go. It ingests `.dem` replay files, extracts game events, stores them in ClickHouse, generates per-round tactical narratives via DeepSeek, produces vector embeddings via Voyage AI, and indexes them in Qdrant for semantic search.

## Architecture: Hexagonal (Ports & Adapters)

This project follows a strict hexagonal architecture. The dependency rule is: **code in `core/` never imports anything from `adapters/`**. Dependencies always point inward toward the domain.

| Layer | Location | Purpose |
|-------|----------|---------|
| Entities | `internal/core/entities/` | Pure domain structs with validation. No JSON/DB tags. |
| Ports | `internal/core/ports/` | Pure Go interfaces (input and output). No implementations. |
| Services | `internal/core/services/` | Reusable domain logic. Depends only on entities + ports. |
| Use Cases | `internal/core/usecases/` | E2E orchestration. Implements input ports, depends on output ports. |
| Adapters (in) | `internal/adapters/in/` | HTTP handlers, queue consumers. Know only port interfaces. |
| Adapters (out) | `internal/adapters/out/` | DB drivers, HTTP clients. Implement output port interfaces. |
| Entry points | `cmd/` | Composition root — the only place where everything is wired together. |

## Commands

```bash
# Build the worker binary (the runnable CLI)
make build

# Run all tests with race detection (single pass, no cache)
make test

# Start local infrastructure (ClickHouse + Qdrant) using podman
make infra-up

# Stop local infrastructure
make infra-down

# Clean build artifacts
make clean
```

**Note**: This project uses `podman`, not `docker`. The `docker-compose.yml` is for production deployment via `podman-compose`.

## Infrastructure

All infrastructure is containerized. Environment variables control connectivity:

| Variable | Default | Purpose |
|----------|---------|---------|
| `CLICKHOUSE_ADDR` | `127.0.0.1:9000` | ClickHouse native protocol address |
| `CLICKHOUSE_DB` | `magicstrike` | ClickHouse database name |
| `CLICKHOUSE_USER` | `default` | ClickHouse user |
| `CLICKHOUSE_PASSWORD` | `test` | ClickHouse password |
| `QDRANT_URL` | `http://127.0.0.1:6333` | Qdrant HTTP API URL |
| `DEEPSEEK_API_KEY` | (none) | DeepSeek API key. If unset, falls back to rule-based narratives. |
| `VOYAGE_API_KEY` | (none) | Voyage AI API key. If unset, returns zero-filled 1024-dim mock embeddings. |

## Ingestion Pipeline (`cmd/worker`)

```
CS2 .dem file → ParserService (demoinfocs) → Event entities → ClickHouse (batch insert, 10k per chunk)
                                                              ↓
                                          NarrativeService reads events, groups by round
                                                              ↓
                                          DeepSeek generates per-round narrative (with static fallback)
                                                              ↓
                                          Voyage AI generates 1024-dim embedding
                                                              ↓
                                          Qdrant upsert with deterministic UUID (MD5 namespace of matchID+round)
```

The worker is idempotent for re-runs: Qdrant record IDs are deterministic UUID v5 hashes of `{matchID}-round-{N}`, so upserts are safe.

On parser failure, a rollback deletes all partial inserts for that match ID from ClickHouse.

## Key Design Decisions

- **Flattened wide Event struct**: One struct maps directly to a single ClickHouse MergeTree wide table, partitioned by month and ordered by `(match_id, round, timestamp, id)`.
- **Entity IDs**: Events use `uuid.NewString()` (UUID v4). Domain entities (Match, Session, User) use ULIDs via `ulid.Make()`. Qdrant records use deterministic UUID v5 for idempotency.
- **Graceful degradation**: Both DeepSeek and Voyage gracefully handle missing API keys rather than failing — zero-vector mocks and rule-based narrative fallbacks keep the pipeline running for local dev/testing.
- **DeepSeek retry**: 3 attempts with exponential backoff (200ms base, doubles each attempt).
- **Entities do NOT have JSON/DB tags** in domain structs — the adapter layer handles serialization/deserialization separately. The `entities.Event` struct is an exception because it maps 1:1 with the ClickHouse wide table; it carries `json:"..."` tags used for both JSON and column mapping.

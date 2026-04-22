# Project Log

## Current Architecture
Single-process Go monolith (standard library only) for Raspberry Pi Zero 2 W. Current scaffold includes HTTP server + `/health`, env-based config defaults, and placeholder modules for RAM buffer, segmented append-only spool, flusher/tasks, and future SQLite durable storage. Design remains single-writer friendly with low-RAM and recovery-first priorities.

## Decisions
- Decision: Use Go standard library HTTP stack (`net/http`, `http.ServeMux`)
  Reason: Keep memory usage low and avoid unnecessary framework/dependency overhead.
- Decision: Use RAM + segmented spool + SQLite
  Reason: Reduce SD wear while preserving crash recovery and simple operations.
- Decision: Keep config environment-driven with sensible defaults
  Reason: Simple deployment on Raspberry Pi and easy recovery/restart behavior.
- Decision: Add explicit spool/buffer/flush durability knobs via environment variables
  Reason: Allow Raspberry Pi tuning for RAM limits and SD-card wear without code changes.
- Decision: Avoid Redis/PostgreSQL and unnecessary goroutine complexity
  Reason: Lower operational footprint and fewer moving parts on constrained hardware.
- Decision: Keep batch flusher as a simple single-writer loop with explicit requeue-on-failure
  Reason: Preserve RAM-buffered records on transient DB errors while keeping SQLite writes serialized and low-overhead.
- Decision: Run startup spool recovery before starting HTTP server
  Reason: Deterministic crash recovery ensures uncommitted spool records are replayed and durably flushed before new ingest arrives.
- Decision: Use minimal single-user device management first, with API-key device auth helper
  Reason: Keep auth scope small for now while enabling secure per-device ingest gating in the next step.
- Decision: Keep ingest handlers thin and route payload flow through parser -> spool -> RAM buffer
  Reason: Preserve single-writer durability model and avoid direct SQLite writes on request path.
- Decision: Expose lightweight operational state via JSON status endpoint (no metrics framework)
  Reason: Provide low-overhead observability on Raspberry Pi without introducing Prometheus or extra dependencies.
- Decision: Serve a minimal status web UI directly from Go handlers (no frontend build step)
  Reason: Keep RAM/CPU/storage overhead low on Raspberry Pi Zero 2 W while providing immediate operational visibility.

## Change History

### 2026-04-21 - Phase 1 (Scaffold)
Implemented:
- Created project scaffold folders for `cmd/server`, `internal/*`, `migrations`, and `scripts`.
- Added minimal Go entrypoint server with graceful shutdown.
- Added `GET /health` JSON endpoint.
- Added env-based config loading with defaults.
- Added `Makefile` (`build`, `run`, `test`, `fmt`) and updated `README.md`.
- Added and initialized `NEXT_STEPS.md`.
- Recorded baseline coding preferences and constraints.

Files changed:
- `README.md`
- `go.mod`
- `Makefile`
- `cmd/server/main.go`
- `internal/api/health.go`
- `internal/config/config.go`
- `internal/ingest/.gitkeep`
- `internal/spool/.gitkeep`
- `internal/buffer/.gitkeep`
- `internal/flusher/.gitkeep`
- `internal/store/.gitkeep`
- `internal/tasks/.gitkeep`
- `migrations/.gitkeep`
- `scripts/.gitkeep`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go version`
- `make fmt`
- `make test`
- `make build`
- `make run`
- `curl http://localhost:8080/health`

Pending:
- Install/verify Go toolchain in current environment.
- Run format/tests/build once Go is available.
- Implement RAM buffer + segmented spool flow.
- Add first SQLite migration and minimal store interface.

Known issues:
- `go` command is unavailable in this shell environment, so build/test/format were not executed yet.

### 2026-04-21 - Phase 2 (Planning Update)
Implemented:
- Updated next-session execution plan to focus on spool append and replay work.
- Defined immediate tasks: spool append API, checkpoint file support, and replay tests.
- Captured segmented spool requirement explicitly for implementation guidance.

Files changed:
- `NEXT_STEPS.md`
- `PROJECT_LOG.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement spool append API in `internal/spool`.
- Add checkpoint state file support for replay progress.
- Add replay-focused tests for crash/restart recovery behavior.

Known issues:
- `go` command availability remains unverified in this shell session.

### 2026-04-21 - Phase 3 (Core Domain Models)
Implemented:
- Added canonical tracker domain models in `internal/ingest/models.go`.
- Added `CanonicalPoint` with required fields and optional telemetry fields.
- Added `SpoolRecord` with `Seq`, `DeviceID`, `ReceivedAt`, and embedded `Point`.
- Added lightweight `BufferStats` counters for RAM batching visibility.
- Added maintainability comments and kept implementation logic-free.
- Updated README with a brief pointer to core model definitions.

Files changed:
- `internal/ingest/models.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement spool append API using `SpoolRecord`.
- Add checkpoint file support for replay progress tracking.
- Add replay tests for segmented spool recovery flows.

Known issues:
- `go` command is still unavailable in this shell environment, so tests were not run here.

### 2026-04-21 - Phase 4 (Config Extension)
Implemented:
- Extended environment-driven config for HTTP, SQLite path, spool settings, buffer limits, and flush controls.
- Added spool durability knobs: fsync mode (`always`, `balanced`, `low-wear`), fsync interval, and fsync byte threshold.
- Added simple parsing helpers for positive integers, durations, and fsync mode normalization.
- Updated README with documentation for each configuration setting and default.

Files changed:
- `internal/config/config.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement segmented spool append API using new config knobs (`APP_SPOOL_*`).
- Implement checkpoint file handling for replay progress.
- Add replay tests and include fsync/buffer/flush config coverage.

Known issues:
- `go` command is unavailable in this shell environment, so local test/build validation could not be run.

### 2026-04-21 - Phase 5 (OwnTracks Ingestion Parsing)
Implemented:
- Added isolated OwnTracks ingestion parser in `internal/ingest`.
- Implemented `ParseOwnTracksLocation(raw []byte) (CanonicalPoint, error)`.
- Supported common OwnTracks fields: `_type`, `lat`, `lon`, `tst`, `acc`, `alt`, `batt`, `vel`, `tid`, `topic`.
- Enforced location-only events (`_type=location`) with clear validation errors.
- Validated required fields (`lat`, `lon`, `tst`) and coordinate ranges.
- Converted `tst` UNIX seconds into UTC `TimestampUTC`.
- Preserved original JSON payload bytes in `RawPayload`.
- Populated `IngestHash` using SHA-256 of the normalized raw payload.
- Added realistic unit tests for success, required-field failures, invalid values, non-location events, and malformed JSON.
- Updated README with parser location and test file references.

Files changed:
- `internal/ingest/owntracks_parser.go`
- `internal/ingest/owntracks_parser_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement segmented spool append API to consume parsed `CanonicalPoint` events.
- Implement checkpoint file support for replay offsets.
- Add spool replay tests (including cross-segment and restart recovery paths).

Known issues:
- `go` command is unavailable in this shell environment, so parser tests could not be executed here.

### 2026-04-21 - Phase 6 (Overland Ingestion Parsing)
Implemented:
- Added isolated Overland batch parser in `internal/ingest`.
- Implemented `ParseOverlandBatch(raw []byte) ([]CanonicalPoint, error)`.
- Supported Overland `locations` batch payloads.
- Parsed common fields: `coordinates`, `timestamp`, `horizontal_accuracy`, `altitude`, `speed`, and `motion`/`activity`.
- Validated coordinate ranges and timestamp format.
- Converted parsed timestamps to UTC.
- Kept memory usage low by preserving `RawPayload` only for single-location payloads; omitted for multi-location batches.
- Added realistic unit tests for success paths, invalid coordinates, invalid timestamps, missing locations, and invalid JSON.

Files changed:
- `internal/ingest/overland_parser.go`
- `internal/ingest/overland_parser_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement segmented spool append API to persist parsed points (`OwnTracks` and `Overland`) as `SpoolRecord`.
- Add checkpoint file support for replay position and crash-safe recovery.
- Add spool replay tests across segment boundaries and restart scenarios.

Known issues:
- `go` command is unavailable in this shell environment, so parser tests could not be executed here.

### 2026-04-21 - Phase 7 (Deterministic Ingest Hash Helper)
Implemented:
- Added deterministic ingest hash helper for `CanonicalPoint`.
- Implemented `GenerateDeterministicIngestHash(point CanonicalPoint) string`.
- Hash input now uses a stable representation of:
- source type
- device id
- UTC timestamp (`RFC3339Nano`)
- latitude/longitude rounded to 5 decimal places (dedupe-friendly precision).
- Added unit tests for stability, coordinate-rounding dedupe behavior, identity changes, and timezone normalization.
- Kept helper isolated and did not wire it into the full ingest pipeline yet.

Files changed:
- `internal/ingest/ingest_hash.go`
- `internal/ingest/ingest_hash_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Wire deterministic hash helper into spool append path where `IngestHash` is assigned.
- Implement segmented spool append API to persist parsed points as `SpoolRecord`.
- Add checkpoint file support and replay tests across segment boundaries/restarts.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 8 (Initial Spool Subsystem Structure)
Implemented:
- Created initial `internal/spool` package structure.
- Added spool architecture/replay documentation (`internal/spool/doc.go`).
- Added `SpoolManager` interface and initial concrete `FileSpoolManager`.
- Defined sequence-based segment naming scheme (`segment-%020d.ndjson`) and parsing helpers.
- Added checkpoint file format with `last_committed_seq` and UTC update timestamp.
- Added record serialization/deserialization as newline-delimited JSON (NDJSON) for `ingest.SpoolRecord`.
- Added focused unit tests for record serialization/deserialization round-trip and invalid input handling.
- Kept full replay and compaction out of scope for this change.

Files changed:
- `internal/spool/doc.go`
- `internal/spool/manager.go`
- `internal/spool/segment.go`
- `internal/spool/checkpoint.go`
- `internal/spool/record_codec.go`
- `internal/spool/record_codec_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement append file I/O with segment rollover based on max segment bytes.
- Implement checkpoint read/write file I/O using the checkpoint JSON format.
- Implement initial replay scanner that iterates segments and skips records <= checkpoint sequence.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 9 (Spool Append Support)
Implemented:
- Added append support from `[]ingest.CanonicalPoint` to on-disk `SpoolRecord` NDJSON entries.
- Added monotonic sequence assignment in single-writer-safe design (`sync.Mutex` protected manager state).
- Added active segment management with rollover when `segmentMaxBytes` is exceeded.
- Added fsync policy support:
- `always`: sync each appended write
- `balanced`: sync on interval and byte-threshold triggers
- `low-wear`: avoid per-append sync; sync on managed close/rollover behavior
- Added auto-assignment of `IngestHash` via `GenerateDeterministicIngestHash` when missing.
- Added focused unit tests for append behavior, sequence monotonicity across calls, and segment rollover.

Files changed:
- `internal/spool/manager.go`
- `internal/spool/append.go`
- `internal/spool/append_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Implement checkpoint read/write file I/O operations around `checkpoint.json`.
- Implement initial replay scanner to read segments in order and skip records <= checkpoint sequence.
- Add replay/scanner tests including restart scenarios and checkpoint interactions.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 10 (Checkpointing and Replay)
Implemented:
- Added checkpoint file operations on spool manager:
- `ReadCheckpoint()` to load highest committed sequence (default `0` when missing)
- `AdvanceCheckpoint(lastCommittedSeq)` to monotonically advance and persist `checkpoint.json`
- Added replay operation:
- `ReplayAfterCheckpoint()` to return all records with `seq > checkpoint.last_committed_seq`
- Implemented replay across multiple segment files by reading sequence-sorted segment names.
- Kept implementation simple and readable using existing NDJSON record codec.
- Added tests for:
- replay after simulated restart (new manager instance over same spool dir)
- partially committed spool replay behavior
- checkpoint advancement monotonicity and persistence

Files changed:
- `internal/spool/manager.go`
- `internal/spool/replay.go`
- `internal/spool/replay_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Add explicit replay API that streams/iterates records to reduce RAM usage for large backlogs.
- Add checkpoint + replay integration with flusher workflow.
- Add replay robustness tests for malformed/corrupt segment line handling strategy.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 11 (Spool Compaction)
Implemented:
- Added spool compaction method: `CompactCommittedSegments()`.
- Compaction now deletes segment files whose highest record sequence is fully committed (`<= checkpoint.last_committed_seq`).
- Kept implementation safe and simple:
- delete-only strategy (no in-place rewriting)
- skips active writable segment while open
- keeps segments that may contain uncommitted records
- Added documentation for when compaction should run (after checkpoint advancement and/or low-frequency maintenance).
- Added focused tests for:
- committed segment cleanup behavior
- active segment safety during compaction

Files changed:
- `internal/spool/manager.go`
- `internal/spool/compaction.go`
- `internal/spool/compaction_test.go`
- `internal/spool/doc.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Integrate checkpoint advancement + compaction into flusher commit workflow.
- Add streaming replay API (iterator/callback) to avoid large in-memory replay slices.
- Add replay/compaction robustness tests for malformed segment data and failure handling policy.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 12 (In-Memory Buffer Manager)
Implemented:
- Created `internal/buffer` package with a simple FIFO RAM buffer manager for `ingest.SpoolRecord`.
- Added hard-limit enforcement for:
- max buffered points
- max buffered bytes
- Added required APIs:
- `Enqueue(records)` (all-or-nothing on limit checks)
- `DrainBatch(maxPoints)` (FIFO draining)
- `Stats()` (total points, total bytes, oldest buffered age)
- Added light per-device accounting (`deviceCounts`) to support device-oriented organization without extra complexity.
- Kept implementation low-memory and simple (slice queue + mutex, no dedupe).
- Added unit tests for enqueue/drain/statistics behavior and both hard-limit paths.

Files changed:
- `internal/buffer/manager.go`
- `internal/buffer/manager_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Wire buffer manager into ingest->flush flow.
- Connect flusher commit success to checkpoint advancement + compaction calls.
- Add end-to-end tests that cover spool replay -> buffer -> drain batches.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 13 (Lightweight In-Memory Dedupe)
Implemented:
- Added lightweight per-device in-memory dedupe in buffer enqueue path.
- Suppresses near-duplicates from the same device when both are true:
- timestamp difference <= configurable max time delta
- coordinate distance <= configurable max distance meters
- Added configurable dedupe thresholds via `NewManagerWithDedupe(...)`.
- Added documented defaults:
- `DefaultDedupeMaxTimeDelta = 2s`
- `DefaultDedupeMaxDistanceM = 10m`
- Kept design low-memory and replay-friendly:
- stores only last accepted point state per device (`timestamp`, `lat`, `lon`)
- no historical index, no heavy dedupe cache
- Added unit tests showing:
- near-duplicates are suppressed
- real movement is retained even with small timestamp gap

Files changed:
- `internal/buffer/manager.go`
- `internal/buffer/manager_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Wire dedupe-enabled buffer manager into ingest->flush path.
- Validate dedupe threshold values against real field traces and tune if needed.
- Add end-to-end tests for replay -> buffer dedupe -> drain behavior.

Known issues:
- `go` command is unavailable in this shell environment, so tests could not be executed here.

### 2026-04-21 - Phase 14 (SQLite Schema + Migrations)
Implemented:
- Added initial SQL migration with minimal forward-compatible schema:
- `users`
- `devices`
- `raw_points`
- `points` (lightweight derived/query table)
- Added required `raw_points` fields including `seq`, geo/time fields, optional telemetry fields, `raw_payload_json`, unique `ingest_hash`, and `created_at`.
- Added `devices` fields including `user_id`, `name`, `source_type`, `api_key`, `last_seen_at`, and `last_seq_received`.
- Added practical indexes for user/device + time access patterns.
- Added simple migration runner (`cmd/migrate`) and reusable store migrator (`internal/store/migrator.go`) using `sqlite3` CLI.
- Added `schema_migrations` tracking table support.
- Added Raspberry Pi-friendly SQLite pragmas in runner, including WAL mode.
- Updated `Makefile` with `migrate` target and documented migration usage in README.

Files changed:
- `migrations/0001_init_schema.sql`
- `internal/store/migrator.go`
- `cmd/migrate/main.go`
- `Makefile`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `make migrate`
- `go run ./cmd/migrate`
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Integrate database write path from flusher into `raw_points`/`points`.
- Wire checkpoint advancement/compaction after durable DB commit.
- Add migration/DB integration tests once Go + sqlite3 are available in environment.

Known issues:
- `go` command is unavailable in this shell environment, so tests/build were not executed here.
- `sqlite3` CLI availability is not verified in this shell.

### 2026-04-21 - Phase 15 (SQLite Store Batch Insert Layer)
Implemented:
- Added concrete SQLite store layer with transactional batch insert for `[]ingest.SpoolRecord`.
- Implemented `OpenSQLiteStore(dbPath)` with Pi-friendly SQLite pragmas (including WAL mode).
- Implemented `InsertSpoolBatch(records)` with:
- single transaction for whole batch
- prepared statements for efficient inserts/updates
- idempotency via `raw_points.ingest_hash` uniqueness (`ON CONFLICT DO NOTHING`)
- derived `points` inserts for newly inserted raw rows
- device state updates (`last_seen_at`, `last_seq_received`) during commit
- return value: highest successfully committed sequence in batch
- Added fallback user/device creation logic (`default` user and per-device upsert using stable synthetic `api_key`).
- Added unit tests for:
- successful batch insert
- replay inserting duplicates (idempotent)
- partial duplicate batches
- multi-device last sequence updates

Files changed:
- `go.mod`
- `internal/store/sqlite_store.go`
- `internal/store/sqlite_store_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Wire flusher path to call `InsertSpoolBatch`.
- Advance checkpoint only after successful DB batch commit and then compact segments.
- Add integration tests across spool replay -> buffer -> store -> checkpoint/compaction flow.

Known issues:
- `go` command is unavailable in this shell environment, so tests/build were not executed here.
- `sqlite3` CLI availability remains unverified in this shell.

### 2026-04-21 - Phase 16 (Batch Flusher)
Implemented:
- Added `internal/flusher` batch flusher with periodic timer-driven flushing to SQLite.
- Added size-triggered flush path (`TriggerFlush`) so callers can request flush when buffer pressure is high.
- Added graceful shutdown behavior (`Stop(ctx)`) that flushes remaining buffered records before exit.
- Added flush flow:
- drain batch from RAM buffer
- insert batch into SQLite store
- advance spool checkpoint only after successful commit
- Added transient failure handling:
- on SQLite insert failure, drained batch is requeued to the front of RAM buffer
- checkpoint is not advanced on failed commit
- Added focused unit tests for:
- successful flush
- failed flush without checkpoint advancement
- retry behavior (first failure, second success)
- Added `RequeueFront` support in `internal/buffer` manager to preserve drained records after transient downstream failure.

Files changed:
- `internal/flusher/flusher.go`
- `internal/flusher/flusher_test.go`
- `internal/buffer/manager.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`

Pending:
- Wire flusher into a runtime task that calls `TriggerFlush` when buffer size thresholds are crossed.
- Call spool compaction after successful checkpoint advancement in the commit path.
- Add higher-level integration tests for spool replay -> buffer -> flusher -> SQLite -> checkpoint/compaction.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 17 (Startup Recovery)
Implemented:
- Added startup recovery task in `internal/tasks/startup_recovery.go`.
- Recovery flow now:
- read spool checkpoint
- replay records where `seq > checkpoint`
- enqueue replayed records into RAM buffer in bounded batches
- flush replayed batches through flusher/SQLite before normal service start
- Added deterministic low-RAM behavior for large replay:
- bounded enqueue batching
- if RAM limits are hit during replay enqueue, flush and retry the same chunk
- Integrated runtime wiring in `cmd/server/main.go`:
- initialize spool manager, SQLite store, buffer manager, and flusher at startup
- run startup recovery before `ListenAndServe`
- start periodic flusher loop after recovery completes
- Added integration-style startup recovery tests in `internal/tasks/startup_recovery_test.go`:
- restart after append before DB commit
- restart after DB commit + checkpoint advancement
- Updated README with a brief startup recovery note.

Files changed:
- `internal/tasks/startup_recovery.go`
- `internal/tasks/startup_recovery_test.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Trigger size-based flush (`TriggerFlush`) from ingest path once HTTP ingest handlers are wired.
- Run spool compaction after successful checkpoint advancement in flusher commit path.
- Add broader end-to-end tests including ingest request -> spool append -> recovery -> durable query path.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 18 (Device Management + API Key Auth Helper)
Implemented:
- Added minimal device store layer in `internal/store/devices.go` with:
- `CreateDevice(...)`
- `ListDevices(...)`
- `GetDeviceByAPIKey(...)`
- Added minimal device model and params structs for management flows.
- Added `ErrDeviceNotFound` for explicit API-key lookup behavior.
- Added HTTP device management endpoints:
- `POST /api/v1/devices`
- `GET /api/v1/devices`
- Added route dependency wiring so API can use injected store from server startup.
- Added API key auth helper middleware in `internal/api/auth.go`:
- `RequireDeviceAPIKeyAuth(...)`
- `DeviceFromContext(...)`
- supports `X-API-Key` and `Authorization: Bearer <key>`
- Added tests for:
- device store create/list/lookup behavior
- device API create/list handlers
- API key auth middleware success + failure cases
- Updated README with minimal device API usage and current auth assumption notes.

Files changed:
- `internal/store/devices.go`
- `internal/store/devices_test.go`
- `internal/api/health.go`
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `internal/api/auth.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Add actual ingest HTTP endpoint(s) and apply `RequireDeviceAPIKeyAuth(...)` directly to those routes.
- Connect device-authenticated ingest requests to parser -> spool append -> buffer/flush pipeline.
- Add integration tests that include auth-protected ingest requests and end-to-end persistence.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 19 (Ingestion HTTP Endpoints)
Implemented:
- Added authenticated ingestion HTTP endpoints:
- `POST /api/v1/owntracks`
- `POST /api/v1/overland/batches`
- Added route dependency interfaces for ingest pipeline wiring:
- spool append dependency
- RAM buffer enqueue dependency
- optional flush trigger dependency
- Added endpoint request flow:
- parse payload with existing parser
- normalize to canonical points bound to authenticated device
- ensure ingest hash when missing
- append to segmented spool
- enqueue returned spool records into RAM buffer
- return compact JSON success payload (`ok`, `source`, `accepted`, `spooled`, `enqueued`, `max_seq`)
- Enforced device API key auth on ingest routes via existing middleware.
- Added bounded request body read (1 MiB cap) to keep memory usage controlled.
- Added endpoint tests for:
- valid OwnTracks request
- valid Overland request
- invalid payload
- bad API key
- Updated server route wiring to inject spool, buffer, and flusher dependencies into API registration.
- Updated README with ingest endpoint/auth details and handler flow note.

Files changed:
- `internal/api/health.go`
- `internal/api/ingest.go`
- `internal/api/ingest_test.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Add explicit ingest size-pressure policy (for example buffer stats threshold) before calling `TriggerFlush`.
- Integrate spool compaction call after successful checkpoint advancement in flusher commit path.
- Add end-to-end tests covering authenticated ingest -> spool -> flusher -> SQLite visibility.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 20 (Operational Status Endpoints)
Implemented:
- Added operational status endpoint:
- `GET /api/v1/status`
- Status response now includes:
- current buffer points and bytes
- oldest buffered age in seconds
- spool segment count
- current checkpoint sequence
- last flush result (timestamp/success/error) when available
- Added spool status support:
- new `SegmentCount()` method on `FileSpoolManager`
- Added flusher status snapshot support:
- new `LastFlushResult` type and `LastFlushResult()` accessor
- flusher now records result after each flush attempt (`FlushNow`, timer/trigger pass, shutdown flush)
- Added tests:
- API status endpoint test for operational snapshot JSON
- spool segment count unit test
- flusher test for last flush result recording
- Updated README with operational endpoint notes.

Files changed:
- `internal/api/health.go`
- `internal/api/status.go`
- `internal/api/status_test.go`
- `internal/api/ingest_test.go`
- `internal/spool/status.go`
- `internal/spool/status_test.go`
- `internal/flusher/flusher.go`
- `internal/flusher/flusher_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Add explicit ingest size-pressure policy before calling `TriggerFlush`.
- Integrate spool compaction call after successful checkpoint advancement in flusher commit path.
- Add end-to-end tests for authenticated ingest -> spool -> flush -> SQLite plus status endpoint consistency checks.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 21 (Minimal Web UI Scaffold)
Implemented:
- Added a minimal web UI status page served directly by backend routes.
- Added UI routes:
- `GET /`
- `GET /ui/status`
- Implemented lightweight page in `internal/api/ui.go`:
- plain HTML/CSS/vanilla JS (no SPA, no bundler)
- polls existing JSON endpoints (`/health`, `/api/v1/status`, `/api/v1/devices`)
- displays:
- service health
- devices table
- buffer stats
- last flush status
- Kept frontend approach minimal and Pi-friendly (single embedded page, no extra dependencies).
- Added UI route test for root page serving and content type/title checks.
- Updated README with notes on how the frontend is served.

Files changed:
- `internal/api/health.go`
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Add lightweight polling backoff/interval config for UI refresh cadence if needed.
- Consider masking/removing sensitive device fields in device list API responses for UI safety.
- Add optional minimal UI smoke test that validates key cards render from mocked API responses.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 22 (UI Resilience Fix for Devices API Failures)
Implemented:
- Fixed status UI refresh behavior so `/api/v1/devices` failure no longer breaks the whole page.
- Updated UI polling flow in `internal/api/ui.go`:
- load `/health` and `/api/v1/status` first
- fetch `/api/v1/devices` separately with isolated error handling
- render devices section as unavailable on devices fetch failure while keeping health/buffer/spool/flush cards live
- Added lightweight warning text in update timestamp when devices fetch fails.
- Root cause addressed: previous `Promise.all` caused complete UI failure when any single endpoint (like devices) returned HTTP 500.

Files changed:
- `internal/api/ui.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Investigate why `/api/v1/devices` returns 500 in the current runtime (commonly missing migrations/schema).
- Consider graceful empty-list fallback in devices API when schema is not initialized.
- Add targeted UI JS test coverage if frontend test harness is introduced later.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-21 - Phase 23 (UI Route Matching Fix)
Implemented:
- Fixed UI route matching so typo paths no longer render the dashboard page.
- Updated root UI route registration from broad `GET /` to exact-only `GET /{$}`.
- Kept explicit UI route `GET /ui/status` unchanged.
- Added regression test to ensure typo path (`/ui/statu`) returns 404.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `go run ./cmd/server`
- `make migrate`

Pending:
- Investigate why `/api/v1/devices` returns 500 in the current runtime (commonly missing migrations/schema).
- Consider graceful empty-list fallback in devices API when schema is not initialized.
- Add targeted UI JS test coverage if frontend test harness is introduced later.

Known issues:
- `go` command is unavailable in this shell environment (`CommandNotFoundException`), so tests were not executed here.

### 2026-04-22 12:20 - Phase 24 (Runtime Verification Pass)
Implemented:
- Verified toolchain availability in this environment: `go version go1.22.2 linux/amd64`.
- Ran full test suite successfully (`go test ./...` via `make test`).
- Attempted migrations and confirmed blocker: `sqlite3` CLI missing, so `make migrate` fails before schema setup.
- Ran live server smoke checks and validated:
- `GET /health` returns `200` with expected JSON.
- `GET /api/v1/status` returns `200` with operational snapshot.
- `GET /` and `GET /ui/status` return `200` and serve UI HTML.
- `GET /ui/statu` returns `404` (route matching fix confirmed).
- Verified current runtime failure mode matches pending notes:
- `GET /api/v1/devices` returns `500` (`no such table: devices`).
- `POST /api/v1/devices` returns `500` (`no such table: users`).
- Ingest endpoints return `401` without API key.
- Ingest endpoints return `500` with API key while schema/device tables are missing (`device lookup failed`).

Files changed:
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go version`
- `make test`
- `make migrate`
- `go run ./cmd/server`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/health`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/api/v1/status`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/ui/status`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/ui/statu`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/api/v1/devices`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/devices -H 'Content-Type: application/json' -d '{"name":"phone-main","source_type":"owntracks","api_key":"dev-key-1"}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/owntracks -H 'Content-Type: application/json' -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/overland/batches -H 'Content-Type: application/json' -d '{"locations":[{"properties":{"timestamp":"2026-04-22T12:00:00Z","device_id":"phone-main"},"geometry":{"type":"Point","coordinates":[-87.0,41.0]}}]}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/owntracks -H 'Content-Type: application/json' -H 'X-API-Key: dev-key-1' -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/overland/batches -H 'Content-Type: application/json' -H 'X-API-Key: dev-key-1' -d '{"locations":[{"properties":{"timestamp":"2026-04-22T12:00:00Z","device_id":"phone-main"},"geometry":{"type":"Point","coordinates":[-87.0,41.0]}}]}'`

Pending:
- Install `sqlite3` CLI in runtime environment and re-run `make migrate` to initialize schema.
- Re-run live API flow after migrations: create device -> authenticated ingest -> verify status and DB visibility.
- Add an automated integration check that fails fast when required migration tables (`users`, `devices`) are missing.

Known issues:
- `make migrate` currently fails because `sqlite3` CLI is not installed (`exec: "sqlite3": executable file not found in $PATH`).
- Without migrations, device + authenticated ingest paths return `500` due missing schema tables.

### 2026-04-22 12:35 - Phase 25 (Post-Migration End-to-End Verification)
Implemented:
- Re-ran verification after `sqlite3` installation and successful schema migration.
- Confirmed environment/tooling:
- `go version` returns `go1.22.2`.
- `sqlite3 --version` returns `3.45.1`.
- Confirmed test suite passes (`make test`).
- Confirmed migration runner now succeeds (`make migrate`).
- Verified live runtime behavior on `:8080`:
- `GET /health` -> `200`
- `GET /api/v1/status` -> `200`
- `GET /api/v1/devices` -> `200` with empty list before device creation
- `GET /` and `GET /ui/status` -> `200`
- `GET /ui/statu` -> `404`
- Verified device management works after migration:
- `POST /api/v1/devices` -> `201` and created device `phone-main` with API key `dev-key-1`
- `GET /api/v1/devices` returns created device
- Verified ingest auth + ingestion path:
- owntracks without API key -> `401`
- owntracks with API key -> `200` (`accepted:1`, `spooled:1`, `max_seq:1`)
- overland with corrected payload shape (`locations[].coordinates`) and API key -> `200` (`accepted:1`, `max_seq:2`)
- Verified persisted data via SQLite:
- `users=1`, `devices=2` (existing device + new one), `raw_points=2`, `points=2`
- `raw_points` contains expected seq/source/timestamp rows for owntracks and overland records.

Files changed:
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go version`
- `sqlite3 --version`
- `make test`
- `make migrate`
- `go run ./cmd/server`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/health`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/api/v1/status`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' http://127.0.0.1:8080/api/v1/devices`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/devices -H 'Content-Type: application/json' -d '{"name":"phone-main","source_type":"owntracks","api_key":"dev-key-1"}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/owntracks -H 'Content-Type: application/json' -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/owntracks -H 'Content-Type: application/json' -H 'X-API-Key: dev-key-1' -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'`
- `curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/overland/batches -H 'Content-Type: application/json' -H 'X-API-Key: dev-key-1' -d '{"device_id":"phone-main","locations":[{"coordinates":[-87.001,41.001],"timestamp":"2026-04-22T12:00:00Z","horizontal_accuracy":7.5}]}'`
- `sqlite3 ./data/plexplore.db "SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM devices; SELECT COUNT(*) FROM raw_points; SELECT COUNT(*) FROM points;"`
- `sqlite3 ./data/plexplore.db "SELECT seq, source_type, timestamp_utc, lat, lon FROM raw_points ORDER BY seq;"`

Pending:
- Implement explicit ingest size-pressure policy to call flusher trigger under high buffer pressure.
- Integrate spool compaction immediately after successful checkpoint advancement in flusher commit workflow.
- Add an automated end-to-end integration test that covers migrated schema + device creation + authenticated ingest + persistence assertions.

Known issues:
- Overland endpoint strictly expects `locations[].coordinates` array format (`[lon, lat]`); geometry-wrapped payloads fail with `400` by design.
- Device API currently returns `api_key` in list/create responses; consider masking before broader deployment.

### 2026-04-22 12:42 - Phase 26 (Repo Ignore Baseline)
Implemented:
- Added repo-root `.gitignore` for generated/local-only files.
- Ignored build outputs, coverage/profiling artifacts, runtime data directory, local temp/log/pid files, `node_modules`, env files, and common IDE/OS metadata.
- Goal: prevent accidental commits of local runtime state (`data/`) and dependency cache (`node_modules/`) while keeping repository clean.

Files changed:
- `.gitignore`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `date -Iseconds`

Pending:
- Continue with durability/pipeline improvements from current milestone (flush pressure policy, spool compaction, end-to-end integration coverage).

Known issues:
- `.gitignore` does not untrack files already committed in git history; if any ignored paths were previously tracked, they must be removed from index separately.

### 2026-04-22 12:55 - Phase 27 (Ingest Pipeline Integration Coverage)
Implemented:
- Added robust end-to-end integration tests in `internal/tasks/ingest_pipeline_integration_test.go` using temporary spool directories and temporary SQLite databases.
- Covered required scenarios:
- OwnTracks ingest -> spool append -> RAM buffer -> flush -> SQLite insert.
- Overland ingest -> spool append -> RAM buffer -> flush -> SQLite insert.
- Duplicate ingest does not create duplicate SQLite rows.
- Checkpoint advances only after successful SQLite commit (with injected one-time SQLite failure).
- Startup recovery replays spool records after simulated crash before flush.
- Spool segment rollover replays correctly after restart/recovery.
- Added small reusable test helpers for:
- schema setup from `migrations/0001_init_schema.sql`
- authenticated ingest requests against real route wiring
- DB row count / query assertions
- restart runtime assembly for recovery checks
- Kept tests deterministic (fixed payload timestamps/coordinates, direct `FlushNow`, no background ticker dependency) and fast (<3s for integration package run).

Architectural decisions:
- Decision: Keep integration tests inside `internal/tasks` and run through real component wiring (`api` + `spool` + `buffer` + `flusher` + `sqlite`), rather than introducing new test-only abstractions.
  Reason: Preserve current architecture and validate actual production flow with minimal added code.

Files changed:
- `internal/tasks/ingest_pipeline_integration_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/tasks/ingest_pipeline_integration_test.go`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`

Pending:
- Implement explicit ingest size-pressure policy to call flusher trigger under high buffer pressure.
- Integrate spool compaction immediately after successful checkpoint advancement in flusher commit workflow.
- Add handling for duplicate records filtered by RAM dedupe so replay-pending duplicate spool seq can still advance checkpoint without waiting for restart recovery.

Known issues:
- In current behavior, immediate duplicate ingest can be filtered by RAM dedupe before flush; SQLite rows stay deduplicated correctly, but checkpoint may remain behind latest spool seq until a restart recovery replay processes that duplicate record.
- Device API still returns `api_key` in list/create responses; consider masking for safer default operations.

### 2026-04-22 13:01 - Phase 28 (Operational Status Endpoint Expansion)
Implemented:
- Extended existing `GET /api/v1/status` endpoint to include requested operational fields while keeping the response simple and low-overhead.
- Added response fields:
- `service_health`
- `buffer_points`
- `buffer_bytes`
- `oldest_buffered_age_seconds`
- `spool_dir_path`
- `spool_segment_count`
- `checkpoint_seq`
- `last_flush_attempt_at_utc`
- `last_flush_success_at_utc`
- `last_flush_error` (when present)
- `sqlite_db_path`
- Kept existing `last_flush` object for backwards compatibility with current UI and clients.
- Wired runtime path metadata into status dependencies from server config (`SpoolDir`, `SQLitePath`).
- Extended flusher status bookkeeping so each flush result includes both last attempt time and most recent successful flush time.
- Added/updated endpoint tests in `internal/api/status_test.go` for:
- expanded operational fields on success
- last flush error payload behavior when latest attempt fails
- Updated README status section with explicit `curl` command and example JSON response.

Architectural decisions:
- Decision: Expand the existing `/api/v1/status` schema additively (instead of introducing a new status endpoint or replacing fields).
  Reason: Preserve compatibility for current UI/clients while meeting operational visibility requirements with minimal overhead.

Files changed:
- `internal/flusher/flusher.go`
- `internal/api/health.go`
- `internal/api/status.go`
- `internal/api/status_test.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/flusher/flusher.go internal/api/health.go internal/api/status.go internal/api/status_test.go cmd/server/main.go`
- `go test ./internal/api ./internal/flusher`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device API still returns `api_key` in list/create responses and should be hardened later.

### 2026-04-22 13:12 - Phase 29 (Status Route Alias Fix)
Implemented:
- Added status endpoint alias route `GET /status` mapped to the same handler as `GET /api/v1/status`.
- Added API test coverage for alias route behavior in `internal/api/status_test.go`.
- Updated README operational status section to document alias usage and example `curl` against `/status`.
- Root cause addressed: endpoint existed at `/api/v1/status`, but `/status` was not previously registered, which caused `404 page not found`.

Files changed:
- `internal/api/status.go`
- `internal/api/status_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/status.go internal/api/status_test.go`
- `go test ./internal/api`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device API still returns `api_key` in list/create responses and should be hardened later.

### 2026-04-22 13:18 - Phase 30 (Graceful Shutdown Hardening)
Implemented:
- Hardened shutdown request-path behavior:
- added ingest draining gate via `Dependencies.IsDraining` callback
- ingest endpoints now return `503 service is shutting down` while draining is active
- Hardened server shutdown sequence in `cmd/server/main.go`:
- on signal, set draining mode before shutdown
- disable keep-alives to reduce new request reuse during drain
- run `server.Shutdown(...)` with explicit server timeout (in-flight completion window)
- run `batchFlusher.Stop(...)` with a separate flush timeout to drain RAM buffer to SQLite
- Hardened spool durability on shutdown/checkpoint path:
- spool segment close path now syncs pending bytes regardless of fsync mode when closing for shutdown
- checkpoint advancement now writes via open/write/sync/close path instead of unsynced `os.WriteFile`
- Added/updated tests:
- ingest API test verifies new ingest is rejected during shutdown drain and does not append/enqueue
- existing spool checkpoint/replay tests still pass under updated checkpoint write path
- Added README shutdown documentation:
- manual validation procedure
- explicit behavior differences for clean shutdown vs forced kill vs crash/power loss

Architectural decisions:
- Decision: Introduce a minimal ingest drain gate tied to process shutdown state.
  Reason: Stop accepting new ingest promptly during shutdown without redesigning routing/flusher architecture.

Files changed:
- `cmd/server/main.go`
- `internal/api/health.go`
- `internal/api/ingest.go`
- `internal/api/ingest_test.go`
- `internal/spool/append.go`
- `internal/spool/replay.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w cmd/server/main.go internal/api/health.go internal/api/ingest.go internal/api/ingest_test.go internal/spool/append.go internal/spool/replay.go`
- `go test ./internal/api ./internal/spool ./cmd/server`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- Clean shutdown remains best-effort and bounded by shutdown timeouts; a forced termination can still interrupt in-flight flush progress.
- Device API still returns `api_key` in list/create responses and should be hardened later.

### 2026-04-22 13:29 - Phase 31 (Startup Recovery Correctness Review)
Implemented:
- Reviewed startup path ordering and confirmed recovery executes before HTTP listen path:
- `RecoverFromSpool(...)` runs before route registration/server `ListenAndServe`
- Added startup recovery tests for required correctness cases:
- replay after crash before flush (already covered and retained)
- replay after partial progress (`checkpoint=2`, replay remaining `seq=3`)
- replay when checkpoint is already advanced (already covered and retained)
- stale checkpoint replay does not duplicate SQLite rows (`ingest_hash` uniqueness dedupe)
- Added README section `Startup Recovery Flow` documenting:
- checkpoint-based replay behavior (`seq > checkpoint`)
- flush/commit/checkpoint progression
- duplicate-safe replay behavior when checkpoint is stale
- startup ordering (recovery before ingest traffic)

Architectural decisions:
- Decision: Keep startup recovery checkpoint-driven and rely on SQLite uniqueness (`ingest_hash`) for idempotent replay safety when checkpoint lags.
  Reason: Preserves current architecture with minimal changes while improving correctness guarantees.

Files changed:
- `internal/tasks/startup_recovery_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/tasks/startup_recovery_test.go`
- `go test ./internal/tasks -run TestRecoverFromSpool -count=1`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device API still returns `api_key` in list/create responses and should be hardened later.

### 2026-04-22 13:35 - Phase 32 (Device API Key Hygiene + Rotation)
Implemented:
- Improved device API key handling:
- create endpoint returns full `api_key` once (`POST /api/v1/devices`)
- list/read endpoints no longer return full key; they return `api_key_preview` mask
- added device read endpoint: `GET /api/v1/devices/{id}`
- added API key rotation endpoint: `POST /api/v1/devices/{id}/rotate-key`
- Added timestamp fields on device responses:
- `created_at`
- `updated_at`
- `last_seen_at` (when available)
- Added store-level device methods:
- `GetDeviceByID(...)`
- `RotateDeviceAPIKey(...)`
- Updated ingest persistence path so device `updated_at` is maintained on writes.
- Added migration `0002_devices_updated_at.sql` to add and backfill `devices.updated_at`.
- Updated test migration loaders to apply all `.sql` migrations in order.
- Added/updated tests for required behaviors:
- create returns full key once
- list/read do not return full key (masked preview only)
- rotate key invalidates old key and returns new key

Architectural decisions:
- Decision: Keep API key masking strictly at API response layer while retaining full key in store for authentication lookup.
  Reason: Minimal change to existing auth flow with immediate hygiene improvement for list/read output.
- Decision: Add incremental schema migration for `devices.updated_at` instead of mutating existing migration in place.
  Reason: Safer evolution path for already-initialized databases.

Files changed:
- `internal/api/health.go`
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `internal/store/devices.go`
- `internal/store/devices_test.go`
- `internal/store/sqlite_store.go`
- `internal/store/sqlite_store_test.go`
- `internal/tasks/startup_recovery_test.go`
- `internal/tasks/ingest_pipeline_integration_test.go`
- `migrations/0002_devices_updated_at.sql`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/devices.go internal/api/devices_test.go internal/store/devices.go internal/store/devices_test.go internal/store/sqlite_store.go internal/store/sqlite_store_test.go internal/tasks/startup_recovery_test.go internal/tasks/ingest_pipeline_integration_test.go`
- `go test ./internal/api ./internal/store`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device list/read now mask API keys, but endpoint auth/authorization model is still minimal single-user and should be hardened for multi-user contexts.

### 2026-04-22 13:43 - Phase 33 (Task 1 Client Setup Documentation)
Implemented:
- Added practical README client setup documentation for real device testing:
- `Connect OwnTracks` section with endpoint, auth method, required headers, and payload example
- `Connect Overland` section with endpoint, auth method, required headers, and payload example
- Added known caveats for both client paths
- Added concise ingest troubleshooting guidance for `400`, `401`, and `500` responses
- Kept content aligned with current code behavior (no unsupported UI-specific claims)

Files changed:
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `date -Iseconds`

Pending:
- Complete Task 2 from `codex_tasks.md`: add `GET /api/v1/points/recent` with `device_id` and `limit` query support, tests, and README examples.
- Continue core durability follow-ups from active milestone (flush trigger policy, spool compaction, duplicate-checkpoint lag).

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:03 - Phase 34 (Task 2 Recent Points Inspection Endpoint)
Implemented:
- Added compact debugging endpoint: `GET /api/v1/points/recent`.
- Added optional query params:
- `device_id` (device-name filter)
- `limit` (positive integer; default `50`, capped at `500`)
- Added SQLite-backed recent-point query in store:
- `ListRecentPoints(ctx, deviceID, limit)` joining `raw_points` and `devices`
- returns compact fields: `seq`, `device_id`, `source_type`, `timestamp_utc`, `lat`, `lon`
- Wired new `PointStore` dependency and route registration in API/server.
- Added tests:
- API handler tests for default behavior, filter+limit behavior, and invalid limit (`400`)
- store test for ordering, filtering, and limit behavior against real SQLite inserts
- Updated README with recent-points endpoint examples.
- Updated migration SQL loading in tests to apply all migration files in lexical order (supports new migrations without brittle single-file assumptions).

Architectural decisions:
- Decision: Expose recent-point inspection as a lightweight read-only endpoint in existing API mux.
  Reason: Improves operational debugging without introducing map/timeline features or heavy dependencies.

Files changed:
- `internal/store/points.go`
- `internal/api/health.go`
- `internal/api/points.go`
- `internal/api/points_test.go`
- `internal/store/sqlite_store_test.go`
- `internal/tasks/startup_recovery_test.go`
- `internal/tasks/ingest_pipeline_integration_test.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/points.go internal/api/health.go internal/api/points.go internal/api/points_test.go internal/store/sqlite_store_test.go cmd/server/main.go`
- `go test ./internal/api ./internal/store`
- `go test ./...`

Pending:
- Continue `codex_tasks.md` in order with Task 3 (GeoJSON export endpoint + filters/tests/docs).
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:07 - Phase 35 (Task 3 GeoJSON Export)
Implemented:
- Added export endpoint: `GET /api/v1/exports/geojson`.
- Added optional filters:
- `device_id` (device name)
- `from` (RFC3339)
- `to` (RFC3339)
- Added lightweight GeoJSON `FeatureCollection` response generation with per-point `Feature` + `Point` geometry.
- Added store query for export data: `ListPointsForExport(...)` with ordered output and optional filters.
- Added tests:
- API tests for valid GeoJSON structure and invalid timestamp query handling
- store test for export filtering behavior
- Updated README with GeoJSON endpoint usage examples.

Files changed:
- `internal/store/points.go`
- `internal/api/health.go`
- `internal/api/exports.go`
- `internal/api/exports_test.go`
- `internal/api/points_test.go`
- `internal/store/sqlite_store_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/points.go internal/api/health.go internal/api/exports.go internal/api/exports_test.go internal/api/points_test.go internal/store/sqlite_store_test.go`
- `go test ./internal/api ./internal/store`
- `go test ./...`

Pending:
- Complete `codex_tasks.md` Task 4: add `GET /api/v1/exports/gpx` with `from/to/device_id` filters, tests, and README examples.
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:09 - Phase 36 (Task 4 GPX Export)
Implemented:
- Added export endpoint: `GET /api/v1/exports/gpx`.
- Reused optional filters:
- `device_id` (device name)
- `from` (RFC3339)
- `to` (RFC3339)
- Added lightweight GPX 1.1 response generation (`application/gpx+xml`) with:
- `<gpx>` root
- `<trk>/<trkseg>`
- `<trkpt lat=\"...\" lon=\"...\"><time>...</time></trkpt>` entries
- Added API tests for:
- valid GPX structure/content (including track points and coordinates)
- invalid timestamp query returns `400`
- Updated README with GPX usage examples.

Architectural decisions:
- Decision: Share export filter parsing between GeoJSON and GPX endpoints.
  Reason: Keep implementation simple/consistent and reduce duplicated filter logic.

Files changed:
- `internal/api/exports.go`
- `internal/api/exports_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/exports.go internal/api/exports_test.go`
- `go test ./internal/api`
- `go test ./...`

Pending:
- Add explicit size-based flush trigger policy from buffer pressure stats in ingest path.
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without restart recovery.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:19 - Phase 37 (Task 5 Lightweight Admin/Status Page)
Implemented:
- Extended minimal web UI to include a recent points preview table.
- UI now fetches `/api/v1/points/recent?limit=10` and renders:
- sequence number
- device id
- UTC timestamp
- latitude/longitude (fixed precision)
- Added graceful fallback rendering when recent points endpoint is unavailable.
- Kept UI lightweight (same plain HTML/CSS/vanilla JS, no build tooling/dependencies).
- Added UI test assertion to ensure recent points section is present in rendered page.
- Updated README minimal UI section to include spool/checkpoint visibility and recent points preview.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api`

Pending:
- Complete `codex_tasks.md` Task 6 (Raspberry Pi deployment assets and docs).
- Complete `codex_tasks.md` Task 7 (lightweight Dockerization and docs).
- Continue durability follow-up items (flush trigger policy, compaction-after-commit, duplicate-checkpoint lag).

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:21 - Phase 38 (Task 6 Raspberry Pi Deployment Prep)
Implemented:
- Added sample systemd unit for service deployment.
- Added sample environment file with low-overhead defaults and persistent `/var/lib/plexplore` paths.
- Added minimal install/setup script to:
- create service user and directories
- install binary/service/env files
- enable and start the service
- Added README deployment documentation covering:
- DB path and spool path
- start/stop/restart/status commands
- log inspection with `journalctl`
- backup procedure for DB and spool

Architectural decisions:
- Decision: Provide deployment as file templates + a small shell installer instead of adding packaging tooling.
  Reason: Keep Raspberry Pi operations simple, transparent, and low-overhead.

Files changed:
- `deploy/systemd/plexplore.service`
- `deploy/systemd/plexplore.env.sample`
- `scripts/install_systemd.sh`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `ls -l scripts/install_systemd.sh`

Pending:
- Complete `codex_tasks.md` Task 7 (lightweight Dockerization and docs).
- Continue durability follow-up items (flush trigger policy, compaction-after-commit, duplicate-checkpoint lag).

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:27 - Phase 39 (Task 7 Lightweight Dockerization)
Implemented:
- Added lightweight multi-stage Docker build:
- build stage compiles `plexplore-server` and `plexplore-migrate`
- runtime stage uses Alpine with `sqlite` CLI for migration runner compatibility
- Added container entrypoint script to run migrations then start server.
- Added `.dockerignore` to keep build context small and avoid including runtime state/log artifacts.
- Added optional `compose.yaml` single-container setup with `/data` bind mount and runtime env defaults.
- Updated README with:
- `docker build`
- `docker run`
- `docker compose up/down`
- `/data` volume mapping details
- key env vars for container runtime
- Raspberry Pi Zero 2 W caveats
- Verified status and export endpoints from running container on host-mapped port.

Architectural decisions:
- Decision: Keep Docker runtime as single container with SQLite+spool persisted to mounted `/data` volume.
  Reason: Matches low-overhead architecture and avoids introducing Redis/PostgreSQL or multi-service operational complexity.

Files changed:
- `Dockerfile`
- `.dockerignore`
- `compose.yaml`
- `scripts/docker-entrypoint.sh`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./...`
- `docker --version`
- `docker compose config`
- `docker build -t plexplore:dev .`
- `docker run --rm -p 18080:8080 -v /mnt/d/Code/plexplore/data:/data plexplore:dev`
- `curl -sS -w "\n%{http_code}\n" http://127.0.0.1:18080/status`
- `curl -sS -w "\n%{http_code}\n" http://127.0.0.1:18080/api/v1/exports/geojson`
- `curl -sS -w "\n%{http_code}\n" http://127.0.0.1:18080/api/v1/exports/gpx`

Pending:
- Resume durability follow-up items (flush trigger policy, compaction-after-commit, duplicate-checkpoint lag).

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:36 - Phase 40 (Ingest Pressure-Based Flush Trigger)
Implemented:
- Added explicit best-effort pressure-based flush triggering in ingest path.
- After successful spool append + RAM enqueue, ingest now checks buffer stats and triggers flusher when either threshold is crossed:
- point count threshold
- byte threshold
- Kept request handlers thin and non-blocking:
- no direct SQLite writes from request path
- flush trigger uses existing async `TriggerFlush()` path
- periodic flush loop remains unchanged.
- Added config knobs with env-driven loading:
- `APP_FLUSH_TRIGGER_POINTS`
- `APP_FLUSH_TRIGGER_BYTES`
- Defaults are `75%` of `APP_BUFFER_MAX_POINTS` and `APP_BUFFER_MAX_BYTES`.
- Wired thresholds into API dependencies from server startup config.
- Added ingest API tests covering required behavior:
- no pressure does not trigger flush
- point-threshold crossing triggers flush
- byte-threshold crossing triggers flush
- Updated deployment templates with new env knobs:
- `deploy/systemd/plexplore.env.sample`
- `compose.yaml`
- Updated README with policy summary and new config documentation.

Architectural decisions:
- Decision: Implement pressure handling as an ingest-path async flush trigger gate (stats check + `TriggerFlush`) rather than synchronous flush in request handlers.
  Reason: Keeps ingest low-latency and architecture unchanged while improving high-pressure responsiveness.

Files changed:
- `internal/config/config.go`
- `internal/api/health.go`
- `internal/api/ingest.go`
- `internal/api/ingest_test.go`
- `cmd/server/main.go`
- `deploy/systemd/plexplore.env.sample`
- `compose.yaml`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/config/config.go internal/api/health.go internal/api/ingest.go internal/api/ingest_test.go cmd/server/main.go`
- `go test ./internal/api ./internal/config ./cmd/server`
- `go test ./...`

Pending:
- Run spool compaction after successful checkpoint advancement in flusher commit workflow.
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without requiring restart recovery.
- Add a focused runtime/integration assertion that pressure-triggered flush path advances checkpoint under sustained ingest.

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:40 - Phase 41 (Flusher Commit-Path Spool Compaction)
Implemented:
- Integrated spool compaction into successful flusher commit workflow.
- Flusher behavior is now:
  1. SQLite batch insert succeeds
  2. checkpoint advancement succeeds
  3. best-effort compact committed spool segments
- Compaction is best-effort by design:
- compaction failure does not roll back/invalidates successful commit
- compaction failure is logged with lightweight `log.Printf(...)`
- Preserved failure gates:
- SQLite insert failure: no checkpoint advance, no compaction
- checkpoint advance failure: no compaction
- Added/updated flusher unit tests for required cases:
- successful commit advances checkpoint and triggers compaction
- failed SQLite commit does not compact
- failed checkpoint advancement does not compact
- compaction failure does not corrupt successful commit behavior
- Updated README flush policy notes to include post-checkpoint best-effort spool compaction.

Architectural decisions:
- Decision: Keep compaction coupled to successful checkpoint advancement but non-fatal.
  Reason: Maintains durability correctness while reclaiming spool space automatically with minimal runtime overhead.

Files changed:
- `internal/flusher/flusher.go`
- `internal/flusher/flusher_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/flusher/flusher.go internal/flusher/flusher_test.go`
- `go test ./internal/flusher`
- `go test ./...`

Pending:
- Resolve duplicate-dedupe checkpoint lag so replay-pending duplicate spool seq can advance without requiring restart recovery.
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Consider improving checkpoint-failure handling to requeue drained batch safely (currently existing behavior remains unchanged in this change).

Known issues:
- In current behavior, immediate duplicates can still remain replay-pending in spool until recovery because RAM dedupe may drop them before flush.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:44 - Phase 42 (Duplicate-Dedupe Checkpoint Lag Fix)
Implemented:
- Fixed duplicate-dedupe checkpoint lag without removing spool durability or dedupe behavior.
- RAM dedupe now converts near-duplicate records into lightweight checkpoint-only markers instead of dropping them entirely.
- Checkpoint-only marker behavior:
- preserves spool sequence progression through normal flusher path
- strips raw payload in memory to reduce RAM impact
- skips SQLite insert work
- Flusher commit logic now splits drained batch into:
- write batch (normal records) for SQLite
- checkpoint sequence watermark over full batch (including checkpoint-only markers)
- Result:
- duplicate rows remain deduplicated in SQLite
- checkpoint now advances through duplicate spool sequence during normal runtime
- replay-pending duplicate residues no longer require restart recovery to clear
- Added flusher unit test:
- checkpoint-only batch advances checkpoint without SQLite store writes
- Updated integration tests:
- immediate duplicate ingest still does not create duplicate SQLite rows
- checkpoint advances through duplicate spooled records during normal runtime
- restart recovery remains correct with no duplicate replay backlog after normal flush
- Updated README flush policy section with brief checkpoint-only marker explanation.

Architectural decisions:
- Decision: Represent deduped duplicates as checkpoint-only records in RAM buffer + flusher pipeline.
  Reason: Keeps architecture minimal and single-writer friendly while preserving dedupe intent and fixing checkpoint progression correctness.

Files changed:
- `internal/ingest/models.go`
- `internal/buffer/manager.go`
- `internal/buffer/manager_test.go`
- `internal/flusher/flusher.go`
- `internal/flusher/flusher_test.go`
- `internal/tasks/ingest_pipeline_integration_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/ingest/models.go internal/buffer/manager.go internal/buffer/manager_test.go internal/flusher/flusher.go internal/flusher/flusher_test.go internal/tasks/ingest_pipeline_integration_test.go`
- `go test ./internal/buffer ./internal/flusher ./internal/tasks -run 'TestIntegration_Duplicate|TestManager_Dedupe|TestFlusher_CheckpointOnlyBatchAdvancesWithoutStoreWrite' -count=1`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Continue auth hardening beyond minimal single-user assumptions.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:55 - Phase 43 (Point History Query Endpoint)
Implemented:
- Added lightweight map-friendly point history endpoint: `GET /api/v1/points`.
- Endpoint supports optional query params:
- `from` (RFC3339)
- `to` (RFC3339)
- `device_id`
- `limit`
- Response shape remains compact using existing point projection fields:
- `seq`
- `device_id`
- `source_type`
- `timestamp_utc`
- `lat`
- `lon`
- Added API query validation for invalid timestamps and invalid limit.
- Added SQLite store query path for point history with:
- optional range/device filtering
- ascending timestamp/sequence ordering
- bounded limit for low-memory behavior (default `500`, max `5000`)
- Reused existing export filter model and existing response DTO patterns to keep implementation small and consistent.
- Added tests:
- API tests for default query, range filtering, device filtering, invalid query params
- store test for ascending order + filters + limit behavior
- Updated README with `/api/v1/points` usage and curl examples.

Architectural decisions:
- Decision: Add `ListPoints(...)` in store with SQL-level limit/filter/order rather than in-memory filtering.
  Reason: Keeps endpoint Raspberry Pi friendly by bounding memory and avoiding unnecessary row materialization.

Files changed:
- `internal/api/health.go`
- `internal/api/points.go`
- `internal/api/points_test.go`
- `internal/store/points.go`
- `internal/store/sqlite_store_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/health.go internal/api/points.go internal/api/points_test.go internal/store/points.go internal/store/sqlite_store_test.go`
- `go test ./internal/api ./internal/store`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Continue auth hardening beyond current minimal single-user assumptions.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 14:58 - Phase 44 (Lightweight Map UI Page)
Implemented:
- Added new minimal map UI route: `GET /ui/map`.
- Implemented map page with plain HTML/CSS/vanilla JS (no build tooling, no SPA framework).
- Loaded Leaflet from CDN and OpenStreetMap tiles directly from browser.
- Map page fetches backend point history via `GET /api/v1/points`.
- Added simple filter controls:
- `device_id`
- `from`
- `to`
- `limit`
- Render behavior:
- draws track polyline from ordered points
- adds lightweight point markers for smaller sets (<=500) to keep overhead low
- Added focused UI test verifying `/ui/map`:
- returns HTML successfully
- includes expected map container structure
- includes Leaflet references
- Updated README Minimal Web UI section with map page route and behavior.

Architectural decisions:
- Decision: Use CDN Leaflet + existing `/api/v1/points` endpoint for map rendering instead of adding backend GIS/tile components.
  Reason: Keeps implementation lightweight and Pi-friendly while delivering usable map visualization quickly.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Continue auth hardening beyond current minimal single-user assumptions.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 15:01 - Phase 45 (Map UI Date + Device Filtering)
Implemented:
- Extended map UI filters for date and device selection with lightweight controls:
- date range inputs (`from` day / `to` day)
- device dropdown (`/api/v1/devices`) when available
- refresh button
- Added sensible default range when no date filters are provided:
- auto-populates last 7 days (UTC day boundaries)
- Map query now translates day inputs to RFC3339 range (`T00:00:00Z` through `T23:59:59.999Z`).
- Map reload and redraw behavior remains simple:
- refresh clears old layers
- re-fetches from `GET /api/v1/points`
- redraws polyline and optional lightweight markers
- Added practical UI test assertions to confirm map page contains:
- device select control
- date range inputs
- refresh button text
- Leaflet/map structure
- Updated README map section with new filtering behavior and default range note.

Architectural decisions:
- Decision: Keep filtering logic in browser and reuse existing `/api/v1/points` + `/api/v1/devices` endpoints.
  Reason: Avoids extra backend complexity and keeps UI responsive and low-overhead for Pi deployment.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(MapPageServedAtUIMap|StatusPageServedAtRoot)' -count=1`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Continue auth hardening beyond current minimal single-user assumptions.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 15:07 - Phase 46 (Lightweight Visit Detection)
Implemented:
- Added lightweight deterministic visit detection for stored points.
- Added visit persistence schema migration:
- `migrations/0003_visits.sql` with `visits` table and device/start index
- Implemented small visit detector package:
- configurable `MinDwell` and `MaxRadiusMeters`
- deterministic centroid/radius window detection
- no clustering dependencies, no heavy background services
- Added store-side visit system methods:
- `RebuildVisitsForDevice(ctx, deviceID, cfg)` to detect from stored points and rewrite visit rows for that device
- `ListVisits(ctx, deviceID, limit)` to read persisted visits
- Visit model stores required fields:
- `id`
- `device_id`
- `start_at`
- `end_at`
- `centroid_lat`
- `centroid_lon`
- `point_count`
- Added deterministic tests covering required scenarios against SQLite-backed stored points:
- stationary points become a visit
- moving points do not become visits
- two separate visits are not merged
- Updated README with short visit detection explanation.

Architectural decisions:
- Decision: Use a simple per-device rebuild pass over stored points with deterministic radius+dwell checks.
  Reason: Keeps CPU/RAM/storage overhead low on Pi Zero 2 W while remaining correct and understandable.
- Decision: Persist visits in a separate `visits` table linked to `devices`.
  Reason: Keeps visit data queryable without changing raw point durability flow.

Files changed:
- `migrations/0003_visits.sql`
- `internal/visits/detector.go`
- `internal/store/visits.go`
- `internal/store/visits_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/visits/detector.go internal/store/visits.go internal/store/visits_test.go`
- `go test ./internal/store -run 'TestVisitDetection_' -count=1`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Decide when visit detection should run operationally (manual trigger vs scheduled/background pass) and expose it via a lightweight API/command.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 15:13 - Phase 47 (Visit Generation Workflow Integration)
Implemented:
- Integrated visit generation into application workflow via lightweight on-demand API.
- Added visit routes:
- `POST /api/v1/visits/generate`
- `GET /api/v1/visits`
- Generation endpoint behavior:
- requires `device_id`
- supports bounded optional `from`/`to` RFC3339 range
- defaults to recent 14-day window when range is omitted
- optional tuning params: `min_dwell` (duration), `max_radius_m` (meters)
- Bounded generation implementation:
- added `RebuildVisitsForDeviceRange(...)` in store
- avoids full-history recomputation by operating on device + date window
- rewrites only visits whose `start_at` is inside the target window
- Kept existing full-device helper by delegating `RebuildVisitsForDevice(...)` to range method with nil bounds.
- Added tests for workflow:
- API tests for generate/list endpoints and invalid params
- store range test proving bounded generation only persists visits from selected window
- Wired store into server dependencies as `VisitStore`.
- Updated README with explicit visit generation workflow and curl examples.

Architectural decisions:
- Decision: Use explicit on-demand endpoint as first workflow integration mechanism.
  Reason: Keeps operations simple/observable while avoiding always-on background CPU usage on Pi.
- Decision: Use bounded per-device window generation by default (14 days).
  Reason: Reduces recomputation cost and keeps memory/CPU usage predictable.

Files changed:
- `internal/api/health.go`
- `internal/api/visits.go`
- `internal/api/visits_test.go`
- `internal/store/visits.go`
- `internal/store/visits_test.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/health.go internal/api/visits.go internal/api/visits_test.go internal/store/visits.go internal/store/visits_test.go cmd/server/main.go`
- `go test ./internal/api -run 'TestGenerateVisitsEndpoint_|TestListVisitsEndpoint' -count=1`
- `go test ./internal/store -run 'TestVisitDetection_' -count=1`
- `go test ./...`

Pending:
- Add targeted integration check that pressure-triggered ingest flush advances checkpoint without waiting for timer tick.
- Harden checkpoint-failure path to preserve drained records safely (requeue strategy) without breaking current flow.
- Consider lightweight scheduling trigger for visit generation (optional periodic maintenance) after on-demand workflow stabilizes.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Visit range generation rewrites visits by `start_at` window, so visits spanning window edges may be clipped by chosen range.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

### 2026-04-22 21:03 UTC - Phase 48 (Visits Listing Filters + Map Visits + UI Summary)
Implemented:
- Completed `codex_tasks.md` Tasks 1-3 sequentially and verified each stage.
- Task 1: Expanded `GET /api/v1/visits` query support to include optional:
- `device_id`
- `from` (RFC3339)
- `to` (RFC3339)
- `limit`
- Added `from <= to` validation on visits list endpoint.
- Extended visit store listing query to apply optional time-range filters (`start_at` bounded by `from`/`to`) in SQL.
- Added/updated tests for:
- list endpoint with device + time range
- invalid visit list params
- store-level visit time-range filtering behavior
- Task 2: Extended map UI to render visits:
- loads visits from `/api/v1/visits` using selected device/date filters
- draws lightweight centroid markers with visit popups (start/end/point_count/device)
- keeps existing track/polyline rendering flow intact
- adds graceful fallback message when no visits exist (or visits endpoint is unavailable)
- Task 3: Added lightweight visits summary UI section:
- small summary table below map
- columns include start time, end time, duration, and device id
- summary updates from the same visits API payload used for map markers
- Added route/render test assertions for visits endpoint usage, visit layer hook, and summary section structure.
- Updated README with:
- filtered `GET /api/v1/visits` curl example (`from`/`to`)
- map page note for visit marker rendering
- map page note for visits summary table

Architectural decisions:
- Decision: Reuse existing `GET /api/v1/visits` for both map markers and summary table (single lightweight fetch path).
  Reason: Keeps UI logic simple and low-overhead for Raspberry Pi Zero 2 W without extra endpoints or state layers.
- Decision: Apply visit time filtering in SQLite query (`start_at` bounds) instead of post-filtering in API/UI.
  Reason: Reduces response size and memory work on constrained hardware.

Files changed:
- `internal/api/health.go`
- `internal/api/visits.go`
- `internal/api/visits_test.go`
- `internal/store/visits.go`
- `internal/store/visits_test.go`
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'Test(ListVisitsEndpoint|ListVisitsEndpoint_InvalidParams|GenerateVisitsEndpoint_)' -count=1`
- `go test ./internal/store -run 'Test(VisitDetection_|ListVisits_FilterByTimeRange)' -count=1`
- `go test ./internal/api -run 'TestMapPageServedAtUIMap|TestStatusPageServedAtRoot|TestListVisitsEndpoint|TestGenerateVisitsEndpoint_' -count=1`
- `go test ./internal/api -count=1`
- `go test ./internal/store -count=1`
- `go test ./... -count=1`

Pending:
- Add API tests that exercise `GET /api/v1/visits` against real SQLite store via integration wiring (not only fake-store handler tests).
- Consider adding optional `point_count` or duration filters to visits query if map dataset grows.
- Continue durability hardening follow-up for checkpoint advancement failure requeue path in flusher.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.
- Visit range generation rewrites visits by `start_at` window, so visits spanning window edges may be clipped by chosen range.
- Device auth model remains single-user/minimal and should be hardened for multi-user scenarios.

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

### 2026-04-22 21:38 UTC - Phase 49 (Optional Visit-Centroid Reverse Geocode Cache)
Implemented:
- Added optional reverse geocode cache flow for visit centroids only (no per-point geocoding).
- Added SQLite cache migration:
- `migrations/0004_visit_place_cache.sql`
- table: `visit_place_cache(provider, lat_key, lon_key, label, updated_at)`
- Added store cache methods:
- `GetVisitPlaceLabel(...)`
- `UpsertVisitPlaceLabel(...)`
- Added pluggable reverse-geocode resolver under `internal/visits`:
- provider interface (`Name`, `ReverseGeocode`)
- cache interface (`GetVisitPlaceLabel`, `UpsertVisitPlaceLabel`)
- optional resolver (`Enabled`, `ResolveVisitLabel`, per-request provider budget)
- Added lightweight Nominatim provider implementation (std-lib HTTP only, timeout + User-Agent support).
- Wired optional resolver into server startup via config; disabled by default.
- Extended `GET /api/v1/visits` response with optional `place_label`.
- Visit list behavior now:
- cache-first lookup for each returned visit centroid
- provider called only on cache miss and only while per-request budget remains
- no resolver errors fail the visit list response (best-effort enrichment)
- Updated map UI popup to show place label when available.
- Added tests for cache behavior:
- resolver cache-hit skips provider
- resolver miss stores label and subsequent lookup uses cache
- disallowed provider mode returns no label without network call
- provider error path remains contained
- Added SQLite-backed cache upsert/read test.
- Added API test ensuring label resolver budget is respected and labels are included when available.
- Updated README with reverse geocode cache behavior and environment knobs.

Architectural decisions:
- Decision: Keep reverse geocoding optional and centered on `GET /api/v1/visits` output.
  Reason: Avoids background/network churn and keeps compute/network overhead low on Raspberry Pi.
- Decision: Cache by rounded centroid keys (`APP_REVERSE_GEOCODE_CACHE_DECIMALS`) in SQLite.
  Reason: Simple deterministic keying reduces duplicate lookups while keeping storage/query cost low.
- Decision: Bound provider calls per request (`APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST`).
  Reason: Limits network usage spikes and keeps endpoint latency predictable.

Files changed:
- `migrations/0004_visit_place_cache.sql`
- `internal/store/visit_place_cache.go`
- `internal/visits/reverse_geocode_resolver.go`
- `internal/visits/reverse_geocode_provider_nominatim.go`
- `internal/visits/reverse_geocode_resolver_test.go`
- `internal/config/config.go`
- `internal/api/health.go`
- `internal/api/visits.go`
- `internal/api/visits_test.go`
- `internal/store/visits_test.go`
- `internal/api/ui.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/visits -count=1`
- `go test ./internal/store -run 'TestVisit(Detection_|PlaceCache_)|TestListVisits_FilterByTimeRange' -count=1`
- `go test ./internal/api -run 'Test(ListVisitsEndpoint|ListVisitsEndpoint_InvalidParams|ListVisitsEndpoint_WithVisitLabelResolver|GenerateVisitsEndpoint_)' -count=1`
- `go test ./... -count=1`

Pending:
- Add optional lightweight cache invalidation/refresh policy (currently cache entries persist until overwritten).
- Consider exposing provider/cache status counters in `/api/v1/status` if operational visibility is needed.
- Continue flusher checkpoint-failure requeue hardening work.

Known issues:
- Reverse geocode labels depend on external provider availability/terms when enabled.
- `gofmt -w` was blocked in this session by read-only filesystem policy for direct write commands (code compiles/tests pass).
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 00:01 UTC - Phase 50 (Lightweight Dark Mode Toggle UI)
Implemented:
- Added lightweight dark mode support to both existing server-rendered UI pages:
- status page (`GET /`, `GET /ui/status`)
- map page (`GET /ui/map`)
- Added top-of-page accessible theme toggle button (`id="theme_toggle"`) with sun/moon icon behavior.
- Theme behavior:
- toggles light/dark immediately without page reload
- persists preference in `localStorage` (`plexplore.theme`)
- applies saved preference on page load
- falls back to system preference (`prefers-color-scheme: dark`) when no saved preference exists
- Updated CSS tokens to support dark mode across existing UI components:
- page background
- text/muted text
- cards and borders
- tables/status sections/recent points/visits summary
- form controls on map page (`input`, `select`, `button`)
- Kept layout and architecture unchanged (plain HTML/CSS/vanilla JS; no build tooling/dependencies).
- Updated UI tests to verify:
- theme toggle is rendered on status page and map page
- dark-mode script hooks are present (`localStorage`, `prefers-color-scheme`)
- Updated README with short dark mode behavior notes for both status and map pages.

Architectural decisions:
- Decision: Keep theme logic embedded in existing server-rendered templates rather than introducing shared frontend tooling.
  Reason: Preserves low-overhead Pi-friendly deployment model and avoids adding build/runtime dependencies.
- Decision: Use CSS variables with `data-theme` switching on document root.
  Reason: Minimal code change applies theme consistently across current UI sections.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|StatusPage_DoesNotMatchTypoPath)' -count=1`
- `go test ./internal/api -count=1`
- `go test ./... -count=1`

Pending:
- Consider adding `prefers-color-scheme` live-change listener for users who switch OS theme while page is open and no explicit preference is saved.
- Keep UI route tests focused; avoid snapshot-heavy HTML tests.
- Continue flusher checkpoint-failure requeue hardening.

Known issues:
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 00:55 UTC - Phase 51 (Task 1: User Auth Schema + Store Foundations)
Implemented:
- Added user-auth schema migration for admin-created-user model foundations:
- `migrations/0005_users_auth_fields.sql`
- Added users columns:
- `password_hash` (non-null, default empty string)
- `is_admin` (non-null integer flag, default `0`)
- `updated_at` (non-null, default empty string then backfilled)
- Added safe data backfill in migration for:
- null emails -> empty string
- empty/null `updated_at` -> `created_at` fallback (or current UTC fallback)
- Added unique index for non-empty emails:
- `idx_users_email_unique_nonempty` on `users(email)` where `email <> ''`
- Added user store model/methods:
- `CreateUser(...)`
- `GetUserByEmail(...)`
- `GetUserByID(...)`
- `ListUsers(...)`
- Added user store tests:
- create + get by email (case-insensitive)
- list users
- not-found handling
- schema column presence check (`PRAGMA table_info(users)`)
- Updated README with a short "Multi-User Auth Foundation (In Progress)" section.

Architectural decisions:
- Decision: Keep `email` uniqueness via partial unique index (`email <> ''`) instead of forcing immediate full table rewrite.
  Reason: Keeps migration lightweight and compatible with existing default-user bootstrap behavior while enabling account-auth email uniqueness for real accounts.
- Decision: Keep task scope to schema/store only (no login/session yet).
  Reason: Follows ordered milestone plan and keeps change atomic.

Files changed:
- `migrations/0005_users_auth_fields.sql`
- `internal/store/users.go`
- `internal/store/users_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/users.go internal/store/users_test.go`
- `go test ./internal/store ./...`
- `APP_SQLITE_PATH=./data/task1fresh.db make migrate`
- `sqlite3 ./data/task1fresh.db ".schema users"`
- `go test ./internal/store -count=1`

Pending:
- Task 2: Add password hashing helpers (`HashPassword`, `VerifyPassword`) with focused unit tests.
- Task 3+: Continue ordered multi-user auth milestone tasks from `codex_tasks.md`.

Known issues:
- In this shell environment, some write/build commands require elevated execution due sandbox restrictions (`/tmp` write for `go run`/`gofmt`).
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 01:02 UTC - Phase 52 (Task 2: Password Hashing Helpers)
Implemented:
- Added password hashing helpers in existing auth-related package:
- `internal/api/password.go`
- `HashPassword(plain string) (string, error)`
- `VerifyPassword(hash, plain string) bool`
- Implemented bcrypt-based hashing/verification via `golang.org/x/crypto/bcrypt`.
- Added basic validation:
- empty/whitespace password rejected by `HashPassword` (`ErrEmptyPassword`)
- verify returns `false` for empty hash/password
- Added focused unit tests:
- valid hash/verify roundtrip
- wrong password fails verification
- empty password rejected
- Added dependency pin:
- `golang.org/x/crypto v0.24.0` (Go 1.22-compatible)
- Restored module baseline to `go 1.22.0` in `go.mod` after transient `go get` toolchain bump attempt.
- Updated README auth-foundation section with password helper notes.

Architectural decisions:
- Decision: Place helpers in `internal/api` auth-related package due environment constraint preventing creation of new internal directory for a separate `security` package.
  Reason: Keep Task 2 atomic and unblock ordered milestone progress while maintaining isolated helper functions.
- Decision: Use bcrypt with default cost for first pass.
  Reason: Standard, lightweight, and sufficient for current scope without introducing heavier config surface yet.

Files changed:
- `internal/api/password.go`
- `internal/api/password_test.go`
- `go.mod`
- `go.sum`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'Test(HashAndVerifyPassword|VerifyPassword_WrongPasswordFails|HashPassword_EmptyRejected)' -count=1`
- `go test ./... -count=1`
- `gofmt -w internal/api/password.go internal/api/password_test.go`

Pending:
- Task 3: Add admin bootstrap path (preferred CLI tool) without public signup.
- Task 4+: Continue sequential multi-user auth plan.

Known issues:
- In this shell environment, some write/build commands require elevated execution due sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 01:24 UTC - Phase 53 (Task 3: Admin Bootstrap CLI Path)
Implemented:
- Added admin bootstrap path without public signup by extending CLI workflow in `cmd/migrate`:
- new mode flag: `--create-admin`
- required flags in admin mode: `--email`, `--password`
- optional flags: `--db`, `--migrations`, `--is-admin` (must be true)
- CLI behavior in `--create-admin` mode:
- runs migrations first
- checks for existing user by email
- blocks duplicate admin creation for same email
- blocks creating admin over existing non-admin account with same email
- hashes password via bcrypt helper before persistence
- creates user with `is_admin=true`
- Added CLI tests:
- successful admin bootstrap creation
- duplicate bootstrap blocked
- validation errors for missing/invalid flags
- Updated README with explicit admin bootstrap usage and behavior notes.
- Performed manual validation on fresh DB:
- migrate
- bootstrap admin
- query users table for admin row
- confirm password hash length/non-plaintext

Architectural decisions:
- Decision: Implement bootstrap mode as `go run ./cmd/migrate --create-admin ...` instead of new `cmd/createadmin` binary due current environment constraint on creating a new command directory.
  Reason: Keeps behavior explicit/safe and unblocks sequential task delivery while preserving lightweight CLI operation.

Files changed:
- `cmd/migrate/main.go`
- `cmd/migrate/main_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./cmd/migrate -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db make migrate`
- `go run ./cmd/migrate --db ./data/task3bootstrap.db --migrations ./migrations --create-admin --email admin@example.com --password testpass`
- `sqlite3 ./data/task3bootstrap.db "SELECT COUNT(*), COALESCE(MAX(email),''), COALESCE(MAX(is_admin),0) FROM users;"`
- `sqlite3 ./data/task3bootstrap.db "SELECT email, is_admin, LENGTH(password_hash), password_hash='testpass' FROM users;"`

Pending:
- Task 4: Add session model/migration and middleware for browser auth.
- Task 5+: Continue sequential multi-user auth tasks.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 05:16 UTC - Phase 79 (Baseline Security Headers + Self-Hosted Leaflet Assets)
Implemented:
- Switched map UI from CDN Leaflet to self-hosted local assets served by Plexplore.
- Vendored Leaflet assets into repo:
- `internal/api/assets/leaflet/leaflet.js`
- `internal/api/assets/leaflet/leaflet.css`
- `internal/api/assets/leaflet/images/marker-icon.png`
- `internal/api/assets/leaflet/images/marker-icon-2x.png`
- `internal/api/assets/leaflet/images/marker-shadow.png`
- Added embedded static asset serving route:
- `GET /ui/assets/{path...}` via `internal/api/ui_assets.go`
- Updated map page HTML to load local Leaflet assets:
- `/ui/assets/leaflet/leaflet.css`
- `/ui/assets/leaflet/leaflet.js`
- Added baseline security-header helpers and applied them to UI and JSON responses:
- `Content-Security-Policy` (HTML pages)
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: geolocation=(), camera=(), microphone=()`
- Applied HTML header set to status/map/users/login pages; applied common security headers to JSON responses via shared `writeJSON`.
- Updated tests to verify:
- map page references local Leaflet URLs and no longer references CDN URLs
- local Leaflet asset routes serve successfully (CSS + marker icon)
- expected security headers are present on UI responses
- `/health` still works and returns security hardening header(s)
- Updated README notes for self-hosted Leaflet and current CSP posture.

Architectural decisions:
- Decision: Serve Leaflet as embedded local static assets under `/ui/assets`.
  Reason: Remove third-party CDN dependency while keeping the UI lightweight and deployment-simple for self-hosted Raspberry Pi environments.
- Decision: Enforce baseline response hardening headers via lightweight helper functions, with CSP on HTML responses.
  Reason: Improve browser-side safety with minimal code and no new dependencies.
- Decision: Keep CSP compatible with existing inline scripts/styles by allowing `'unsafe-inline'` temporarily.
  Reason: Preserve current lightweight no-build UI behavior while still restricting origins and moving toward stricter CSP in a future step.

Files changed:
- `internal/api/security_headers.go`
- `internal/api/ui_assets.go`
- `internal/api/ui.go`
- `internal/api/login.go`
- `internal/api/devices.go`
- `internal/api/health.go`
- `internal/api/ui_test.go`
- `internal/api/status_test.go`
- `internal/api/assets/leaflet/leaflet.js`
- `internal/api/assets/leaflet/leaflet.css`
- `internal/api/assets/leaflet/images/marker-icon.png`
- `internal/api/assets/leaflet/images/marker-icon-2x.png`
- `internal/api/assets/leaflet/images/marker-shadow.png`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/security_headers.go internal/api/ui_assets.go internal/api/ui.go internal/api/login.go internal/api/devices.go internal/api/health.go internal/api/ui_test.go internal/api/status_test.go`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|UIAssets_LeafletServedLocally|UIAssets_LeafletIconServedLocally|HealthEndpoint_RemainsPublic)' -count=1`
- `go test ./...`
- `curl -sS -D - -o /dev/null http://127.0.0.1:8080/ui/map`
- `curl -sS http://127.0.0.1:8080/ui/map | rg 'ui/assets/leaflet|unpkg.com/leaflet'`
- `curl -sS -D - -o /dev/null http://127.0.0.1:8080/ui/assets/leaflet/leaflet.css`

Pending:
- Move inline UI scripts/styles to external static files to remove CSP `'unsafe-inline'` allowance.
- Optionally self-host map tiles (or support configurable tile provider) for fully local/offline deployments.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 06:24 UTC - Phase 85 (Route Helper Fallback Audit Verification)
Implemented:
- Audited shared runtime route helper functions to verify no remaining permissive fallback registrations exist.
- Confirmed shared helpers are fail-closed and dependency-explicit:
- `registerDeviceRoutesWithAuth` requires `deviceStore + userStore + sessionStore` and panics if missing
- `registerPointRoutes` requires `pointStore + deviceStore + userStore + sessionStore` and panics if missing
- `registerExportRoutes` requires `pointStore + deviceStore + userStore + sessionStore` and panics if missing
- `registerVisitRoutes` requires `visitStore + deviceStore + userStore + sessionStore` and panics if missing
- `registerUIRoutes` requires `userStore + sessionStore` and panics if missing
- `registerUserRoutes` requires `userStore + sessionStore` and panics if missing
- Confirmed runtime registration path remains explicit/fail-closed in `RegisterRoutesWithDependencies(...)` (protected routes only registered when required deps are present).
- Confirmed permissive route wiring remains test-only via `registerRoutesWithTestFallbacks(...)` in `*_test.go`.
- Re-ran focused route-security tests to validate:
- helper fail-closed behavior
- protected route unauth denial
- alias protection consistency
- admin denial for non-admin sessions
- typo/unknown 404 behavior

Architectural decisions:
- Decision: No additional runtime code changes required in this task.
  Reason: Shared route helper fallback removal and fail-closed contracts were already correctly implemented; this task is verified complete by audit + tests.

Files changed:
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `rg -n "func register(DeviceRoutesWithAuth|PointRoutes|ExportRoutes|VisitRoutes|UIRoutes|UserRoutes|StatusRoutes)|RegisterRoutesWithDependencies|registerRoutesWithTestFallbacks" internal/api/*.go internal/api/*_test.go`
- `go test ./internal/api -run 'Test(RuntimeRouter_|RouteHelpers_FailClosed_WhenAuthDepsMissing|UIRoutesRequireSession_WhenSessionDepsProvided|StatusPage_DoesNotMatchTypoPath|AdminUsersPageDeniedForNonAdminSession|HealthEndpoint_RemainsPublic)' -count=1`

Pending:
- Optional: add small package-level comments in route helper files documenting required dependencies explicitly near each helper signature.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 06:19 UTC - Phase 84 (Clarify HSTS Ownership + Future In-App TLS Condition)
Implemented:
- Clarified HSTS strategy in README to remove ambiguity:
- reverse proxy owns HSTS for current production topology
- app should not emit HSTS on plain HTTP responses
- in-app HSTS is future-only and should only be added if app directly terminates HTTPS/TLS
- Added explicit note in security header middleware comment:
- `setCommonSecurityHeaders(...)` intentionally does not set HSTS today
- Added/updated tests to enforce no in-app HSTS expectation on local HTTP responses:
- `internal/api/status_test.go` now checks `/health` does not include `Strict-Transport-Security`
- `internal/api/ui_test.go` now checks status UI response does not include `Strict-Transport-Security`
- Validated header behavior with local HTTP `curl -I` check:
- response headers did not include `Strict-Transport-Security`

Architectural decisions:
- Decision: Keep HSTS exclusively at TLS-terminating reverse proxy layer until direct in-app HTTPS support exists.
  Reason: Avoids misleading HTTP responses and keeps HTTPS policy enforcement at the actual TLS boundary.

Files changed:
- `README.md`
- `internal/api/security_headers.go`
- `internal/api/status_test.go`
- `internal/api/ui_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/security_headers.go internal/api/status_test.go internal/api/ui_test.go`
- `go test ./...`
- `go run ./cmd/server`
- `curl -I http://127.0.0.1:8080/`

Pending:
- If direct HTTPS termination is ever added to the app, revisit HSTS strategy and gate in-app HSTS emission behind explicit HTTPS-mode config.
- Keep reverse-proxy HSTS examples aligned with deployment templates as proxy docs evolve.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 06:12 UTC - Phase 83 (Reverse Proxy HSTS Documentation + Examples)
Implemented:
- Added production reverse-proxy HSTS guidance to README.
- Documented that HSTS should be configured at the TLS-terminating reverse proxy layer (not in-app, unless app directly terminates HTTPS in the future).
- Added conservative HSTS recommendation:
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- Explicitly documented:
- do not enable HSTS for local HTTP development
- do not use `preload` by default
- use preload only when all subdomains are permanently HTTPS-only and operator understands implications
- Added minimal practical proxy configuration examples directly in README:
- Caddy site block with HSTS and reverse proxy to app
- nginx server block with HSTS, proxy headers, and `X-Forwarded-Proto https`
- Aligned docs with current cookie/TLS production settings and explicit insecure dev mode behavior.

Architectural decisions:
- Decision: Keep HSTS enforcement at reverse proxy layer for current architecture.
  Reason: App currently serves HTTP behind TLS-terminating proxy in production; proxy is the correct TLS/HSTS boundary.

Files changed:
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `curl -I https://your-domain.example`
- `curl -I https://your-domain.example | rg -i 'strict-transport-security'`
- `curl -I http://127.0.0.1:8080 | rg -i 'strict-transport-security'`
- `curl -I https://your-domain.example | rg -i 'set-cookie|strict-transport-security|x-frame-options|content-security-policy'`

Pending:
- Optional: add dedicated reverse-proxy example files under `deploy/proxy/` when writable environment permits.
- Optional: add Traefik example labels/snippet for operators using container-native proxy stacks.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 06:03 UTC - Phase 82 (Shared Route Helper Fail-Closed Hardening)
Implemented:
- Removed remaining permissive fallback behavior from shared protected route helper functions.
- Hardened helpers to fail closed with explicit dependency checks (panic on missing required deps):
- `registerDeviceRoutesWithAuth(...)` now requires non-nil `deviceStore`, `userStore`, `sessionStore`
- `registerPointRoutes(...)` now requires non-nil `pointStore`, `deviceStore`, `userStore`, `sessionStore`
- `registerExportRoutes(...)` now requires non-nil `pointStore`, `deviceStore`, `userStore`, `sessionStore`
- `registerVisitRoutes(...)` now requires non-nil `visitStore`, `deviceStore`, `userStore`, `sessionStore`
- `registerUIRoutes(...)` now requires non-nil `userStore`, `sessionStore`
- `registerUserRoutes(...)` now requires non-nil `userStore`, `sessionStore`
- Removed legacy permissive shared wrapper `registerDeviceRoutes(...)` that previously delegated with nil auth deps.
- Kept runtime wiring explicit and fail-closed via `RegisterRoutesWithDependencies(...)` gate checks from prior phase.
- Updated test-only router builder `registerRoutesWithTestFallbacks(...)` to register permissive handler routes directly (test scope only), instead of calling fail-closed shared helpers with missing auth deps.
- Added route-helper hardening tests:
- `TestRouteHelpers_FailClosed_WhenAuthDepsMissing` validates missing-dependency panics for protected helper registrations.
- Confirmed runtime behavior still blocks accidental protected exposure and public routes remain minimal.

Architectural decisions:
- Decision: Shared protected route helpers now fail fast on missing auth/session dependencies.
  Reason: Prevents future alternate entrypoints from accidentally exposing protected UI/API routes via helper misuse.
- Decision: Keep permissive fallback registration confined to clearly named test-only helper code.
  Reason: Maintains targeted handler tests without weakening production/runtime helper semantics.

Files changed:
- `internal/api/devices.go`
- `internal/api/points.go`
- `internal/api/exports.go`
- `internal/api/visits.go`
- `internal/api/ui.go`
- `internal/api/users.go`
- `internal/api/routes_test_helpers_test.go`
- `internal/api/router_security_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/devices.go internal/api/points.go internal/api/exports.go internal/api/visits.go internal/api/ui.go internal/api/users.go internal/api/routes_test_helpers_test.go internal/api/router_security_test.go`
- `go test ./internal/api -count=1`
- `go test ./...`

Pending:
- Optional: add package-level router registration docs/comments showing expected dependency contracts per helper for faster onboarding and safer future entrypoint additions.
- Optional: evaluate replacing helper panics with explicit error-returning registration API if startup error propagation is preferred over panic semantics.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 05:52 UTC - Phase 81 (Docker/Runtime Production-Safe Defaults + Explicit Insecure Dev Opt-In)
Implemented:
- Hardened runtime startup security validation with fail-fast behavior:
- `APP_COOKIE_SECURE_MODE=never` now requires explicit `APP_ALLOW_INSECURE_HTTP=true`
- `APP_DEPLOYMENT_MODE=production` now requires `APP_COOKIE_SECURE_MODE=always`
- `APP_DEPLOYMENT_MODE=production` now rejects `APP_ALLOW_INSECURE_HTTP=true`
- Added explicit config knob:
- `APP_ALLOW_INSECURE_HTTP` (default `false`)
- Updated startup warning behavior to avoid proxy-warning noise when cookie mode is already `always`.
- Updated Docker image defaults to production-oriented safe posture:
- `APP_DEPLOYMENT_MODE=production`
- `APP_COOKIE_SECURE_MODE=always`
- `APP_EXPECT_TLS_TERMINATION=true`
- `APP_TRUST_PROXY_HEADERS=false`
- `APP_ALLOW_INSECURE_HTTP=false`
- `APP_HTTP_LISTEN_ADDR=0.0.0.0:8080` (container bind)
- Updated `compose.yaml` to keep production-oriented defaults explicit and include `APP_ALLOW_INSECURE_HTTP=false`.
- Updated systemd sample env to include explicit insecure-mode toggle (`APP_ALLOW_INSECURE_HTTP=false`).
- Updated README:
- clear Docker production vs local-insecure modes
- explicit insecure local HTTP opt-in instructions
- fail-fast runtime security rules
- updated container default env documentation
- Added/updated tests:
- config defaults and explicit insecure opt-in behavior
- server runtime security validation for production/insecure combinations

Architectural decisions:
- Decision: Use fail-fast startup validation for unsafe cookie/runtime combinations rather than warning-only.
  Reason: Users often run container images directly; failing fast prevents silent insecure deployment drift and enforces explicit intent for insecure local mode.
- Decision: Keep insecure local HTTP possible only through explicit `APP_ALLOW_INSECURE_HTTP=true`.
  Reason: Preserves developer ergonomics while making insecure behavior obvious and deliberate.

Files changed:
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `Dockerfile`
- `compose.yaml`
- `deploy/systemd/plexplore.env.sample`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/config/config.go internal/config/config_test.go cmd/server/main.go cmd/server/main_test.go`
- `go test ./internal/config -count=1`
- `go test ./cmd/server -count=1`
- `go test ./...`
- `docker build -t plexplore:latest .`
- `docker run --rm -p 127.0.0.1:8080:8080 -v "$(pwd)/data:/data" plexplore:latest`
- `docker run --rm -p 127.0.0.1:8080:8080 -v "$(pwd)/data:/data" -e APP_DEPLOYMENT_MODE=development -e APP_COOKIE_SECURE_MODE=never -e APP_ALLOW_INSECURE_HTTP=true -e APP_EXPECT_TLS_TERMINATION=false plexplore:latest`

Pending:
- Add optional startup self-check endpoint/diagnostic output for deployment-mode/security-mode summary to reduce operator confusion.
- Evaluate adding a separate development compose override file for explicit insecure local workflows.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 05:40 UTC - Phase 80 (Route Registration Hardening: Remove Runtime Auth Fallbacks)
Implemented:
- Audited route registration and removed runtime reliance on unauthenticated fallback registration for protected functionality.
- Tightened `RegisterRoutesWithDependencies` to only register protected routes when required auth dependencies are present:
- UI protected pages (`/`, `/ui/status`, `/ui/map`, `/ui/admin/users`) now register only when `UserStore` + `SessionStore` are configured.
- Device management APIs now register only when `DeviceStore` + `UserStore` + `SessionStore` are configured.
- Points/exports routes now register only when `PointStore` + `DeviceStore` + `UserStore` + `SessionStore` are configured.
- Visits routes now register only when `VisitStore` + `DeviceStore` + `UserStore` + `SessionStore` are configured.
- Login/user-admin routes remain explicit and require `UserStore` + `SessionStore`.
- Kept ingest device-auth endpoints separate (API-key protected) and unchanged.
- Hardened detailed status registration:
- `/api/v1/status` is now registered only when session auth dependencies exist.
- `/status` remains public-safe.
- Simplified `registerUIRoutes(...)` to always apply session-auth middleware and removed its unauthenticated runtime fallback branch.
- Added explicit test-only fallback wiring helper:
- `registerRoutesWithTestFallbacks(...)` in `internal/api/routes_test_helpers_test.go`
- preserves legacy unauth route wiring only for handler-behavior tests that intentionally bypass auth setup.
- Updated tests using legacy fallback behavior (`devices/points/exports/visits`) to use test-only helper.
- Added routing security coverage in `internal/api/router_security_test.go`:
- verifies protected routes are not registered via runtime fallback without auth deps
- verifies anonymous denial across protected UI/API routes when auth deps are configured
- verifies canonical/alias protection consistency (`/` and `/ui/status`)
- verifies admin-only routes reject authenticated non-admin sessions
- verifies intentional public routes remain reachable (`/health`, `/status`, `/login`)
- Updated README with explicit public/authenticated/admin route model and note about test-only fallback wiring.

Architectural decisions:
- Decision: Runtime router must not expose protected routes without auth dependencies; missing auth deps should result in non-registration (404), not public fallback.
  Reason: Eliminates accidental unauthenticated access paths through alternate/fallback route wiring.
- Decision: Keep fallback wiring only in test helpers.
  Reason: Preserves focused handler tests without weakening production/runtime route safety.

Files changed:
- `internal/api/health.go`
- `internal/api/ui.go`
- `internal/api/status.go`
- `internal/api/routes_test_helpers_test.go`
- `internal/api/router_security_test.go`
- `internal/api/devices_test.go`
- `internal/api/points_test.go`
- `internal/api/exports_test.go`
- `internal/api/visits_test.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/health.go internal/api/ui.go internal/api/status.go internal/api/routes_test_helpers_test.go internal/api/points_test.go internal/api/exports_test.go internal/api/visits_test.go internal/api/devices_test.go internal/api/ui_test.go internal/api/router_security_test.go`
- `go test ./internal/api -count=1`
- `go test ./...`

Pending:
- Consider enforcing explicit startup validation/fatal logs when required auth dependencies for protected feature sets are missing in non-test runtime wiring.
- Review whether any legacy helper constructors still imply unauthenticated behavior and document them clearly as test-only patterns.

Known issues:
- CSP currently still allows `'unsafe-inline'` for scripts/styles to preserve existing inline UI behavior.
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 01:29 UTC - Phase 54 (Task 4: Session Model + Session Loader Middleware)
Implemented:
- Added session persistence migration:
- `migrations/0006_sessions.sql`
- table: `sessions(token, user_id, expires_at, created_at)`
- Added session store methods in `internal/store/sessions.go`:
- `CreateSession(ctx, userID)` with secure random token generation and default TTL
- `GetSession(ctx, token)` with expiration enforcement (expired sessions treated as missing and deleted best-effort)
- `DeleteSession(ctx, token)`
- Added session store tests:
- create/get/delete success
- expired session behavior
- Added API-level session user loader middleware in `internal/api/session_auth.go`:
- `LoadCurrentUserFromSession(...)`
- `CurrentUserFromContext(...)`
- Middleware behavior:
- reads HttpOnly-style session cookie name (`plexplore_session`)
- loads session + user on valid token
- leaves request anonymous for missing/invalid tokens
- keeps device API key auth path unchanged
- Added middleware tests for valid and invalid session-cookie paths.
- Extended API dependency interfaces with `UserStore` and `SessionStore` for upcoming route protection tasks.
- Updated README with session foundation notes.

Architectural decisions:
- Decision: Use server-side SQLite session storage with opaque random token.
  Reason: Lightweight, Pi-friendly, and supports simple revocation/delete behavior without external cache services.
- Decision: Keep session loader middleware non-blocking for now (enrichment-only).
  Reason: Task 4 requires load helper; strict route protection is deferred to Task 6 as planned.

Files changed:
- `migrations/0006_sessions.sql`
- `internal/store/sessions.go`
- `internal/store/sessions_test.go`
- `internal/api/session_auth.go`
- `internal/api/session_auth_test.go`
- `internal/api/health.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/store -run 'TestSQLiteStore_(CreateGetDeleteSession|GetSession_Expired)' -count=1`
- `go test ./internal/api -run 'TestLoadCurrentUserFromSession_' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db make migrate`
- `sqlite3 ./data/task3bootstrap.db ".tables"`

Pending:
- Task 5: Add login/logout endpoints and minimal sign-in page.
- Task 6: Add route protection helpers and enforce session auth on relevant routes.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 01:40 UTC - Phase 55 (Task 5: Login/Logout Endpoints + Sign-In Page)
Implemented:
- Added login routes:
- `GET /login` (minimal server-rendered sign-in page)
- `POST /login` (email/password verification -> create session -> set HttpOnly cookie)
- `POST /logout` (delete session token and expire cookie)
- Added lightweight login page HTML with form-based submission (no frontend framework/tooling).
- Login flow details:
- uses `GetUserByEmail(...)`
- verifies password hash via `VerifyPassword(...)`
- creates server-side session via `CreateSession(...)`
- sets `plexplore_session` cookie (`HttpOnly`, `SameSite=Lax`, path `/`, expiry from session TTL)
- Logout flow details:
- best-effort deletes current session token
- clears cookie via `MaxAge=-1`
- redirects to `/login`
- Wired server dependencies so login/session routes are active in normal server startup:
- `UserStore: sqliteStore`
- `SessionStore: sqliteStore`
- Added API tests:
- login page render
- successful login sets session cookie
- invalid credentials denied
- logout deletes session and clears cookie
- Performed manual validation on running server instance against bootstrap DB:
- `GET /login` returns 200
- `POST /login` returns 303 with `Set-Cookie: plexplore_session=...`
- DB session count increases on login and decreases on logout
- Updated README with login/logout endpoint and curl example.

Architectural decisions:
- Decision: Keep login as form POST and redirect-based flow.
  Reason: Minimal server-rendered UX with low client complexity and zero extra dependencies.
- Decision: Keep session cookie settings simple (`HttpOnly`, `SameSite=Lax`) pending final hardening task.
  Reason: Sufficient baseline for current milestone stage while deferring stricter secure-cookie/CSRF posture to Task 18.

Files changed:
- `internal/api/login.go`
- `internal/api/login_test.go`
- `internal/api/health.go`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LogoutClearsSession)' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db APP_SPOOL_DIR=./data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18080 go run ./cmd/server`
- `curl -sS -w "%{http_code}\n" http://127.0.0.1:18080/login -o /tmp/login_page.html`
- `curl -sS -D - -o /dev/null -X POST http://127.0.0.1:18080/login -H "Content-Type: application/x-www-form-urlencoded" --data "email=admin@example.com&password=testpass"`
- `sqlite3 ./data/task3bootstrap.db "SELECT COUNT(*) FROM sessions;"`
- `curl -sS -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:18080/logout -H "Cookie: plexplore_session=<token>"`

Pending:
- Task 6: add explicit route protection helpers (`RequireUserSessionAuth`, redirect/401 behavior split).
- Task 7+: admin-only user management and full user-data scoping tasks.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 03:03 UTC - Phase 56 (Task 6: Authenticated Route Protection Helpers)
Implemented:
- Added explicit session-auth protection helpers in `internal/api/session_auth.go`:
- `RequireUserSessionAuth(...)` for JSON endpoints (returns `401` JSON when anonymous)
- `RequireUserSessionAuthHTML(...)` for HTML routes (redirects to `/login` when anonymous)
- Updated UI route registration to enforce session auth when session/user dependencies are provided:
- protected routes now include:
- `GET /`
- `GET /ui/status`
- `GET /ui/map`
- Behavior remains backward-compatible for tests/contexts where auth dependencies are not wired.
- Kept device API key ingest auth path unchanged.
- Added/updated tests:
- JSON helper unauthorized behavior
- HTML helper redirect behavior
- UI route protection (anonymous redirect to `/login`)
- UI route success with valid session cookie
- existing UI/login tests remain passing
- Updated README session/login sections with new protection helper and redirect behavior notes.

Architectural decisions:
- Decision: Apply protection to UI routes at registration time using composed middleware (`LoadCurrentUserFromSession` + `RequireUserSessionAuthHTML`).
  Reason: Keeps implementation simple and explicit without introducing global middleware side effects on all endpoints.
- Decision: Keep JSON auth helper added but not broadly applied yet.
  Reason: Task 6 requires helper and behavior; endpoint-by-endpoint JSON enforcement is scheduled in subsequent scoping tasks (Tasks 7+).

Files changed:
- `internal/api/session_auth.go`
- `internal/api/session_auth_test.go`
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `internal/api/health.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'Test(LoadCurrentUserFromSession_|RequireUserSessionAuth_|RequireUserSessionAuthHTML_|UIRoutesRequireSession_|UIRoutesAllowSession_)' -count=1`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|LoginPageServed)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 7: admin-only user management endpoints (`GET/POST /api/v1/users`).
- Task 8+: apply user-scoping and auth enforcement across device/points/export/visits APIs.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 03:22 UTC - Phase 57 (Task 7: Admin-Only User Management Endpoints)
Implemented:
- Added admin-only user management routes:
- `GET /api/v1/users`
- `POST /api/v1/users`
- Route protection chain:
- session load (`LoadCurrentUserFromSession`)
- authenticated check (`RequireUserSessionAuth`)
- admin check (`RequireAdmin`)
- `POST /api/v1/users` behavior:
- validates JSON body with required `email` + `password`
- hashes password via bcrypt helper before persistence
- supports `is_admin` flag
- returns created user fields without exposing `password_hash`
- `GET /api/v1/users` behavior:
- returns list of users without password hashes
- Added tests for required Task 7 scenarios:
- admin can create user
- non-admin cannot create user (`403`)
- unauthenticated request denied (`401`)
- list users response excludes `password_hash`
- Manual Task 7 validation completed on temporary server using bootstrap DB:
- admin login succeeded
- created second user via `POST /api/v1/users` (`201`)
- SQLite rows confirmed (`SELECT id,email,is_admin FROM users`)
- list response verified to exclude password hash fields
- Updated README with admin user management endpoint docs and example.

Architectural decisions:
- Decision: Implement explicit `RequireAdmin` middleware helper.
  Reason: Keeps role checks consistent and reusable as additional admin-only routes are introduced.
- Decision: Keep user-management API JSON-only and lightweight.
  Reason: Aligns with minimal server design and avoids introducing complex admin UI at this stage.

Files changed:
- `internal/api/session_auth.go`
- `internal/api/users.go`
- `internal/api/users_test.go`
- `internal/api/health.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'TestUsers_' -count=1`
- `go test ./internal/api -run 'Test(LoadCurrentUserFromSession_|RequireUserSessionAuth_|RequireAdmin|LoginPageServed)' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db APP_SPOOL_DIR=./data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18081 go run ./cmd/server`
- `curl -sS -c /tmp/task7_cookie.txt -o /tmp/task7_login_body.txt -w "%{http_code}" -X POST http://127.0.0.1:18081/login -H "Content-Type: application/x-www-form-urlencoded" --data "email=admin@example.com&password=testpass"`
- `curl -sS -b /tmp/task7_cookie.txt -o /tmp/task7_create_user.json -w "%{http_code}" -X POST http://127.0.0.1:18081/api/v1/users -H "Content-Type: application/json" --data '{"email":"user2@example.com","password":"user2pass","is_admin":false}'`
- `sqlite3 ./data/task3bootstrap.db "SELECT id,email,is_admin FROM users ORDER BY id;"`
- `curl -sS -b /tmp/task7_cookie.txt http://127.0.0.1:18081/api/v1/users`

Pending:
- Task 8: scope device list/read routes to current signed-in user ownership.
- Task 9+: continue ownership and per-user data scoping tasks.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 03:42 UTC - Phase 58 (Task 8: Device Route Session Scope + Ownership Enforcement)
Implemented:
- Updated device route wiring to use session auth when user/session dependencies are present:
- composed middleware for `/api/v1/devices*`:
- `LoadCurrentUserFromSession`
- `RequireUserSessionAuth`
- Device route ownership behavior with session auth enabled:
- `GET /api/v1/devices` returns only devices owned by current signed-in user
- `GET /api/v1/devices/{id}` returns `404` for non-owner device ids (no cross-user enumeration leak)
- `POST /api/v1/devices/{id}/rotate-key` returns `403` for non-owner
- `POST /api/v1/devices` always associates new device with current session user id (ignores body `user_id`)
- Kept compatibility fallback for test contexts without session/user dependencies (legacy behavior unchanged in those contexts).
- Added Task 8-focused API tests for:
- user sees only own devices
- user cannot fetch another user's device
- rotate key denied for non-owner
- create uses session user ownership
- Manual validation with two non-admin users:
- user2 and user3 each created one device
- list endpoint for each user returned only that user's device
- cross-user direct GET returned `404` with `{"error":"device not found"}`
- Updated README device management section with session-auth scoping semantics.

Architectural decisions:
- Decision: Scope device routes by ownership at API layer while reusing existing store interfaces.
  Reason: Minimal change set for current task without broad store refactor; keeps behavior explicit and easy to audit.
- Decision: Return `404` for non-owner device reads.
  Reason: Avoids leaking resource existence across users.

Files changed:
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `internal/api/health.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'TestDevicesAPI_(UserSeesOnlyOwnDevices_WhenSessionAuthEnabled|UserCannotFetchAnotherUsersDevice_WhenSessionAuthEnabled|RotateKeyDeniedForNonOwner_WhenSessionAuthEnabled|CreateUsesCurrentSessionUser_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db APP_SPOOL_DIR=./data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18082 go run ./cmd/server`
- `curl -sS -c /tmp/t8_user2_cookie.txt -o /tmp/t8_user2_login.txt -w "%{http_code}" -X POST http://127.0.0.1:18082/login -H "Content-Type: application/x-www-form-urlencoded" --data "email=user2@example.com&password=user2pass"`
- `curl -sS -c /tmp/t8_user3_cookie.txt -o /tmp/t8_user3_login.txt -w "%{http_code}" -X POST http://127.0.0.1:18082/login -H "Content-Type: application/x-www-form-urlencoded" --data "email=user3@example.com&password=user3pass"`
- `curl -sS -b /tmp/t8_user2_cookie.txt -X POST http://127.0.0.1:18082/api/v1/devices -H "Content-Type: application/json" --data '{"name":"u2-phone","source_type":"owntracks","api_key":"u2-key-1"}'`
- `curl -sS -b /tmp/t8_user3_cookie.txt -X POST http://127.0.0.1:18082/api/v1/devices -H "Content-Type: application/json" --data '{"name":"u3-phone","source_type":"owntracks","api_key":"u3-key-1"}'`
- `curl -sS -b /tmp/t8_user2_cookie.txt http://127.0.0.1:18082/api/v1/devices`
- `curl -sS -b /tmp/t8_user3_cookie.txt http://127.0.0.1:18082/api/v1/devices`
- `curl -sS -o /tmp/t8_cross_get.txt -w "%{http_code}" -b /tmp/t8_user3_cookie.txt http://127.0.0.1:18082/api/v1/devices/1`

Pending:
- Task 9: finalize multi-user device creation model with explicit admin override policy where appropriate.
- Task 10+: scope points/export/visits APIs to signed-in user.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 03:53 UTC - Phase 59 (Task 9: Device Ownership Model Finalization)
Implemented:
- Finalized device creation ownership model:
- signed-in non-admin users can create only their own devices
- admin users can create for another user when `user_id` is explicitly provided
- API behavior changes in `POST /api/v1/devices` with session auth:
- non-admin request with mismatched `user_id` now returns `403` (`cannot create device for another user`)
- admin request with explicit `user_id` creates device for that target owner
- Added/updated tests for Task 9:
- non-admin cannot create device for another user (`403`)
- admin can create device for specific user id
- kept Task 8 ownership/scope tests passing
- Manual Task 9 validation completed:
- user2 self-created device row persisted with `user_id=2`
- admin created device for user3 persisted with `user_id=3`
- DB check confirmed ownership assignment via:
- `SELECT id,user_id,name FROM devices ORDER BY id;`
- Updated README device management section with explicit admin override behavior.

Architectural decisions:
- Decision: enforce cross-user create attempts at API layer with explicit admin gate.
  Reason: simple and clear ownership policy while preserving single-writer/lightweight store behavior.
- Decision: keep store interfaces unchanged (ownership enforcement in API layer for now).
  Reason: minimizes refactor scope and keeps milestone progress incremental.

Files changed:
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'TestDevicesAPI_(CreateForAnotherUserDeniedForNonAdmin_WhenSessionAuthEnabled|AdminCanCreateForSpecificUser_WhenSessionAuthEnabled|UserSeesOnlyOwnDevices_WhenSessionAuthEnabled|UserCannotFetchAnotherUsersDevice_WhenSessionAuthEnabled|RotateKeyDeniedForNonOwner_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db APP_SPOOL_DIR=./data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18083 go run ./cmd/server`
- `curl -sS -b /tmp/t9_user2_cookie.txt -X POST http://127.0.0.1:18083/api/v1/devices -H "Content-Type: application/json" --data '{"name":"u2-self-device","source_type":"owntracks","api_key":"u2-self-key"}'`
- `curl -sS -X POST http://127.0.0.1:18083/api/v1/devices -H "Content-Type: application/json" -H "Cookie: plexplore_session=<admin-token>" --data '{"user_id":3,"name":"admin-created-for-u3","source_type":"owntracks","api_key":"u3-admin-key"}'`
- `sqlite3 ./data/task3bootstrap.db "SELECT id,user_id,name FROM devices ORDER BY id;"`

Pending:
- Task 10: scope `/api/v1/points/recent` by signed-in user.
- Task 11+: apply same user scoping pattern to points history/export/visits endpoints.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 03:58 UTC - Phase 60 (Task 10: Recent Points Session Scope)
Implemented:
- Scoped `GET /api/v1/points/recent` to authenticated signed-in user context.
- Route registration now applies session middleware when dependencies are present:
- `LoadCurrentUserFromSession`
- `RequireUserSessionAuth`
- Added ownership filtering for recent points:
- resolves current user-owned devices
- returns only points whose `device_id` is in that owned set
- blocks cross-user leakage even when device filter attempts target another user's device
- unauthenticated requests now return `401` for protected recent-points route
- Added Task 10 API tests:
- user sees only own recent points
- cross-user device filter trick returns zero points
- unauthenticated access denied
- Manual Task 10 validation completed:
- ingested points for user2/user3 devices via API keys
- user2 recent query returned only user2 point(s)
- user3 recent query returned only user3 point(s)
- anonymous recent query returned `401` with authentication error
- Updated README recent-points section with session/auth scoping notes.

Architectural decisions:
- Decision: Apply ownership filter at API layer using current user device set.
  Reason: Minimal incremental change compatible with existing store interfaces while maintaining isolation guarantees.

Files changed:
- `internal/api/points.go`
- `internal/api/points_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `go test ./internal/api -run 'TestRecentPointsEndpoint_(UserSeesOnlyOwnPoints_WhenSessionAuthEnabled|DeviceFilterTrickBlocked_WhenSessionAuthEnabled|UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestRecentPointsEndpoint_|TestPointsEndpoint_' -count=1`
- `go test ./... -count=1`
- `APP_SQLITE_PATH=./data/task3bootstrap.db APP_SPOOL_DIR=./data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18084 go run ./cmd/server`
- `curl -sS -X POST http://127.0.0.1:18084/api/v1/owntracks -H "Content-Type: application/json" -H "X-API-Key: u2-key-1" -d '{"_type":"location","lat":41.1111,"lon":-87.1111,"tst":1776902400}'`
- `curl -sS -X POST http://127.0.0.1:18084/api/v1/owntracks -H "Content-Type: application/json" -H "X-API-Key: u3-key-1" -d '{"_type":"location","lat":42.2222,"lon":-88.2222,"tst":1776902460}'`
- `curl -sS -b /tmp/t10_u2_cookie.txt "http://127.0.0.1:18084/api/v1/points/recent?limit=10"`
- `curl -sS -b /tmp/t10_u3_cookie.txt "http://127.0.0.1:18084/api/v1/points/recent?limit=10"`
- `curl -sS -o /tmp/t10_anon_recent.txt -w "%{http_code}" "http://127.0.0.1:18084/api/v1/points/recent?limit=10"`

Pending:
- Task 11: scope point history/map endpoints to current signed-in user.
- Task 12+: scope export and visit endpoints similarly.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:05 UTC - Phase 61 (Task 11: Point History Session Scope)
Implemented:
- Scoped `GET /api/v1/points` to session-authenticated users when auth dependencies are present.
- Route now composes:
- `LoadCurrentUserFromSession`
- `RequireUserSessionAuth`
- Added ownership filtering for point history:
- resolves current user-owned device IDs from `DeviceStore`
- applies device ownership gate for explicit `device_id` filter (cross-user filter returns empty result)
- post-filters returned points to current user's device set
- Added Task 11 tests:
- `TestPointsEndpoint_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled`
- `TestPointsEndpoint_DeviceFilterTrickBlocked_WhenSessionAuthEnabled`
- `TestPointsEndpoint_UnauthenticatedDenied_WhenSessionAuthEnabled`
- Updated README point history docs to note session requirement and user scoping.

Architectural decisions:
- Decision: Keep per-user scoping at API layer using existing store interfaces.
  Reason: Minimal, low-risk change consistent with current architecture and avoids broad store refactor.

Files changed:
- `internal/api/points.go`
- `internal/api/points_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/points.go internal/api/points_test.go`
- `go test ./internal/api -run 'TestPointsEndpoint_(DefaultQuery|RangeFiltering|DeviceFiltering|InvalidQueryParams|UserSeesOnlyOwnPoints_WhenSessionAuthEnabled|DeviceFilterTrickBlocked_WhenSessionAuthEnabled|UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 12: scope export endpoints to signed-in user.
- Task 13: scope visits endpoints/generation to signed-in user.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:10 UTC - Phase 62 (Task 12: Export Endpoint Session Scope)
Implemented:
- Scoped export routes when auth dependencies are present:
- `GET /api/v1/exports/geojson`
- `GET /api/v1/exports/gpx`
- Routes now require signed-in user session via:
- `LoadCurrentUserFromSession`
- `RequireUserSessionAuth`
- Added user ownership filtering for export output:
- explicit cross-user `device_id` filter returns empty export payload
- unfiltered exports are post-filtered to current user's devices only
- Added focused export isolation tests:
- `TestGeoJSONExport_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled`
- `TestGeoJSONExport_UnauthenticatedDenied_WhenSessionAuthEnabled`
- `TestGPXExport_DeviceFilterTrickBlocked_WhenSessionAuthEnabled`
- Updated README export sections with session/auth scoping notes.

Architectural decisions:
- Decision: Apply export data isolation in route handlers using current-user device ownership map.
  Reason: Keeps implementation lightweight, deterministic, and consistent with existing point/recent scoping approach.

Files changed:
- `internal/api/exports.go`
- `internal/api/exports_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/exports.go internal/api/exports_test.go`
- `go test ./internal/api -run 'Test(GeoJSONExport_|GPXExport_)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 13: scope visits endpoints/generation to signed-in user.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:16 UTC - Phase 63 (Task 13: Visits Endpoint Session Scope)
Implemented:
- Updated visit route registration to use dependency-based auth routing with session protection when available.
- Scoped both visit endpoints to signed-in user session:
- `POST /api/v1/visits/generate`
- `GET /api/v1/visits`
- Visit ownership behavior:
- generation is allowed only for devices owned by current signed-in user
- cross-user `device_id` in generate returns `404` (`device not found`)
- list results are filtered to current user's devices
- explicit cross-user `device_id` in list returns empty result
- Added Task 13 tests:
- `TestListVisitsEndpoint_UserSeesOnlyOwnVisits_WhenSessionAuthEnabled`
- `TestGenerateVisitsEndpoint_CrossUserDeviceDenied_WhenSessionAuthEnabled`
- `TestVisitsEndpoints_UnauthenticatedDenied_WhenSessionAuthEnabled`
- Updated README visit generation/list docs to note session requirement and user scoping.

Architectural decisions:
- Decision: Enforce visit authorization through device ownership checks before calling visit generation/list store operations.
  Reason: Keeps access control centralized at API layer and avoids cross-user data leakage without store schema changes.

Files changed:
- `internal/api/health.go`
- `internal/api/visits.go`
- `internal/api/visits_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/visits.go internal/api/visits_test.go internal/api/health.go`
- `go test ./internal/api -run 'Test(GenerateVisitsEndpoint_|ListVisitsEndpoint|ListVisitsEndpoint_InvalidParams|ListVisitsEndpoint_WithVisitLabelResolver|ListVisitsEndpoint_UserSeesOnlyOwnVisits_WhenSessionAuthEnabled|GenerateVisitsEndpoint_CrossUserDeviceDenied_WhenSessionAuthEnabled|VisitsEndpoints_UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 14: protect UI pages with signed-in indicator and logout control.
- Task 15: verify device API key ingest auth correctness in multi-user mode.
- Task 16: add broader authorization/isolation integration coverage.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:28 UTC - Phase 64 (Task 14: Session-Aware UI Header/Logout)
Implemented:
- Updated lightweight UI pages (`/`, `/ui/status`, `/ui/map`) to include:
- signed-in user email indicator in top bar
- logout form/button posting to `POST /logout`
- Added a small render helper to inject current user email from session context into static UI HTML.
- Kept existing dark mode toggle and map/status functionality unchanged.
- Added/updated UI tests to validate:
- top bar includes session user indicator
- logout control exists
- valid session rendering includes current user email
- Updated README minimal UI section with session-aware header/logout note.

Architectural decisions:
- Decision: Keep UI pages as static HTML strings with lightweight placeholder substitution for current user email.
  Reason: Preserves no-build/no-framework constraints and keeps runtime overhead minimal on Raspberry Pi.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|UIRoutesRequireSession_WhenSessionDepsProvided|UIRoutesAllowSession_WhenValidSessionCookiePresent|StatusPage_DoesNotMatchTypoPath)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 15: verify device API key ingest auth remains correct in multi-user mode.
- Task 16: add full authorization/isolation integration tests across app.
- Task 17+: optional admin UI and hardening tasks.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:41 UTC - Phase 65 (Task 15: Device API Key Ingest Ownership Hardening)
Implemented:
- Hardened SQLite ingest commit path to preserve authenticated owner context:
- `InsertSpoolBatch` now honors `record.Point.UserID` when present (fallback to default user only when absent/invalid).
- `ensureDevice` now resolves device by `(user_id, name)` first, then creates fallback device with user-scoped key (`auto:<user_id>:<device>`), preventing cross-user collisions.
- Added integration coverage for multi-user ingest key isolation:
- valid key ingest for two users with same device name persists rows under the correct owning `user_id` and `device_id`
- no fallback auto-devices are created when managed devices already exist
- invalid API key request returns `401` and persists no rows
- Updated store tests to align with user-scoped fallback key format.
- Updated README ingest section with explicit multi-user ownership behavior note.

Architectural decisions:
- Decision: Resolve ingest device ownership by `(user_id, device_name)` and use user-scoped fallback key format.
  Reason: Ensures API-key-authenticated ingest persists to the correct owner/device in shared multi-user instances, even when device names overlap.

Files changed:
- `internal/store/sqlite_store.go`
- `internal/store/sqlite_store_test.go`
- `internal/tasks/ingest_pipeline_integration_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/sqlite_store.go internal/tasks/ingest_pipeline_integration_test.go internal/store/sqlite_store_test.go`
- `go test ./internal/tasks -run 'TestIntegration_(DeviceAPIKeyIngestPersistsUnderCorrectOwnerAndDevice|InvalidDeviceAPIKeyRejected_NoDataPersisted)' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_InsertSpoolBatch_(Success|PartialDuplicates|MultipleDevices)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 16: add full authorization/isolation integration coverage across user workflows.
- Task 17: add lightweight admin user management UI page.
- Task 18: final hardening/documentation pass.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:51 UTC - Phase 66 (Task 16: Authorization Isolation Integration Coverage)
Implemented:
- Added full integration test coverage for multi-user authorization boundaries:
- admin login + admin user creation via `POST /api/v1/users`
- separate user sessions and per-user device creation
- device API key ingest for each user
- non-owner rotate key denial (`403`)
- per-user isolation on `/api/v1/devices`, `/api/v1/points`, and `/api/v1/exports/geojson`
- DB-level ownership verification (`raw_points.user_id` matches owning device/user mapping)
- Added dedicated integration test:
- `TestIntegration_MultiUserAuthorizationIsolation`
- Hardened API-layer user scoping for points/exports against same-name device collisions across users:
- store point projections now include persisted `user_id`
- points/recent and export handlers now enforce both:
- allowed device membership
- matching persisted point owner `user_id == current session user id`
- Updated README with explicit note that point/export scoping is enforced by persisted ownership IDs even when device names overlap.

Architectural decisions:
- Decision: Use persisted row ownership (`raw_points.user_id`) in addition to device-name allowlists for user-scoped point/export filtering.
  Reason: Device names are not globally unique in multi-user deployments; owner-ID checks prevent same-name cross-user leakage while keeping current APIs lightweight.

Files changed:
- `internal/tasks/multi_user_auth_integration_test.go` (new)
- `internal/store/points.go`
- `internal/api/points.go`
- `internal/api/exports.go`
- `internal/api/points_test.go`
- `internal/api/exports_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/tasks/multi_user_auth_integration_test.go internal/store/points.go internal/api/points.go internal/api/exports.go internal/api/points_test.go internal/api/exports_test.go`
- `go test ./internal/tasks -run 'TestIntegration_(MultiUserAuthorizationIsolation|DeviceAPIKeyIngestPersistsUnderCorrectOwnerAndDevice|InvalidDeviceAPIKeyRejected_NoDataPersisted)' -count=1`
- `go test ./internal/api -run 'Test(PointsEndpoint_|RecentPointsEndpoint_|GeoJSONExport_|GPXExport_)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 17: add lightweight admin user management page.
- Task 18: final hardening pass (cookie/session defaults, CSRF review, docs/manual checklist).

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 04:54 UTC - Phase 67 (Task 17: Admin User Management UI)
Implemented:
- Added lightweight admin-only user management page:
- route: `GET /ui/admin/users`
- page lists users through existing `GET /api/v1/users`
- page creates users through existing `POST /api/v1/users`
- added simple form (email/password/is_admin), status messaging, and table rendering
- Added route protection for admin UI page:
- session required
- admin role required (`403` for non-admin)
- Extended status/map top bars to include optional admin navigation link when current user is admin.
- Kept implementation lightweight (server-rendered HTML + plain JS, no framework/build step).
- Added UI tests:
- admin session can load `/ui/admin/users`
- non-admin session is denied (`403`)
- Updated README with admin users page usage notes.

Architectural decisions:
- Decision: Reuse existing admin JSON APIs from a minimal server-rendered admin page rather than adding a separate backend path.
  Reason: Keeps implementation small, testable, and consistent with current low-overhead UI architecture.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|UIRoutesRequireSession_WhenSessionDepsProvided|UIRoutesAllowSession_WhenValidSessionCookiePresent|AdminUsersPageServedForAdminSession|AdminUsersPageDeniedForNonAdminSession)' -count=1`
- `go test ./... -count=1`

Pending:
- Task 18: final hardening pass (session/cookie defaults, CSRF for form POSTs, sensitive-response review, final docs checklist).

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 05:03 UTC - Phase 68 (Task 18: Final Hardening and Docs)
Implemented:
- Added lightweight CSRF protection primitives:
- `plexplore_csrf` cookie generation and token helpers (`internal/api/csrf.go`)
- request token validation via hidden form field (`csrf_token`) or `X-CSRF-Token` header
- Enforced CSRF validation on form/session-sensitive POST routes:
- `POST /login`
- `POST /logout`
- `POST /api/v1/users` (admin create-user)
- Updated UI/login pages to include CSRF tokens:
- login page now renders hidden `csrf_token` field
- status/map/admin pages logout forms now include hidden `csrf_token`
- admin users page create-user fetch now sends `X-CSRF-Token` header
- Updated auth integration helper flows to acquire CSRF token from `/login` before posting credentials/admin actions.
- Added/updated tests:
- login page includes CSRF field
- login success/invalid/logout flows include CSRF token handling
- missing CSRF on login is rejected (`403`)
- admin create user without CSRF is rejected (`403`)
- UI tests assert CSRF token presence in logout forms/admin script header usage
- Ran full regression suite (`go test ./...`) successfully.
- Updated README hardening notes with session cookie and CSRF behavior.

Architectural decisions:
- Decision: Use synchronized CSRF cookie + request token (hidden field/header) for form/lightweight JS routes.
  Reason: Adds practical CSRF mitigation without heavy dependencies and fits server-rendered/plain-JS architecture on Raspberry Pi.

Files changed:
- `internal/api/csrf.go` (new)
- `internal/api/login.go`
- `internal/api/users.go`
- `internal/api/ui.go`
- `internal/api/login_test.go`
- `internal/api/users_test.go`
- `internal/api/ui_test.go`
- `internal/tasks/multi_user_auth_integration_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/csrf.go internal/api/login.go internal/api/users.go internal/api/ui.go internal/api/login_test.go internal/api/users_test.go internal/api/ui_test.go internal/tasks/multi_user_auth_integration_test.go`
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LogoutClearsSession|LoginRejectsMissingCSRFToken|Users_|StatusPageServedAtRoot|MapPageServedAtUIMap|AdminUsersPageServedForAdminSession|AdminUsersPageDeniedForNonAdminSession)' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_MultiUserAuthorizationIsolation' -count=1`
- `go test ./... -count=1`

Pending:
- Multi-user auth milestone tasks are complete (1-18).
- Next work should focus on deployment/manual validation on Raspberry Pi and post-hardening operational checks.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 05:26 UTC - Phase 69 (Login UX: Inline Browser Error Rendering)
Implemented:
- Fixed browser login failure flow so invalid credentials no longer render raw JSON.
- `POST /login` now distinguishes browser-form requests from JSON/API-style requests:
- browser form failures re-render the login HTML with inline error state
- JSON/API-style failures continue to return JSON errors
- Added inline login error UI behavior:
- clear red error message (`Invalid email or password`) rendered near form
- semantic accessibility marker (`role="alert"`)
- preserved entered email value on failure
- password is not preserved
- Kept successful login behavior unchanged (`303` redirect with session cookie).

Architectural decisions:
- Decision: Add lightweight request-mode detection (`Content-Type`/`Accept`) for login failures.
  Reason: Preserve API-style JSON error behavior while improving browser UX without adding dependencies or separate routes.

Files changed:
- `internal/api/login.go`
- `internal/api/login_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/login.go internal/api/login_test.go`
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LoginInvalidCredentials_JSONStillReturnsJSON|LogoutClearsSession|LoginRejectsMissingCSRFToken)' -count=1`
- `go test ./... -count=1`

Pending:
- Manual browser verification on deployment target to confirm inline login UX under real reverse-proxy/browser conditions.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 05:38 UTC - Phase 70 (Post-Login Redirect Default To Map)
Implemented:
- Changed successful browser login default redirect target from status/root to map page:
- now defaults to `GET /ui/map`
- Added lightweight safe `next` redirect support:
- unauthenticated HTML route middleware now redirects to `/login?next=<original_path>`
- successful login uses `next` when present and safe
- unsafe/looping targets (for example external URLs or `/login`) are ignored and fall back to `/ui/map`
- Kept failed-login inline HTML behavior and JSON/API invalid-login behavior unchanged.
- Updated tests:
- success login default redirect now asserts `/ui/map`
- added test for `next` precedence (`/ui/status`)
- updated HTML auth redirect tests to assert `next` query preservation.

Architectural decisions:
- Decision: Support only same-origin absolute-path `next` values for post-login redirects.
  Reason: Avoid open redirects and redirect loops while preserving protected-page return flow.

Files changed:
- `internal/api/login.go`
- `internal/api/login_test.go`
- `internal/api/session_auth.go`
- `internal/api/session_auth_test.go`
- `internal/api/ui_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/login.go internal/api/login_test.go internal/api/session_auth.go internal/api/session_auth_test.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LoginInvalidCredentials_JSONStillReturnsJSON|LoginSuccess_WithNextParamRedirectsToRequestedPage|LogoutClearsSession|LoginRejectsMissingCSRFToken|RequireUserSessionAuthHTML_RedirectWhenAnonymous|UIRoutesRequireSession_WhenSessionDepsProvided)' -count=1`
- `go test ./... -count=1`

Pending:
- Manual browser validation on deployment target for full login -> map and protected-route return flow.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 05:43 UTC - Phase 71 (Status/Map Cross-Navigation Buttons)
Implemented:
- Added top-navigation cross-links for easier UI switching:
- status page now renders a `Map` nav button/link (`id="status_to_map_link"`, `href="/ui/map"`)
- map page now renders a `Status` nav button/link (`id="map_to_status_link"`, `href="/ui/status"`)
- Kept styling/placement consistent with existing `Admin Users` nav link (`nav-link` in `top-actions`).
- Preserved existing admin navigation behavior; added coverage that admin link still renders for admin session.
- No unrelated auth/ingest/page behavior was changed.

Architectural decisions:
- Decision: Reuse existing top-bar `nav-link` styling and static HTML links in status/map templates.
  Reason: Minimal, lightweight change aligned with plain HTML/CSS/vanilla JS UI approach.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|StatusPage_AdminLinkStillRendersForAdminSession|AdminUsersPageServedForAdminSession|AdminUsersPageDeniedForNonAdminSession)' -count=1`
- `go test ./... -count=1`

Pending:
- Manual browser check in deployed environment to confirm navigation ergonomics and keyboard/tab flow.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-23 05:47 UTC - Phase 72 (Users Page Rename + Dark Mode)
Implemented:
- Renamed visible Users-management UI labels from "Admin Users" to "Users":
- page title changed to `Plexplore Users`
- page heading changed to `Users`
- top-nav admin link label on status/map pages changed from `Admin Users` to `Users`
- Kept routes unchanged (`GET /ui/admin/users`) to avoid breaking existing navigation/API integrations.
- Added shared dark mode behavior to the Users page to match status/map:
- added theme toggle button in top bar (`id="theme_toggle"`)
- added `localStorage` preference persistence and system preference fallback (`prefers-color-scheme`)
- added light/dark CSS variable sets covering background, text, cards, table/form controls, links/buttons, and status text colors.
- Kept existing Users page functionality/layout intact; only label/theme/navigation polish was applied.

Architectural decisions:
- Decision: Reuse existing page-local theme toggle pattern used by status/map pages.
  Reason: Keeps implementation lightweight and consistent without introducing a separate theming system.

Files changed:
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|AdminUsersPageServedForAdminSession|StatusPage_AdminLinkStillRendersForAdminSession|MapPage_AdminLinkLabelIsUsersForAdminSession|AdminUsersPageDeniedForNonAdminSession)' -count=1`
- `go test ./... -count=1`

Pending:
- Manual browser validation in deployed environment for Users page dark mode and nav label readability.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 02:54 UTC - Phase 73 (Cookie/Proxy TLS Hardening)
Implemented:
- Added explicit cookie/proxy security config knobs in runtime config:
- `APP_COOKIE_SECURE_MODE` (`auto|always|never`)
- `APP_TRUST_PROXY_HEADERS` (`true|false`)
- `APP_EXPECT_TLS_TERMINATION` (`true|false`)
- Added lightweight request-aware cookie security policy used by both session and CSRF cookie issuance.
- Updated cookie setting behavior:
- login session cookie now sets `Secure` according to policy
- logout cookie-clear response now preserves policy-driven `Secure`
- CSRF cookie issuance now sets `Secure` according to policy
- trusted `X-Forwarded-Proto=https` affects cookie `Secure` only when proxy trust is explicitly enabled.
- Added startup deployment warnings for risky combinations:
- public bind with non-`always` cookie mode
- TLS termination expected but proxy headers not trusted
- explicit `APP_COOKIE_SECURE_MODE=never`
- Updated deployment/config docs and samples:
- README security/deployment guidance for local HTTP dev vs direct HTTPS vs reverse-proxy TLS
- `compose.yaml` includes explicit cookie/proxy env knobs
- `deploy/systemd/plexplore.env.sample` includes cookie/proxy env knobs and usage notes
- Added/updated tests for cookie security behavior:
- local HTTP default path keeps non-secure cookies for dev flow
- `always` mode enforces `Secure` session cookie
- trusted proxy header path sets `Secure` CSRF cookie
- untrusted proxy headers do not affect `Secure` cookie behavior
- direct TLS and mode semantics covered by policy unit tests
- Verified full test suite passes after changes.

Architectural decisions:
- Decision: Centralize cookie `Secure` decision in a small policy object (`CookieSecurityPolicy`) and inject it via API dependencies.
  Reason: Keep auth handlers thin and consistent while supporting local HTTP, direct HTTPS, and explicitly trusted reverse-proxy TLS deployments without redesigning auth/session architecture.

Files changed:
- `internal/config/config.go`
- `internal/api/cookie_security.go`
- `internal/api/cookie_security_test.go`
- `internal/api/csrf.go`
- `internal/api/health.go`
- `internal/api/login.go`
- `internal/api/login_test.go`
- `internal/api/ui.go`
- `cmd/server/main.go`
- `README.md`
- `compose.yaml`
- `deploy/systemd/plexplore.env.sample`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/config/config.go internal/api/health.go internal/api/cookie_security.go internal/api/csrf.go internal/api/login.go internal/api/ui.go internal/api/login_test.go internal/api/cookie_security_test.go cmd/server/main.go`
- `go test ./internal/api -run 'Test(Login|CookieSecurityPolicy|LoadCurrentUserFromSession|RequireUserSessionAuth)' -count=1`
- `go test ./cmd/server -count=1`
- `go test ./...`

Pending:
- Manual end-to-end validation behind an actual HTTPS reverse proxy (for example Caddy/Nginx) to confirm browser cookie behavior in deployed topology.
- Decide and set production baseline env values for cookie mode/trusted proxy in each deployment manifest.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 03:21 UTC - Phase 74 (Device API Keys Hashed At Rest)
Implemented:
- Replaced plaintext-at-rest device credential usage with hash-at-rest flow:
- added migration `0007_device_api_key_hash.sql` with `devices.api_key_hash` and `devices.api_key_preview` plus lookup index
- device auth now hashes presented API key and looks up by `api_key_hash`
- create/rotate now persist `api_key_hash` + `api_key_preview`; `devices.api_key` is stored as a non-secret sentinel, not plaintext credential
- create/rotate API responses still return plaintext key exactly once to caller
- list/read responses continue to expose only `api_key_preview`
- Added safe transition/backfill for existing databases:
- on store open, legacy plaintext `devices.api_key` rows are backfilled to hash + preview
- legacy plaintext values are replaced with deterministic non-secret sentinel values (`hashonly:<id>`)
- Updated ingest-side auto-device creation to avoid using plaintext API keys.
- Added/updated tests:
- store tests now verify no plaintext key is persisted and rotation invalidates old key while keeping hash lookup functional
- added backfill test for legacy plaintext rows
- added integration coverage that ingest still authenticates with presented key while DB stores only hash/sentinel
- Updated README with hashed-at-rest and one-time display guidance.

Architectural decisions:
- Decision: Use deterministic SHA-256 hashing for device API key verification (`api_key_hash`) with a dedicated indexed lookup column.
  Reason: API keys are high-entropy secrets and require deterministic server-side lookup for lightweight auth on Raspberry Pi without introducing heavyweight KDF-based scan patterns or extra services.
- Decision: Keep legacy `devices.api_key` column but replace values with non-secret sentinels immediately after backfill/write.
  Reason: Avoid risky table-rebuild migration complexity while ensuring plaintext credentials are not retained at rest.

Files changed:
- `migrations/0007_device_api_key_hash.sql`
- `internal/store/device_keys.go`
- `internal/store/devices.go`
- `internal/store/sqlite_store.go`
- `internal/store/devices_test.go`
- `internal/store/sqlite_store_test.go`
- `internal/api/devices.go`
- `internal/tasks/multi_user_auth_integration_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/device_keys.go internal/store/devices.go internal/store/sqlite_store.go internal/store/devices_test.go internal/store/sqlite_store_test.go internal/api/devices.go internal/tasks/multi_user_auth_integration_test.go`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateAndLookupDeviceByAPIKey|GetDeviceByID_AndRotateAPIKey|BackfillPlaintextDeviceKeyToHash|GetDeviceByAPIKey_NotFound|ListDevices)' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_|TestRequireDeviceAPIKeyAuth' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_(MultiUserAuthorizationIsolation|DeviceAPIKeyStoredHashedAtRest)' -count=1`
- `go test ./...`

Pending:
- Optional follow-up migration can remove/deprecate `devices.api_key` column entirely via table rebuild if schema strictness is desired.
- Validate production rollout against a copy of real DB backup to confirm backfill latency and credentials continuity.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 03:27 UTC - Phase 75 (Status Endpoint Exposure Hardening)
Implemented:
- Hardened operational status exposure with a split public/private model:
- `GET /health` remains public and minimal
- `GET /status` is now public-safe and minimal (`service_health`, `service`)
- `GET /api/v1/status` remains detailed but now requires authenticated user session when session dependencies are enabled
- Sensitive operational fields are no longer exposed via public `/status`, including:
- filesystem/config paths (`spool_dir_path`, `sqlite_db_path`)
- spool internals (`spool_segment_count`)
- checkpoint details (`checkpoint_seq`)
- flush internals and errors (`last_flush_*`, nested `last_flush`)
- Reused existing session middleware (`LoadCurrentUserFromSession` + `RequireUserSessionAuth`) for low-overhead protection.
- Kept status UI behavior intact; UI already runs under authenticated session and continues using `/api/v1/status`.
- Added/updated tests for:
- authenticated access success on `/api/v1/status`
- unauthenticated denial on `/api/v1/status` when session auth is configured
- public `/status` alias excludes sensitive fields
- public `/health` remains accessible
- Updated README operational status documentation and examples to reflect public-safe vs authenticated endpoints.

Architectural decisions:
- Decision: Use split status exposure instead of fully blocking all status routes.
  Reason: Preserves lightweight unauthenticated monitoring via `/status` while preventing sensitive runtime metadata disclosure and keeping existing authenticated UI operations unchanged.

Files changed:
- `internal/api/status.go`
- `internal/api/status_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/status.go internal/api/status_test.go`
- `go test ./internal/api -run 'TestStatusEndpoint_|TestHealthEndpoint_RemainsPublic|TestStatusPageServedAtRoot|TestUIRoutesRequireSession_WhenSessionDepsProvided|TestUIRoutesAllowSession_WhenValidSessionCookiePresent' -count=1`
- `go test ./...`

Pending:
- If stricter posture is desired, consider making detailed `/api/v1/status` admin-only rather than authenticated-user scope.
- Review any external monitoring that previously scraped `/api/v1/status` without session and switch it to `/status` or authenticated access.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 03:38 UTC - Phase 76 (Login/Admin Route Rate Limiting)
Implemented:
- Added lightweight in-process fixed-window rate limiting (memory map + mutex, no external services).
- Added route-scoped limiter model with separate policies:
- strict login limiter for `POST /login`
- moderate admin-sensitive limiter for:
- `GET /api/v1/users`
- `POST /api/v1/users`
- `POST /api/v1/devices`
- `POST /api/v1/devices/{id}/rotate-key`
- Added safe client-IP keying with explicit proxy trust behavior:
- default uses direct `RemoteAddr`
- `X-Forwarded-For` is used only when `APP_TRUST_PROXY_HEADERS=true`
- Added `429` responses with `Retry-After` header when limited.
- Preserved existing login/auth/session behavior for normal traffic volume.
- Added deterministic tests for:
- repeated login attempts hitting limiter (429)
- window reset allowing later attempts
- admin-sensitive route limiting
- non-sensitive routes (e.g. `/health`) unaffected
- proxy-trust keying behavior
- successful login still works under expected request volume
- Added config knobs and wiring through server startup and API dependencies.
- Updated README and deployment env samples with rate limit documentation.

Architectural decisions:
- Decision: Use fixed-window in-process rate limiting keyed by client IP and route scope.
  Reason: Low-RAM, low-CPU, no external infrastructure, and simple operational model aligned with single-process Raspberry Pi deployment.
- Decision: Reuse existing `APP_TRUST_PROXY_HEADERS` for limiter IP extraction behavior.
  Reason: Avoid introducing duplicate proxy-trust configuration and keep trust semantics explicit and conservative.

Files changed:
- `internal/api/rate_limit.go`
- `internal/api/rate_limit_test.go`
- `internal/api/health.go`
- `internal/api/login.go`
- `internal/api/login_test.go`
- `internal/api/users.go`
- `internal/api/users_test.go`
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `internal/config/config.go`
- `cmd/server/main.go`
- `README.md`
- `compose.yaml`
- `deploy/systemd/plexplore.env.sample`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/rate_limit.go internal/api/rate_limit_test.go internal/api/health.go internal/api/login.go internal/api/users.go internal/api/devices.go internal/api/login_test.go internal/api/users_test.go internal/api/devices_test.go internal/config/config.go cmd/server/main.go`
- `go test ./internal/api -run 'Test(LoginRateLimit_|FixedWindowLimiter_|RateLimitKeyForRequest_|RateLimit_NonSensitiveHealthRouteUnaffected|Users_AdminRoutesRateLimited|DevicesAPI_AdminSensitiveWritesRateLimited|LoginPageServed|LoginSuccessSetsSessionCookie|Users_AdminCanCreateUser)' -count=1`
- `go test ./cmd/server -count=1`
- `go test ./...`

Pending:
- Tune route limit values for real deployment traffic patterns (especially admin automation/scripts).
- Consider optional email+IP composite keying for login if abuse patterns require narrower targeting.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 03:54 UTC - Phase 77 (Production-Oriented Cookie/Deployment Defaults)
Implemented:
- Added explicit deployment profile knob `APP_DEPLOYMENT_MODE` (`development|production`) in config.
- Refined security-oriented defaults:
- default bind address changed to `127.0.0.1:8080` (safer local default)
- cookie default now depends on deployment mode:
- development default: `APP_COOKIE_SECURE_MODE=auto`
- production default: `APP_COOKIE_SECURE_MODE=always`
- TLS-termination expectation default now depends on deployment mode:
- development default: `APP_EXPECT_TLS_TERMINATION=false`
- production default: `APP_EXPECT_TLS_TERMINATION=true`
- Added extra startup warning for inconsistent production cookie posture when `APP_DEPLOYMENT_MODE=production` but settings are not TLS-backed by default.
- Updated production-oriented sample deployment defaults:
- `compose.yaml`: `APP_DEPLOYMENT_MODE=production`, loopback-only host publish (`127.0.0.1:8080:8080`), `APP_COOKIE_SECURE_MODE=always`, `APP_EXPECT_TLS_TERMINATION=true`
- `deploy/systemd/plexplore.env.sample`: production mode, loopback bind, secure cookie defaults, TLS-termination expectation enabled
- Updated README with clearly separated local-development vs production HTTPS guidance and revised defaults.
- Added config tests validating deployment-mode-derived defaults and explicit development override behavior.
- Re-ran targeted + full test suite successfully.

Architectural decisions:
- Decision: Introduce deployment-mode-driven defaults instead of hardcoding one cookie policy for all environments.
  Reason: Preserve local HTTP development ergonomics while making production defaults explicitly TLS-backed and safer by default.

Files changed:
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/server/main.go`
- `compose.yaml`
- `deploy/systemd/plexplore.env.sample`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/config/config.go internal/config/config_test.go cmd/server/main.go`
- `go test ./internal/config -count=1`
- `go test ./internal/api -run 'Test(LoginSuccess_SetsSecureSessionCookie_WhenAlwaysMode|TestLoginSuccessSetsSessionCookie|TestLoginPageCSRFCookie_UsesTrustedForwardedProtoWhenEnabled|TestLoginPageCSRFCookie_IgnoresForwardedProtoWhenUntrusted)' -count=1`
- `go test ./...`

Pending:
- Validate final production deployment topology docs against actual reverse proxy config (Caddy/Nginx) to ensure operator examples remain exact.
- Consider adding explicit `APP_TRUST_PROXY_HEADERS=true` sample profile for operators who choose `APP_COOKIE_SECURE_MODE=auto` behind trusted TLS proxy.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 04:10 UTC - Phase 78 (Server-Generated Device Keys Only)
Implemented:
- Hardened device credential issuance so client-provided API keys are no longer effective:
- `POST /api/v1/devices` now always generates a server-side API key and ignores supplied `api_key` input
- `POST /api/v1/devices/{id}/rotate-key` now always generates a fresh server-side API key and ignores supplied `api_key` input
- Increased generated device key entropy from 16 random bytes to 32 random bytes (hex-encoded 64-char bearer key).
- Kept one-time key display contract:
- create/rotate responses return plaintext key exactly once
- list/read responses remain masked preview only
- Preserved hash-at-rest authentication model:
- ingest auth continues to verify presented key via `api_key_hash`
- DB continues to persist hash + preview and non-secret sentinel in legacy `api_key` column
- Updated integration and API tests to assert:
- user-supplied create/rotate keys are ignored
- generated key is returned once and required for ingest
- old key is invalid after rotation
- plaintext is not stored at rest
- Updated README and operational notes to document server-generated key workflow and revised curl examples.

Architectural decisions:
- Decision: Ignore (rather than reject) incoming `api_key` fields for create/rotate while always generating server keys.
  Reason: Preserves backward compatibility for existing clients that still send `api_key` fields, without allowing weak/supplied credentials to become effective.

Files changed:
- `internal/api/devices.go`
- `internal/api/devices_test.go`
- `internal/tasks/multi_user_auth_integration_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/devices.go internal/api/devices_test.go internal/tasks/multi_user_auth_integration_test.go`
- `go test ./internal/api -run 'TestDevicesAPI_|TestRequireDeviceAPIKeyAuth|TestDevicesAPI_AdminSensitiveWritesRateLimited' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_(MultiUserAuthorizationIsolation|DeviceAPIKeyStoredHashedAtRest)' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateAndLookupDeviceByAPIKey|GetDeviceByID_AndRotateAPIKey|BackfillPlaintextDeviceKeyToHash)' -count=1`
- `go test ./...`

Pending:
- Optional future tightening: explicitly reject `api_key` fields with validation error once all known clients are migrated away from sending them.
- Optional migration cleanup: remove legacy sentinel `devices.api_key` column via table rebuild when operationally convenient.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.

### 2026-04-28 19:45 CDT - Phase 89 (Visit Scheduler Telemetry in Status API/UI)
Implemented:
- Added lightweight visit scheduler telemetry snapshot support in runtime scheduler:
- enabled/running state
- last run start/finish timestamps
- last successful run timestamp
- last error message
- last run counters (processed/skipped/updated/created/errors)
- compact watermark summary (devices with watermark, min/max seq, last processed timestamp, lag seconds)
- Extended authenticated `GET /api/v1/status` response with `visit_scheduler` payload.
- Kept public `GET /status` minimal and unchanged (no scheduler internals).
- Added status UI scheduler card (`Visit Scheduler`) to `/` and `/ui/status` pages.
- Updated status UI polling JS to render scheduler state/metadata with graceful fallback when unavailable.
- Added tests for:
- scheduler status default/success/error behavior
- `/api/v1/status` scheduler telemetry presence
- public `/status` excludes scheduler telemetry
- status page renders scheduler section
- Updated README Operational Status section with example `visit_scheduler` response fields.

Architectural decisions:
- Decision: Keep scheduler telemetry in an in-memory snapshot updated by scheduler runs, then expose via authenticated status endpoint.
  Reason: avoids expensive per-request queries and keeps overhead low for Raspberry Pi-class deployments.
- Decision: Use a small adapter in `cmd/server` to map `tasks.VisitSchedulerStatus` to API snapshot type.
  Reason: prevents `api` <-> `tasks` import cycle while keeping interfaces explicit.

Files changed:
- `internal/tasks/visit_scheduler.go`
- `internal/tasks/visit_scheduler_test.go`
- `internal/api/health.go`
- `internal/api/status.go`
- `internal/api/status_test.go`
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `internal/api/assets/app/status.js`
- `cmd/server/main.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/tasks/visit_scheduler.go internal/tasks/visit_scheduler_test.go internal/api/health.go internal/api/status.go internal/api/status_test.go internal/api/ui.go internal/api/ui_test.go cmd/server/main.go`
- `go test ./internal/tasks -run TestVisit -count=1`
- `go test ./internal/api -run 'TestStatusEndpoint|TestStatusPageServedAtRoot' -count=1`
- `go test ./...`
- `go run ./cmd/server` (startup verified in local session)
- `curl -sS http://127.0.0.1:8080/status` (not reachable from separate command sandbox in this environment)
- `curl -sS http://127.0.0.1:8080/api/v1/status` (not reachable from separate command sandbox in this environment)

Pending:
- Add checkpoint-retry pressure fields (checkpoint failure count/last failure timestamp) to authenticated status payload.
- Add SQLite-backed integration test for scheduler telemetry and watermark summary values.
- Add authenticated browser smoke test for status scheduler card refresh behavior.

Known issues:
- In this shell environment, `go run` and `curl` commands ran in isolated execution contexts, so live `curl` status checks could not reach the server process started in a separate session.

### 2026-04-28 20:18 CDT - Phase 91 (Dynamic CSP img-src by Map Tile Mode)
Implemented:
- Tightened HTML CSP `img-src` generation to be dynamic and tile-mode-aware instead of broad `http:`/`https:` wildcards.
- Added dynamic CSP builder in `internal/api/security_headers.go`:
  - always includes `'self'` and `data:`
  - `none`/`blank`/`local`/`self-hosted` modes: no external origins
  - `osm` mode: explicit OSM origins only (`tile.openstreetmap.org` and `a/b/c` subdomains when template uses `{s}`)
  - `custom` mode: parses configured tile URL template and includes only extracted origin(s)
  - unknown/misconfigured templates fall back to restrictive `'self' data:`
- Updated HTML handlers to pass tile config into CSP generation:
  - login/status/users/devices pages use restrictive local-only mode
  - map page uses configured `MapTileConfig`
- Added/updated tests:
  - new CSP unit tests for none/osm/custom/invalid custom template behavior
  - UI tests asserting restrictive/default CSP, custom origin inclusion, and OSM origin inclusion
  - assertions that wildcard scheme tokens (`http:`/`https:`) are not present in `img-src`
- Updated README security notes documenting tile-mode-driven `img-src` behavior.

Architectural decisions:
- Decision: Build `img-src` from tile mode + parsed template origin, not from raw wildcard schemes.
  Reason: minimizes third-party request surface while preserving map tile functionality per explicit configuration.

Files changed:
- `internal/api/security_headers.go`
- `internal/api/security_headers_test.go`
- `internal/api/ui.go`
- `internal/api/login.go`
- `internal/api/ui_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/security_headers.go internal/api/security_headers_test.go internal/api/ui_test.go internal/api/ui.go internal/api/login.go`
- `go test ./internal/api`
- `go test ./...`

Pending:
- Manual runtime header verification in a single shared process/network context:
  - `curl -I http://127.0.0.1:8080/`
  - map page checks for `none`, `osm`, `custom` tile modes.

Known issues:
- In this shell environment, `go run` and `curl` may execute in isolated contexts, so manual live header checks can require a separate explicit runtime session.

### 2026-04-28 20:02 CDT - Phase 90 (Visit Scheduler Restart/Watermark SQLite Integration)
Implemented:
- Added SQLite-backed integration test proving visit scheduler watermark/progress survives restart and remains incremental:
  - creates temp SQLite DB and applies real migrations
  - creates two users with same device name (`phone`) to verify stable device/user isolation
  - inserts deterministic persisted points via real store path
  - runs scheduler once and verifies visits + watermark persistence
  - recreates runtime/scheduler (restart simulation), reruns without new points, verifies no duplicate visits
  - inserts new points after watermark, reruns, verifies only new visit is added and watermark advances
- The restart scenario confirms persisted `visit_generation_state` is reused correctly across process restart.

Architectural decisions:
- Decision: Integration test uses real SQLite store + real migrations + real `VisitScheduler.RunOnce` and avoids timer-based scheduling.
  Reason: deterministic, low-overhead, and closest to production behavior without background timing flakiness.

Files changed:
- `internal/tasks/visit_scheduler_restart_integration_test.go`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/tasks/visit_scheduler_restart_integration_test.go`
- `go test ./internal/tasks -run TestVisit -count=1`
- `go test ./internal/tasks -run TestScheduler -count=1`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`

Pending:
- Add checkpoint retry pressure fields to `/api/v1/status` (failure count and last checkpoint error timestamp).
- Add authenticated browser smoke flow for `/login` -> `/ui/status` and scheduler section refresh.

Known issues:
- `go test ./internal/tasks -run TestScheduler -count=1` currently reports `[no tests to run]` because scheduler tests are named `TestVisitScheduler*`; command kept for compatibility with existing task checklist.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 15:48 UTC - Phase 81 (Production-Hardening Follow-up Verification)
Implemented:
- Re-validated the three follow-ups against current codebase state:
- CSP hardening remains in effect (`script-src 'self'`, `style-src 'self'`, no `'unsafe-inline'`)
- migration robustness remains in effect (transactional apply+record flow, sqlite `-bail`, partial-state recovery tests)
- map tile privacy behavior remains in effect (default `APP_MAP_TILE_MODE=none`, custom/osm explicit configuration paths)
- Re-ran full and targeted validations, including runtime header checks and migration rerun checks.
- Updated `NEXT_STEPS.md` note to remove outdated statement that CSP still allows `'unsafe-inline'`.

Architectural decisions:
- Decision: No new architecture/code changes were required for these follow-ups after verification.
  Reason: Existing implementation already satisfied requested hardening requirements; only tracking docs required correction.

Files changed:
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `rg -n "unsafe-inline" internal README.md`
- `rg -n "<style>|<script>" internal/api/ui.go internal/api/login.go`
- `rg -n "APP_MAP_TILE_MODE|data-tile-mode|tile.openstreetmap.org" internal/api internal/config README.md Dockerfile compose.yaml deploy/systemd/plexplore.env.sample`
- `go test ./internal/api`
- `go test ./internal/store`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`
- `gofmt -w internal/api/*.go internal/store/*.go internal/config/*.go cmd/server/*.go`
- `go run ./cmd/server`
- `curl -I http://127.0.0.1:8080/`
- `curl -I http://127.0.0.1:8080/login`
- `curl -I http://127.0.0.1:8080/ui/map`
- `go test ./internal/api -run 'TestMapPageServedAtUIMap|TestMapPage_UsesConfiguredExternalTileProvider|TestUIAssets_MapScriptContainsEscapedPopupFields' -count=1`
- `make migrate`
- `make migrate`

Pending:
- Add authenticated browser smoke test coverage for `/login` -> `/ui/map`.
- Consider dynamic CSP `img-src` tightening when `APP_MAP_TILE_MODE=none`.
- Add explicit fixture for partial-apply recovery of `0005_users_auth_fields.sql` with missing migration record.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 08:42 UTC - Phase 80 (CSP Tightening, Migration Robustness, Tile Privacy Config)
Implemented:
- Removed inline UI CSS/JS from login/status/map/users pages and moved behavior/styling to local static assets under `/ui/assets/app/*`.
- Tightened HTML CSP by removing `'unsafe-inline'` from both `script-src` and `style-src`.
- Kept lightweight UI behavior intact (dark mode toggle, status refresh, map filters/track/visits, users page create/list).
- Added map tile provider configuration:
- `APP_MAP_TILE_MODE=none|osm|custom`
- `APP_MAP_TILE_URL_TEMPLATE`
- `APP_MAP_TILE_ATTRIBUTION`
- Default is now `none` (blank/privacy mode) so map works without external tile requests unless explicitly enabled.
- Hardened migrator execution:
- migration SQL + migration record now run in one SQLite transaction (`BEGIN IMMEDIATE ... INSERT schema_migrations ... COMMIT`)
- `sqlite3 -bail` is now used so failures stop immediately and cannot incorrectly record migration success
- added partial-migration recovery for known additive migrations (`0002`, `0005`, `0007`) by validating schema state and recording migration when already effectively applied
- preserves failed non-recoverable migrations as unrecorded.
- Updated Docker/systemd sample env defaults/docs with tile privacy knobs.
- Added/updated tests for:
- CSP without `unsafe-inline`
- externalized UI assets on rendered pages
- escaped map popup fields from local `map.js`
- map tile mode default (`none`) and configured custom tile template
- migrator partial-state recovery and failed-migration non-recording.

Architectural decisions:
- Decision: Serve all UI JS/CSS from self-hosted static assets and enforce strict `'self'` CSP for scripts/styles.
  Reason: Remove inline script/style CSP exception while keeping frontend lightweight and dependency-free.
- Decision: Default map tile mode to `none`, requiring explicit opt-in for external tiles.
  Reason: Prefer privacy-preserving defaults for self-hosted deployments.
- Decision: Use `sqlite3 -bail` plus transactional migration wrapper and targeted schema-aware recovery.
  Reason: Prevent false-positive migration recording and recover safely from prior duplicate-column partial states.

Files changed:
- `internal/api/ui.go`
- `internal/api/login.go`
- `internal/api/security_headers.go`
- `internal/api/ui_assets.go`
- `internal/api/health.go`
- `internal/api/routes_test_helpers_test.go`
- `internal/api/ui_test.go`
- `internal/api/login_test.go`
- `internal/api/assets/app/app.css`
- `internal/api/assets/app/status.css`
- `internal/api/assets/app/map.css`
- `internal/api/assets/app/users.css`
- `internal/api/assets/app/login.css`
- `internal/api/assets/app/common.js`
- `internal/api/assets/app/status.js`
- `internal/api/assets/app/map.js`
- `internal/api/assets/app/users.js`
- `internal/store/migrator.go`
- `internal/store/migrator_test.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/server/main.go`
- `README.md`
- `compose.yaml`
- `deploy/systemd/plexplore.env.sample`
- `Dockerfile`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w cmd/server/main.go internal/api/health.go internal/api/login.go internal/api/routes_test_helpers_test.go internal/api/security_headers.go internal/api/ui.go internal/api/ui_test.go internal/config/config.go internal/config/config_test.go internal/store/migrator.go internal/store/migrator_test.go`
- `go test ./internal/api`
- `go test ./internal/store`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`
- `go run ./cmd/server`
- `curl -I http://127.0.0.1:8080/login`
- `curl -I http://127.0.0.1:8080/`
- `curl -I http://127.0.0.1:8080/ui/map`
- `curl -I http://127.0.0.1:8080/ui/assets/app/map.js`
- `make migrate`
- `make migrate`

Pending:
- Add one authenticated UI smoke test (login + `/ui/map`) that verifies tile-mode metadata and map script execution path together.
- Consider narrowing `img-src` CSP dynamically when tile mode is `none` (currently allows `http(s)` to support optional external/custom tile modes).
- Add a dedicated migration fixture test for real-world `0005_users_auth_fields.sql` partial state with missing index to keep recovery behavior guarded.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 03:20 UTC - Phase 79 (Security Fixes: CSRF Coverage, Map Popup Escaping, Password Policy)
Implemented:
- Added CSRF validation to session-authenticated write endpoints:
- `POST /api/v1/devices`
- `POST /api/v1/devices/{id}/rotate-key`
- `POST /api/v1/visits/generate`
- Kept ingest API-key endpoints unchanged (no CSRF required for `/api/v1/owntracks` and `/api/v1/overland/batches`).
- Escaped map popup stored fields in `/ui/map` script:
- `timestamp_utc` and `device_id` in point marker popup now pass through `escapeHTML(...)`.
- Enforced minimum password length policy:
- Added `MinPasswordLength = 12`
- Added `ErrPasswordTooShort`
- Updated `HashPassword(...)` to reject passwords shorter than 12 characters.
- Updated tests for CSRF-required writes and minimum password policy:
- Added explicit CSRF missing/invalid/valid coverage for device create/rotate and visit generation.
- Updated session-auth integration helpers to include CSRF for device write calls.
- Updated login/user/migrate tests to use >=12-character passwords.
- Updated README examples and password policy note.

Architectural decisions:
- Decision: Reuse existing double-submit CSRF pattern (cookie + `X-CSRF-Token`) for newly protected write endpoints instead of introducing new middleware.
  Reason: Small, focused change that matches existing auth flows and keeps handler behavior consistent.
- Decision: Set minimum password length to 12 in password helper.
  Reason: Improves baseline credential hygiene with minimal implementation overhead.

Files changed:
- `internal/api/devices.go`
- `internal/api/visits.go`
- `internal/api/ui.go`
- `internal/api/password.go`
- `internal/api/password_test.go`
- `internal/api/devices_test.go`
- `internal/api/visits_test.go`
- `internal/api/users_test.go`
- `internal/api/ui_test.go`
- `internal/api/login_test.go`
- `internal/tasks/multi_user_auth_integration_test.go`
- `cmd/migrate/main_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w /mnt/d/code/plexplore/internal/api/devices.go /mnt/d/code/plexplore/internal/api/visits.go /mnt/d/code/plexplore/internal/api/ui.go /mnt/d/code/plexplore/internal/api/password.go /mnt/d/code/plexplore/internal/api/password_test.go /mnt/d/code/plexplore/internal/api/devices_test.go /mnt/d/code/plexplore/internal/api/visits_test.go /mnt/d/code/plexplore/internal/api/users_test.go /mnt/d/code/plexplore/internal/api/login_test.go /mnt/d/code/plexplore/internal/tasks/multi_user_auth_integration_test.go /mnt/d/code/plexplore/cmd/migrate/main_test.go`
- `go test ./internal/api`
- `go test ./...`
- `timeout 6s go run ./cmd/server`

Pending:
- Medium follow-up: CSP still allows `'unsafe-inline'`; move inline UI CSS/JS into static assets and tighten policy.
- Medium follow-up: make migrations robust against partial-apply states without relying on SQLite features unavailable in older sqlite3 builds.
- Low/medium follow-up: make map tile provider privacy posture explicit/configurable for blank/local/custom tiles.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 16:31 UTC - Phase 82 (Admin Device UI + Key Rotation + Visit Generation)
Implemented:
- Added a new admin-only devices management UI page at `GET /ui/admin/devices`.
- Added lightweight browser workflows for:
- listing devices with `id`, `name`, `owner/user`, `created_at`, `updated_at`, and masked `api_key_preview`
- creating devices with selectable owner and source type
- rotating API keys per device with one-time plaintext key display and copy button
- triggering visit generation for one device or all listed devices with optional date range
- Reused existing backend APIs and CSRF/session patterns (`X-CSRF-Token` from page meta).
- Added a clear UI note that delete/disable is not available because backend endpoints do not exist.
- Updated top navigation for admin users:
- status/map pages now show both `Devices` and `Users` admin links
- users page now includes a `Devices` link
- Expanded UI and router security tests:
- admin devices page render coverage
- non-admin denial for `/ui/admin/devices`
- route protection checks include `/ui/admin/devices`
- devices asset test verifies rotate/generate workflow hooks and CSRF header usage
- Expanded API tests so admin users can list all devices and rotate keys across owners, while non-admin ownership restrictions remain.
- Updated README with new admin devices UI workflow documentation and CSRF coverage list.

Architectural decisions:
- Decision: Implement admin device management as a dedicated admin-only UI page (`/ui/admin/devices`) instead of mixing the workflow into status/map pages.
  Reason: Keeps existing pages lightweight/read-only and isolates privileged write actions in one explicit admin screen.
- Decision: Reuse existing device/visit APIs rather than introducing new admin-specific endpoints.
  Reason: Minimal change, lower maintenance overhead, and preserves current auth/CSRF behavior.

Files changed:
- `internal/api/ui.go`
- `internal/api/devices.go`
- `internal/api/ui_test.go`
- `internal/api/router_security_test.go`
- `internal/api/devices_test.go`
- `internal/api/assets/app/devices.js`
- `internal/api/assets/app/devices.css`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/ui.go internal/api/ui_test.go internal/api/router_security_test.go internal/api/devices.go internal/api/devices_test.go`
- `go test ./internal/api`
- `go test ./...`
- `timeout 6s go run ./cmd/server`

Pending:
- Add authenticated browser smoke test that covers login -> `/ui/admin/devices` and validates create/rotate/generate UI actions with CSRF token flow end-to-end.
- Consider adding backend support for device disable/delete if operational requirements need key revocation without rotation.
- Keep previously tracked hardening follow-ups: dynamic CSP `img-src` by tile mode and explicit migration fixture for `0005` partial-state recovery.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 16:58 UTC - Phase 83 (Backup/Restore Workflow for SQLite + Spool)
Implemented:
- Added practical backup and restore scripts:
- `scripts/backup.sh` for timestamped archive creation
- `scripts/restore.sh` for safe restore with pre-restore safety copy
- Backup script supports:
- online-safe DB snapshot mode using `sqlite3 .backup` (default)
- offline mode (`--offline`) for stopped-service file copy
- optional no-compression output (`--no-compress`)
- configurable sqlite/spool/output paths via flags and existing env defaults
- Restore script supports:
- required archive input
- explicit stop-service warning
- restore to configurable sqlite/spool targets
- automatic pre-restore snapshot of existing target files
- forced non-interactive restore mode (`--force`)
- Updated README with:
- online backup workflow
- offline backup workflow
- restore workflow
- retention guidance
- cron and systemd timer automation examples
- relevant config/env path references (`APP_SQLITE_PATH`, `APP_SPOOL_DIR`)
- Validated end-to-end in isolated temp paths:
- created online backup archive from current runtime data
- restored into temp sqlite/spool target
- started server successfully using restored data paths

Architectural decisions:
- Decision: Use shell scripts + tar archives and SQLite `.backup` instead of adding in-app backup endpoints or dependencies.
  Reason: Keeps the solution simple, low-overhead, and aligned with single-process Raspberry Pi operations.
- Decision: Restore always takes a pre-restore safety copy.
  Reason: Reduces operator risk during manual disaster-recovery operations.

Files changed:
- `scripts/backup.sh`
- `scripts/restore.sh`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `bash -n scripts/backup.sh scripts/restore.sh`
- `scripts/backup.sh --sqlite-path ./data/plexplore.db --spool-dir ./data/spool --output-dir /tmp/plexplore-backup-test/backups`
- `scripts/restore.sh --archive /tmp/plexplore-backup-test/backups/plexplore-backup-20260424-165232.tar.gz --sqlite-path /tmp/plexplore-backup-test/restore-data/plexplore-restored.db --spool-dir /tmp/plexplore-backup-test/restore-data/spool --force`
- `/bin/bash -lc "APP_SQLITE_PATH=/tmp/plexplore-backup-test/restore-data/plexplore-restored.db APP_SPOOL_DIR=/tmp/plexplore-backup-test/restore-data/spool APP_HTTP_LISTEN_ADDR=127.0.0.1:18090 timeout 6s go run ./cmd/server"`
- `go test ./...`

Pending:
- Add optional prune helper for retention (for example keep last N daily archives) to reduce manual cleanup.
- Add one restore drill checklist section for operators (post-restore `make migrate`, `/health`, `/status`, and quick ingest test).
- Keep previously tracked hardening follow-ups: authenticated UI smoke test, dynamic CSP `img-src` by tile mode, and migration fixture for `0005` partial state.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 17:11 UTC - Phase 84 (Points Pagination + Streamed Exports)
Implemented:
- Added hard-capped pagination for `GET /api/v1/points`:
- `limit` now defaults to `500` and is hard-capped at `1000`
- new `cursor` query param (based on `seq`) for forward pagination
- response now includes `next_cursor` when additional rows are available
- Updated points query path to request `limit+1` and compute next-page metadata without loading unnecessary rows.
- Added owner scoping directly in point/export SQL filter (`user_id`) to avoid cross-user post-filtering buffers.
- Added export streaming path to reduce RAM usage:
- new store callback method `StreamPointsForExport(...)`
- GeoJSON export now streams FeatureCollection entries row-by-row
- GPX export now streams `<trkpt>` entries row-by-row
- export routes support optional `limit` with default `5000` and hard cap `20000`
- Added downloadable filename headers:
- GeoJSON: `Content-Disposition: attachment; filename="plexplore-export.geojson"`
- GPX: `Content-Disposition: attachment; filename="plexplore-export.gpx"`
- Preserved existing filters: `device_id`, `from`, `to`.
- Added/updated tests for:
- points limit cap behavior
- points cursor pagination behavior
- export limit cap behavior
- export streamed code path usage and content-type/header correctness
- store-level cursor and stream iteration behavior

Architectural decisions:
- Decision: Use cursor pagination (`seq`) instead of offset pagination for point history.
  Reason: Cursor pagination is simpler, stable for append-heavy workloads, and avoids expensive offset scans.
- Decision: Stream export responses directly from SQLite row iteration.
  Reason: Avoids large in-memory slices during export on Raspberry Pi Zero 2 W.

Files changed:
- `internal/api/points.go`
- `internal/api/exports.go`
- `internal/api/health.go`
- `internal/api/points_test.go`
- `internal/api/exports_test.go`
- `internal/store/points.go`
- `internal/store/sqlite_store_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/points.go internal/api/health.go internal/api/points.go internal/api/exports.go internal/api/points_test.go internal/api/exports_test.go internal/store/sqlite_store_test.go`
- `go test ./internal/api`
- `go test ./internal/store`
- `go test ./internal/api -run 'Test(PointsEndpoint_LimitCapApplied|PointsEndpoint_PaginationCursor|GeoJSONExport_ValidStructure|GPXExport_ValidStructureAndContent|ExportEndpoints_LimitCapApplied)' -count=1`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`
- `timeout 6s go run ./cmd/server`

Pending:
- Optional follow-up: add stable export cursor/chunk checkpoint API for resumable very-large exports over unreliable links.
- Keep previously tracked follow-ups: authenticated `/ui/admin/devices` smoke test, dynamic CSP `img-src` tightening by tile mode, and migration fixture for `0005` partial state.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 17:21 UTC - Phase 85 (Map Performance: Downsampling + Clustering)
Implemented:
- Added optional backend point simplification controls on `GET /api/v1/points`:
- `simplify=true|false`
- `max_points` target output count
- when simplification is active, backend downsamples deterministically (keeps route shape endpoints) and returns metadata:
- `sampled: true`
- `sampled_from: <original_count>`
- Added simplify-mode query caps for low-RAM safety:
- simplified fetch limit defaults to `5000`, hard max `20000`
- simplified output cap defaults to `1000`, hard max `5000`
- Maintained cursor pagination behavior with `next_cursor` while simplifying point payload size.
- Added map UI threshold handling and performance modes in `map.js`:
- small desired set (`<=2k`): full track mode
- medium desired set: sampled track + clustered markers
- large desired set: stronger sampling and marker suppression (polyline retained)
- Added lightweight client-side grid clustering for markers when point count is moderate.
- Added explicit map sampling/performance note in UI (`#sampling_note`) when sampling/clustering/suppression is active.
- Updated UI/map tests to verify:
- sampling note element exists
- map script requests `simplify` and `max_points`
- clustering helper exists in served map script
- Added API/store tests for:
- large simplified query reducing result count
- cursor pagination behavior
- export and query path compatibility after simplification changes

Architectural decisions:
- Decision: Combine backend deterministic downsampling with lightweight frontend clustering.
  Reason: Keeps payload and rendering costs low on Raspberry Pi while preserving a useful route path.
- Decision: Keep implementation dependency-free (no heavy clustering/simplification library).
  Reason: Maintain low memory/CPU overhead and operational simplicity.

Files changed:
- `internal/api/points.go`
- `internal/api/points_test.go`
- `internal/api/ui.go`
- `internal/api/ui_test.go`
- `internal/api/assets/app/map.js`
- `internal/api/assets/app/map.css`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/api/points.go internal/api/points_test.go internal/api/exports.go internal/api/ui.go internal/api/ui_test.go internal/store/points.go`
- `go test ./internal/api`
- `go test ./internal/store`
- `go test ./internal/api -run 'Test(MapPageServedAtUIMap|UIAssets_MapScriptContainsEscapedPopupFields|PointsEndpoint_SimplifyReducesLargeResponse|PointsEndpoint_PaginationCursor)' -count=1`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`
- `timeout 6s go run ./cmd/server`

Pending:
- Optional refinement: tune clustering cell size by zoom/motion for denser city tracks.
- Keep previously tracked follow-ups: authenticated `/ui/admin/devices` smoke test, dynamic CSP `img-src` tightening, and migration fixture for `0005` partial state.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 17:40 UTC - Phase 86 (Scheduled Incremental Visit Generation)
Implemented:
- Added lightweight in-process visit scheduler wiring to runtime startup/shutdown:
- scheduler starts with the service and runs periodic/background visit generation when enabled
- scheduler is stopped during shutdown before final flusher stop
- Added incremental watermark persistence for automatic visit generation:
- migration `0008_visit_generation_state.sql` adds `visit_generation_state`
- watermark stores per-device `last_processed_seq` and `updated_at`
- scheduler processes only devices with new points (`max_seq > watermark`)
- scheduler applies configurable lookback overlap for visit-boundary safety
- scheduler skips overlapping concurrent runs
- manual trigger via `POST /api/v1/visits/generate` remains unchanged
- Added focused scheduler tests:
- background scheduler trigger path
- incremental behavior across repeated runs
- overlap prevention for concurrent runs
- Updated README with scheduler behavior and config knobs.

Architectural decisions:
- Decision: Implement visit automation as an optional in-process scheduler with persisted per-device watermark.
  Reason: Keeps CPU/RAM overhead low, preserves single-process operation, and avoids external scheduler dependencies.
- Decision: Keep manual visit generation endpoint active alongside scheduler.
  Reason: Preserves explicit operator control and bounded ad-hoc recomputation workflows.

Files changed:
- `cmd/server/main.go`
- `internal/tasks/visit_scheduler_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w cmd/server/main.go internal/tasks/visit_scheduler_test.go`
- `go test ./internal/tasks -run TestVisitScheduler -count=1`
- `go test ./cmd/server -count=1`
- `go test ./internal/api -run 'TestGenerateVisitsEndpoint_' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(ListVisits|VisitDetection_)' -count=1`
- `go test ./...`
- `timeout 6s go run ./cmd/server`

Pending:
- Add scheduler status fields to authenticated `/api/v1/status` (last run/result) for easier operations visibility.
- Add store-level integration coverage asserting persisted watermark behavior against SQLite (not only fake-store scheduler tests).
- Evaluate device-identifier uniqueness assumptions in visit rebuild paths to avoid ambiguity when multiple users have the same device name.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-24 18:35 UTC - Phase 87 (Visit Isolation by Stable Device/User Identity)
Implemented:
- Fixed cross-user visit isolation bug by removing runtime visit identity logic based on `devices.name`.
- Visit store APIs now operate on stable device row IDs and authenticated user scope:
- `RebuildVisitsForDeviceRange(... deviceID int64 ...)`
- `ListVisits(... userID int64, deviceID *int64 ...)`
- Visit queries now scope by `devices.user_id` and optional stable `visits.device_id`.
- Visit generation now requires numeric `device_id` query param (stable device row ID) and validates ownership against the authenticated session user.
- Visit list filtering now accepts numeric `device_id` and returns `device_name` as display label while preserving stable `device_id` in output.
- Scheduler now iterates and tracks progress by stable `devices.id`:
- removed name-based dedupe path that could merge same-name devices across users
- watermark/progress methods now use stable `device_id`
- Added migration `0009_visit_generation_state_device_id.sql`:
- migrates `visit_generation_state` from `device_name` key to `device_id` key
- backfills existing rows by joining to `devices` and preserving last processed sequence where mapping is possible
- Updated map/admin UI visit workflows to use stable numeric device IDs for visits API calls while keeping device names as display labels.
- Added regression coverage:
- store test for two users with same device name and isolated visit generation/listing
- scheduler test for same-name devices across users with independent watermark/progress
- API tests for same-name cross-user isolation on list/generate and numeric device filter validation
- migrator test covering 0009 backfill behavior.

Architectural decisions:
- Decision: Make visit identity boundaries stable (`user_id` + `device_id`) and treat device name strictly as display label.
  Reason: Prevent cross-user collisions/leakage when multiple users have same device name.
- Decision: Require numeric `device_id` for visit generate/list filters.
  Reason: Eliminate ambiguity and avoid name-based authorization decisions.

Files changed:
- `internal/store/visits.go`
- `internal/store/visit_generation_state.go`
- `internal/store/visits_test.go`
- `internal/store/migrator_test.go`
- `internal/tasks/visit_scheduler.go`
- `internal/tasks/visit_scheduler_test.go`
- `internal/api/health.go`
- `internal/api/visits.go`
- `internal/api/visits_test.go`
- `internal/api/assets/app/map.js`
- `internal/api/assets/app/devices.js`
- `migrations/0009_visit_generation_state_device_id.sql`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/store/visits.go internal/store/visit_generation_state.go internal/store/visits_test.go internal/store/migrator_test.go internal/tasks/visit_scheduler.go internal/tasks/visit_scheduler_test.go internal/api/health.go internal/api/visits.go internal/api/visits_test.go`
- `go test ./...`
- `go test ./internal/api`
- `go test ./internal/store`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./internal/tasks -run TestVisit -count=1`

Pending:
- Align points/recent/export query filters to accept stable numeric device IDs (currently device-name based filters remain for those endpoints).
- Add an authenticated browser smoke test for `/login` -> `/ui/admin/devices` -> visit generation path with CSRF + numeric device IDs.
- Add scheduler status fields (last run/last error) to authenticated `/api/v1/status`.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.
- On checkpoint advancement failure, current flusher behavior does not requeue already-drained batch; this pre-existing behavior should be addressed in a focused follow-up.

### 2026-04-25 09:25 UTC - Phase 88 (Flusher Checkpoint-Failure Retry Safety)
Implemented:
- Fixed flusher checkpoint-failure behavior so drained RAM batches are not dropped after a successful SQLite commit.
- `flushOneBatch()` now requeues drained batch to buffer front when `AdvanceCheckpoint(...)` fails.
- Preserved existing durability/ordering rules:
- SQLite insert failure => requeue + no checkpoint + no compaction
- checkpoint advancement still occurs only after SQLite commit
- compaction still occurs only after checkpoint success
- Added/updated flusher tests:
- checkpoint failure requeues drained batch and does not compact
- retry after checkpoint failure eventually advances checkpoint
- duplicate durable rows are not created on retry path (idempotent commit behavior modeled by unique seq tracking)
- last flush result clearly records checkpoint failure
- Updated README startup/recovery notes to document normal-runtime retry behavior after checkpoint failure.

Architectural decisions:
- Decision: Use requeue-on-checkpoint-failure (Option A) rather than adding a separate pending-checkpoint state machine.
  Reason: smallest focused change, keeps single-writer architecture simple, and leverages existing idempotent SQLite insert semantics.

Files changed:
- `internal/flusher/flusher.go`
- `internal/flusher/flusher_test.go`
- `README.md`
- `PROJECT_LOG.md`
- `NEXT_STEPS.md`

Commands:
- `gofmt -w internal/flusher/flusher.go internal/flusher/flusher_test.go`
- `go test ./internal/flusher`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./...`

Pending:
- Add status surfacing for checkpoint-retry pressure (for example repeated checkpoint failures count or latest checkpoint failure timestamp) in authenticated `/api/v1/status`.
- Keep prior follow-ups: points/recent/export device filter identity alignment and authenticated browser admin smoke coverage.

Known issues:
- In this shell environment, some commands require elevated execution because of sandbox restrictions.

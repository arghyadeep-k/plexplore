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

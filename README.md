# Plexplore

Lightweight personal location history server scaffold for Raspberry Pi Zero 2 W.

## Requirements

- Go 1.22+

## Setup

```bash
go mod tidy
```

Also install `sqlite3` CLI (used by the lightweight migration runner).

## Run

```bash
make run
```

Server defaults to `0.0.0.0:8080`.
On startup, the service replays spool records newer than checkpoint into RAM
and performs a recovery flush before serving normal ingest traffic.

## Health Check

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{"status":"ok","service":"plexplore"}
```

## Device Management (Minimal)

- `POST /api/v1/devices` creates a device.
- `GET /api/v1/devices` lists devices.

Example create request:

```bash
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Content-Type: application/json" \
  -d '{"name":"phone-main","source_type":"owntracks","api_key":"dev-key-1"}'
```

Assumption for this phase: single-user deployment.
If `user_id` is omitted, device creation defaults to user `1` (`default` user).

API key auth helper is available in `internal/api/auth.go` and is intended to be
applied to ingest endpoints as they are added.

## Ingestion Endpoints

- `POST /api/v1/owntracks`
- `POST /api/v1/overland/batches`

Both endpoints require device API key auth via `X-API-Key` (or
`Authorization: Bearer <api_key>`). Request handling flow is:
parse payload -> canonical points -> ensure ingest hash -> append spool ->
enqueue RAM buffer. Handlers do not write directly to SQLite.

## Operational Status

- `GET /api/v1/status` returns a small JSON snapshot for operations:
- current buffer points/bytes
- oldest buffered age (seconds)
- spool segment count
- current checkpoint sequence
- last flush result (when available)

## Minimal Web UI

- `GET /` serves a lightweight status page.
- `GET /ui/status` serves the same page explicitly.

The page is intentionally minimal (plain HTML/CSS/vanilla JS, no SPA build
toolchain) and is served directly by the Go HTTP server. It reads existing JSON
endpoints (`/health`, `/api/v1/status`, `/api/v1/devices`) to show:
- service health
- devices
- buffer stats
- last flush status

## Environment Variables

- `APP_HTTP_LISTEN_ADDR` (default: `0.0.0.0:8080`): HTTP bind address.
- `APP_SQLITE_PATH` (default: `./data/plexplore.db`): SQLite database file path.
- `APP_MIGRATIONS_DIR` (default: `./migrations`): SQL migration files directory.
- `APP_SPOOL_DIR` (default: `./data/spool`): directory for segmented spool files.
- `APP_SPOOL_SEGMENT_MAX_BYTES` (default: `4194304`): max bytes per spool segment.
- `APP_SPOOL_FSYNC_MODE` (default: `balanced`): fsync policy (`always`, `balanced`, `low-wear`).
- `APP_SPOOL_FSYNC_INTERVAL` (default: `2s`): periodic fsync interval for non-`always` modes.
- `APP_SPOOL_FSYNC_BYTE_THRESHOLD` (default: `65536`): bytes written before forced fsync.
- `APP_BUFFER_MAX_POINTS` (default: `256`): max points held in RAM buffer.
- `APP_BUFFER_MAX_BYTES` (default: `262144`): approximate max bytes held in RAM buffer.
- `APP_FLUSH_INTERVAL` (default: `10s`): periodic flush interval from buffer to durable store.
- `APP_FLUSH_BATCH_SIZE` (default: `128`): max points per flush batch.
- `APP_READ_TIMEOUT_SECONDS` (default: `5`): HTTP read timeout in seconds.
- `APP_WRITE_TIMEOUT_SECONDS` (default: `10`): HTTP write timeout in seconds.
- `APP_IDLE_TIMEOUT_SECONDS` (default: `30`): HTTP idle timeout in seconds.

## Database Migrations

Run migrations:

```bash
make migrate
```

or:

```bash
go run ./cmd/migrate
```

The migration runner keeps a `schema_migrations` table and applies `.sql` files
from `APP_MIGRATIONS_DIR` in filename order.

SQLite pragmas applied by migration runner (Pi-friendly defaults):

- `journal_mode=WAL`
- `synchronous=NORMAL`
- `wal_autocheckpoint=1000`
- `busy_timeout=5000`
- `cache_size=-4096` (about 4MB page cache target)
- `temp_store=MEMORY`
- `foreign_keys=ON`

## Core Models

- Canonical domain models live in `internal/ingest/models.go`:
- `CanonicalPoint`
- `SpoolRecord`
- `BufferStats`

## Ingestion Parsing

- OwnTracks payload parsing is implemented in `internal/ingest/owntracks_parser.go`.
- Parser tests are in `internal/ingest/owntracks_parser_test.go`.

## Known Caveats

- Overland ingestion currently expects `locations[].coordinates` in `[lon, lat]` order.
  GeoJSON-style `geometry.coordinates` payloads are not yet supported and return 400.

- Device API currently returns `api_key` in responses for ease of development and testing.
  This should be hardened later so full keys are only shown at creation or rotation time.
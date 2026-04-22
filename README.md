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
- `GET /api/v1/devices/{id}` returns one device.
- `POST /api/v1/devices/{id}/rotate-key` rotates device API key.

Device API key hygiene:
- create and rotate responses include full `api_key` once
- list/read responses return only `api_key_preview` (masked)

Example workflow:

```bash
# 1) Create device (full api_key returned once)
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Content-Type: application/json" \
  -d '{"name":"phone-main","source_type":"owntracks","api_key":"dev-key-1"}'

# 2) List devices (masked api_key_preview only)
curl -sS http://localhost:8080/api/v1/devices

# 3) Read one device (masked api_key_preview only)
curl -sS http://localhost:8080/api/v1/devices/1

# 4) Rotate API key (new full api_key returned once)
curl -X POST http://localhost:8080/api/v1/devices/1/rotate-key \
  -H "Content-Type: application/json" \
  -d '{"api_key":"dev-key-rotated-1"}'
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

## Connect OwnTracks

Endpoint:
- `POST http://<host>:8080/api/v1/owntracks`

Authentication:
- header: `X-API-Key: <device_api_key>`
- alternatively: `Authorization: Bearer <device_api_key>`

Headers:
- `Content-Type: application/json`

Payload expectations:
- OwnTracks location event (`_type: "location"`)
- required fields: `lat`, `lon`, `tst` (unix seconds)
- example:

```bash
curl -X POST http://localhost:8080/api/v1/owntracks \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-key-1" \
  -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'
```

Known caveats:
- only location events are accepted (`_type=location`)
- invalid coordinate/timestamp values return `400`

## Connect Overland

Endpoint:
- `POST http://<host>:8080/api/v1/overland/batches`

Authentication:
- header: `X-API-Key: <device_api_key>`
- alternatively: `Authorization: Bearer <device_api_key>`

Headers:
- `Content-Type: application/json`

Payload expectations:
- top-level `locations` array is required
- each location requires `coordinates: [lon, lat]` and `timestamp` (RFC3339)
- optional top-level `device_id` is accepted; authenticated device identity is enforced server-side
- example:

```bash
curl -X POST http://localhost:8080/api/v1/overland/batches \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-key-1" \
  -d '{"device_id":"phone-main","locations":[{"coordinates":[-87.0,41.0],"timestamp":"2026-04-22T12:00:00Z"}]}'
```

Known caveats:
- GeoJSON `geometry.coordinates` payloads are not supported
- use `locations[].coordinates` only

## Ingest Troubleshooting

- `400 Bad Request`:
  payload shape/fields are invalid (for example missing `lat/lon/tst` in OwnTracks, invalid Overland `coordinates` or timestamp format)
- `401 Unauthorized`:
  API key missing or invalid; verify `X-API-Key`/`Authorization` and device key rotation state
- `500 Internal Server Error`:
  server-side issue (commonly migration/schema mismatch or SQLite/runtime failure); run `make migrate`, check `/api/v1/status`, and inspect server logs

## Operational Status

- `GET /api/v1/status` (and alias `GET /status`) returns a small JSON snapshot for operations.

Example:

```bash
curl -sS http://localhost:8080/status
```

Example response:

```json
{
  "service_health": "ok",
  "buffer_points": 0,
  "buffer_bytes": 0,
  "oldest_buffered_age_seconds": 0,
  "spool_dir_path": "./data/spool",
  "spool_segment_count": 1,
  "checkpoint_seq": 42,
  "last_flush_attempt_at_utc": "2026-04-22T17:35:10.148337224Z",
  "last_flush_success_at_utc": "2026-04-22T17:35:10.148337224Z",
  "sqlite_db_path": "./data/plexplore.db",
  "last_flush": {
    "at_utc": "2026-04-22T17:35:10.148337224Z",
    "success": true
  }
}
```

Included fields (when available):
- service health
- buffer points/bytes and oldest buffered age
- spool directory path, active segment count, checkpoint sequence
- last flush attempt time, last successful flush time, last flush error
- SQLite database path

## Recent Points (Debug)

- `GET /api/v1/points/recent` returns compact recent stored points from SQLite.
- query params:
- `device_id` (optional): device name filter
- `limit` (optional): max rows (default `50`, max `500`)

Examples:

```bash
# recent points across devices
curl -sS "http://localhost:8080/api/v1/points/recent"

# recent points for one device
curl -sS "http://localhost:8080/api/v1/points/recent?device_id=phone-main&limit=20"
```

## Point History (Map-Friendly)

- `GET /api/v1/points` returns stored points in ascending timestamp order.
- optional query params:
- `from` (RFC3339 timestamp)
- `to` (RFC3339 timestamp)
- `device_id` (device name)
- `limit` (default `500`, max `5000`)

Response fields:
- `seq`
- `device_id`
- `source_type`
- `timestamp_utc`
- `lat`
- `lon`

Examples:

```bash
# default point history query
curl -sS "http://localhost:8080/api/v1/points"

# filtered point history for map view
curl -sS "http://localhost:8080/api/v1/points?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z&limit=1000"
```

## Visit Detection (Lightweight)

Visit detection is implemented as a deterministic pass over stored points per
device:
- points must remain within a configurable max radius
- dwell time between first and last point in the candidate window must meet a
  configurable minimum

Current persistence model stores visits with:
- `id`
- `device_id`
- `start_at`
- `end_at`
- `centroid_lat`
- `centroid_lon`
- `point_count`

Optional place-label cache (visit centroids only):
- `GET /api/v1/visits` can include `place_label` when reverse geocode cache is enabled
- lookup is performed only for visit centroids (not for every point)
- cached labels are persisted locally in SQLite (`visit_place_cache`)
- provider calls are bounded per request and disabled by default

Implementation is intentionally simple (no clustering libraries) for Raspberry
Pi-friendly resource usage.

Visit generation workflow:
- visits are generated on-demand via `POST /api/v1/visits/generate`
- requires `device_id`
- supports bounded window with optional `from` / `to` RFC3339 params
- if `from` / `to` are omitted, generation defaults to a recent 14-day window
- generated visits can be listed via `GET /api/v1/visits` with optional
  `device_id`, `from`, `to`, and `limit` filters
- optional tuning params:
- `min_dwell` (duration, default `15m`)
- `max_radius_m` (meters, default `35`)

Examples:

```bash
# generate visits for a device in the default recent window
curl -X POST "http://localhost:8080/api/v1/visits/generate?device_id=phone-main"

# generate visits for a bounded range
curl -X POST "http://localhost:8080/api/v1/visits/generate?device_id=phone-main&from=2026-04-20T00:00:00Z&to=2026-04-22T23:59:59Z&min_dwell=20m&max_radius_m=40"

# list generated visits
curl -sS "http://localhost:8080/api/v1/visits?device_id=phone-main&limit=100"

# list visits for a bounded range
curl -sS "http://localhost:8080/api/v1/visits?device_id=phone-main&from=2026-04-20T00:00:00Z&to=2026-04-22T23:59:59Z&limit=100"
```

## GeoJSON Export

- `GET /api/v1/exports/geojson` returns stored points as GeoJSON FeatureCollection.
- optional filters:
- `from` (RFC3339 timestamp)
- `to` (RFC3339 timestamp)
- `device_id` (device name)

Examples:

```bash
# all points as GeoJSON
curl -sS "http://localhost:8080/api/v1/exports/geojson"

# filtered GeoJSON export
curl -sS "http://localhost:8080/api/v1/exports/geojson?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z"
```

## GPX Export

- `GET /api/v1/exports/gpx` returns stored points as GPX 1.1.
- optional filters:
- `from` (RFC3339 timestamp)
- `to` (RFC3339 timestamp)
- `device_id` (device name)

Examples:

```bash
# all points as GPX
curl -sS "http://localhost:8080/api/v1/exports/gpx"

# filtered GPX export
curl -sS "http://localhost:8080/api/v1/exports/gpx?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z"
```

## Shutdown And Recovery Behavior

Shutdown handling is designed to be simple and reliability-first:
- on `SIGINT`/`SIGTERM`, service enters draining mode and rejects new ingest requests (`503`)
- HTTP server stops accepting new work and lets in-flight requests finish (within shutdown timeout)
- flusher attempts to drain pending RAM buffer records to SQLite before exit
- spool files and checkpoint are synced on close/write paths used during shutdown

Behavior differences:
- Clean shutdown (`SIGINT`/`SIGTERM`): best effort to complete in-flight requests, flush buffer to SQLite, and advance checkpoint.
- Forced kill (`SIGKILL`): no graceful hooks run; buffered (not-yet-flushed) points may be lost from RAM and checkpoint may lag.
- Crash/power loss: similar to forced kill; on next startup, spool replay recovers records with sequence > checkpoint.

Manual validation procedure:
1. Start service:
```bash
make run
```
2. Create a device and ingest one point:
```bash
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Content-Type: application/json" \
  -d '{"name":"phone-main","source_type":"owntracks","api_key":"dev-key-1"}'

curl -X POST http://localhost:8080/api/v1/owntracks \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-key-1" \
  -d '{"_type":"location","lat":41.0,"lon":-87.0,"tst":1713777600}'
```
3. Send graceful signal (replace `<pid>`):
```bash
kill -TERM <pid>
```
4. Restart service and verify status/checkpoint:
```bash
make run
curl -sS http://localhost:8080/status
```
Expected: service starts cleanly; status shows no request-path errors and checkpoint is at or beyond recently flushed sequence.

## Startup Recovery Flow

Startup recovery runs before normal ingest traffic begins:
1. server opens spool, SQLite store, and RAM buffer
2. `RecoverFromSpool(...)` reads current checkpoint
3. spool replays records with `seq > checkpoint.last_committed_seq`
4. replayed records are enqueued into RAM buffer in bounded chunks
5. flusher runs an immediate flush to SQLite
6. checkpoint advances only after successful SQLite commit
7. HTTP server starts listening after recovery step completes

Notes:
- replay is checkpoint-based, so already-committed records are normally skipped
- if checkpoint is stale, replay may attempt already-committed rows; SQLite `ingest_hash` uniqueness prevents duplicate `raw_points` rows
- after crash/power loss, recovery replays uncheckpointed spool data on next startup

## Minimal Web UI

- `GET /` serves a lightweight status page.
- `GET /ui/status` serves the same page explicitly.
- `GET /ui/map` serves a lightweight map page.

The page is intentionally minimal (plain HTML/CSS/vanilla JS, no SPA build
toolchain) and is served directly by the Go HTTP server. It reads existing JSON
endpoints (`/health`, `/api/v1/status`, `/api/v1/devices`, `/api/v1/points/recent`) to show:
- service health
- devices
- buffer stats
- spool/checkpoint status
- last flush status
- recent points preview

Map page notes:
- uses Leaflet (CDN-loaded) with OpenStreetMap tiles
- fetches track points from `GET /api/v1/points`
- renders an ordered track polyline
- renders lightweight point markers for smaller result sets
- renders lightweight visit centroid markers from `/api/v1/visits` with popup details (including `place_label` when available)
- includes a small visits summary table (start, end, duration, device) below the map
- supports filtering by device and date range (`from`/`to` day inputs)
- defaults to a recent 7-day range when no date filters are set

## Raspberry Pi Deployment (systemd)

Sample deployment files are included:
- `deploy/systemd/plexplore.service`
- `deploy/systemd/plexplore.env.sample`
- `scripts/install_systemd.sh`

Suggested persistent paths on Pi:
- SQLite DB: `/var/lib/plexplore/plexplore.db`
- Spool dir: `/var/lib/plexplore/spool`

Build and install:

```bash
go build -o plexplore-server ./cmd/server
sudo ./scripts/install_systemd.sh
```

Service operations:

```bash
sudo systemctl start plexplore
sudo systemctl stop plexplore
sudo systemctl restart plexplore
sudo systemctl status plexplore
```

Inspect logs:

```bash
sudo journalctl -u plexplore -f
sudo journalctl -u plexplore --since "1 hour ago"
```

Back up DB and spool (recommended while service is stopped):

```bash
sudo systemctl stop plexplore
sudo cp /var/lib/plexplore/plexplore.db /var/lib/plexplore/plexplore.db.bak
sudo tar -czf /var/lib/plexplore/spool-$(date +%F-%H%M%S).tgz -C /var/lib/plexplore spool
sudo systemctl start plexplore
```

## Docker (Single Container)

This repo includes a lightweight multi-stage Docker setup suitable for ARM and x86.
The container stores SQLite and spool state under `/data` (mounted volume).

Files:
- `Dockerfile`
- `.dockerignore`
- `compose.yaml` (optional)

Build image:

```bash
docker build -t plexplore:dev .
```

Run container:

```bash
docker run --rm \
  -p 8080:8080 \
  -v "$(pwd)/data:/data" \
  -e APP_HTTP_LISTEN_ADDR=0.0.0.0:8080 \
  -e APP_SQLITE_PATH=/data/plexplore.db \
  -e APP_SPOOL_DIR=/data/spool \
  plexplore:dev
```

The container entrypoint runs migrations first, then starts the server.

Compose:

```bash
docker compose up --build -d
docker compose down
```

Environment variables commonly used in containers:
- `APP_HTTP_LISTEN_ADDR` (default in image: `0.0.0.0:8080`)
- `APP_SQLITE_PATH` (default in image: `/data/plexplore.db`)
- `APP_SPOOL_DIR` (default in image: `/data/spool`)
- `APP_MIGRATIONS_DIR` (default in image: `/app/migrations`)
- `APP_BUFFER_MAX_POINTS`
- `APP_BUFFER_MAX_BYTES`
- `APP_FLUSH_INTERVAL`
- `APP_FLUSH_BATCH_SIZE`
- `APP_FLUSH_TRIGGER_POINTS`
- `APP_FLUSH_TRIGGER_BYTES`
- `APP_SPOOL_FSYNC_MODE`
- `APP_SPOOL_FSYNC_INTERVAL`
- `APP_SPOOL_FSYNC_BYTE_THRESHOLD`

Raspberry Pi Zero 2 W caveats:
- Prefer running on local Pi storage or a reliable SSD-backed USB volume; avoid unstable network mounts for `/data`.
- Use conservative defaults (`balanced` fsync mode, modest buffer sizes) to reduce RAM and SD-card wear.
- For best compatibility, build on the target Pi (or use `docker buildx` for the exact target architecture/variant).
- Keep only one service instance writing to a given `/data` volume.

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
- `APP_FLUSH_TRIGGER_POINTS` (default: `75%` of `APP_BUFFER_MAX_POINTS`): best-effort ingest-path flush trigger when buffered points reaches threshold.
- `APP_FLUSH_TRIGGER_BYTES` (default: `75%` of `APP_BUFFER_MAX_BYTES`): best-effort ingest-path flush trigger when buffered bytes reaches threshold.
- `APP_READ_TIMEOUT_SECONDS` (default: `5`): HTTP read timeout in seconds.
- `APP_WRITE_TIMEOUT_SECONDS` (default: `10`): HTTP write timeout in seconds.
- `APP_IDLE_TIMEOUT_SECONDS` (default: `30`): HTTP idle timeout in seconds.
- `APP_REVERSE_GEOCODE_ENABLED` (default: `false`): enable optional visit-centroid place-label enrichment.
- `APP_REVERSE_GEOCODE_PROVIDER` (default: `nominatim`): reverse geocode provider id.
- `APP_REVERSE_GEOCODE_NOMINATIM_URL` (default: `https://nominatim.openstreetmap.org/reverse`): Nominatim reverse endpoint.
- `APP_REVERSE_GEOCODE_USER_AGENT` (default: `plexplore/1.0 (+self-hosted)`): User-Agent sent to provider.
- `APP_REVERSE_GEOCODE_TIMEOUT` (default: `2s`): provider HTTP timeout.
- `APP_REVERSE_GEOCODE_CACHE_DECIMALS` (default: `4`): centroid coordinate rounding for cache keys (higher precision = more cache entries).
- `APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST` (default: `3`): hard cap on provider calls during one `GET /api/v1/visits`.

Flush trigger policy:
- periodic flush loop remains active (`APP_FLUSH_INTERVAL`).
- after ingest appends to spool and enqueues RAM buffer, service checks buffer stats.
- if points or bytes threshold is crossed, service issues a non-blocking best-effort flusher trigger.
- request handlers still do not write directly to SQLite.
- near-duplicates suppressed by RAM dedupe are retained as lightweight checkpoint-only markers so checkpoint can still advance through their spool sequence during normal runtime.
- after successful SQLite commit and checkpoint advancement, service best-effort compacts fully committed spool segments.

Reverse geocode cache policy:
- disabled by default; enable only if you want place labels in visits output.
- applies to visit centroids only (`GET /api/v1/visits`), never to every raw point.
- local SQLite cache is checked first; provider is used only on cache miss.
- provider calls are bounded by `APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST` to limit network usage on Raspberry Pi deployments.

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

- Device API returns full `api_key` only on create/rotate responses.
  List/read responses return masked `api_key_preview`.

# Next Steps

## Current milestone
Stabilize visits UX and verification after completing visits API/map Tasks 1-3

## Next 3 tasks
1. Add an integration-style API test for `GET /api/v1/visits` against real SQLite-backed fixtures (device + from/to + limit)
2. Add a lightweight map UI test assertion for visits summary rendering fallback text path (`No visits for current filter`) where practical
3. Continue flusher durability hardening for checkpoint-failure requeue behavior without redesigning pipeline

## Commands
- `go test ./...`
- `go test ./internal/flusher`
- `go test ./internal/tasks -run 'TestIntegration_Duplicate' -count=1`
- `go test ./internal/api`
- `go test ./internal/api -run 'Test(MapPageServedAtUIMap|StatusPageServedAtRoot)' -count=1`
- `go test ./internal/api -run 'TestPointsEndpoint_' -count=1`
- `go test ./internal/api -run 'TestGenerateVisitsEndpoint_|TestListVisitsEndpoint' -count=1`
- `go test ./internal/api -run 'Test(ListVisitsEndpoint|ListVisitsEndpoint_InvalidParams|GenerateVisitsEndpoint_)' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_ListPoints_WithFiltersAndAscendingOrder' -count=1`
- `go test ./internal/store -run 'TestVisitDetection_' -count=1`
- `go test ./internal/store -run 'TestListVisits_FilterByTimeRange' -count=1`
- `go test ./internal/api -run 'TestIngestOwnTracks_(NoPressure_DoesNotTriggerFlush|PointPressure_TriggersFlush|BytePressure_TriggersFlush)' -count=1`
- `go test ./internal/tasks -run TestRecoverFromSpool -count=1`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `docker build -t plexplore:dev .`
- `docker run --rm -p 18080:8080 -v $(pwd)/data:/data plexplore:dev`
- `docker compose up --build`
- `docker compose down`
- `make migrate`
- `sqlite3 ./data/plexplore.db ".schema visits"`
- `go run ./cmd/server`
- `curl -sS http://127.0.0.1:8080/health`
- `curl -sS http://127.0.0.1:8080/status`
- `curl -sS http://127.0.0.1:8080/api/v1/status`
- `curl -sS http://127.0.0.1:8080/api/v1/devices`
- `curl -sS http://127.0.0.1:8080/api/v1/devices/1`
- `curl -X POST http://127.0.0.1:8080/api/v1/devices/1/rotate-key -H "Content-Type: application/json" -d '{"api_key":"rotated-key"}'`
- `curl -sS "http://127.0.0.1:8080/api/v1/points/recent?limit=20"`
- `curl -sS "http://127.0.0.1:8080/api/v1/points?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z&limit=1000"`
- `curl -X POST "http://127.0.0.1:8080/api/v1/visits/generate?device_id=phone-main"`
- `curl -X POST "http://127.0.0.1:8080/api/v1/visits/generate?device_id=phone-main&from=2026-04-20T00:00:00Z&to=2026-04-22T23:59:59Z&min_dwell=20m&max_radius_m=40"`
- `curl -sS "http://127.0.0.1:8080/api/v1/visits?device_id=phone-main&limit=100"`
- `curl -sS "http://127.0.0.1:8080/api/v1/visits?device_id=phone-main&from=2026-04-20T00:00:00Z&to=2026-04-22T23:59:59Z&limit=100"`
- `curl -sS http://127.0.0.1:8080/ui/map`
- `curl -sS "http://127.0.0.1:8080/api/v1/exports/geojson?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z"`
- `curl -sS "http://127.0.0.1:8080/api/v1/exports/gpx?device_id=phone-main&from=2026-04-22T00:00:00Z&to=2026-04-23T00:00:00Z"`
- `sqlite3 ./data/plexplore.db "SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM devices; SELECT COUNT(*) FROM raw_points; SELECT COUNT(*) FROM points;"`

## Notes
Use segmented spool files, not a single giant file.
Tune behavior through env config (segment size, fsync mode/interval/threshold, buffer limits, flush interval/batch).
`sqlite3` CLI is installed in current environment and migrations are verified working.
Run `make migrate` before server run against a fresh database to ensure required tables exist.
.gitignore baseline is now present; runtime state (`data/`) and `node_modules/` are ignored to avoid accidental commits.
On transient SQLite failure, keep drained records by requeueing them to the RAM buffer front.
Current auth assumption is single-user deployment; device records are keyed by API key.
Device create/rotate responses return full `api_key` once; list/read responses only return masked `api_key_preview`.
Ingest handlers do not write directly to SQLite; they only parse -> spool -> RAM buffer.
Ingest now triggers best-effort async flush when `APP_FLUSH_TRIGGER_POINTS` or `APP_FLUSH_TRIGGER_BYTES` threshold is crossed.
Flusher now best-effort compacts committed spool segments immediately after successful checkpoint advancement.
RAM dedupe now emits checkpoint-only markers so duplicate spool sequences can advance checkpoint during normal runtime without creating duplicate SQLite rows.
Recent points debug endpoint is available at `GET /api/v1/points/recent` with optional `device_id` and `limit`.
Point history endpoint is available at `GET /api/v1/points` with optional `from`, `to`, `device_id`, and `limit` (ascending timestamp order).
Visits are generated on-demand through `POST /api/v1/visits/generate` with bounded device/date windows (default recent 14 days).
`GET /api/v1/visits` now supports optional `device_id`, `from`, `to`, and `limit` for compact list/filtering.
GeoJSON export is available at `GET /api/v1/exports/geojson` with optional `from`, `to`, and `device_id`.
GPX export is available at `GET /api/v1/exports/gpx` with optional `from`, `to`, and `device_id`.
Operational status endpoint is `GET /api/v1/status` (lightweight JSON, no Prometheus).
Alias route `GET /status` now points to the same JSON status handler.
Status endpoint now includes service health, buffer/spool/checkpoint state, spool/sqlite paths, and flush attempt/success/error fields.
Shutdown behavior now includes ingest draining (`503` for new ingest during shutdown), keep-alive disable on signal, separate server/flush shutdown windows, and synced spool/checkpoint close/write paths.
Minimal web UI is served directly by backend at `GET /` (also `GET /ui/status`).
Lightweight map page is available at `GET /ui/map` and uses Leaflet + `/api/v1/points`.
Map now also loads `/api/v1/visits` for centroid markers and shows a small visits summary table (start/end/duration/device).
Map UI now supports date-range and device filtering with a default recent 7-day range.
UI now tolerates `/api/v1/devices` failures and still shows health/status cards.
Typos such as `/ui/statu` now return 404 instead of showing the dashboard.
Overland ingestion currently expects `locations[].coordinates` (`[lon, lat]`) payload format.
Integration suite now covers OwnTracks/Overland ingest flow, duplicate-row protection, checkpoint-on-commit, startup recovery replay, and segment rollover replay.
Duplicate replay-pending lag issue is fixed: immediate duplicates now advance checkpoint during normal runtime.
README now documents clean shutdown vs forced kill vs crash/power-loss behavior and a manual shutdown verification procedure.
README now also documents startup recovery flow (checkpoint read -> replay `seq > checkpoint` -> flush -> checkpoint advance before HTTP listen).
Run `make migrate` to ensure latest migrations (`0002_devices_updated_at.sql`, `0003_visits.sql`) are applied.
README now includes practical OwnTracks/Overland setup and troubleshooting for `400/401/500` ingest responses.
Minimal web UI now also shows recent points preview from `/api/v1/points/recent?limit=10`.
Raspberry Pi deployment templates now exist under `deploy/systemd/` with installer `scripts/install_systemd.sh`.
Docker setup now exists with `Dockerfile`, `.dockerignore`, `compose.yaml`, and `scripts/docker-entrypoint.sh` (runs migrate then server).
Task sequence in `codex_tasks.md` is complete through Task 7.

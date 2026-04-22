# Next Steps

## Current milestone
Stabilize ingest durability + post-migration operations behavior

## Next 3 tasks
1. Add explicit size-based flush trigger policy from buffer pressure stats in ingest path
2. Run spool compaction after successful checkpoint advancement in flusher commit workflow
3. Add end-to-end integration coverage for migrated schema + device creation + authenticated ingest + persistence checks

## Commands
- `go test ./...`
- `make migrate`
- `go run ./cmd/server`
- `curl -sS http://127.0.0.1:8080/health`
- `curl -sS http://127.0.0.1:8080/api/v1/status`
- `curl -sS http://127.0.0.1:8080/api/v1/devices`
- `sqlite3 ./data/plexplore.db "SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM devices; SELECT COUNT(*) FROM raw_points; SELECT COUNT(*) FROM points;"`

## Notes
Use segmented spool files, not a single giant file.
Tune behavior through env config (segment size, fsync mode/interval/threshold, buffer limits, flush interval/batch).
`sqlite3` CLI is installed in current environment and migrations are verified working.
Run `make migrate` before server run against a fresh database to ensure required tables exist.
.gitignore baseline is now present; runtime state (`data/`) and `node_modules/` are ignored to avoid accidental commits.
On transient SQLite failure, keep drained records by requeueing them to the RAM buffer front.
Current auth assumption is single-user deployment; device records are keyed by API key.
Ingest handlers do not write directly to SQLite; they only parse -> spool -> RAM buffer.
Operational status endpoint is `GET /api/v1/status` (lightweight JSON, no Prometheus).
Minimal web UI is served directly by backend at `GET /` (also `GET /ui/status`).
UI now tolerates `/api/v1/devices` failures and still shows health/status cards.
Typos such as `/ui/statu` now return 404 instead of showing the dashboard.
Overland ingestion currently expects `locations[].coordinates` (`[lon, lat]`) payload format.

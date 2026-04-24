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

Server defaults to `127.0.0.1:8080`.
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

Session auth and scoping:
- these device management routes require a signed-in user session
- list/read only return devices owned by the current signed-in user
- rotate key is denied for non-owner devices
- create associates the device to the current signed-in user by default
- admin users may create devices for another user by supplying `user_id` in create request

Device API key hygiene:
- device API keys are stored hashed at rest (`api_key_hash`), not plaintext
- create and rotate responses include server-generated full `api_key` once
- list/read responses return only `api_key_preview` (masked)
- save the returned key at creation/rotation time; it is not recoverable later

Example workflow:

```bash
# 1) Create device (server-generated full api_key returned once)
curl -X POST http://localhost:8080/api/v1/devices \
  -H "Content-Type: application/json" \
  -d '{"name":"phone-main","source_type":"owntracks"}'

# 2) List devices (masked api_key_preview only)
curl -sS http://localhost:8080/api/v1/devices

# 3) Read one device (masked api_key_preview only)
curl -sS http://localhost:8080/api/v1/devices/1

# 4) Rotate API key (new server-generated full api_key returned once)
curl -X POST http://localhost:8080/api/v1/devices/1/rotate-key \
  -H "Content-Type: application/json" \
  -d '{}'
```

Assumption for this phase: single-user deployment.
If `user_id` is omitted, device creation defaults to user `1` (`default` user).

API key auth helper is available in `internal/api/auth.go` and is intended to be
applied to ingest endpoints as they are added.

## Multi-User Auth Foundation (In Progress)

Account-auth schema foundations are now present in SQLite (admin-created users,
no public signup flow yet):
- `users.email` (unique for non-empty values)
- `users.password_hash`
- `users.is_admin`
- `users.created_at`
- `users.updated_at`

Store-layer methods now available in `internal/store/users.go`:
- `CreateUser(...)`
- `GetUserByEmail(...)`
- `GetUserByID(...)`
- `ListUsers(...)`

Password helper functions are now available in `internal/api/password.go`:
- `HashPassword(plain string) (string, error)`
- `VerifyPassword(hash, plain string) bool`

Helpers reject empty passwords and use bcrypt hashes.

This phase adds schema/store building blocks only. Login/session/admin
management endpoints are added in later tasks.

## Admin Bootstrap (No Public Signup)

Public self-signup is not enabled. Bootstrap admin creation is done explicitly
via CLI mode on the migrate command:

```bash
go run ./cmd/migrate --create-admin \
  --email admin@example.com \
  --password 'testpass'
```

Optional flags:
- `--db` (default from `APP_SQLITE_PATH` or `./data/plexplore.db`)
- `--migrations` (default from `APP_MIGRATIONS_DIR` or `./migrations`)
- `--is-admin` (must be `true` for this bootstrap mode)

Behavior:
- runs migrations first
- prevents duplicate admin creation for the same email
- stores password as bcrypt hash (not plaintext)

## Session Foundation (In Progress)

Session storage and loading primitives are now available:
- SQLite table: `sessions` (token, user_id, expires_at, created_at)
- Store methods in `internal/store/sessions.go`:
- `CreateSession(userID)`
- `GetSession(token)` (expired sessions are treated as missing)
- `DeleteSession(token)`
- Middleware helper in `internal/api/session_auth.go`:
- `LoadCurrentUserFromSession(...)`
- `CurrentUserFromContext(...)`
- `RequireUserSessionAuth(...)` (JSON 401 for anonymous)
- `RequireUserSessionAuthHTML(...)` (redirects anonymous to `/login`)

Device API key auth for ingest remains separate and unchanged.

## Login / Logout (Admin-Created Users)

Endpoints:
- `GET /login`
- `POST /login`
- `POST /logout`

Behavior:
- no public signup
- login checks email + password hash and issues HttpOnly session cookie (`plexplore_session`)
- login/logout now require CSRF token validation (`plexplore_csrf` cookie + request token)
- logout deletes session and expires cookie
- protected HTML UI routes (`/`, `/ui/status`, `/ui/map`) require session and redirect to `/login` when anonymous

Example login request:

```bash
curl -X POST http://127.0.0.1:8080/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data "email=admin@example.com&password=testpass" -i
```

## Admin User Management API

Admin-only endpoints (session auth required):
- `GET /api/v1/users`
- `POST /api/v1/users`

Behavior:
- unauthenticated requests receive `401`
- non-admin authenticated requests receive `403`
- admin `POST /api/v1/users` requires CSRF token (`X-CSRF-Token` header matching `plexplore_csrf` cookie)
- responses do not expose `password_hash`

Example create request (admin session cookie required):

```bash
curl -X POST http://127.0.0.1:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Cookie: plexplore_session=<admin-session-token>" \
  -d '{"email":"user2@example.com","password":"user2pass","is_admin":false}'
```

## Ingestion Endpoints

- `POST /api/v1/owntracks`
- `POST /api/v1/overland/batches`

Both endpoints require device API key auth via `X-API-Key` (or
`Authorization: Bearer <api_key>`). Request handling flow is:
parse payload -> canonical points -> ensure ingest hash -> append spool ->
enqueue RAM buffer. Handlers do not write directly to SQLite.
In multi-user mode, ingest remains API-key based (no browser session required),
and persisted rows are attributed to the owning user/device resolved from that
API key.

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
  server-side issue (commonly migration/schema mismatch or SQLite/runtime failure); run `make migrate`, check authenticated `/api/v1/status` (or public-safe `/status`), and inspect server logs

## Operational Status

- `GET /health` is public and minimal (`{"status":"ok","service":"plexplore"}`).
- `GET /status` is public-safe and minimal (`service_health`, `service` only).
- `GET /api/v1/status` returns detailed operational data and requires authenticated user session.

Example:

```bash
# Public-safe status
curl -sS http://localhost:8080/status

# Detailed status (authenticated session required)
curl -sS -H "Cookie: plexplore_session=<session-token>" http://localhost:8080/api/v1/status
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
- Public `/status`: service health and service name only.
- Authenticated `/api/v1/status`: buffer points/bytes, oldest buffered age, spool/checkpoint state, flush timing/error, and configured spool/sqlite paths.

## Recent Points (Debug)

- `GET /api/v1/points/recent` returns compact recent stored points from SQLite.
- requires signed-in user session
- results are scoped to current user's devices only
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
- requires signed-in user session
- results are scoped to current user's devices only
- user scoping is enforced by persisted ownership IDs, so same device names across users remain isolated
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
- endpoints require signed-in user session
- generate accepts only devices owned by current user
- requires `device_id`
- supports bounded window with optional `from` / `to` RFC3339 params
- if `from` / `to` are omitted, generation defaults to a recent 14-day window
- generated visits can be listed via `GET /api/v1/visits` with optional
  `device_id`, `from`, `to`, and `limit` filters
- visit list results are scoped to current user's devices only
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
- requires signed-in user session
- exports only include current user's devices/points
- user scoping is enforced by persisted ownership IDs, even when device names overlap across users
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
- requires signed-in user session
- exports only include current user's devices/points
- user scoping is enforced by persisted ownership IDs, even when device names overlap across users
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
  -d '{"name":"phone-main","source_type":"owntracks"}'

# copy returned api_key from previous response and use it below
curl -X POST http://localhost:8080/api/v1/owntracks \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <returned-device-api-key>" \
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
- dark mode toggle (sun/moon) with localStorage persistence and system-preference fallback
- signed-in user email indicator and logout control in the top bar (session-aware UI)
- admin-only user management page at `GET /ui/admin/users` (shown as "Users" in UI) for listing users and creating admin-created accounts
- logout actions in UI pages include CSRF hidden token fields

Map page notes:
- uses self-hosted Leaflet assets served by Plexplore at:
- `/ui/assets/leaflet/leaflet.css`
- `/ui/assets/leaflet/leaflet.js`
- `/ui/assets/leaflet/images/*` (marker icons/shadow)
- map tiles still come from OpenStreetMap (`https://{s}.tile.openstreetmap.org/...`)
- fetches track points from `GET /api/v1/points`
- renders an ordered track polyline
- renders lightweight point markers for smaller result sets
- renders lightweight visit centroid markers from `/api/v1/visits` with popup details (including `place_label` when available)
- includes a small visits summary table (start, end, duration, device) below the map
- supports filtering by device and date range (`from`/`to` day inputs)
- defaults to a recent 7-day range when no date filters are set
- includes the same dark mode toggle with saved preference behavior

Users page notes:
- route: `GET /ui/admin/users` (admin session required)
- lists users via `GET /api/v1/users`
- creates users via `POST /api/v1/users`
- creates users with CSRF header (`X-CSRF-Token`) derived from current UI session
- no public self-signup is introduced
- supports the same dark mode toggle behavior used on status/map pages

## Security Hardening Notes

Baseline browser security headers:
- `Content-Security-Policy` on HTML pages
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: geolocation=(), camera=(), microphone=()`

CSP notes:
- current UI pages use inline `<style>` and `<script>`, so CSP currently allows `'unsafe-inline'` for styles/scripts to avoid breaking existing lightweight pages
- CSP still restricts sources to self for most resource classes, with explicit image allowance for OpenStreetMap tile hosts and `data:` marker/icon use
- this is an intentional transitional compromise until inline code/styles are moved to external static files

Route access model:
- public routes:
- `GET /health`
- `GET /status` (public-safe operational status)
- `GET /login`
- `POST /login`
- `GET /ui/assets/...` (static UI assets)
- authenticated user routes:
- `GET /` and `GET /ui/status`
- `GET /ui/map`
- `POST /logout`
- `GET /api/v1/status`
- `GET/POST /api/v1/devices...`
- `GET /api/v1/points...`
- `GET /api/v1/exports/...`
- `GET /api/v1/visits`
- `POST /api/v1/visits/generate`
- admin-only routes:
- `GET /ui/admin/users`
- `GET /api/v1/users`
- `POST /api/v1/users`

Runtime route registration now avoids unauthenticated fallback wiring for protected routes. Test-only fallback route wiring is isolated to internal API test helpers.
Shared protected route helper functions now fail closed (panic on missing required auth dependencies) so alternate/future entrypoints cannot silently downgrade to unauthenticated registration.

- Session cookie:
- name: `plexplore_session`
- attributes: `HttpOnly`, `SameSite=Lax`, path `/`, server-side expiration, `Secure` based on cookie security mode and request/proxy context
- CSRF cookie:
- name: `plexplore_csrf`
- attributes: `SameSite=Lax`, path `/`, `Secure` based on cookie security mode and request/proxy context (readable by UI JS for lightweight fetch protection)
- CSRF validation is enforced on:
- `POST /login`
- `POST /logout`
- `POST /api/v1/users`

Device API key storage:
- device ingest credentials are verified by hashing the presented key and matching `devices.api_key_hash`
- plaintext device keys are not stored in SQLite after migration/backfill
- list/read device endpoints expose only `api_key_preview`
- create/rotate endpoints return full key once for operator capture

Rate limiting:
- lightweight in-process fixed-window limiter (no Redis/external service)
- rate-limit key is client IP
- `X-Forwarded-For` is considered only when `APP_TRUST_PROXY_HEADERS=true`
- protected routes:
- `POST /login` (strict)
- `GET /api/v1/users` and `POST /api/v1/users` (admin-sensitive)
- `POST /api/v1/devices` and `POST /api/v1/devices/{id}/rotate-key` (admin-sensitive write protection)
- limited responses return `429` and `Retry-After`

Cookie/proxy knobs:
- `APP_DEPLOYMENT_MODE=development|production` (default `development`)
- `APP_COOKIE_SECURE_MODE=auto|always|never` (default `auto` in development, `always` in production)
- `APP_ALLOW_INSECURE_HTTP=true|false` (default `false`; required to explicitly allow insecure local HTTP mode with `APP_COOKIE_SECURE_MODE=never`)
- `APP_TRUST_PROXY_HEADERS=true|false` (default `false`)
- `APP_EXPECT_TLS_TERMINATION=true|false` (default `false` in development, `true` in production; startup warning aid)

Deployment guidance:
- Local development (HTTP):
- set `APP_DEPLOYMENT_MODE=development`
- keep default bind `127.0.0.1:8080`
- use `APP_COOKIE_SECURE_MODE=auto` (or explicitly `never` when needed)
- Production behind HTTPS reverse proxy:
- set `APP_DEPLOYMENT_MODE=production`
- prefer app bind to loopback/internal interface only
- keep `APP_COOKIE_SECURE_MODE=always` (recommended) or `auto` + `APP_TRUST_PROXY_HEADERS=true`
- set `APP_EXPECT_TLS_TERMINATION=true`
- do not enable proxy header trust unless traffic is actually coming from your trusted reverse proxy
- Production TLS-backed cookies:
- `APP_COOKIE_SECURE_MODE=always` is the safest default and does not rely on forwarded headers.

### Reverse Proxy TLS + HSTS (Production)

In production, TLS should terminate at the reverse proxy. HSTS should be set at
that reverse proxy layer, not in the app.

- Plexplore app:
- listen on localhost/private network only
- keep `APP_DEPLOYMENT_MODE=production`
- keep `APP_COOKIE_SECURE_MODE=always`
- keep `APP_EXPECT_TLS_TERMINATION=true`
- keep `APP_TRUST_PROXY_HEADERS=false` unless you explicitly need trusted forwarded proto behavior
- Reverse proxy:
- terminate HTTPS
- forward traffic to Plexplore over localhost/private network
- set `Strict-Transport-Security` header
- set `X-Forwarded-Proto https` only if you intentionally enable proxy header trust in app config

Conservative HSTS example value:

```text
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

Do not enable HSTS for local HTTP development.
Do not add `preload` by default. Use preload only if you understand the impact
and every subdomain is permanently HTTPS-only.

Minimal Caddy example:

```caddyfile
plexplore.example.com {
	encode zstd gzip
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"

	reverse_proxy 127.0.0.1:8080 {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-For {remote_host}
	}
}
```

Minimal nginx example:

```nginx
server {
    listen 443 ssl http2;
    server_name plexplore.example.com;

    # TLS config omitted for brevity (ssl_certificate, ssl_certificate_key, etc.)

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

The service logs a startup warning for risky combinations (for example public bind with non-`always` cookie mode, or expected TLS termination without trusted proxy headers).
The server now fails fast on unsafe cookie/runtime combinations:
- `APP_COOKIE_SECURE_MODE=never` requires `APP_ALLOW_INSECURE_HTTP=true`
- `APP_DEPLOYMENT_MODE=production` requires `APP_COOKIE_SECURE_MODE=always`
- `APP_DEPLOYMENT_MODE=production` rejects `APP_ALLOW_INSECURE_HTTP=true`

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
docker build -t plexplore:latest .
```

Production-oriented container run (behind HTTPS reverse proxy):

```bash
docker run --rm \
  -p 127.0.0.1:8080:8080 \
  -v "$(pwd)/data:/data" \
  -e APP_DEPLOYMENT_MODE=production \
  -e APP_HTTP_LISTEN_ADDR=0.0.0.0:8080 \
  -e APP_COOKIE_SECURE_MODE=always \
  -e APP_ALLOW_INSECURE_HTTP=false \
  -e APP_TRUST_PROXY_HEADERS=false \
  -e APP_EXPECT_TLS_TERMINATION=true \
  -e APP_SQLITE_PATH=/data/plexplore.db \
  -e APP_SPOOL_DIR=/data/spool \
  plexplore:latest
```

Local insecure HTTP development (explicit opt-in):

```bash
docker run --rm \
  -p 127.0.0.1:8080:8080 \
  -v "$(pwd)/data:/data" \
  -e APP_DEPLOYMENT_MODE=development \
  -e APP_HTTP_LISTEN_ADDR=0.0.0.0:8080 \
  -e APP_COOKIE_SECURE_MODE=never \
  -e APP_ALLOW_INSECURE_HTTP=true \
  -e APP_TRUST_PROXY_HEADERS=false \
  -e APP_EXPECT_TLS_TERMINATION=false \
  -e APP_SQLITE_PATH=/data/plexplore.db \
  -e APP_SPOOL_DIR=/data/spool \
  plexplore:latest
```

Local development note:
- use HTTP without HSTS
- use explicit insecure mode only when needed (`APP_COOKIE_SECURE_MODE=never` + `APP_ALLOW_INSECURE_HTTP=true`)

The container entrypoint runs migrations first, then starts the server.
The image defaults are production-oriented (`APP_DEPLOYMENT_MODE=production`, secure cookies enabled, insecure HTTP disabled).

Compose:

```bash
docker compose up --build -d
docker compose down
```

Environment variables commonly used in containers:
- `APP_DEPLOYMENT_MODE` (default in image: `production`)
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
- `APP_COOKIE_SECURE_MODE`
- `APP_ALLOW_INSECURE_HTTP`
- `APP_TRUST_PROXY_HEADERS`
- `APP_EXPECT_TLS_TERMINATION`
- `APP_RATE_LIMIT_ENABLED`
- `APP_RATE_LIMIT_LOGIN_MAX_REQUESTS`
- `APP_RATE_LIMIT_LOGIN_WINDOW`
- `APP_RATE_LIMIT_ADMIN_MAX_REQUESTS`
- `APP_RATE_LIMIT_ADMIN_WINDOW`
- `APP_SPOOL_FSYNC_MODE`
- `APP_SPOOL_FSYNC_INTERVAL`
- `APP_SPOOL_FSYNC_BYTE_THRESHOLD`

Raspberry Pi Zero 2 W caveats:
- Prefer running on local Pi storage or a reliable SSD-backed USB volume; avoid unstable network mounts for `/data`.
- Use conservative defaults (`balanced` fsync mode, modest buffer sizes) to reduce RAM and SD-card wear.
- For best compatibility, build on the target Pi (or use `docker buildx` for the exact target architecture/variant).
- Keep only one service instance writing to a given `/data` volume.

## Environment Variables

- `APP_DEPLOYMENT_MODE` (default: `development`): deployment profile (`development` or `production`) used for safer cookie/TLS defaults.
- `APP_HTTP_LISTEN_ADDR` (default: `127.0.0.1:8080`): HTTP bind address.
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
- `APP_COOKIE_SECURE_MODE` (default: `auto` in development, `always` in production): cookie `Secure` policy (`auto`, `always`, `never`).
- `APP_ALLOW_INSECURE_HTTP` (default: `false`): explicit opt-in for insecure local HTTP mode; required when `APP_COOKIE_SECURE_MODE=never`.
- `APP_TRUST_PROXY_HEADERS` (default: `false`): allow trusted `X-Forwarded-Proto` to influence cookie `Secure` behavior.
- `APP_EXPECT_TLS_TERMINATION` (default: `false` in development, `true` in production): deployment hint used for startup warnings when proxy/TLS settings look inconsistent.
- `APP_RATE_LIMIT_ENABLED` (default: `true`): enable auth/admin in-process route limiting.
- `APP_RATE_LIMIT_LOGIN_MAX_REQUESTS` (default: `10`): allowed `POST /login` attempts per window per client IP.
- `APP_RATE_LIMIT_LOGIN_WINDOW` (default: `1m`): login limiter window duration.
- `APP_RATE_LIMIT_ADMIN_MAX_REQUESTS` (default: `30`): allowed admin-sensitive requests per window per client IP.
- `APP_RATE_LIMIT_ADMIN_WINDOW` (default: `1m`): admin-sensitive limiter window duration.
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

Device key hash migration:
- migration `0007_device_api_key_hash.sql` adds `devices.api_key_hash` and `devices.api_key_preview`
- on store open, legacy plaintext device keys are backfilled to hash + preview and `devices.api_key` is replaced with a non-secret sentinel value
- after backfill, ingest auth relies on `api_key_hash` lookups only

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

- Device API returns server-generated full `api_key` only on create/rotate responses.
  List/read responses return masked `api_key_preview`.

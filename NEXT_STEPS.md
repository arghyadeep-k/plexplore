# Next Steps

## Current milestone
Authenticated browser admin smoke workflow is now covered end-to-end (login/session/CSRF/device rotate/visit generate)

## Next 3 tasks
1. Add checkpoint retry pressure fields to `/api/v1/status` (failure count and last checkpoint error time)
2. Add store-backed integration test for scheduler telemetry + watermark summary values under mixed device update/no-op runs
3. Run manual runtime CSP validation for map tile modes (`none`, `osm`, `custom`) with live `curl -I` and browser map load checks

## Commands
- `go test ./...`
- `go test ./internal/tasks -run TestVisitScheduler -count=1`
- `go test ./internal/flusher`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `go test ./internal/api -run 'Test(GenerateVisitsEndpoint_SameDeviceNameAcrossUsers_IsScopedByDeviceRowID|ListVisitsEndpoint_SameDeviceNameAcrossUsers_IsScopedBySessionUser)' -count=1`
- `go test ./internal/store -run 'TestVisitIsolation_SameDeviceNameAcrossUsers' -count=1`
- `go test ./internal/api`
- `go test ./internal/store`
- `gofmt -w internal/api/*.go internal/tasks/*.go cmd/migrate/*.go`
- `timeout 6s go run ./cmd/server`
- `go test ./internal/api -run 'Test(PointsEndpoint_LimitCapApplied|PointsEndpoint_PaginationCursor|GeoJSONExport_ValidStructure|GPXExport_ValidStructureAndContent|ExportEndpoints_LimitCapApplied)' -count=1`
- `go test ./internal/api -run 'Test(MapPageServedAtUIMap|UIAssets_MapScriptContainsEscapedPopupFields|PointsEndpoint_SimplifyReducesLargeResponse|PointsEndpoint_PaginationCursor)' -count=1`
- `bash -n scripts/backup.sh scripts/restore.sh`
- `scripts/backup.sh --sqlite-path ./data/plexplore.db --spool-dir ./data/spool --output-dir ./backups`
- `scripts/restore.sh --archive ./backups/plexplore-backup-YYYYMMDD-HHMMSS.tar.gz --sqlite-path ./data/plexplore.db --spool-dir ./data/spool`
- `curl -I https://your-domain.example`
- `curl -I https://your-domain.example | rg -i 'strict-transport-security'`
- `curl -I http://127.0.0.1:8080 | rg -i 'strict-transport-security'`
- `curl -I http://127.0.0.1:8080/`
- `go test ./internal/config -count=1`
- `go test ./cmd/server -count=1`
- `go test ./internal/api -run 'TestRuntimeRouter_' -count=1`
- `go test ./internal/api -run 'TestRouteHelpers_FailClosed_WhenAuthDepsMissing' -count=1`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|UIAssets_LeafletServedLocally|UIAssets_LeafletIconServedLocally|HealthEndpoint_RemainsPublic)' -count=1`
- `curl -sS -D - -o /dev/null http://127.0.0.1:8080/ui/map`
- `curl -sS http://127.0.0.1:8080/ui/map | rg 'ui/assets/leaflet|unpkg.com/leaflet'`
- `curl -sS -D - -o /dev/null http://127.0.0.1:8080/ui/assets/leaflet/leaflet.css`
- `go test ./internal/api -run 'TestDevicesAPI_|TestRequireDeviceAPIKeyAuth|TestDevicesAPI_AdminSensitiveWritesRateLimited' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_(MultiUserAuthorizationIsolation|DeviceAPIKeyStoredHashedAtRest)' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateAndLookupDeviceByAPIKey|GetDeviceByID_AndRotateAPIKey|BackfillPlaintextDeviceKeyToHash)' -count=1`
- `go test ./internal/config -count=1`
- `go test ./internal/api -run 'Test(LoginSuccess_SetsSecureSessionCookie_WhenAlwaysMode|TestLoginSuccessSetsSessionCookie|TestLoginPageCSRFCookie_UsesTrustedForwardedProtoWhenEnabled|TestLoginPageCSRFCookie_IgnoresForwardedProtoWhenUntrusted)' -count=1`
- `go test ./internal/api -run 'Test(LoginRateLimit_|FixedWindowLimiter_|RateLimitKeyForRequest_|RateLimit_NonSensitiveHealthRouteUnaffected|Users_AdminRoutesRateLimited|DevicesAPI_AdminSensitiveWritesRateLimited)' -count=1`
- `go test ./internal/api -run 'TestStatusEndpoint_|TestHealthEndpoint_RemainsPublic|TestStatusPageServedAtRoot|TestUIRoutesRequireSession_WhenSessionDepsProvided|TestUIRoutesAllowSession_WhenValidSessionCookiePresent' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateAndLookupDeviceByAPIKey|GetDeviceByID_AndRotateAPIKey|BackfillPlaintextDeviceKeyToHash|GetDeviceByAPIKey_NotFound|ListDevices)' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_|TestRequireDeviceAPIKeyAuth' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_(MultiUserAuthorizationIsolation|DeviceAPIKeyStoredHashedAtRest)' -count=1`
- `go test ./internal/api -run 'Test(Login|CookieSecurityPolicy|LoadCurrentUserFromSession|RequireUserSessionAuth)' -count=1`
- `go test ./cmd/server -count=1`
- `go test ./internal/flusher`
- `go test ./internal/tasks -run 'TestIntegration_Duplicate' -count=1`
- `go test ./internal/api`
- `go test ./internal/api -run 'Test(MapPageServedAtUIMap|StatusPageServedAtRoot)' -count=1`
- `go test ./internal/api -run 'TestPointsEndpoint_' -count=1`
- `go test ./internal/api -run 'TestGenerateVisitsEndpoint_|TestListVisitsEndpoint' -count=1`
- `go test ./internal/api -run 'Test(ListVisitsEndpoint|ListVisitsEndpoint_InvalidParams|GenerateVisitsEndpoint_)' -count=1`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|StatusPage_DoesNotMatchTypoPath)' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_(DeviceAPIKeyIngestPersistsUnderCorrectOwnerAndDevice|InvalidDeviceAPIKeyRejected_NoDataPersisted)' -count=1`
- `go test ./internal/tasks -run 'TestIntegration_MultiUserAuthorizationIsolation' -count=1`
- `go test ./internal/api -run 'Test(AdminUsersPageServedForAdminSession|AdminUsersPageDeniedForNonAdminSession)' -count=1`
- `go test ./internal/api -run 'Test(HashAndVerifyPassword|VerifyPassword_WrongPasswordFails|HashPassword_EmptyRejected)' -count=1`
- `go test ./cmd/migrate -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateGetDeleteSession|GetSession_Expired)' -count=1`
- `go test ./internal/api -run 'TestLoadCurrentUserFromSession_' -count=1`
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LogoutClearsSession)' -count=1`
- `go test ./internal/api -run 'Test(LoginPageServed|LoginSuccessSetsSessionCookie|LoginInvalidCredentials|LogoutClearsSession|LoginRejectsMissingCSRFToken|Users_)' -count=1`
- `go test ./internal/api -run 'Test(LoginInvalidCredentials|LoginInvalidCredentials_JSONStillReturnsJSON)' -count=1`
- `go test ./internal/api -run 'Test(LoginSuccessSetsSessionCookie|LoginSuccess_WithNextParamRedirectsToRequestedPage|RequireUserSessionAuthHTML_RedirectWhenAnonymous|UIRoutesRequireSession_WhenSessionDepsProvided)' -count=1`
- `go test ./internal/api -run 'Test(StatusPageServedAtRoot|MapPageServedAtUIMap|StatusPage_AdminLinkStillRendersForAdminSession)' -count=1`
- `go test ./internal/api -run 'Test(AdminUsersPageServedForAdminSession|StatusPage_AdminLinkStillRendersForAdminSession|MapPage_AdminLinkLabelIsUsersForAdminSession)' -count=1`
- `go test ./internal/api -run 'Test(LoadCurrentUserFromSession_|RequireUserSessionAuth_|RequireUserSessionAuthHTML_|UIRoutesRequireSession_|UIRoutesAllowSession_)' -count=1`
- `go test ./internal/api -run 'TestUsers_' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_(UserSeesOnlyOwnDevices_WhenSessionAuthEnabled|UserCannotFetchAnotherUsersDevice_WhenSessionAuthEnabled|RotateKeyDeniedForNonOwner_WhenSessionAuthEnabled|CreateUsesCurrentSessionUser_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestDevicesAPI_(CreateForAnotherUserDeniedForNonAdmin_WhenSessionAuthEnabled|AdminCanCreateForSpecificUser_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestRecentPointsEndpoint_(UserSeesOnlyOwnPoints_WhenSessionAuthEnabled|DeviceFilterTrickBlocked_WhenSessionAuthEnabled|UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'TestPointsEndpoint_(UserSeesOnlyOwnPoints_WhenSessionAuthEnabled|DeviceFilterTrickBlocked_WhenSessionAuthEnabled|UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'Test(GeoJSONExport_UserSeesOnlyOwnPoints_WhenSessionAuthEnabled|GeoJSONExport_UnauthenticatedDenied_WhenSessionAuthEnabled|GPXExport_DeviceFilterTrickBlocked_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/api -run 'Test(ListVisitsEndpoint_UserSeesOnlyOwnVisits_WhenSessionAuthEnabled|GenerateVisitsEndpoint_CrossUserDeviceDenied_WhenSessionAuthEnabled|VisitsEndpoints_UnauthenticatedDenied_WhenSessionAuthEnabled)' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_ListPoints_WithFiltersAndAscendingOrder' -count=1`
- `go test ./internal/store -run 'TestVisitDetection_' -count=1`
- `go test ./internal/store -run 'TestListVisits_FilterByTimeRange' -count=1`
- `go test ./internal/store -run 'TestVisitPlaceCache_UpsertAndRead' -count=1`
- `go test ./internal/store -run 'TestSQLiteStore_(CreateAndGetUserByEmail|ListUsers|GetUserNotFound|UsersSchemaHasAuthFields)' -count=1`
- `go test ./internal/visits -count=1`
- `go test ./internal/api -run 'TestIngestOwnTracks_(NoPressure_DoesNotTriggerFlush|PointPressure_TriggersFlush|BytePressure_TriggersFlush)' -count=1`
- `go test ./internal/api -run 'TestListVisitsEndpoint_WithVisitLabelResolver' -count=1`
- `go test ./internal/tasks -run TestRecoverFromSpool -count=1`
- `go test ./internal/tasks -run TestIntegration -count=1`
- `docker build -t plexplore:dev .`
- `docker build -t plexplore:latest .`
- `docker run --rm -p 127.0.0.1:8080:8080 -v "$(pwd)/data:/data" plexplore:latest`
- `docker run --rm -p 127.0.0.1:8080:8080 -v "$(pwd)/data:/data" -e APP_DEPLOYMENT_MODE=development -e APP_COOKIE_SECURE_MODE=never -e APP_ALLOW_INSECURE_HTTP=true -e APP_EXPECT_TLS_TERMINATION=false plexplore:latest`
- `docker run --rm -p 18080:8080 -v $(pwd)/data:/data plexplore:dev`
- `docker compose up --build`
- `docker compose down`
- `make migrate`
- `APP_SQLITE_PATH=./data/task1fresh.db make migrate`
- `APP_SQLITE_PATH=./data/task3bootstrap.db make migrate`
- `sqlite3 ./data/plexplore.db ".schema visits"`
- `sqlite3 ./data/plexplore.db ".schema visit_place_cache"`
- `sqlite3 ./data/task1fresh.db ".schema users"`
- `go run ./cmd/server`
- `curl -sS http://127.0.0.1:8080/health`
- `curl -sS http://127.0.0.1:8080/status`
- `curl -sS http://127.0.0.1:8080/api/v1/status`
- `curl -sS http://127.0.0.1:8080/api/v1/devices`
- `curl -sS http://127.0.0.1:8080/api/v1/devices/1`
- `curl -X POST http://127.0.0.1:8080/api/v1/devices/1/rotate-key -H "Content-Type: application/json" -d '{}'`
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
Leaflet map assets are now self-hosted under `/ui/assets/leaflet/*`; CDN references were removed from map UI.
Baseline browser security headers are now applied, and CSP on HTML pages no longer allows `'unsafe-inline'`; UI CSS/JS are served from local static assets.
Runtime routing now does not register protected UI/API routes when auth dependencies are missing; legacy fallback wiring is test-only (`registerRoutesWithTestFallbacks`).
Shared protected route helpers now panic on missing required auth deps to enforce fail-closed registration in future entrypoints.
Route helper fallback audit re-verified: no remaining permissive runtime helper branches were found.
HSTS is now documented as reverse-proxy responsibility for production TLS deployments; local HTTP dev should not use HSTS.
In-app HSTS should only be added in the future if app-level direct HTTPS termination is implemented and explicitly enabled.
Insecure local HTTP mode now requires explicit `APP_ALLOW_INSECURE_HTTP=true` when `APP_COOKIE_SECURE_MODE=never`.
Production mode now fails fast unless `APP_COOKIE_SECURE_MODE=always`, and rejects `APP_ALLOW_INSECURE_HTTP=true`.
On transient SQLite failure, keep drained records by requeueing them to the RAM buffer front.
Current auth model is multi-user with admin-created accounts and per-user data isolation.
Device create/rotate responses return full `api_key` once; list/read responses only return masked `api_key_preview`.
Ingest handlers do not write directly to SQLite; they only parse -> spool -> RAM buffer.
Ingest now triggers best-effort async flush when `APP_FLUSH_TRIGGER_POINTS` or `APP_FLUSH_TRIGGER_BYTES` threshold is crossed.
Flusher now best-effort compacts committed spool segments immediately after successful checkpoint advancement.
RAM dedupe now emits checkpoint-only markers so duplicate spool sequences can advance checkpoint during normal runtime without creating duplicate SQLite rows.
Recent points debug endpoint is available at `GET /api/v1/points/recent` with optional `device_id` and `limit`.
Point history endpoint is available at `GET /api/v1/points` with optional `from`, `to`, `device_id`, and `limit` (ascending timestamp order).
Visits are generated on-demand through `POST /api/v1/visits/generate` with bounded device/date windows (default recent 14 days).
`GET /api/v1/visits` now supports optional `device_id`, `from`, `to`, and `limit` for compact list/filtering.
Optional reverse geocode label enrichment is available for visit centroids on `GET /api/v1/visits` when `APP_REVERSE_GEOCODE_ENABLED=true`.
Reverse geocode cache persists in SQLite table `visit_place_cache`; provider calls are capped per request by `APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST`.
Task 1 of multi-user auth milestone is complete: users auth columns + user store methods now exist (`CreateUser`, `GetUserByEmail`, `GetUserByID`, `ListUsers`).
Task 2 is complete: bcrypt password helpers are available (`HashPassword`, `VerifyPassword`) with empty-password validation.
Task 3 is complete: admin bootstrap now supported via `go run ./cmd/migrate --create-admin --email ... --password ...` (no public signup).
Task 4 is complete: SQLite-backed sessions and current-user session loader middleware primitives are in place.
Task 5 is complete: `/login` and `/logout` flows now issue/delete server-side session cookies for admin-created users.
Task 6 is complete: auth-required helper middleware exists, and UI routes now require session with anonymous redirect to `/login`.
Task 7 is complete: admin-only `GET/POST /api/v1/users` endpoints are implemented with role enforcement and no password_hash exposure.
Task 8 is complete: device routes are session-authenticated and scoped to current user ownership (list/read/rotate/create behavior adjusted).
Task 9 is complete: device ownership model finalized (user self-create by default, admin override via `user_id`).
Task 10 is complete: `/api/v1/points/recent` is session-authenticated and scoped to current user's devices.
Task 11 is complete: `/api/v1/points` history endpoint is now session-authenticated and scoped to current user's devices.
Task 12 is complete: `/api/v1/exports/geojson` and `/api/v1/exports/gpx` are session-authenticated and scoped to current user's devices.
Task 13 is complete: `/api/v1/visits` and `/api/v1/visits/generate` are session-authenticated and scoped to current user's device ownership.
Task 14 is complete: protected UI pages now render a signed-in user indicator and logout control.
Task 15 is complete: device API key ingest remains session-independent and now persists points under the correct owning user/device in multi-user mode.
Task 16 is complete: full multi-user authorization/isolation integration coverage now verifies login separation, per-user devices/points/exports, ingest ownership, and non-owner rotate denial.
Task 17 is complete: admin-only user management UI page is now available at `/ui/admin/users`.
Task 18 is complete: CSRF validation and cookie/session hardening are now enforced for login/logout/admin user creation.
Login UX hardening is complete: invalid browser logins now re-render `/login` with inline red error and preserved email (password not preserved).
Post-login browser redirect default is now `/ui/map` (with safe `next` precedence for protected-page redirects).
UI cross-navigation update is complete: status page has a Map link and map page has a Status link in the top navigation.
Users page UI refresh is complete: "Admin Users" labels were renamed to "Users" and the page now supports the shared dark mode toggle behavior.
GeoJSON export is available at `GET /api/v1/exports/geojson` with optional `from`, `to`, and `device_id`.
GPX export is available at `GET /api/v1/exports/gpx` with optional `from`, `to`, and `device_id`.
Operational status endpoint is `GET /api/v1/status` (lightweight JSON, no Prometheus).
Alias route `GET /status` now points to the same JSON status handler.
Status endpoint now includes service health, buffer/spool/checkpoint state, spool/sqlite paths, and flush attempt/success/error fields.
Shutdown behavior now includes ingest draining (`503` for new ingest during shutdown), keep-alive disable on signal, separate server/flush shutdown windows, and synced spool/checkpoint close/write paths.
Minimal web UI is served directly by backend at `GET /` (also `GET /ui/status`).
Lightweight map page is available at `GET /ui/map` and uses Leaflet + `/api/v1/points`.
Status and map pages now include a lightweight dark-mode toggle with localStorage persistence and system-preference fallback.
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
Run `make migrate` to ensure latest migrations include `0004_visit_place_cache.sql`.
README now includes practical OwnTracks/Overland setup and troubleshooting for `400/401/500` ingest responses.
Minimal web UI now also shows recent points preview from `/api/v1/points/recent?limit=10`.
Raspberry Pi deployment templates now exist under `deploy/systemd/` with installer `scripts/install_systemd.sh`.
Docker setup now exists with `Dockerfile`, `.dockerignore`, `compose.yaml`, and `scripts/docker-entrypoint.sh` (runs migrate then server).
Cookie/proxy TLS hardening adds:
- `APP_COOKIE_SECURE_MODE=auto|always|never` (default `auto`)
- `APP_TRUST_PROXY_HEADERS=true|false` (default `false`)
- `APP_EXPECT_TLS_TERMINATION=true|false` (default `false`)
Cookie `Secure` behavior now:
- direct TLS (`r.TLS`) => `Secure` in `auto`
- trusted `X-Forwarded-Proto=https` => `Secure` only when `APP_TRUST_PROXY_HEADERS=true`
- local HTTP dev can use `APP_COOKIE_SECURE_MODE=never` (or `auto` without TLS)
Startup logs now warn for risky cookie/proxy/public-bind combinations.
Device API key hardening adds:
- API keys are authenticated via `devices.api_key_hash` (SHA-256 of presented key).
- DB backfill on store open migrates legacy plaintext `devices.api_key` to hash + preview and replaces plaintext with non-secret sentinel.
- `POST /api/v1/devices` and `POST /api/v1/devices/{id}/rotate-key` now always generate server-side high-entropy keys and return full key once; supplied `api_key` fields are ignored; list/read remain masked.
Status hardening adds:
- `/health` stays public and minimal.
- `/status` is public-safe and excludes internal runtime metadata.
- `/api/v1/status` remains detailed but requires authenticated session when session auth is configured.
Rate limiting adds:
- `POST /login` protected by strict per-IP fixed-window limiter.
- `GET/POST /api/v1/users` and `POST /api/v1/devices`, `POST /api/v1/devices/{id}/rotate-key` protected by moderate per-IP limiter.
- `APP_TRUST_PROXY_HEADERS=true` enables trusted `X-Forwarded-For` use for limiter keys; otherwise direct remote address is used.
- New knobs: `APP_RATE_LIMIT_ENABLED`, `APP_RATE_LIMIT_LOGIN_MAX_REQUESTS`, `APP_RATE_LIMIT_LOGIN_WINDOW`, `APP_RATE_LIMIT_ADMIN_MAX_REQUESTS`, `APP_RATE_LIMIT_ADMIN_WINDOW`.
Production cookie/deployment hardening adds:
- `APP_DEPLOYMENT_MODE=development|production` controls safer defaults.
- Default bind is now `127.0.0.1:8080`.
- Development defaults: `APP_COOKIE_SECURE_MODE=auto`, `APP_EXPECT_TLS_TERMINATION=false`.
- Production defaults: `APP_COOKIE_SECURE_MODE=always`, `APP_EXPECT_TLS_TERMINATION=true`.
Task sequence in `codex_tasks.md` is complete through Task 7.
Current active sequence is the newer 18-task multi-user auth plan in `codex_tasks.md`; continue strictly in order from Task 2.
Continue strictly in order from Task 3 next.
Continue strictly in order from Task 4 next.
Continue strictly in order from Task 5 next.
Continue strictly in order from Task 7 next.
Continue strictly in order from Task 8 next.
Continue strictly in order from Task 10 next.
Continue strictly in order from Task 11 next.
Continue strictly in order from Task 9 next.
Continue strictly in order from Task 6 next.
UI CSP now removes `'unsafe-inline'`; UI pages serve local assets from `/ui/assets/app/*`.
Map tiles default to privacy mode via `APP_MAP_TILE_MODE=none`; external tiles require explicit `osm` or `custom` mode.
Migrator now executes with `sqlite3 -bail` and transaction-wrapped apply+record logic; known additive partial migrations (`0002`, `0005`, `0007`) recover safely.

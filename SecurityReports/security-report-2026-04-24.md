# Plexplore Security Report

- Report date: 2026-04-24
- Analyzer: Codex (GPT-5)
- Repository: `/mnt/d/Code/plexplore`
- Assessment type: Codebase security and production-readiness review
- Review mode: Static code review plus local test verification
- Verification command: `go test ./...`
- Assessment result: Production-ready with standard deployment hardening assumptions

## Scope

This review covered the current application code and deployment assets with emphasis on:

- Authentication and authorization boundaries
- Session and cookie security
- CSRF protection
- Device API key generation, storage, and exposure
- Protected route registration and fail-closed behavior
- Browser/UI hardening headers
- Production deployment defaults in Docker and systemd samples
- Public status and health exposure
- Rate limiting on sensitive routes

## Analyzer Assumptions

This conclusion assumes:

- TLS is terminated by a trusted reverse proxy
- HSTS is enforced at that reverse proxy, not in-app
- `APP_TRUST_PROXY_HEADERS` is enabled only when traffic actually comes through that trusted proxy
- Production deployments keep `APP_ALLOW_INSECURE_HTTP=false`
- Host filesystem permissions, SQLite file access, backup handling, and log access are managed appropriately

## What Was Verified

### Runtime security posture

- The main server validates runtime security configuration on startup and fails fast on insecure production combinations.
- Production mode requires secure cookie behavior.
- Explicit insecure HTTP mode requires an opt-in flag and is rejected in production mode.

Relevant files:

- [cmd/server/main.go](/mnt/d/Code/plexplore/cmd/server/main.go)
- [internal/config/config.go](/mnt/d/Code/plexplore/internal/config/config.go)

### Route registration and auth boundaries

- Shared runtime registration now behaves fail-closed in the active server path.
- Protected routes are only registered when the required auth/session dependencies are present.
- No active production route path was found that silently downgrades protected routes to unauthenticated access.
- Protected aliases such as `/` and `/ui/status` share the same auth guard path.
- Admin-only routes remain guarded by session plus admin middleware.

Relevant files:

- [internal/api/health.go](/mnt/d/Code/plexplore/internal/api/health.go)
- [internal/api/ui.go](/mnt/d/Code/plexplore/internal/api/ui.go)
- [internal/api/users.go](/mnt/d/Code/plexplore/internal/api/users.go)
- [internal/api/devices.go](/mnt/d/Code/plexplore/internal/api/devices.go)
- [internal/api/points.go](/mnt/d/Code/plexplore/internal/api/points.go)
- [internal/api/exports.go](/mnt/d/Code/plexplore/internal/api/exports.go)
- [internal/api/visits.go](/mnt/d/Code/plexplore/internal/api/visits.go)

### Session, cookie, and CSRF controls

- Session cookies are `HttpOnly`.
- Production configuration enforces secure cookie usage.
- CSRF tokens are issued and validated for browser-backed flows.
- Login redirection handling rejects external redirect targets.

Relevant files:

- [internal/api/login.go](/mnt/d/Code/plexplore/internal/api/login.go)
- [internal/api/csrf.go](/mnt/d/Code/plexplore/internal/api/csrf.go)
- [internal/api/cookie_security.go](/mnt/d/Code/plexplore/internal/api/cookie_security.go)

### API key handling

- Device API keys are server-generated only.
- Keys are generated with 32 random bytes and hex-encoded.
- Only hashed device keys are stored; plaintext values are not persisted.
- API responses expose the full API key only at creation/rotation time.
- Normal list/read responses expose preview values only.

Relevant files:

- [internal/api/devices.go](/mnt/d/Code/plexplore/internal/api/devices.go)
- [internal/store/devices.go](/mnt/d/Code/plexplore/internal/store/devices.go)
- [internal/store/device_keys.go](/mnt/d/Code/plexplore/internal/store/device_keys.go)

### Browser/UI hardening

- Local UI assets are bundled instead of loaded from third-party CDNs.
- CSP is set for HTML responses.
- Additional headers include frame denial, no-sniff, referrer policy, cross-origin opener policy, and permissions policy.

Relevant files:

- [internal/api/security_headers.go](/mnt/d/Code/plexplore/internal/api/security_headers.go)
- [internal/api/ui_assets.go](/mnt/d/Code/plexplore/internal/api/ui_assets.go)
- [internal/api/ui.go](/mnt/d/Code/plexplore/internal/api/ui.go)

### Deployment defaults

- Dockerfile now defaults to production mode.
- Dockerfile sets secure-cookie-oriented defaults and disables insecure HTTP mode.
- `compose.yaml` binds to localhost on the host side by default.
- The systemd sample is production-oriented and localhost-bound.

Relevant files:

- [Dockerfile](/mnt/d/Code/plexplore/Dockerfile)
- [compose.yaml](/mnt/d/Code/plexplore/compose.yaml)
- [deploy/systemd/plexplore.env.sample](/mnt/d/Code/plexplore/deploy/systemd/plexplore.env.sample)

### Test verification

The full Go test suite passed during this review.

Verified with:

```bash
go test ./...
```

## Findings

### No critical or high-severity issue found in the active production path

The current main runtime path does not show an obvious exploitable authentication, authorization, session, or secret-handling flaw.

### Low-severity residual concern: helper misuse risk

Some route helper files still contain internal fallback logic patterns, but the active `RegisterRoutesWithDependencies(...)` path no longer wires sensitive routes without their required auth/session dependencies. Based on the current code, this is a maintainability concern rather than an active exposure.

Impact:

- Low in the current shipped runtime
- Higher only if a future alternate entrypoint bypasses the main registration discipline

Recommendation:

- Keep route registration centralized
- Avoid introducing alternate runtime entrypoints without equivalent dependency checks
- Continue keeping permissive route setup restricted to test-only helpers

### Low-severity residual concern: HSTS not set in-app

The application does not set `Strict-Transport-Security` itself.

Impact:

- Low if TLS termination is handled by a reverse proxy that already sets HSTS
- Potentially relevant only if the app later terminates HTTPS directly

Recommendation:

- Keep HSTS at the reverse proxy for the current architecture
- Add in-app HSTS only if direct HTTPS termination is introduced later

## Production Readiness Conclusion

Based on the current code and deployment defaults, the application is production-ready provided the deployment follows the intended architecture:

- HTTPS terminated by a trusted reverse proxy
- Proxy sets HSTS
- Proxy/header trust configured correctly
- Host-level persistence, backup, permissions, and monitoring handled competently

I do not currently see an impending critical security risk or active vulnerability in the main shipped server path.

## Recommended Operational Controls

- Enforce HTTPS at the reverse proxy
- Set HSTS at the reverse proxy
- Restrict network exposure to the proxy boundary
- Limit access to SQLite files, spool directories, and backups
- Monitor auth failures and rate-limit events
- Rotate device API keys if compromise is suspected
- Keep dependencies and base images patched

## Summary

Final assessment: the current Plexplore codebase is production-ready from an application security perspective, with remaining concerns limited to normal operational hardening and low-severity maintenance safeguards rather than active architectural weaknesses.

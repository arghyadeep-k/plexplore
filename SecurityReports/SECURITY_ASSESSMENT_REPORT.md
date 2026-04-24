# Plexplore Security Assessment Report

**Analyzer:** opencode/minimax-m2.5-free  
**Date:** 2026-04-24  
**Target:** Plexplore - Self-hosted Location Tracker  
**Assessment Type:** Security Audit & Production Readiness

---

## Executive Summary

Plexplore is a Go-based location tracker service designed for lightweight self-hosted deployment (e.g., Raspberry Pi Zero 2W). The codebase demonstrates strong security practices suitable for production use within its intended scope.

**Verdict: CONDITIONALLY PRODUCTION READY**  
The service is suitable for single-instance self-hosted deployment. For multi-instance or cloud deployments, minor architectural changes are required.

---

## 1. Authentication & Session Management

### 1.1 Password Storage
- **Algorithm:** bcrypt with DefaultCost (10)
- **Status:** ✅ SECURE
- **Location:** `internal/api/password.go:18`
- **Notes:**
  - Uses `bcrypt.GenerateFromPassword` with cost factor 10
  - Input normalization (trimming) applied before hashing
  - Empty password validation implemented

### 1.2 Session Tokens
- **Generation:** 32-byte cryptographic random
- **Storage:** SQLite with expiration timestamps
- **Status:** ✅ SECURE
- **Location:** `internal/store/sessions.go:113`
- **Lifetime:** 7 days default (`defaultSessionTTL = 7 * 24 * time.Hour`)
- **Validation:** Expiration checked on each retrieval (line 88-91)

### 1.3 Device API Keys
- **Storage:** SHA256 hash (not plaintext)
- **Preview:** Masked display (first 4 + last 4 chars)
- **Rotation:** Supported with immediate invalidation
- **Status:** ✅ SECURE
- **Location:** `internal/store/device_keys.go`

---

## 2. CSRF Protection

### Implementation
- **Token Generation:** 16-byte random, hex-encoded
- **Delivery:** HTTP-only cookie
- **Validation:** Double-submit pattern (cookie + header/form)
- **Status:** ✅ SECURE
- **Location:** `internal/api/csrf.go`

### Notes
- CSRF cookie is NOT HttpOnly (intentional for SPA JavaScript access)
- Uses `SameSiteLaxMode` for browser compatibility
- Token persisted per-session, not per-request (trade-off for simplicity)

---

## 3. Rate Limiting

### Configuration
- **Type:** Fixed-window in-memory limiter
- **Scopes:**
  - Login: 10 requests/minute
  - Admin routes: 30 requests/minute
- **Status:** ⚠️ REQUIRES ATTENTION
- **Location:** `internal/api/rate_limit.go`

### Limitations
- Not shared across multiple instances
- Single-process only (Pi Zero use case mitigates this)
- Cleanup threshold: 2048 entries

---

## 4. Security Headers

### Headers Applied (HTML responses)
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: geolocation=(), camera=(), microphone=()`
- `Content-Security-Policy: default-src 'self'; ...`

### Status
- ✅ HEADERS CONFIGURED
- Location: `internal/api/security_headers.go`

### Notes
- HSTS intentionally omitted (delegated to reverse proxy)
- CSP includes `'unsafe-inline'` for scripts/styles (acceptable for self-hosted)

---

## 5. Cookie Security

### Policy Modes
- `auto` - Secure only if request appears HTTPS
- `always` - Always secure (production default)
- `never` - Development only

### Production Enforcement
- `APP_DEPLOYMENT_MODE=production` requires:
  - `APP_COOKIE_SECURE_MODE=always`
  - `APP_ALLOW_INSECURE_HTTP=false`
  - If using TLS termination via proxy: `APP_TRUST_PROXY_HEADERS=true`

### Status
- ✅ ENFORCED
- Location: `cmd/server/main.go:167-183`

---

## 6. Input Validation & SQL Injection

### Observations
- All SQL queries use parameterized statements (? placeholders)
- Input trimming/normalization applied consistently
- No raw SQL concatenation found

### Status
- ✅ SECURE
- Evidence: `internal/store/*.go`

---

## 7. Data Integrity

### Measures
- **Foreign Keys:** Enforced with `ON DELETE CASCADE`
- **Spool Durability:** Segmented append-only files
- **Deduplication:** Near-duplicate suppression in buffer
- **Checkpointing:** Sequence-based recovery

### Status
- ✅ IMPLEMENTED
- Location: `internal/buffer/`, `internal/spool/`

---

## 8. Deployment Security

### Production Considerations

| Item | Status | Notes |
|------|--------|-------|
| TLS Termination | ⚠️ EXTERNAL | Expect at reverse proxy (Docker configured for this) |
| Secrets Storage | ✅ ENV-BASED | No hardcoded secrets |
| File Permissions | ✅ CONFIGURABLE | umask 0o755 for directories |
| Non-root User | ✅ CONFIGURED | Dockerfile runs as `plexplore` user |

### Dockerfile Security
- Non-root execution: ✅ (`USER plexplore`)
- Read-only filesystem recommended: ⚠️ (requires VOLUME for /data)
- External ports: 8080 (non-privileged)

---

## 9. Vulnerability Summary

| Vulnerability | Severity | Status | Notes |
|---------------|----------|--------|-------|
| SQL Injection | N/A | ✅ NOT PRESENT | Parameterized queries |
| XSS (stored/reflected) | LOW | ⚠️ PARTIAL | CSP has unsafe-inline |
| CSRF | N/A | ✅ MITIGATED | Double-submit tokens |
| Session Hijacking | N/A | ✅ MITIGATED | Secure cookies + expiration |
| Password Recovery | N/A | ⚠️ MANUAL | No email-based recovery (by design) |
| Rate Limiting Bypass | LOW | ⚠️ IN-MEMORY | Acceptable for single-instance |
| SD Card Wear | MEDIUM | ⚠️ CONFIG | Use low-wear mode on Pi |

---

## 10. Architecture Suitability

### Target: Raspberry Pi Zero 2W (ARM6, 512MB RAM)

| Requirement | Fit | Notes |
|------------|-----|-------|
| Lightweight | ✅ EXCELLENT | Static Go binary, SQLite only |
| Single-instance | ✅ IDEAL | No distributed systems needed |
| SD card friendly | ⚠️ CONFIG | Use `APP_SPOOL_FSYNC_MODE=low-wear` |
| Memory efficient | ✅ EXCELLENT | Conservative buffer defaults |
| GPIO/ARM ready | ✅ YES | No external dependencies |

### Recommended Pi Configuration
```bash
APP_DEPLOYMENT_MODE=production
APP_SPOOL_FSYNC_MODE=low-wear
APP_SPOOL_FSYNC_INTERVAL=10s
APP_FLUSH_INTERVAL=30s
APP_BUFFER_MAX_POINTS=128
APP_BUFFER_MAX_BYTES=262144
APP_RATE_LIMIT_ENABLED=true
```

---

## 11. Test Coverage

### Test Status
- **Build:** ✅ PASSING
- **Unit Tests:** ✅ ALL PASSING

### Test Modules Verified
- `internal/api` - Auth, sessions, CSRF, rate limiting
- `internal/buffer` - Deduplication, queue management
- `internal/store` - CRUD operations
- `internal/config` - Configuration validation

---

## 12. Recommendations

### Before Production Deployment

1. ✅ **Configure TLS termination** at reverse proxy (nginx, traefik, etc.)
2. ✅ **Set `APP_SPOOL_FSYNC_MODE=low-wear`** to extend SD card lifespan
3. ✅ **Enable authentication** for admin routes
4. ⚠️ **Add monitoring** (prometheus metrics endpoint)
5. ⚠️ **Consider backup strategy** for SQLite database

### Future Enhancements (Optional)

1. Consider PostgreSQL for multi-instance deployments
2. Add Redis-based rate limiting for horizontal scaling
3. Implement request logging/tracing
4. Add Prometheus metrics endpoint

---

## Conclusion

Plexplore is a **well-engineered, security-conscious** application suitable for its intended lightweight self-hosted use case. The codebase implements appropriate security controls for a personal location tracker service.

**Production Readiness: APPROVED** (for single-instance self-hosted deployment)

---

*Report generated by opencode/minimax-m2.5-free on 2026-04-24*
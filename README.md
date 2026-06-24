# Exploripi

Lightweight self-hosted location history tracker for Raspberry Pi Zero 2 W. Written in Go with SQLite storage, spool-based durability, and a minimal web UI.

## Quick Start

```bash
go mod tidy
make run
```

Server defaults to `127.0.0.1:8080`.

### Bootstrap Admin

```bash
go run ./cmd/migrate --create-admin --email admin@example.com --password 'your-password'
```

### Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok","service":"exploripi"}
```

## API Endpoints

### Ingest (Device API Key Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/owntracks` | OwnTracks location event |
| POST | `/api/v1/overland/batches` | Overland batch ingestion |

Auth: `X-API-Key: <key>` or `Authorization: Bearer <key>`

### Devices (Session Auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/devices` | Create device |
| GET | `/api/v1/devices` | List devices |
| GET | `/api/v1/devices/{id}` | Get device |
| POST | `/api/v1/devices/{id}/rotate-key` | Rotate API key |

### Points (Session Auth)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/points` | Point history (paginated, filterable) |
| GET | `/api/v1/points/recent` | Recent points |

### Visits (Session Auth)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/visits` | List visits |
| POST | `/api/v1/visits/generate` | Generate visits for device |

### Exports (Session Auth)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/exports/geojson` | GeoJSON export |
| GET | `/api/v1/exports/gpx` | GPX 1.1 export |

### Users (Admin Only)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/users` | List users |
| POST | `/api/v1/users` | Create user |

### Status & Auth

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Public health check |
| GET | `/status` | Public status |
| GET | `/api/v1/status` | Detailed operational status (auth required) |
| GET | `/login` | Login page |
| POST | `/login` | Login |
| POST | `/logout` | Logout |

## Web UI

- `/ui/status` — service status dashboard
- `/ui/map` — interactive map with track, visits, device/date filters
- `/ui/admin/users` — admin user management
- `/ui/admin/devices` — admin device management & key rotation

## Docker

```bash
docker compose up --build -d
```

Binds to `127.0.0.1:8080` by default. Production defaults: secure cookies, insecure HTTP disabled, TLS termination expected at reverse proxy.

## Raspberry Pi (systemd)

Build natively on the Pi, then install:

```bash
go build -o exploripi-server ./cmd/server
sudo ./scripts/install_systemd.sh
sudo systemctl start exploripi
```

### Target architecture & cross-compiling

The Pi Zero 2 W (Cortex-A53) runs either a 64-bit or 32-bit Raspberry Pi OS. Match
the build target to your OS:

- 64-bit OS: `GOARCH=arm64`
- 32-bit OS: `GOARCH=arm GOARM=7`

`go-sqlite3` requires CGO, so cross-compiling from another machine needs a matching C
cross-toolchain. For example, building a 64-bit binary on a Debian/Ubuntu workstation:

```bash
sudo apt-get install gcc-aarch64-linux-gnu
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
  go build -o exploripi-server ./cmd/server
```

Compiling on the Pi itself works but is slow on 512 MB of RAM; enable swap if the CGO
build is killed. The Docker image builds for ARM via emulation, e.g.
`docker buildx build --platform linux/arm64 -t exploripi .`.

## Backup & Restore

```bash
scripts/backup.sh --sqlite-path ./data/exploripi.db --spool-dir ./data/spool --output-dir ./backups
scripts/restore.sh --archive ./backups/exploripi-backup-*.tar.gz --sqlite-path ./data/exploripi.db --spool-dir ./data/spool
```

## Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_DEPLOYMENT_MODE` | `development` | `development` or `production` |
| `APP_HTTP_LISTEN_ADDR` | `127.0.0.1:8080` | HTTP bind address |
| `APP_SQLITE_PATH` | `./data/exploripi.db` | SQLite database path |
| `APP_SPOOL_DIR` | `./data/spool` | Spool directory |
| `APP_COOKIE_SECURE_MODE` | `auto` | `auto`, `always`, or `never` |
| `APP_ALLOW_INSECURE_HTTP` | `false` | Allow insecure HTTP (dev only) |
| `APP_TRUST_PROXY_HEADERS` | `false` | Trust X-Forwarded headers |
| `APP_RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `APP_SPOOL_FSYNC_MODE` | `balanced` | `always`, `balanced`, or `low-wear` |
| `APP_MAP_TILE_MODE` | `none` | `none`, `osm`, or `custom` |
| `APP_VISIT_SCHEDULER_ENABLED` | `false` | Automatic visit generation |

Full variable reference: `APP_BUFFER_MAX_POINTS`, `APP_BUFFER_MAX_BYTES`, `APP_FLUSH_INTERVAL`, `APP_FLUSH_BATCH_SIZE`, `APP_FLUSH_TRIGGER_POINTS`, `APP_FLUSH_TRIGGER_BYTES`, `APP_SPOOL_SEGMENT_MAX_BYTES`, `APP_SPOOL_FSYNC_INTERVAL`, `APP_SPOOL_FSYNC_BYTE_THRESHOLD`, `APP_RATE_LIMIT_LOGIN_MAX_REQUESTS`, `APP_RATE_LIMIT_LOGIN_WINDOW`, `APP_RATE_LIMIT_ADMIN_MAX_REQUESTS`, `APP_RATE_LIMIT_ADMIN_WINDOW`, `APP_READ_TIMEOUT_SECONDS`, `APP_WRITE_TIMEOUT_SECONDS`, `APP_IDLE_TIMEOUT_SECONDS`, `APP_REVERSE_GEOCODE_ENABLED`, `APP_REVERSE_GEOCODE_PROVIDER`, `APP_REVERSE_GEOCODE_NOMINATIM_URL`, `APP_REVERSE_GEOCODE_USER_AGENT`, `APP_REVERSE_GEOCODE_TIMEOUT`, `APP_REVERSE_GEOCODE_CACHE_DECIMALS`, `APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST`, `APP_MAP_TILE_URL_TEMPLATE`, `APP_MAP_TILE_ATTRIBUTION`, `APP_VISIT_SCHEDULER_INTERVAL`, `APP_VISIT_SCHEDULER_DEVICE_BATCH_SIZE`, `APP_VISIT_SCHEDULER_LOOKBACK`, `APP_VISIT_SCHEDULER_MIN_DWELL`, `APP_VISIT_SCHEDULER_MAX_RADIUS_METERS`, `APP_MIGRATIONS_DIR`, `APP_EXPECT_TLS_TERMINATION`.

## Security

- Passwords: bcrypt (cost 10), 12-char minimum
- Session tokens: 32-byte crypto/rand, 7-day TTL, HttpOnly cookies
- Device API keys: SHA-256 hashed at rest, full key shown only at create/rotate
- CSRF: double-submit cookie pattern on all state-changing endpoints
- Security headers: CSP, X-Frame-Options DENY, nosniff, Referrer-Policy, COOP, Permissions-Policy
- Production mode: enforces secure cookies, blocks insecure HTTP, fail-fast on unsafe config
- Rate limiting: in-process fixed-window (login, admin-sensitive routes)
- HSTS: set at reverse proxy, not in-app

## Migrations

```bash
make migrate
```

SQLite pragmas: WAL mode, NORMAL sync, foreign keys ON, 5s busy timeout.

## Shutdown & Recovery

- SIGINT/SIGTERM triggers graceful drain: rejects new ingest (503), flushes buffer to SQLite, advances checkpoint
- On crash/power loss, startup recovery replays uncheckpointed spool records
- Idempotent inserts (ingest_hash) prevent duplicates after recovery

## License

This project is licensed under **PolyForm Noncommercial 1.0.0**. See `LICENSE`.
Commercial use (including offering as a service) requires a separate commercial license.

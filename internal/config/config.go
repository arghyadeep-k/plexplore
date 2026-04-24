package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// HTTPListenAddr is the bind address for the API server (host:port).
	HTTPListenAddr string
	// SQLitePath is the on-disk SQLite database file path.
	SQLitePath string
	// SpoolDir is where segmented append-only spool files are stored.
	SpoolDir string
	// SpoolSegmentMaxBytes is the maximum size of one spool segment file.
	SpoolSegmentMaxBytes int
	// SpoolFSyncMode controls durability vs. SD wear policy.
	// Allowed values: always, balanced, low-wear.
	SpoolFSyncMode string
	// SpoolFSyncInterval is the periodic fsync cadence used by non-always modes.
	SpoolFSyncInterval time.Duration
	// SpoolFSyncByteThreshold forces fsync after this many newly written bytes.
	SpoolFSyncByteThreshold int
	// BufferMaxPoints is the maximum number of points in RAM before flushing.
	BufferMaxPoints int
	// BufferMaxBytes is the approximate RAM budget for buffered points.
	BufferMaxBytes int
	// FlushInterval controls periodic flush cadence from RAM to durable store.
	FlushInterval time.Duration
	// FlushBatchSize is the max number of points persisted per flush batch.
	FlushBatchSize int
	// FlushTriggerPoints triggers a best-effort flush when buffered points
	// reaches or exceeds this threshold.
	FlushTriggerPoints int
	// FlushTriggerBytes triggers a best-effort flush when buffered bytes
	// reaches or exceeds this threshold.
	FlushTriggerBytes int
	// CookieSecureMode controls Secure cookie behavior.
	// Allowed values: auto, always, never.
	CookieSecureMode string
	// TrustProxyHeaders enables trusted X-Forwarded-Proto handling.
	TrustProxyHeaders bool
	// ExpectTLSTermination indicates deployment expects TLS at a reverse proxy.
	ExpectTLSTermination bool
	// RateLimitEnabled controls in-process auth/admin route rate limiting.
	RateLimitEnabled bool
	// RateLimitLoginMaxRequests is the max login attempts per window.
	RateLimitLoginMaxRequests int
	// RateLimitLoginWindow is the login limiter window.
	RateLimitLoginWindow time.Duration
	// RateLimitAdminMaxRequests is the max admin-sensitive requests per window.
	RateLimitAdminMaxRequests int
	// RateLimitAdminWindow is the admin-sensitive limiter window.
	RateLimitAdminWindow time.Duration

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Reverse geocode cache (optional, visit-centroid only).
	ReverseGeocodeEnabled              bool
	ReverseGeocodeProvider             string
	ReverseGeocodeNominatimURL         string
	ReverseGeocodeUserAgent            string
	ReverseGeocodeTimeout              time.Duration
	ReverseGeocodeCacheDecimals        int
	ReverseGeocodeMaxLookupsPerRequest int
}

func Load() Config {
	bufferMaxPoints := getEnvInt("APP_BUFFER_MAX_POINTS", 256)
	bufferMaxBytes := getEnvInt("APP_BUFFER_MAX_BYTES", 256*1024)

	return Config{
		HTTPListenAddr:                     getEnv("APP_HTTP_LISTEN_ADDR", "0.0.0.0:8080"),
		SQLitePath:                         getEnv("APP_SQLITE_PATH", "./data/plexplore.db"),
		SpoolDir:                           getEnv("APP_SPOOL_DIR", "./data/spool"),
		SpoolSegmentMaxBytes:               getEnvInt("APP_SPOOL_SEGMENT_MAX_BYTES", 4*1024*1024),
		SpoolFSyncMode:                     getFsyncMode("APP_SPOOL_FSYNC_MODE", "balanced"),
		SpoolFSyncInterval:                 getEnvDuration("APP_SPOOL_FSYNC_INTERVAL", 2*time.Second),
		SpoolFSyncByteThreshold:            getEnvInt("APP_SPOOL_FSYNC_BYTE_THRESHOLD", 64*1024),
		BufferMaxPoints:                    bufferMaxPoints,
		BufferMaxBytes:                     bufferMaxBytes,
		FlushInterval:                      getEnvDuration("APP_FLUSH_INTERVAL", 10*time.Second),
		FlushBatchSize:                     getEnvInt("APP_FLUSH_BATCH_SIZE", 128),
		FlushTriggerPoints:                 getEnvInt("APP_FLUSH_TRIGGER_POINTS", defaultFlushTriggerThreshold(bufferMaxPoints)),
		FlushTriggerBytes:                  getEnvInt("APP_FLUSH_TRIGGER_BYTES", defaultFlushTriggerThreshold(bufferMaxBytes)),
		CookieSecureMode:                   getCookieSecureMode("APP_COOKIE_SECURE_MODE", "auto"),
		TrustProxyHeaders:                  getEnvBool("APP_TRUST_PROXY_HEADERS", false),
		ExpectTLSTermination:               getEnvBool("APP_EXPECT_TLS_TERMINATION", false),
		RateLimitEnabled:                   getEnvBool("APP_RATE_LIMIT_ENABLED", true),
		RateLimitLoginMaxRequests:          getEnvInt("APP_RATE_LIMIT_LOGIN_MAX_REQUESTS", 10),
		RateLimitLoginWindow:               getEnvDuration("APP_RATE_LIMIT_LOGIN_WINDOW", time.Minute),
		RateLimitAdminMaxRequests:          getEnvInt("APP_RATE_LIMIT_ADMIN_MAX_REQUESTS", 30),
		RateLimitAdminWindow:               getEnvDuration("APP_RATE_LIMIT_ADMIN_WINDOW", time.Minute),
		ReadTimeout:                        time.Duration(getEnvInt("APP_READ_TIMEOUT_SECONDS", 5)) * time.Second,
		WriteTimeout:                       time.Duration(getEnvInt("APP_WRITE_TIMEOUT_SECONDS", 10)) * time.Second,
		IdleTimeout:                        time.Duration(getEnvInt("APP_IDLE_TIMEOUT_SECONDS", 30)) * time.Second,
		ReverseGeocodeEnabled:              getEnvBool("APP_REVERSE_GEOCODE_ENABLED", false),
		ReverseGeocodeProvider:             strings.ToLower(getEnv("APP_REVERSE_GEOCODE_PROVIDER", "nominatim")),
		ReverseGeocodeNominatimURL:         getEnv("APP_REVERSE_GEOCODE_NOMINATIM_URL", "https://nominatim.openstreetmap.org/reverse"),
		ReverseGeocodeUserAgent:            getEnv("APP_REVERSE_GEOCODE_USER_AGENT", "plexplore/1.0 (+self-hosted)"),
		ReverseGeocodeTimeout:              getEnvDuration("APP_REVERSE_GEOCODE_TIMEOUT", 2*time.Second),
		ReverseGeocodeCacheDecimals:        getEnvInt("APP_REVERSE_GEOCODE_CACHE_DECIMALS", 4),
		ReverseGeocodeMaxLookupsPerRequest: getEnvInt("APP_REVERSE_GEOCODE_MAX_LOOKUPS_PER_REQUEST", 3),
	}
}

func (c Config) Address() string {
	return c.HTTPListenAddr
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getFsyncMode(key, fallback string) string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if mode == "" {
		return fallback
	}
	switch mode {
	case "always", "balanced", "low-wear":
		return mode
	default:
		return fallback
	}
}

func defaultFlushTriggerThreshold(maxValue int) int {
	threshold := (maxValue * 3) / 4
	if threshold <= 0 {
		return 1
	}
	return threshold
}

func getCookieSecureMode(key, fallback string) string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if mode == "" {
		return fallback
	}
	switch mode {
	case "auto", "always", "never":
		return mode
	default:
		return fallback
	}
}

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

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load() Config {
	return Config{
		HTTPListenAddr:         getEnv("APP_HTTP_LISTEN_ADDR", "0.0.0.0:8080"),
		SQLitePath:             getEnv("APP_SQLITE_PATH", "./data/plexplore.db"),
		SpoolDir:               getEnv("APP_SPOOL_DIR", "./data/spool"),
		SpoolSegmentMaxBytes:   getEnvInt("APP_SPOOL_SEGMENT_MAX_BYTES", 4*1024*1024),
		SpoolFSyncMode:         getFsyncMode("APP_SPOOL_FSYNC_MODE", "balanced"),
		SpoolFSyncInterval:     getEnvDuration("APP_SPOOL_FSYNC_INTERVAL", 2*time.Second),
		SpoolFSyncByteThreshold: getEnvInt("APP_SPOOL_FSYNC_BYTE_THRESHOLD", 64*1024),
		BufferMaxPoints:        getEnvInt("APP_BUFFER_MAX_POINTS", 256),
		BufferMaxBytes:         getEnvInt("APP_BUFFER_MAX_BYTES", 256*1024),
		FlushInterval:          getEnvDuration("APP_FLUSH_INTERVAL", 10*time.Second),
		FlushBatchSize:         getEnvInt("APP_FLUSH_BATCH_SIZE", 128),
		ReadTimeout:  time.Duration(getEnvInt("APP_READ_TIMEOUT_SECONDS", 5)) * time.Second,
		WriteTimeout: time.Duration(getEnvInt("APP_WRITE_TIMEOUT_SECONDS", 10)) * time.Second,
		IdleTimeout:  time.Duration(getEnvInt("APP_IDLE_TIMEOUT_SECONDS", 30)) * time.Second,
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

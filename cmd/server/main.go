package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"plexplore/internal/api"
	"plexplore/internal/buffer"
	"plexplore/internal/config"
	"plexplore/internal/flusher"
	"plexplore/internal/spool"
	"plexplore/internal/store"
	"plexplore/internal/tasks"
	"plexplore/internal/visits"
)

func main() {
	cfg := config.Load()
	logCookieSecurityWarnings(cfg)

	if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
		log.Fatalf("create sqlite directory: %v", err)
	}
	if err := os.MkdirAll(cfg.SpoolDir, 0o755); err != nil {
		log.Fatalf("create spool directory: %v", err)
	}

	spoolManager := spool.NewFileSpoolManagerWithOptions(cfg.SpoolDir, cfg.SpoolSegmentMaxBytes, spool.ManagerOptions{
		FSyncMode:          cfg.SpoolFSyncMode,
		FSyncInterval:      cfg.SpoolFSyncInterval,
		FSyncByteThreshold: cfg.SpoolFSyncByteThreshold,
	})
	defer func() {
		if err := spoolManager.Close(); err != nil {
			log.Printf("close spool manager: %v", err)
		}
	}()

	sqliteStore, err := store.OpenSQLiteStore(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			log.Printf("close sqlite store: %v", err)
		}
	}()

	bufferManager := buffer.NewManager(cfg.BufferMaxPoints, cfg.BufferMaxBytes)
	batchFlusher := flusher.New(sqliteStore, spoolManager, bufferManager, flusher.Config{
		FlushInterval:  cfg.FlushInterval,
		FlushBatchSize: cfg.FlushBatchSize,
	})

	recoveryResult, err := tasks.RecoverFromSpool(
		spoolManager,
		bufferManager,
		batchFlusher,
		tasks.RecoveryConfig{EnqueueBatchSize: cfg.FlushBatchSize},
	)
	if err != nil {
		log.Fatalf("startup recovery failed: %v", err)
	}
	log.Printf(
		"startup recovery complete: checkpoint_seq=%d replayed=%d enqueued=%d",
		recoveryResult.CheckpointSeq,
		recoveryResult.Replayed,
		recoveryResult.Enqueued,
	)

	batchFlusher.Start()
	var draining atomic.Bool
	visitLabelResolver, err := visits.NewLabelResolver(visits.ReverseGeocodeConfig{
		Enabled:              cfg.ReverseGeocodeEnabled,
		Provider:             cfg.ReverseGeocodeProvider,
		NominatimURL:         cfg.ReverseGeocodeNominatimURL,
		UserAgent:            cfg.ReverseGeocodeUserAgent,
		Timeout:              cfg.ReverseGeocodeTimeout,
		CacheDecimals:        cfg.ReverseGeocodeCacheDecimals,
		MaxLookupsPerRequest: cfg.ReverseGeocodeMaxLookupsPerRequest,
	}, sqliteStore)
	if err != nil {
		log.Fatalf("configure reverse geocode resolver: %v", err)
	}

	mux := http.NewServeMux()
	rateLimiters := api.RateLimiters{
		TrustProxyHeaders: cfg.TrustProxyHeaders,
	}
	if cfg.RateLimitEnabled {
		rateLimiters.Login = api.NewFixedWindowLimiter(cfg.RateLimitLoginMaxRequests, cfg.RateLimitLoginWindow)
		rateLimiters.AdminSensitive = api.NewFixedWindowLimiter(cfg.RateLimitAdminMaxRequests, cfg.RateLimitAdminWindow)
	}
	api.RegisterRoutesWithDependencies(mux, api.Dependencies{
		DeviceStore:        sqliteStore,
		Spool:              spoolManager,
		Buffer:             bufferManager,
		Flusher:            batchFlusher,
		FlushTriggerPoints: cfg.FlushTriggerPoints,
		FlushTriggerBytes:  cfg.FlushTriggerBytes,
		PointStore:         sqliteStore,
		VisitStore:         sqliteStore,
		VisitLabelResolver: visitLabelResolver,
		UserStore:          sqliteStore,
		SessionStore:       sqliteStore,
		CookieSecurity: api.CookieSecurityPolicy{
			SecureMode:        cfg.CookieSecureMode,
			TrustProxyHeaders: cfg.TrustProxyHeaders,
		},
		RateLimiters: rateLimiters,
		SpoolDir:     cfg.SpoolDir,
		SQLitePath:   cfg.SQLitePath,
		IsDraining: func() bool {
			return draining.Load()
		},
	})

	server := &http.Server{
		Addr:         cfg.Address(),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("server listening on %s", cfg.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	draining.Store(true)
	server.SetKeepAlivesEnabled(false)

	serverCtx, cancelServer := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelServer()
	if err := server.Shutdown(serverCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	flushCtx, cancelFlush := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelFlush()
	if err := batchFlusher.Stop(flushCtx); err != nil {
		log.Printf("flusher stop failed: %v", err)
	}
}

func logCookieSecurityWarnings(cfg config.Config) {
	mode := strings.ToLower(strings.TrimSpace(cfg.CookieSecureMode))
	publicBind := isPublicBind(cfg.HTTPListenAddr)
	if publicBind && mode != "always" {
		log.Printf("warning: HTTP bind is public (%s) and APP_COOKIE_SECURE_MODE=%s; cookies may travel over plain HTTP unless TLS is correctly configured", cfg.HTTPListenAddr, mode)
	}
	if cfg.ExpectTLSTermination && !cfg.TrustProxyHeaders {
		log.Printf("warning: APP_EXPECT_TLS_TERMINATION=true but APP_TRUST_PROXY_HEADERS=false; proxied HTTPS requests may not receive Secure cookies")
	}
	if mode == "never" {
		log.Printf("warning: APP_COOKIE_SECURE_MODE=never disables Secure cookies; use only for local HTTP development")
	}
}

func isPublicBind(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return true
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return true
	}
	return false
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
)

func main() {
	cfg := config.Load()

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

	mux := http.NewServeMux()
	api.RegisterRoutesWithDependencies(mux, api.Dependencies{
		DeviceStore:        sqliteStore,
		Spool:              spoolManager,
		Buffer:             bufferManager,
		Flusher:            batchFlusher,
		FlushTriggerPoints: cfg.FlushTriggerPoints,
		FlushTriggerBytes:  cfg.FlushTriggerBytes,
		PointStore:         sqliteStore,
		SpoolDir:           cfg.SpoolDir,
		SQLitePath:         cfg.SQLitePath,
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

package flusher

import (
	"context"
	"sync"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
)

type Store interface {
	InsertSpoolBatch(records []ingest.SpoolRecord) (uint64, error)
}

type CheckpointManager interface {
	AdvanceCheckpoint(lastCommittedSeq uint64) (spool.Checkpoint, error)
}

type Buffer interface {
	Enqueue(records []ingest.SpoolRecord) error
	DrainBatch(maxPoints int) []ingest.SpoolRecord
	RequeueFront(records []ingest.SpoolRecord) error
	Stats() buffer.Stats
}

type Config struct {
	FlushInterval time.Duration
	FlushBatchSize int
}

// LastFlushResult captures the most recent flush attempt outcome.
type LastFlushResult struct {
	AtUTC   time.Time
	Success bool
	Error   string
}

type Flusher struct {
	store      Store
	checkpoint CheckpointManager
	buffer     Buffer
	config     Config

	startMu   sync.Mutex
	started   bool
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
	triggerCh chan struct{}

	resultMu       sync.RWMutex
	lastResult     LastFlushResult
	lastResultSeen bool
}

func New(store Store, checkpoint CheckpointManager, buf Buffer, cfg Config) *Flusher {
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.FlushBatchSize <= 0 {
		cfg.FlushBatchSize = 128
	}

	return &Flusher{
		store:      store,
		checkpoint: checkpoint,
		buffer:     buf,
		config:     cfg,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
		triggerCh:  make(chan struct{}, 1),
	}
}

func (f *Flusher) Start() {
	f.startMu.Lock()
	defer f.startMu.Unlock()
	if f.started {
		return
	}
	f.started = true

	go f.run()
}

// TriggerFlush requests a size-based flush pass.
func (f *Flusher) TriggerFlush() {
	select {
	case f.triggerCh <- struct{}{}:
	default:
	}
}

// FlushNow runs one immediate timer-style flush pass.
func (f *Flusher) FlushNow() error {
	err := f.flushPass(true)
	f.recordFlushResult(err)
	return err
}

func (f *Flusher) Stop(ctx context.Context) error {
	f.stopOnce.Do(func() {
		close(f.stopCh)
	})

	select {
	case <-f.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *Flusher) run() {
	defer close(f.doneCh)

	ticker := time.NewTicker(f.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := f.flushPass(true)
			f.recordFlushResult(err)
		case <-f.triggerCh:
			err := f.flushPass(false)
			f.recordFlushResult(err)
		case <-f.stopCh:
			err := f.flushUntilEmpty()
			f.recordFlushResult(err)
			return
		}
	}
}

func (f *Flusher) flushPass(timerMode bool) error {
	stats := f.buffer.Stats()
	if stats.TotalBufferedPoints == 0 {
		return nil
	}
	if !timerMode && stats.TotalBufferedPoints < f.config.FlushBatchSize {
		return nil
	}

	for {
		flushed, err := f.flushOneBatch()
		if err != nil {
			return err
		}
		if !flushed {
			return nil
		}

		stats = f.buffer.Stats()
		if stats.TotalBufferedPoints == 0 {
			return nil
		}
		if stats.TotalBufferedPoints < f.config.FlushBatchSize {
			return nil
		}
	}
}

func (f *Flusher) flushUntilEmpty() error {
	for {
		flushed, err := f.flushOneBatch()
		if err != nil {
			return err
		}
		if !flushed {
			return nil
		}
	}
}

func (f *Flusher) flushOneBatch() (bool, error) {
	batch := f.buffer.DrainBatch(f.config.FlushBatchSize)
	if len(batch) == 0 {
		return false, nil
	}

	maxSeq, err := f.store.InsertSpoolBatch(batch)
	if err != nil {
		_ = f.buffer.RequeueFront(batch)
		return false, err
	}

	if maxSeq > 0 {
		if _, err := f.checkpoint.AdvanceCheckpoint(maxSeq); err != nil {
			return false, err
		}
	}

	return true, nil
}

// LastFlushResult returns the latest flush attempt result and whether one exists.
func (f *Flusher) LastFlushResult() (LastFlushResult, bool) {
	f.resultMu.RLock()
	defer f.resultMu.RUnlock()
	return f.lastResult, f.lastResultSeen
}

func (f *Flusher) recordFlushResult(err error) {
	result := LastFlushResult{
		AtUTC:   time.Now().UTC(),
		Success: err == nil,
	}
	if err != nil {
		result.Error = err.Error()
	}

	f.resultMu.Lock()
	defer f.resultMu.Unlock()
	f.lastResult = result
	f.lastResultSeen = true
}

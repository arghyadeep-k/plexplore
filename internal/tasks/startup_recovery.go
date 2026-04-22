package tasks

import (
	"errors"
	"fmt"

	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/spool"
)

// RecoveryConfig controls startup replay behavior.
type RecoveryConfig struct {
	// EnqueueBatchSize bounds per-call enqueue size to keep RAM pressure low.
	EnqueueBatchSize int
}

// RecoveryResult reports deterministic startup replay outcomes.
type RecoveryResult struct {
	CheckpointSeq uint64
	Replayed      int
	Enqueued      int
}

// RecoverFromSpool replays all records newer than checkpoint into RAM and
// forces a flush pass so recovered records can be durably committed before
// normal ingest traffic starts.
func RecoverFromSpool(
	spoolManager spool.SpoolManager,
	bufferManager *buffer.Manager,
	batchFlusher *flusher.Flusher,
	cfg RecoveryConfig,
) (RecoveryResult, error) {
	if spoolManager == nil {
		return RecoveryResult{}, fmt.Errorf("nil spool manager")
	}
	if bufferManager == nil {
		return RecoveryResult{}, fmt.Errorf("nil buffer manager")
	}
	if batchFlusher == nil {
		return RecoveryResult{}, fmt.Errorf("nil flusher")
	}

	if cfg.EnqueueBatchSize <= 0 {
		cfg.EnqueueBatchSize = 64
	}

	checkpoint, err := spoolManager.ReadCheckpoint()
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("read checkpoint: %w", err)
	}

	replayed, err := spoolManager.ReplayAfterCheckpoint()
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("replay after checkpoint: %w", err)
	}
	if len(replayed) == 0 {
		return RecoveryResult{
			CheckpointSeq: checkpoint.LastCommittedSeq,
			Replayed:      0,
			Enqueued:      0,
		}, nil
	}

	enqueued := 0
	for start := 0; start < len(replayed); {
		end := start + cfg.EnqueueBatchSize
		if end > len(replayed) {
			end = len(replayed)
		}
		chunk := replayed[start:end]

		if err := bufferManager.Enqueue(chunk); err != nil {
			if errors.Is(err, buffer.ErrMaxPointsExceeded) || errors.Is(err, buffer.ErrMaxBytesExceeded) {
				if flushErr := batchFlusher.FlushNow(); flushErr != nil {
					return RecoveryResult{}, fmt.Errorf("flush during startup recovery: %w", flushErr)
				}
				if retryErr := bufferManager.Enqueue(chunk); retryErr != nil {
					return RecoveryResult{}, fmt.Errorf("enqueue replayed chunk after flush: %w", retryErr)
				}
			} else {
				return RecoveryResult{}, fmt.Errorf("enqueue replayed chunk: %w", err)
			}
		}

		enqueued += len(chunk)
		start = end
	}

	if err := batchFlusher.FlushNow(); err != nil {
		return RecoveryResult{}, fmt.Errorf("final startup recovery flush: %w", err)
	}

	return RecoveryResult{
		CheckpointSeq: checkpoint.LastCommittedSeq,
		Replayed:      len(replayed),
		Enqueued:      enqueued,
	}, nil
}

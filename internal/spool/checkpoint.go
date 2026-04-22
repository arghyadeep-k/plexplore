package spool

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrCheckpointEmptyData = errors.New("checkpoint data is empty")
	ErrCheckpointInvalid   = errors.New("checkpoint data is invalid")
)

// Checkpoint stores progress for durable commit state.
// last_committed_seq means all records <= this sequence are already committed
// downstream and can be skipped during replay.
type Checkpoint struct {
	LastCommittedSeq uint64    `json:"last_committed_seq"`
	UpdatedAtUTC     time.Time `json:"updated_at_utc"`
}

func SerializeCheckpoint(checkpoint Checkpoint) ([]byte, error) {
	if checkpoint.UpdatedAtUTC.IsZero() {
		checkpoint.UpdatedAtUTC = time.Now().UTC()
	}
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("serialize checkpoint: %w", err)
	}
	return append(data, '\n'), nil
}

func DeserializeCheckpoint(data []byte) (Checkpoint, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return Checkpoint{}, ErrCheckpointEmptyData
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal([]byte(trimmed), &checkpoint); err != nil {
		return Checkpoint{}, fmt.Errorf("%w: %v", ErrCheckpointInvalid, err)
	}
	return checkpoint, nil
}

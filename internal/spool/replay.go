package spool

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"plexplore/internal/ingest"
)

func (m *FileSpoolManager) ReadCheckpoint() (Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.readCheckpointLocked()
}

func (m *FileSpoolManager) AdvanceCheckpoint(lastCommittedSeq uint64) (Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.spoolDir, 0o755); err != nil {
		return Checkpoint{}, fmt.Errorf("ensure spool dir: %w", err)
	}

	current, err := m.readCheckpointLocked()
	if err != nil {
		return Checkpoint{}, err
	}
	if lastCommittedSeq <= current.LastCommittedSeq {
		return current, nil
	}

	next := Checkpoint{
		LastCommittedSeq: lastCommittedSeq,
		UpdatedAtUTC:     m.nowFn().UTC(),
	}

	data, err := SerializeCheckpoint(next)
	if err != nil {
		return Checkpoint{}, err
	}
	file, err := os.OpenFile(m.CheckpointPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("open checkpoint for write: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return Checkpoint{}, fmt.Errorf("write checkpoint: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Checkpoint{}, fmt.Errorf("sync checkpoint: %w", err)
	}
	if err := file.Close(); err != nil {
		return Checkpoint{}, fmt.Errorf("close checkpoint: %w", err)
	}
	return next, nil
}

// ReplayAfterCheckpoint loads all spool records with seq greater than the
// current checkpoint's last committed sequence.
func (m *FileSpoolManager) ReplayAfterCheckpoint() ([]ingest.SpoolRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpoint, err := m.readCheckpointLocked()
	if err != nil {
		return nil, err
	}

	segmentStarts, err := m.listSegmentStartsLocked()
	if err != nil {
		return nil, err
	}

	out := make([]ingest.SpoolRecord, 0)
	for _, startSeq := range segmentStarts {
		records, err := readSegmentFile(m.SegmentPath(startSeq))
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if record.Seq > checkpoint.LastCommittedSeq {
				out = append(out, record)
			}
		}
	}

	return out, nil
}

func (m *FileSpoolManager) readCheckpointLocked() (Checkpoint, error) {
	data, err := os.ReadFile(m.CheckpointPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Checkpoint{LastCommittedSeq: 0}, nil
		}
		return Checkpoint{}, fmt.Errorf("read checkpoint: %w", err)
	}
	return DeserializeCheckpoint(data)
}

func (m *FileSpoolManager) listSegmentStartsLocked() ([]uint64, error) {
	entries, err := os.ReadDir(m.spoolDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("list spool dir: %w", err)
	}

	starts := make([]uint64, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		startSeq, err := ParseSegmentStartSeq(entry.Name())
		if err != nil {
			continue
		}
		starts = append(starts, startSeq)
	}

	slices.Sort(starts)
	return starts, nil
}

func readSegmentFile(path string) ([]ingest.SpoolRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open segment %q: %w", path, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	records := make([]ingest.SpoolRecord, 0)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			record, decodeErr := DeserializeRecord(line)
			if decodeErr != nil {
				return nil, fmt.Errorf("decode record in %q: %w", path, decodeErr)
			}
			records = append(records, record)
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		return nil, fmt.Errorf("read segment %q: %w", path, err)
	}
}

package spool

import (
	"errors"
	"fmt"
	"os"
)

// CompactCommittedSegments deletes immutable segment files where all records
// are already committed (record.seq <= checkpoint.last_committed_seq).
//
// Recommended run points:
// - after advancing checkpoint during flush cycles
// - periodic maintenance (for example, a low-frequency background task)
func (m *FileSpoolManager) CompactCommittedSegments() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpoint, err := m.readCheckpointLocked()
	if err != nil {
		return 0, err
	}

	segmentStarts, err := m.listSegmentStartsLocked()
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, startSeq := range segmentStarts {
		// Never compact the currently writable segment while open.
		if m.activeFile != nil && startSeq == m.activeSegmentStart {
			continue
		}
		// If a segment starts after checkpoint, none of its records can be fully committed.
		if startSeq > checkpoint.LastCommittedSeq {
			continue
		}

		path := m.SegmentPath(startSeq)
		maxSeq, err := maxRecordSeqInSegment(path, startSeq)
		if err != nil {
			return deleted, err
		}
		if maxSeq > checkpoint.LastCommittedSeq {
			continue
		}

		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return deleted, fmt.Errorf("remove compacted segment %q: %w", path, err)
		}
		deleted++
	}

	return deleted, nil
}

func maxRecordSeqInSegment(path string, startSeq uint64) (uint64, error) {
	records, err := readSegmentFile(path)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		if startSeq == 0 {
			return 0, nil
		}
		return startSeq - 1, nil
	}

	maxSeq := records[0].Seq
	for _, record := range records[1:] {
		if record.Seq > maxSeq {
			maxSeq = record.Seq
		}
	}
	return maxSeq, nil
}

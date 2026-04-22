package spool

import (
	"fmt"
	"os"

	"plexplore/internal/ingest"
)

// AppendCanonicalPoints appends canonical points into the spool as sequence-
// ordered SpoolRecord entries and returns the created records.
func (m *FileSpoolManager) AppendCanonicalPoints(points []ingest.CanonicalPoint) ([]ingest.SpoolRecord, error) {
	if len(points) == 0 {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if normalizeFSyncMode(m.fsyncMode) == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidFSyncMode, m.fsyncMode)
	}
	if err := os.MkdirAll(m.spoolDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure spool dir: %w", err)
	}

	appended := make([]ingest.SpoolRecord, 0, len(points))
	for _, point := range points {
		record := ingest.SpoolRecord{
			Seq:        m.nextSeq,
			DeviceID:   point.DeviceID,
			ReceivedAt: m.nowFn().UTC(),
			Point:      point,
		}
		if record.Point.IngestHash == "" {
			record.Point.IngestHash = ingest.GenerateDeterministicIngestHash(record.Point)
		}

		line, err := SerializeRecord(record)
		if err != nil {
			return nil, err
		}

		if err := m.ensureSegmentForWriteLocked(record.Seq, int64(len(line))); err != nil {
			return nil, err
		}
		if _, err := m.activeFile.Write(line); err != nil {
			return nil, fmt.Errorf("write spool record seq=%d: %w", record.Seq, err)
		}

		m.activeSegmentSize += int64(len(line))
		m.bytesSinceSync += len(line)

		if m.shouldFSyncLocked() {
			if err := m.activeFile.Sync(); err != nil {
				return nil, fmt.Errorf("fsync spool segment: %w", err)
			}
			m.bytesSinceSync = 0
			m.lastSyncAt = m.nowFn()
		}

		m.nextSeq++
		appended = append(appended, record)
	}

	return appended, nil
}

func (m *FileSpoolManager) ensureSegmentForWriteLocked(nextSeq uint64, incomingBytes int64) error {
	needNewSegment := m.activeFile == nil
	if !needNewSegment && m.segmentMaxBytes > 0 {
		if m.activeSegmentSize+incomingBytes > int64(m.segmentMaxBytes) && m.activeSegmentSize > 0 {
			needNewSegment = true
		}
	}

	if !needNewSegment {
		return nil
	}

	if err := m.closeActiveLocked(true); err != nil {
		return err
	}
	return m.openNewSegmentLocked(nextSeq)
}

func (m *FileSpoolManager) openNewSegmentLocked(startSeq uint64) error {
	path := m.SegmentPath(startSeq)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open spool segment %q: %w", path, err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat spool segment %q: %w", path, err)
	}

	m.activeFile = file
	m.activeSegmentStart = startSeq
	m.activeSegmentSize = info.Size()
	m.bytesSinceSync = 0
	m.lastSyncAt = m.nowFn()
	return nil
}

func (m *FileSpoolManager) closeActiveLocked(syncBeforeClose bool) error {
	if m.activeFile == nil {
		return nil
	}

	if syncBeforeClose && m.fsyncMode != FSyncModeLowWear && m.bytesSinceSync > 0 {
		if err := m.activeFile.Sync(); err != nil {
			return fmt.Errorf("sync before close spool segment: %w", err)
		}
	}

	if err := m.activeFile.Close(); err != nil {
		return fmt.Errorf("close spool segment: %w", err)
	}

	m.activeFile = nil
	m.activeSegmentStart = 0
	m.activeSegmentSize = 0
	m.bytesSinceSync = 0
	return nil
}

func (m *FileSpoolManager) shouldFSyncLocked() bool {
	switch m.fsyncMode {
	case FSyncModeAlways:
		return true
	case FSyncModeBalanced:
		if m.bytesSinceSync >= m.fsyncThreshold {
			return true
		}
		return m.nowFn().Sub(m.lastSyncAt) >= m.fsyncInterval
	case FSyncModeLowWear:
		return false
	default:
		return false
	}
}

package spool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"plexplore/internal/ingest"
)

const checkpointFileName = "checkpoint.json"

const (
	FSyncModeAlways   = "always"
	FSyncModeBalanced = "balanced"
	FSyncModeLowWear  = "low-wear"
)

var ErrInvalidFSyncMode = errors.New("invalid fsync mode")

// ManagerOptions controls append durability behavior.
type ManagerOptions struct {
	FSyncMode          string
	FSyncInterval      time.Duration
	FSyncByteThreshold int
}

// SpoolManager defines the initial spool subsystem surface area.
// Append/replay file I/O can be implemented behind this interface incrementally.
type SpoolManager interface {
	AppendCanonicalPoints(points []ingest.CanonicalPoint) ([]ingest.SpoolRecord, error)
	ReadCheckpoint() (Checkpoint, error)
	AdvanceCheckpoint(lastCommittedSeq uint64) (Checkpoint, error)
	ReplayAfterCheckpoint() ([]ingest.SpoolRecord, error)
	CompactCommittedSegments() (int, error)
	SegmentPath(startSeq uint64) string
	CheckpointPath() string
	SerializeRecord(record ingest.SpoolRecord) ([]byte, error)
	DeserializeRecord(line []byte) (ingest.SpoolRecord, error)
	SerializeCheckpoint(checkpoint Checkpoint) ([]byte, error)
	DeserializeCheckpoint(data []byte) (Checkpoint, error)
	Close() error
}

// FileSpoolManager is a simple concrete manager for sequence-named segments
// and checkpoint serialization helpers.
type FileSpoolManager struct {
	spoolDir        string
	segmentMaxBytes int
	fsyncMode       string
	fsyncInterval   time.Duration
	fsyncThreshold  int

	mu                 sync.Mutex
	nextSeq            uint64
	activeSegmentStart uint64
	activeSegmentSize  int64
	activeFile         *os.File
	bytesSinceSync     int
	lastSyncAt         time.Time
	nowFn              func() time.Time
}

func NewFileSpoolManager(spoolDir string, segmentMaxBytes int) *FileSpoolManager {
	return NewFileSpoolManagerWithOptions(spoolDir, segmentMaxBytes, ManagerOptions{
		FSyncMode:          FSyncModeBalanced,
		FSyncInterval:      2 * time.Second,
		FSyncByteThreshold: 64 * 1024,
	})
}

func NewFileSpoolManagerWithOptions(spoolDir string, segmentMaxBytes int, options ManagerOptions) *FileSpoolManager {
	mode := normalizeFSyncMode(options.FSyncMode)
	if mode == "" {
		mode = FSyncModeBalanced
	}
	interval := options.FSyncInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	threshold := options.FSyncByteThreshold
	if threshold <= 0 {
		threshold = 64 * 1024
	}

	return &FileSpoolManager{
		spoolDir:        spoolDir,
		segmentMaxBytes: segmentMaxBytes,
		fsyncMode:       mode,
		fsyncInterval:   interval,
		fsyncThreshold:  threshold,
		nextSeq:         1,
		nowFn:           time.Now,
	}
}

func (m *FileSpoolManager) SegmentPath(startSeq uint64) string {
	return filepath.Join(m.spoolDir, SegmentFileName(startSeq))
}

func (m *FileSpoolManager) CheckpointPath() string {
	return filepath.Join(m.spoolDir, checkpointFileName)
}

func (m *FileSpoolManager) SegmentMaxBytes() int {
	return m.segmentMaxBytes
}

func (m *FileSpoolManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.closeActiveLocked(true)
}

func (m *FileSpoolManager) SerializeRecord(record ingest.SpoolRecord) ([]byte, error) {
	return SerializeRecord(record)
}

func (m *FileSpoolManager) DeserializeRecord(line []byte) (ingest.SpoolRecord, error) {
	return DeserializeRecord(line)
}

func (m *FileSpoolManager) SerializeCheckpoint(checkpoint Checkpoint) ([]byte, error) {
	return SerializeCheckpoint(checkpoint)
}

func (m *FileSpoolManager) DeserializeCheckpoint(data []byte) (Checkpoint, error) {
	return DeserializeCheckpoint(data)
}

func normalizeFSyncMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case FSyncModeAlways, FSyncModeBalanced, FSyncModeLowWear:
		return normalized
	default:
		return ""
	}
}

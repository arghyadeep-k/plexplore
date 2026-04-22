package ingest

import "time"

// CanonicalPoint is the normalized in-memory representation of a location event
// after ingestion, before buffering/spooling/flushing decisions are applied.
// Optional fields are pointers to avoid allocating default zero values in JSON
// payloads and to distinguish "missing" from "present but zero".
type CanonicalPoint struct {
	UserID       string
	DeviceID     string
	SourceType   string
	TimestampUTC time.Time
	Lat          float64
	Lon          float64
	Altitude     *float64
	Accuracy     *float64
	Speed        *float64
	Heading      *float64
	Battery      *float64
	MotionType   *string
	RawPayload   []byte
	IngestHash   string
}

// SpoolRecord is the append-only unit written to segmented spool files.
// Seq is a strictly increasing sequence number scoped to the active spool.
type SpoolRecord struct {
	Seq        uint64
	DeviceID   string
	ReceivedAt time.Time
	Point      CanonicalPoint
	// CheckpointOnly marks records that should advance checkpoint sequencing
	// but skip durable SQLite insert (for dedupe-suppressed duplicates).
	CheckpointOnly bool `json:"checkpoint_only,omitempty"`
}

// BufferStats tracks lightweight counters for the RAM batch buffer.
// These metrics are intended for flush heuristics and health visibility.
type BufferStats struct {
	Points        int
	BytesEstimate int
	OldestUTC     *time.Time
	NewestUTC     *time.Time
}

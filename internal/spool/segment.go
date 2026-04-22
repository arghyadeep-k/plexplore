package spool

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const segmentExtension = ".ndjson"

// SegmentFileName returns the sequence-based segment name.
// Example: segment-00000000000000000100.ndjson
func SegmentFileName(startSeq uint64) string {
	return fmt.Sprintf("segment-%020d%s", startSeq, segmentExtension)
}

// ParseSegmentStartSeq extracts the start sequence number from a segment file name.
func ParseSegmentStartSeq(name string) (uint64, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if !strings.HasPrefix(base, "segment-") || !strings.HasSuffix(base, segmentExtension) {
		return 0, fmt.Errorf("invalid segment name: %q", name)
	}

	raw := strings.TrimPrefix(base, "segment-")
	raw = strings.TrimSuffix(raw, segmentExtension)

	startSeq, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid segment sequence in %q: %w", name, err)
	}

	return startSeq, nil
}

package spool

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"plexplore/internal/ingest"
)

var (
	ErrRecordEmptyLine = errors.New("spool record line is empty")
	ErrRecordInvalid   = errors.New("spool record line is invalid")
)

// SerializeRecord encodes one spool record as newline-delimited JSON.
func SerializeRecord(record ingest.SpoolRecord) ([]byte, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("serialize record: %w", err)
	}
	return append(data, '\n'), nil
}

// DeserializeRecord decodes one newline-delimited JSON spool record line.
func DeserializeRecord(line []byte) (ingest.SpoolRecord, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return ingest.SpoolRecord{}, ErrRecordEmptyLine
	}

	var record ingest.SpoolRecord
	if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
		return ingest.SpoolRecord{}, fmt.Errorf("%w: %v", ErrRecordInvalid, err)
	}
	return record, nil
}

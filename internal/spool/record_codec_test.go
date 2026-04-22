package spool

import (
	"errors"
	"testing"
	"time"

	"plexplore/internal/ingest"
)

func TestSerializeAndDeserializeRecord_RoundTrip(t *testing.T) {
	accuracy := 7.5
	raw := []byte(`{"raw":"payload"}`)

	input := ingest.SpoolRecord{
		Seq:        42,
		DeviceID:   "device-a",
		ReceivedAt: time.Date(2026, 4, 21, 20, 30, 0, 0, time.UTC),
		Point: ingest.CanonicalPoint{
			UserID:       "user-1",
			DeviceID:     "device-a",
			SourceType:   "owntracks",
			TimestampUTC: time.Date(2026, 4, 21, 20, 29, 58, 0, time.UTC),
			Lat:          37.42199,
			Lon:          -122.08405,
			Accuracy:     &accuracy,
			RawPayload:   raw,
			IngestHash:   "abc123",
		},
	}

	data, err := SerializeRecord(input)
	if err != nil {
		t.Fatalf("SerializeRecord returned error: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("expected newline-delimited JSON record")
	}

	output, err := DeserializeRecord(data)
	if err != nil {
		t.Fatalf("DeserializeRecord returned error: %v", err)
	}

	if output.Seq != input.Seq {
		t.Fatalf("unexpected Seq: got %d want %d", output.Seq, input.Seq)
	}
	if output.DeviceID != input.DeviceID {
		t.Fatalf("unexpected DeviceID: got %q want %q", output.DeviceID, input.DeviceID)
	}
	if !output.ReceivedAt.Equal(input.ReceivedAt) {
		t.Fatalf("unexpected ReceivedAt: got %v want %v", output.ReceivedAt, input.ReceivedAt)
	}
	if output.Point.IngestHash != input.Point.IngestHash {
		t.Fatalf("unexpected point hash: got %q want %q", output.Point.IngestHash, input.Point.IngestHash)
	}
	if output.Point.Accuracy == nil || *output.Point.Accuracy != *input.Point.Accuracy {
		t.Fatalf("unexpected accuracy: got %v want %v", output.Point.Accuracy, input.Point.Accuracy)
	}
	if string(output.Point.RawPayload) != string(input.Point.RawPayload) {
		t.Fatalf("unexpected raw payload: got %q want %q", output.Point.RawPayload, input.Point.RawPayload)
	}
}

func TestDeserializeRecord_EmptyLine(t *testing.T) {
	_, err := DeserializeRecord([]byte(" \n\t "))
	if !errors.Is(err, ErrRecordEmptyLine) {
		t.Fatalf("expected ErrRecordEmptyLine, got %v", err)
	}
}

func TestDeserializeRecord_InvalidJSON(t *testing.T) {
	_, err := DeserializeRecord([]byte(`{"seq":1`))
	if !errors.Is(err, ErrRecordInvalid) {
		t.Fatalf("expected ErrRecordInvalid, got %v", err)
	}
}

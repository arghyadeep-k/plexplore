package ingest

import (
	"testing"
	"time"
)

func TestGenerateDeterministicIngestHash_IsStable(t *testing.T) {
	point := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 123456789, time.UTC),
		Lat:          37.4219983,
		Lon:          -122.0840575,
	}

	h1 := GenerateDeterministicIngestHash(point)
	h2 := GenerateDeterministicIngestHash(point)

	if h1 == "" || h2 == "" {
		t.Fatal("expected non-empty hashes")
	}
	if h1 != h2 {
		t.Fatalf("expected stable hash, got %q and %q", h1, h2)
	}
}

func TestGenerateDeterministicIngestHash_RoundsCoordinatesForDedupe(t *testing.T) {
	base := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "overland",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 0, time.UTC),
		Lat:          37.4219983,
		Lon:          -122.0840575,
	}
	nearby := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "overland",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 0, time.UTC),
		Lat:          37.42199834,
		Lon:          -122.08405749,
	}

	h1 := GenerateDeterministicIngestHash(base)
	h2 := GenerateDeterministicIngestHash(nearby)

	if h1 != h2 {
		t.Fatalf("expected equal hashes after coordinate rounding, got %q and %q", h1, h2)
	}
}

func TestGenerateDeterministicIngestHash_ChangesWhenIdentityChanges(t *testing.T) {
	point := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 0, time.UTC),
		Lat:          37.4219983,
		Lon:          -122.0840575,
	}
	changedDevice := point
	changedDevice.DeviceID = "device-b"

	if GenerateDeterministicIngestHash(point) == GenerateDeterministicIngestHash(changedDevice) {
		t.Fatal("expected hash difference when device id changes")
	}
}

func TestGenerateDeterministicIngestHash_NormalizesTimezone(t *testing.T) {
	utcPoint := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 20, 0, 1, 0, time.UTC),
		Lat:          37.4219983,
		Lon:          -122.0840575,
	}
	localPoint := CanonicalPoint{
		DeviceID:     "device-a",
		SourceType:   "owntracks",
		TimestampUTC: time.Date(2026, 4, 21, 15, 0, 1, 0, time.FixedZone("CDT", -5*3600)),
		Lat:          37.4219983,
		Lon:          -122.0840575,
	}

	h1 := GenerateDeterministicIngestHash(utcPoint)
	h2 := GenerateDeterministicIngestHash(localPoint)
	if h1 != h2 {
		t.Fatalf("expected same hash for same instant across timezones, got %q and %q", h1, h2)
	}
}

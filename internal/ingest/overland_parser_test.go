package ingest

import (
	"errors"
	"testing"
	"time"
)

func TestParseOverlandBatch_SuccessMultiLocation(t *testing.T) {
	raw := []byte(`{
		"device_id":"phone-01",
		"locations":[
			{
				"coordinates":[-122.084,37.4219983],
				"timestamp":"2026-04-21T20:00:01.123Z",
				"horizontal_accuracy":8.5,
				"altitude":21.0,
				"speed":0.9,
				"motion":"walking"
			},
			{
				"coordinates":[-122.085,37.4221],
				"timestamp":"2026-04-21T20:01:10Z",
				"horizontal_accuracy":10.0,
				"speed":1.2,
				"activity":{"type":"running"}
			}
		]
	}`)

	points, err := ParseOverlandBatch(raw)
	if err != nil {
		t.Fatalf("ParseOverlandBatch returned error: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}

	first := points[0]
	if first.SourceType != "overland" {
		t.Fatalf("expected SourceType overland, got %q", first.SourceType)
	}
	if first.DeviceID != "phone-01" {
		t.Fatalf("expected DeviceID phone-01, got %q", first.DeviceID)
	}
	if first.Lat != 37.4219983 || first.Lon != -122.084 {
		t.Fatalf("unexpected first coordinates lat=%v lon=%v", first.Lat, first.Lon)
	}
	if first.TimestampUTC.Location() != time.UTC {
		t.Fatalf("expected UTC timestamp, got %v", first.TimestampUTC.Location())
	}
	if first.MotionType == nil || *first.MotionType != "walking" {
		t.Fatalf("unexpected first motion type: %v", first.MotionType)
	}
	if first.RawPayload != nil {
		t.Fatalf("expected nil RawPayload for batch payload, got %d bytes", len(first.RawPayload))
	}

	second := points[1]
	if second.MotionType == nil || *second.MotionType != "running" {
		t.Fatalf("unexpected second motion type: %v", second.MotionType)
	}
	if second.Accuracy == nil || *second.Accuracy != 10.0 {
		t.Fatalf("unexpected second accuracy: %v", second.Accuracy)
	}
	if second.IngestHash == "" || first.IngestHash == second.IngestHash {
		t.Fatal("expected unique non-empty ingest hashes per location")
	}
}

func TestParseOverlandBatch_SuccessSingleLocationPreservesRawPayload(t *testing.T) {
	raw := []byte(`{
		"device_id":"device-a",
		"locations":[
			{
				"coordinates":[2.3522,48.8566],
				"timestamp":"2026-04-21T20:10:10Z"
			}
		]
	}`)

	points, err := ParseOverlandBatch(raw)
	if err != nil {
		t.Fatalf("ParseOverlandBatch returned error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if len(points[0].RawPayload) == 0 {
		t.Fatal("expected raw payload to be preserved for single-location payload")
	}
}

func TestParseOverlandBatch_MissingLocations(t *testing.T) {
	_, err := ParseOverlandBatch([]byte(`{"device_id":"x","locations":[]}`))
	if !errors.Is(err, ErrOverlandMissingLocations) {
		t.Fatalf("expected ErrOverlandMissingLocations, got %v", err)
	}
}

func TestParseOverlandBatch_InvalidCoordinates(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "coordinates too short",
			raw:  []byte(`{"locations":[{"coordinates":[1.0],"timestamp":"2026-04-21T20:10:10Z"}]}`),
		},
		{
			name: "latitude out of range",
			raw:  []byte(`{"locations":[{"coordinates":[1.0,100.0],"timestamp":"2026-04-21T20:10:10Z"}]}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOverlandBatch(tc.raw)
			if !errors.Is(err, ErrOverlandInvalidCoordinate) {
				t.Fatalf("expected ErrOverlandInvalidCoordinate, got %v", err)
			}
		})
	}
}

func TestParseOverlandBatch_InvalidTimestamp(t *testing.T) {
	_, err := ParseOverlandBatch([]byte(`{"locations":[{"coordinates":[1.0,2.0],"timestamp":"not-a-time"}]}`))
	if !errors.Is(err, ErrOverlandInvalidTimestamp) {
		t.Fatalf("expected ErrOverlandInvalidTimestamp, got %v", err)
	}
}

func TestParseOverlandBatch_InvalidJSON(t *testing.T) {
	_, err := ParseOverlandBatch([]byte(`{"locations":[`))
	if !errors.Is(err, ErrOverlandInvalidJSON) {
		t.Fatalf("expected ErrOverlandInvalidJSON, got %v", err)
	}
}

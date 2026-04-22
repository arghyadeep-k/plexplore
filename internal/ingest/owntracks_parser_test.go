package ingest

import (
	"errors"
	"testing"
	"time"
)

func TestParseOwnTracksLocation_SuccessWithTopicAndOptionals(t *testing.T) {
	raw := []byte(`{
		"_type":"location",
		"lat":37.4219983,
		"lon":-122.084,
		"tst":1713729600,
		"acc":12,
		"alt":22.5,
		"batt":88,
		"vel":0.4,
		"tid":"ab",
		"topic":"owntracks/alice/phone"
	}`)

	point, err := ParseOwnTracksLocation(raw)
	if err != nil {
		t.Fatalf("ParseOwnTracksLocation returned error: %v", err)
	}

	if point.SourceType != "owntracks" {
		t.Fatalf("expected SourceType owntracks, got %q", point.SourceType)
	}
	if point.UserID != "alice" {
		t.Fatalf("expected UserID alice, got %q", point.UserID)
	}
	if point.DeviceID != "phone" {
		t.Fatalf("expected DeviceID phone, got %q", point.DeviceID)
	}
	if point.Lat != 37.4219983 {
		t.Fatalf("unexpected Lat: %v", point.Lat)
	}
	if point.Lon != -122.084 {
		t.Fatalf("unexpected Lon: %v", point.Lon)
	}
	if point.TimestampUTC.Location() != time.UTC {
		t.Fatalf("timestamp is not UTC: %v", point.TimestampUTC.Location())
	}
	if point.TimestampUTC.Unix() != 1713729600 {
		t.Fatalf("unexpected timestamp unix value: %d", point.TimestampUTC.Unix())
	}
	if point.Accuracy == nil || *point.Accuracy != 12 {
		t.Fatalf("unexpected Accuracy: %v", point.Accuracy)
	}
	if point.Altitude == nil || *point.Altitude != 22.5 {
		t.Fatalf("unexpected Altitude: %v", point.Altitude)
	}
	if point.Battery == nil || *point.Battery != 88 {
		t.Fatalf("unexpected Battery: %v", point.Battery)
	}
	if point.Speed == nil || *point.Speed != 0.4 {
		t.Fatalf("unexpected Speed: %v", point.Speed)
	}
	if len(point.RawPayload) == 0 {
		t.Fatal("expected RawPayload to be preserved")
	}
	if point.IngestHash == "" {
		t.Fatal("expected IngestHash to be populated")
	}
}

func TestParseOwnTracksLocation_SuccessFallbackToTID(t *testing.T) {
	raw := []byte(`{"_type":"location","lat":48.8566,"lon":2.3522,"tst":1713729601,"tid":"zz"}`)

	point, err := ParseOwnTracksLocation(raw)
	if err != nil {
		t.Fatalf("ParseOwnTracksLocation returned error: %v", err)
	}

	if point.UserID != "" {
		t.Fatalf("expected empty UserID when topic missing, got %q", point.UserID)
	}
	if point.DeviceID != "zz" {
		t.Fatalf("expected DeviceID zz from tid fallback, got %q", point.DeviceID)
	}
}

func TestParseOwnTracksLocation_NotLocationType(t *testing.T) {
	raw := []byte(`{"_type":"card","lat":1.0,"lon":2.0,"tst":1713729600}`)

	_, err := ParseOwnTracksLocation(raw)
	if !errors.Is(err, ErrOwnTracksNotLocationEvent) {
		t.Fatalf("expected ErrOwnTracksNotLocationEvent, got %v", err)
	}
}

func TestParseOwnTracksLocation_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "missing lat",
			raw:  []byte(`{"_type":"location","lon":2.0,"tst":1713729600}`),
			want: ErrOwnTracksMissingLat,
		},
		{
			name: "missing lon",
			raw:  []byte(`{"_type":"location","lat":1.0,"tst":1713729600}`),
			want: ErrOwnTracksMissingLon,
		},
		{
			name: "missing tst",
			raw:  []byte(`{"_type":"location","lat":1.0,"lon":2.0}`),
			want: ErrOwnTracksMissingTimestamp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOwnTracksLocation(tc.raw)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestParseOwnTracksLocation_InvalidValues(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "lat out of range",
			raw:  []byte(`{"_type":"location","lat":100.0,"lon":2.0,"tst":1713729600}`),
			want: ErrOwnTracksInvalidLat,
		},
		{
			name: "lon out of range",
			raw:  []byte(`{"_type":"location","lat":1.0,"lon":200.0,"tst":1713729600}`),
			want: ErrOwnTracksInvalidLon,
		},
		{
			name: "invalid timestamp",
			raw:  []byte(`{"_type":"location","lat":1.0,"lon":2.0,"tst":0}`),
			want: ErrOwnTracksInvalidTimestamp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOwnTracksLocation(tc.raw)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestParseOwnTracksLocation_InvalidJSON(t *testing.T) {
	_, err := ParseOwnTracksLocation([]byte(`{"_type":"location",`))
	if !errors.Is(err, ErrOwnTracksInvalidJSON) {
		t.Fatalf("expected ErrOwnTracksInvalidJSON, got %v", err)
	}
}

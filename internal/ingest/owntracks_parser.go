package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrOwnTracksEmptyPayload      = errors.New("owntracks payload is empty")
	ErrOwnTracksInvalidJSON       = errors.New("owntracks payload is invalid json")
	ErrOwnTracksNotLocationEvent  = errors.New("owntracks payload is not a location event")
	ErrOwnTracksMissingLat        = errors.New("owntracks payload missing required field: lat")
	ErrOwnTracksMissingLon        = errors.New("owntracks payload missing required field: lon")
	ErrOwnTracksMissingTimestamp  = errors.New("owntracks payload missing required field: tst")
	ErrOwnTracksInvalidLat        = errors.New("owntracks payload has invalid lat")
	ErrOwnTracksInvalidLon        = errors.New("owntracks payload has invalid lon")
	ErrOwnTracksInvalidTimestamp  = errors.New("owntracks payload has invalid tst")
)

type ownTracksPayload struct {
	Type  string       `json:"_type"`
	Lat   *json.Number `json:"lat"`
	Lon   *json.Number `json:"lon"`
	Tst   *json.Number `json:"tst"`
	Acc   *json.Number `json:"acc"`
	Alt   *json.Number `json:"alt"`
	Batt  *json.Number `json:"batt"`
	Vel   *json.Number `json:"vel"`
	TID   string       `json:"tid"`
	Topic string       `json:"topic"`
}

// ParseOwnTracksLocation parses one OwnTracks JSON payload into one CanonicalPoint.
// It only accepts location events and validates required location fields.
func ParseOwnTracksLocation(raw []byte) (CanonicalPoint, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return CanonicalPoint{}, ErrOwnTracksEmptyPayload
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()

	var payload ownTracksPayload
	if err := decoder.Decode(&payload); err != nil {
		return CanonicalPoint{}, fmt.Errorf("%w: %v", ErrOwnTracksInvalidJSON, err)
	}

	if payload.Type != "location" {
		return CanonicalPoint{}, fmt.Errorf("%w: _type=%q", ErrOwnTracksNotLocationEvent, payload.Type)
	}
	if payload.Lat == nil {
		return CanonicalPoint{}, ErrOwnTracksMissingLat
	}
	if payload.Lon == nil {
		return CanonicalPoint{}, ErrOwnTracksMissingLon
	}
	if payload.Tst == nil {
		return CanonicalPoint{}, ErrOwnTracksMissingTimestamp
	}

	lat, err := payload.Lat.Float64()
	if err != nil || lat < -90 || lat > 90 {
		return CanonicalPoint{}, fmt.Errorf("%w: %v", ErrOwnTracksInvalidLat, payload.Lat)
	}

	lon, err := payload.Lon.Float64()
	if err != nil || lon < -180 || lon > 180 {
		return CanonicalPoint{}, fmt.Errorf("%w: %v", ErrOwnTracksInvalidLon, payload.Lon)
	}

	tstSec, err := payload.Tst.Int64()
	if err != nil || tstSec <= 0 {
		return CanonicalPoint{}, fmt.Errorf("%w: %v", ErrOwnTracksInvalidTimestamp, payload.Tst)
	}

	timestampUTC := time.Unix(tstSec, 0).UTC()
	userID, deviceFromTopic := parseTopic(payload.Topic)
	deviceID := firstNonEmpty(deviceFromTopic, payload.TID, "unknown-device")

	point := CanonicalPoint{
		UserID:       userID,
		DeviceID:     deviceID,
		SourceType:   "owntracks",
		TimestampUTC: timestampUTC,
		Lat:          lat,
		Lon:          lon,
		Accuracy:     jsonNumberToFloat64Ptr(payload.Acc),
		Altitude:     jsonNumberToFloat64Ptr(payload.Alt),
		Battery:      jsonNumberToFloat64Ptr(payload.Batt),
		Speed:        jsonNumberToFloat64Ptr(payload.Vel),
		RawPayload:   append([]byte(nil), []byte(trimmed)...),
		IngestHash:   hashPayload([]byte(trimmed)),
	}

	return point, nil
}

func parseTopic(topic string) (userID string, deviceID string) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) < 2 {
		return "", ""
	}

	if len(parts) >= 3 && parts[0] == "owntracks" {
		return parts[1], parts[2]
	}

	return parts[len(parts)-2], parts[len(parts)-1]
}

func jsonNumberToFloat64Ptr(n *json.Number) *float64 {
	if n == nil {
		return nil
	}
	value, err := n.Float64()
	if err != nil {
		return nil
	}
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hashPayload(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

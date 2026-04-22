package ingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrOverlandEmptyPayload      = errors.New("overland payload is empty")
	ErrOverlandInvalidJSON       = errors.New("overland payload is invalid json")
	ErrOverlandMissingLocations  = errors.New("overland payload missing required field: locations")
	ErrOverlandInvalidCoordinate = errors.New("overland payload has invalid coordinates")
	ErrOverlandInvalidTimestamp  = errors.New("overland payload has invalid timestamp")
)

type overlandBatchPayload struct {
	DeviceID  string             `json:"device_id"`
	Locations []overlandLocation `json:"locations"`
}

type overlandLocation struct {
	Coordinates        []float64       `json:"coordinates"`
	Timestamp          string          `json:"timestamp"`
	HorizontalAccuracy *float64        `json:"horizontal_accuracy"`
	Altitude           *float64        `json:"altitude"`
	Speed              *float64        `json:"speed"`
	MotionRaw          json.RawMessage `json:"motion"`
	ActivityRaw        json.RawMessage `json:"activity"`
}

// ParseOverlandBatch parses one Overland JSON payload into canonical points.
// For multi-location batches, RawPayload is omitted on each point to avoid
// duplicating large payload bytes in memory.
func ParseOverlandBatch(raw []byte) ([]CanonicalPoint, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, ErrOverlandEmptyPayload
	}

	var payload overlandBatchPayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOverlandInvalidJSON, err)
	}

	if len(payload.Locations) == 0 {
		return nil, ErrOverlandMissingLocations
	}

	deviceID := firstNonEmpty(payload.DeviceID, "unknown-device")
	points := make([]CanonicalPoint, 0, len(payload.Locations))

	for i, location := range payload.Locations {
		if len(location.Coordinates) < 2 {
			return nil, fmt.Errorf("%w at location index %d: expected [lon,lat]", ErrOverlandInvalidCoordinate, i)
		}

		lon := location.Coordinates[0]
		lat := location.Coordinates[1]
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, fmt.Errorf("%w at location index %d: lat=%v lon=%v", ErrOverlandInvalidCoordinate, i, lat, lon)
		}

		if strings.TrimSpace(location.Timestamp) == "" {
			return nil, fmt.Errorf("%w at location index %d: missing timestamp", ErrOverlandInvalidTimestamp, i)
		}
		parsedTS, err := time.Parse(time.RFC3339Nano, location.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("%w at location index %d: %v", ErrOverlandInvalidTimestamp, i, err)
		}

		point := CanonicalPoint{
			UserID:       "",
			DeviceID:     deviceID,
			SourceType:   "overland",
			TimestampUTC: parsedTS.UTC(),
			Lat:          lat,
			Lon:          lon,
			Accuracy:     location.HorizontalAccuracy,
			Altitude:     location.Altitude,
			Speed:        location.Speed,
			MotionType:   parseOverlandMotionType(location.MotionRaw, location.ActivityRaw),
			IngestHash:   hashPayload([]byte(fmt.Sprintf("%s#%d", trimmed, i))),
		}

		if len(payload.Locations) == 1 {
			point.RawPayload = append([]byte(nil), []byte(trimmed)...)
		}

		points = append(points, point)
	}

	return points, nil
}

func parseOverlandMotionType(motionRaw, activityRaw json.RawMessage) *string {
	motion := parseMotionString(motionRaw)
	if motion != "" {
		return &motion
	}

	activity := parseActivityString(activityRaw)
	if activity != "" {
		return &activity
	}

	return nil
}

func parseMotionString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return strings.TrimSpace(single)
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, item := range list {
			if value := strings.TrimSpace(item); value != "" {
				return value
			}
		}
	}

	return ""
}

type overlandActivity struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func parseActivityString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return strings.TrimSpace(single)
	}

	var activity overlandActivity
	if err := json.Unmarshal(raw, &activity); err == nil {
		return firstNonEmpty(strings.TrimSpace(activity.Type), strings.TrimSpace(activity.Name))
	}

	return ""
}

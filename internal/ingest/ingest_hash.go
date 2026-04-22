package ingest

import (
	"fmt"
	"math"
	"time"
)

const hashCoordinatePrecision = 5

// GenerateDeterministicIngestHash returns a stable hash for dedupe-oriented
// point identity. It only depends on source type, device id, UTC timestamp,
// and rounded coordinates, so equivalent points hash the same.
func GenerateDeterministicIngestHash(point CanonicalPoint) string {
	roundedLat := roundCoordinate(point.Lat, hashCoordinatePrecision)
	roundedLon := roundCoordinate(point.Lon, hashCoordinatePrecision)
	normalizedTS := point.TimestampUTC.UTC().Format(time.RFC3339Nano)

	representation := fmt.Sprintf(
		"src=%s|dev=%s|ts=%s|lat=%.5f|lon=%.5f",
		point.SourceType,
		point.DeviceID,
		normalizedTS,
		roundedLat,
		roundedLon,
	)

	return hashPayload([]byte(representation))
}

func roundCoordinate(value float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(value*pow) / pow
}

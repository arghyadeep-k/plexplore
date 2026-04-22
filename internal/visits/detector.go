package visits

import (
	"math"
	"time"
)

const earthRadiusMeters = 6371000.0

// Config controls lightweight visit detection behavior.
type Config struct {
	MinDwell        time.Duration
	MaxRadiusMeters float64
}

// Point is the minimal input shape for visit detection.
type Point struct {
	TimestampUTC time.Time
	Lat          float64
	Lon          float64
}

// Visit is the detector output before persistence.
type Visit struct {
	StartAt     time.Time
	EndAt       time.Time
	CentroidLat float64
	CentroidLon float64
	PointCount  int
}

// Detect finds visit windows from timestamp-ascending points where device
// remains within MaxRadiusMeters for at least MinDwell.
func Detect(points []Point, cfg Config) []Visit {
	if len(points) < 2 {
		return nil
	}
	if cfg.MinDwell <= 0 || cfg.MaxRadiusMeters <= 0 {
		return nil
	}

	out := make([]Visit, 0)
	for i := 0; i < len(points); {
		if points[i].TimestampUTC.IsZero() {
			i++
			continue
		}

		sumLat := points[i].Lat
		sumLon := points[i].Lon
		end := i

		for j := i + 1; j < len(points); j++ {
			if points[j].TimestampUTC.IsZero() {
				break
			}
			trialCount := j - i + 1
			trialSumLat := sumLat + points[j].Lat
			trialSumLon := sumLon + points[j].Lon
			centroidLat := trialSumLat / float64(trialCount)
			centroidLon := trialSumLon / float64(trialCount)

			if windowWithinRadius(points, i, j, centroidLat, centroidLon, cfg.MaxRadiusMeters) {
				sumLat = trialSumLat
				sumLon = trialSumLon
				end = j
				continue
			}
			break
		}

		if end > i {
			startAt := points[i].TimestampUTC.UTC()
			endAt := points[end].TimestampUTC.UTC()
			if endAt.Sub(startAt) >= cfg.MinDwell {
				count := end - i + 1
				out = append(out, Visit{
					StartAt:     startAt,
					EndAt:       endAt,
					CentroidLat: sumLat / float64(count),
					CentroidLon: sumLon / float64(count),
					PointCount:  count,
				})
				i = end + 1
				continue
			}
		}

		i++
	}

	return out
}

func windowWithinRadius(points []Point, start, end int, centroidLat, centroidLon, maxRadiusMeters float64) bool {
	for i := start; i <= end; i++ {
		if haversineMeters(points[i].Lat, points[i].Lon, centroidLat, centroidLon) > maxRadiusMeters {
			return false
		}
	}
	return true
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

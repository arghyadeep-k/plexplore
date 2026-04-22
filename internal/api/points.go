package api

import (
	"net/http"
	"strconv"
	"strings"

	"plexplore/internal/store"
)

type recentPointResponse struct {
	Seq          uint64  `json:"seq"`
	DeviceID     string  `json:"device_id"`
	SourceType   string  `json:"source_type"`
	TimestampUTC string  `json:"timestamp_utc"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
}

type recentPointsResponse struct {
	Points []recentPointResponse `json:"points"`
}

func registerPointRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/points/recent", recentPointsHandler(deps.PointStore))
}

func recentPointsHandler(pointStore PointStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))

		limit := 50
		limitParam := strings.TrimSpace(r.URL.Query().Get("limit"))
		if limitParam != "" {
			parsed, err := strconv.Atoi(limitParam)
			if err != nil || parsed <= 0 {
				writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			limit = parsed
		}

		points, err := pointStore.ListRecentPoints(r.Context(), deviceID, limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]recentPointResponse, 0, len(points))
		for _, point := range points {
			out = append(out, recentPointResponse{
				Seq:          point.Seq,
				DeviceID:     point.DeviceID,
				SourceType:   point.SourceType,
				TimestampUTC: point.TimestampUTC.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
				Lat:          point.Lat,
				Lon:          point.Lon,
			})
		}

		writeJSON(w, http.StatusOK, recentPointsResponse{Points: out})
	}
}

func recentPointFromStore(point store.RecentPoint) recentPointResponse {
	return recentPointResponse{
		Seq:          point.Seq,
		DeviceID:     point.DeviceID,
		SourceType:   point.SourceType,
		TimestampUTC: point.TimestampUTC.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		Lat:          point.Lat,
		Lon:          point.Lon,
	}
}

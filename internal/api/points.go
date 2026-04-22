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
	mux.HandleFunc("GET /api/v1/points", pointsHandler(deps.PointStore))
	mux.HandleFunc("GET /api/v1/points/recent", recentPointsHandler(deps.PointStore))
}

func pointsHandler(pointStore PointStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := exportFilterFromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		limit, err := parseOptionalLimitParam(r, 500)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		points, err := pointStore.ListPoints(r.Context(), filter, limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]recentPointResponse, 0, len(points))
		for _, point := range points {
			out = append(out, recentPointFromStore(point))
		}
		writeJSON(w, http.StatusOK, recentPointsResponse{Points: out})
	}
}

func recentPointsHandler(pointStore PointStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))

		limit, err := parseOptionalLimitParam(r, 50)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
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

func parseOptionalLimitParam(r *http.Request, fallback int) (int, error) {
	limit := fallback
	limitParam := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitParam == "" {
		return limit, nil
	}

	parsed, err := strconv.Atoi(limitParam)
	if err != nil || parsed <= 0 {
		return 0, &invalidLimitParamError{value: limitParam}
	}
	return parsed, nil
}

type invalidLimitParamError struct {
	value string
}

func (e *invalidLimitParamError) Error() string {
	return "limit must be a positive integer: " + e.value
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

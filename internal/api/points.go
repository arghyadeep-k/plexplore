package api

import (
	"math"
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

type pointsPageResponse struct {
	Points      []recentPointResponse `json:"points"`
	NextCursor  *uint64               `json:"next_cursor,omitempty"`
	Sampled     bool                  `json:"sampled,omitempty"`
	SampledFrom int                   `json:"sampled_from,omitempty"`
}

const (
	defaultPointsLimit           = 500
	maxPointsLimit               = 1000
	defaultSimplifiedPointsLimit = 5000
	maxSimplifiedPointsLimit     = 20000
	defaultSimplifiedMaxPoints   = 1000
	maxSimplifiedMaxPoints       = 5000
)

func registerPointRoutes(mux *http.ServeMux, deps Dependencies) {
	if deps.PointStore == nil {
		panic("registerPointRoutes requires non-nil pointStore")
	}
	if deps.DeviceStore == nil || deps.UserStore == nil || deps.SessionStore == nil {
		panic("registerPointRoutes requires non-nil deviceStore, userStore, and sessionStore")
	}
	mux.Handle(
		"GET /api/v1/points",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(pointsHandler(deps.PointStore, deps.DeviceStore))),
		),
	)
	mux.Handle(
		"GET /api/v1/points/recent",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(recentPointsHandler(deps.PointStore, deps.DeviceStore))),
		),
	)
}

func pointsHandler(pointStore PointStore, deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := exportFilterFromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		currentUser, ok := CurrentUserFromContext(r.Context())
		if deviceStore != nil && !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if ok {
			filter.UserID = currentUser.ID
		}

		limit, err := parseOptionalLimitParamWithMax(r, defaultPointsLimit, maxPointsLimit)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		cursor, err := parseOptionalCursorParam(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if cursor > 0 {
			filter.AfterSeq = cursor
		}
		simplify := parseOptionalBoolParam(r.URL.Query().Get("simplify"))
		simplifiedMaxPoints := defaultSimplifiedMaxPoints
		if simplify {
			simplifiedMaxPoints, err = parseOptionalMaxPointsParam(r, defaultSimplifiedMaxPoints, maxSimplifiedMaxPoints)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			limit, err = parseOptionalLimitParamWithMax(r, defaultSimplifiedPointsLimit, maxSimplifiedPointsLimit)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		points, err := pointStore.ListPoints(r.Context(), filter, limit+1)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		hasMore := len(points) > limit
		if hasMore {
			points = points[:limit]
		}
		originalCount := len(points)
		if simplify {
			points = downsampleRecentPoints(points, simplifiedMaxPoints)
		}

		out := make([]recentPointResponse, 0, len(points))
		for _, point := range points {
			out = append(out, recentPointFromStore(point))
		}
		resp := pointsPageResponse{Points: out}
		if hasMore && len(points) > 0 {
			next := points[len(points)-1].Seq
			resp.NextCursor = &next
		}
		if simplify && originalCount > len(points) {
			resp.Sampled = true
			resp.SampledFrom = originalCount
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func recentPointsHandler(pointStore PointStore, deviceStore DeviceStore) http.HandlerFunc {
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

		if deviceStore != nil {
			currentUser, ok := CurrentUserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			allowedDeviceIDs, err := currentUserAllowedDeviceIDs(r, deviceStore)
			if err != nil {
				writeJSONError(w, httpStatusFromOwnershipError(err), err.Error())
				return
			}

			filtered := make([]store.RecentPoint, 0, len(points))
			for _, point := range points {
				if _, allowed := allowedDeviceIDs[point.DeviceID]; allowed && point.UserID == currentUser.ID {
					filtered = append(filtered, point)
				}
			}
			points = filtered
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

func currentUserAllowedDeviceIDs(r *http.Request, deviceStore DeviceStore) (map[string]struct{}, error) {
	currentUser, ok := CurrentUserFromContext(r.Context())
	if !ok {
		return nil, errAuthRequired
	}
	devices, err := deviceStore.ListDevices(r.Context())
	if err != nil {
		return nil, errDeviceLookupFailed
	}
	allowedDeviceIDs := make(map[string]struct{})
	for _, d := range devices {
		if d.UserID == currentUser.ID {
			allowedDeviceIDs[d.Name] = struct{}{}
		}
	}
	return allowedDeviceIDs, nil
}

func httpStatusFromOwnershipError(err error) int {
	switch err {
	case errAuthRequired:
		return http.StatusUnauthorized
	case errDeviceLookupFailed:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

var (
	errAuthRequired       = &ownershipError{message: "authentication required"}
	errDeviceLookupFailed = &ownershipError{message: "device lookup failed"}
)

type ownershipError struct {
	message string
}

func (e *ownershipError) Error() string {
	return e.message
}

func parseOptionalLimitParam(r *http.Request, fallback int) (int, error) {
	return parseOptionalLimitParamWithMax(r, fallback, 0)
}

func parseOptionalLimitParamWithMax(r *http.Request, fallback, maxAllowed int) (int, error) {
	limit := fallback
	limitParam := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitParam == "" {
		return limit, nil
	}

	parsed, err := strconv.Atoi(limitParam)
	if err != nil || parsed <= 0 {
		return 0, &invalidLimitParamError{value: limitParam}
	}
	if maxAllowed > 0 && parsed > maxAllowed {
		return maxAllowed, nil
	}
	return parsed, nil
}

func parseOptionalCursorParam(r *http.Request) (uint64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, &invalidCursorParamError{value: raw}
	}
	return parsed, nil
}

func parseOptionalBoolParam(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseOptionalMaxPointsParam(r *http.Request, fallback, maxAllowed int) (int, error) {
	maxPoints := fallback
	maxPointsParam := strings.TrimSpace(r.URL.Query().Get("max_points"))
	if maxPointsParam == "" {
		return maxPoints, nil
	}
	parsed, err := strconv.Atoi(maxPointsParam)
	if err != nil || parsed <= 0 {
		return 0, &invalidMaxPointsParamError{value: maxPointsParam}
	}
	if parsed > maxAllowed {
		return maxAllowed, nil
	}
	return parsed, nil
}

type invalidLimitParamError struct {
	value string
}

func (e *invalidLimitParamError) Error() string {
	return "limit must be a positive integer: " + e.value
}

type invalidCursorParamError struct {
	value string
}

func (e *invalidCursorParamError) Error() string {
	return "cursor must be a positive integer: " + e.value
}

type invalidMaxPointsParamError struct {
	value string
}

func (e *invalidMaxPointsParamError) Error() string {
	return "max_points must be a positive integer: " + e.value
}

func downsampleRecentPoints(points []store.RecentPoint, maxPoints int) []store.RecentPoint {
	if len(points) <= maxPoints || maxPoints <= 0 {
		return points
	}
	if maxPoints == 1 {
		return []store.RecentPoint{points[len(points)-1]}
	}
	step := float64(len(points)-1) / float64(maxPoints-1)
	out := make([]store.RecentPoint, 0, maxPoints)
	prev := -1
	for i := 0; i < maxPoints; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx <= prev {
			idx = prev + 1
		}
		if idx >= len(points) {
			idx = len(points) - 1
		}
		out = append(out, points[idx])
		prev = idx
	}
	return out
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

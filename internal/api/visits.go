package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"plexplore/internal/store"
	"plexplore/internal/visits"
)

type generateVisitsResponse struct {
	OK            bool   `json:"ok"`
	DeviceID      int64  `json:"device_id"`
	FromUTC       string `json:"from_utc,omitempty"`
	ToUTC         string `json:"to_utc,omitempty"`
	CreatedVisits int    `json:"created_visits"`
}

type visitResponse struct {
	ID          int64   `json:"id"`
	DeviceID    int64   `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	StartAt     string  `json:"start_at"`
	EndAt       string  `json:"end_at"`
	CentroidLat float64 `json:"centroid_lat"`
	CentroidLon float64 `json:"centroid_lon"`
	PointCount  int     `json:"point_count"`
	PlaceLabel  string  `json:"place_label,omitempty"`
}

type listVisitsResponse struct {
	Visits []visitResponse `json:"visits"`
}

func registerVisitRoutes(mux *http.ServeMux, deps Dependencies) {
	if deps.VisitStore == nil {
		panic("registerVisitRoutes requires non-nil visitStore")
	}
	if deps.DeviceStore == nil || deps.UserStore == nil || deps.SessionStore == nil {
		panic("registerVisitRoutes requires non-nil deviceStore, userStore, and sessionStore")
	}
	mux.Handle(
		"POST /api/v1/visits/generate",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(generateVisitsHandler(deps.VisitStore, deps.DeviceStore))),
		),
	)
	mux.Handle(
		"GET /api/v1/visits",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(listVisitsHandler(deps.VisitStore, deps.VisitLabelResolver, deps.DeviceStore))),
		),
	)
}

func generateVisitsHandler(visitStore VisitStore, deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateCSRF(r) {
			writeJSONError(w, http.StatusForbidden, "csrf token invalid")
			return
		}

		deviceID, err := parseRequiredDeviceIDParam(r.URL.Query().Get("device_id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "device_id is required")
			return
		}
		if deviceStore != nil {
			allowedDevices, err := currentUserAllowedDevicesByID(r, deviceStore)
			if err != nil {
				writeJSONError(w, httpStatusFromOwnershipError(err), err.Error())
				return
			}
			if _, ok := allowedDevices[deviceID]; !ok {
				writeJSONError(w, http.StatusNotFound, "device not found")
				return
			}
		}

		fromUTC, err := parseOptionalRFC3339Param(r.URL.Query().Get("from"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		toUTC, err := parseOptionalRFC3339Param(r.URL.Query().Get("to"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		applyDefaultVisitGenerationWindow(&fromUTC, &toUTC)
		if fromUTC != nil && toUTC != nil && fromUTC.After(*toUTC) {
			writeJSONError(w, http.StatusBadRequest, "from must be <= to")
			return
		}

		minDwell, err := parseDurationParamOrDefault(r.URL.Query().Get("min_dwell"), 15*time.Minute)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		maxRadius, err := parsePositiveFloatParamOrDefault(r.URL.Query().Get("max_radius_m"), 35)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		created, err := visitStore.RebuildVisitsForDeviceRange(r.Context(), deviceID, fromUTC, toUTC, visits.Config{
			MinDwell:        minDwell,
			MaxRadiusMeters: maxRadius,
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := generateVisitsResponse{
			OK:            true,
			DeviceID:      deviceID,
			CreatedVisits: created,
		}
		if fromUTC != nil {
			resp.FromUTC = fromUTC.UTC().Format(time.RFC3339Nano)
		}
		if toUTC != nil {
			resp.ToUTC = toUTC.UTC().Format(time.RFC3339Nano)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func listVisitsHandler(visitStore VisitStore, labelResolver VisitLabelResolver, deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := CurrentUserFromContext(r.Context())
		if deviceStore != nil && !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID := int64(1)
		if ok {
			userID = currentUser.ID
		}

		deviceID, hasDeviceID, err := parseOptionalDeviceIDParam(r.URL.Query().Get("device_id"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if hasDeviceID && deviceStore != nil {
			allowedDevices, allowedErr := currentUserAllowedDevicesByID(r, deviceStore)
			if allowedErr != nil {
				writeJSONError(w, httpStatusFromOwnershipError(allowedErr), allowedErr.Error())
				return
			}
			if _, allowed := allowedDevices[deviceID]; !allowed {
				writeJSON(w, http.StatusOK, listVisitsResponse{Visits: []visitResponse{}})
				return
			}
		}

		fromUTC, err := parseOptionalRFC3339Param(r.URL.Query().Get("from"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		toUTC, err := parseOptionalRFC3339Param(r.URL.Query().Get("to"))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if fromUTC != nil && toUTC != nil && fromUTC.After(*toUTC) {
			writeJSONError(w, http.StatusBadRequest, "from must be <= to")
			return
		}
		limit, err := parseOptionalLimitParam(r, 100)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		var deviceIDFilter *int64
		if hasDeviceID {
			deviceIDFilter = &deviceID
		}

		items, err := visitStore.ListVisits(r.Context(), userID, deviceIDFilter, fromUTC, toUTC, limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		remainingProviderLookups := 0
		useResolver := labelResolver != nil && labelResolver.Enabled()
		if useResolver {
			remainingProviderLookups = labelResolver.MaxProviderLookupsPerRequest()
			if remainingProviderLookups < 0 {
				remainingProviderLookups = 0
			}
		}

		out := make([]visitResponse, 0, len(items))
		for _, item := range items {
			resp := visitResponse{
				ID:          item.ID,
				DeviceID:    item.DeviceRowID,
				DeviceName:  item.DeviceID,
				StartAt:     item.StartAt.UTC().Format(time.RFC3339Nano),
				EndAt:       item.EndAt.UTC().Format(time.RFC3339Nano),
				CentroidLat: item.CentroidLat,
				CentroidLon: item.CentroidLon,
				PointCount:  item.PointCount,
			}
			if useResolver {
				allowProvider := remainingProviderLookups > 0
				label, usedProvider, resolveErr := labelResolver.ResolveVisitLabel(
					r.Context(),
					item.CentroidLat,
					item.CentroidLon,
					allowProvider,
				)
				if usedProvider && remainingProviderLookups > 0 {
					remainingProviderLookups--
				}
				if resolveErr == nil {
					resp.PlaceLabel = strings.TrimSpace(label)
				}
			}
			out = append(out, resp)
		}
		writeJSON(w, http.StatusOK, listVisitsResponse{Visits: out})
	}
}

func parseRequiredDeviceIDParam(raw string) (int64, error) {
	deviceID, hasValue, err := parseOptionalDeviceIDParam(raw)
	if err != nil {
		return 0, err
	}
	if !hasValue {
		return 0, &invalidDeviceIDParamError{value: strings.TrimSpace(raw)}
	}
	return deviceID, nil
}

func parseOptionalDeviceIDParam(raw string) (int64, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false, &invalidDeviceIDParamError{value: trimmed}
	}
	return parsed, true, nil
}

type invalidDeviceIDParamError struct {
	value string
}

func (e *invalidDeviceIDParamError) Error() string {
	return "device_id must be a positive integer: " + e.value
}

func currentUserAllowedDevicesByID(r *http.Request, deviceStore DeviceStore) (map[int64]store.Device, error) {
	currentUser, ok := CurrentUserFromContext(r.Context())
	if !ok {
		return nil, errAuthRequired
	}
	devices, err := deviceStore.ListDevices(r.Context())
	if err != nil {
		return nil, errDeviceLookupFailed
	}
	allowed := make(map[int64]store.Device)
	for _, d := range devices {
		if d.UserID == currentUser.ID {
			allowed[d.ID] = d
		}
	}
	return allowed, nil
}

func applyDefaultVisitGenerationWindow(fromUTC, toUTC **time.Time) {
	now := time.Now().UTC()
	if *toUTC == nil {
		to := now
		*toUTC = &to
	}
	if *fromUTC == nil {
		from := (*toUTC).Add(-14 * 24 * time.Hour)
		*fromUTC = &from
	}
}

func parseDurationParamOrDefault(raw string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	dur, err := time.ParseDuration(value)
	if err != nil || dur <= 0 {
		return 0, &invalidDurationParamError{value: value}
	}
	return dur, nil
}

type invalidDurationParamError struct {
	value string
}

func (e *invalidDurationParamError) Error() string {
	return "duration must be positive (example: 15m): " + e.value
}

func parsePositiveFloatParamOrDefault(raw string, fallback float64) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, &invalidFloatParamError{value: value}
	}
	return parsed, nil
}

type invalidFloatParamError struct {
	value string
}

func (e *invalidFloatParamError) Error() string {
	return "max_radius_m must be a positive number: " + e.value
}

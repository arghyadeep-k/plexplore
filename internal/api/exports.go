package api

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"plexplore/internal/store"
)

type geoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

type geoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   geoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type geoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

func registerExportRoutes(mux *http.ServeMux, deps Dependencies) {
	if deps.PointStore == nil {
		panic("registerExportRoutes requires non-nil pointStore")
	}
	if deps.DeviceStore == nil || deps.UserStore == nil || deps.SessionStore == nil {
		panic("registerExportRoutes requires non-nil deviceStore, userStore, and sessionStore")
	}
	mux.Handle(
		"GET /api/v1/exports/geojson",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(geoJSONExportHandler(deps.PointStore, deps.DeviceStore))),
		),
	)
	mux.Handle(
		"GET /api/v1/exports/gpx",
		LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.HandlerFunc(gpxExportHandler(deps.PointStore, deps.DeviceStore))),
		),
	)
}

func geoJSONExportHandler(pointStore PointStore, deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := exportFilterFromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		allowedDeviceIDs := map[string]struct{}{}
		currentUserID := int64(0)
		if deviceStore != nil {
			currentUser, ok := CurrentUserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			currentUserID = currentUser.ID
			allowedDeviceIDs, err = currentUserAllowedDeviceIDs(r, deviceStore)
			if err != nil {
				writeJSONError(w, httpStatusFromOwnershipError(err), err.Error())
				return
			}
			if filter.DeviceID != "" {
				if _, ok := allowedDeviceIDs[filter.DeviceID]; !ok {
					writeJSON(w, http.StatusOK, geoJSONFeatureCollection{Type: "FeatureCollection", Features: []geoJSONFeature{}})
					return
				}
			}
		}

		points, err := pointStore.ListPointsForExport(r.Context(), filter)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if deviceStore != nil {
			filtered := make([]store.RecentPoint, 0, len(points))
			for _, point := range points {
				if _, ok := allowedDeviceIDs[point.DeviceID]; ok && point.UserID == currentUserID {
					filtered = append(filtered, point)
				}
			}
			points = filtered
		}

		features := make([]geoJSONFeature, 0, len(points))
		for _, point := range points {
			features = append(features, geoJSONFeature{
				Type: "Feature",
				Geometry: geoJSONGeometry{
					Type:        "Point",
					Coordinates: []float64{point.Lon, point.Lat},
				},
				Properties: map[string]interface{}{
					"seq":           point.Seq,
					"device_id":     point.DeviceID,
					"source_type":   point.SourceType,
					"timestamp_utc": point.TimestampUTC.UTC().Format(time.RFC3339Nano),
				},
			})
		}

		writeJSON(w, http.StatusOK, geoJSONFeatureCollection{
			Type:     "FeatureCollection",
			Features: features,
		})
	}
}

func exportFilterFromRequest(r *http.Request) (store.ExportPointFilter, error) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))

	from, err := parseOptionalRFC3339Param(r.URL.Query().Get("from"))
	if err != nil {
		return store.ExportPointFilter{}, err
	}
	to, err := parseOptionalRFC3339Param(r.URL.Query().Get("to"))
	if err != nil {
		return store.ExportPointFilter{}, err
	}

	return store.ExportPointFilter{
		DeviceID: deviceID,
		FromUTC:  from,
		ToUTC:    to,
	}, nil
}

func parseOptionalRFC3339Param(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, &invalidTimestampParamError{value: value}
	}
	utc := parsed.UTC()
	return &utc, nil
}

type invalidTimestampParamError struct {
	value string
}

func (e *invalidTimestampParamError) Error() string {
	return "timestamp must be RFC3339: " + e.value
}

type gpxDocument struct {
	XMLName xml.Name `xml:"gpx"`
	Version string   `xml:"version,attr"`
	Creator string   `xml:"creator,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Track   gpxTrack `xml:"trk"`
}

type gpxTrack struct {
	Name    string      `xml:"name,omitempty"`
	Segment gpxTrackSeg `xml:"trkseg"`
}

type gpxTrackSeg struct {
	Points []gpxTrackPt `xml:"trkpt"`
}

type gpxTrackPt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time"`
}

func gpxExportHandler(pointStore PointStore, deviceStore DeviceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := exportFilterFromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		allowedDeviceIDs := map[string]struct{}{}
		currentUserID := int64(0)
		if deviceStore != nil {
			currentUser, ok := CurrentUserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			currentUserID = currentUser.ID
			allowedDeviceIDs, err = currentUserAllowedDeviceIDs(r, deviceStore)
			if err != nil {
				writeJSONError(w, httpStatusFromOwnershipError(err), err.Error())
				return
			}
			if filter.DeviceID != "" {
				if _, ok := allowedDeviceIDs[filter.DeviceID]; !ok {
					writeEmptyGPX(w)
					return
				}
			}
		}

		points, err := pointStore.ListPointsForExport(r.Context(), filter)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if deviceStore != nil {
			filtered := make([]store.RecentPoint, 0, len(points))
			for _, point := range points {
				if _, ok := allowedDeviceIDs[point.DeviceID]; ok && point.UserID == currentUserID {
					filtered = append(filtered, point)
				}
			}
			points = filtered
		}

		trackPoints := make([]gpxTrackPt, 0, len(points))
		for _, point := range points {
			trackPoints = append(trackPoints, gpxTrackPt{
				Lat:  point.Lat,
				Lon:  point.Lon,
				Time: point.TimestampUTC.UTC().Format(time.RFC3339Nano),
			})
		}

		doc := gpxDocument{
			Version: "1.1",
			Creator: "plexplore",
			XMLNS:   "http://www.topografix.com/GPX/1/1",
			Track: gpxTrack{
				Name: "plexplore-export",
				Segment: gpxTrackSeg{
					Points: trackPoints,
				},
			},
		}

		writeGPXDoc(w, doc)
	}
}

func writeEmptyGPX(w http.ResponseWriter) {
	writeGPXDoc(w, gpxDocument{
		Version: "1.1",
		Creator: "plexplore",
		XMLNS:   "http://www.topografix.com/GPX/1/1",
		Track: gpxTrack{
			Name: "plexplore-export",
		},
	})
}

func writeGPXDoc(w http.ResponseWriter, doc gpxDocument) {
	output, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to encode gpx")
		return
	}

	w.Header().Set("Content-Type", "application/gpx+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(output)
}

package api

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"plexplore/internal/store"
)

const (
	defaultExportLimit = 5000
	maxExportLimit     = 20000
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
		currentUser, ok := CurrentUserFromContext(r.Context())
		if deviceStore != nil && !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if ok {
			filter.UserID = currentUser.ID
		}
		if filter.DeviceRowID != nil && deviceStore != nil {
			allowedDevices, allowedErr := currentUserAllowedDeviceIDs(r, deviceStore)
			if allowedErr != nil {
				writeJSONError(w, httpStatusFromOwnershipError(allowedErr), allowedErr.Error())
				return
			}
			if _, allowed := allowedDevices[*filter.DeviceRowID]; !allowed {
				writeJSON(w, http.StatusOK, geoJSONFeatureCollection{Type: "FeatureCollection", Features: []geoJSONFeature{}})
				return
			}
		}

		limit, err := parseOptionalLimitParamWithMax(r, defaultExportLimit, maxExportLimit)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="plexplore-export.geojson"`)
		w.WriteHeader(http.StatusOK)

		bw := bufio.NewWriterSize(w, 16*1024)
		_, _ = bw.WriteString(`{"type":"FeatureCollection","features":[`)

		first := true
		_, err = pointStore.StreamPointsForExport(r.Context(), filter, limit, func(point store.RecentPoint) error {
			feature := geoJSONFeature{
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
			}
			blob, marshalErr := json.Marshal(feature)
			if marshalErr != nil {
				return marshalErr
			}
			if !first {
				if _, writeErr := bw.WriteString(","); writeErr != nil {
					return writeErr
				}
			}
			first = false
			_, writeErr := bw.Write(blob)
			return writeErr
		})
		if err != nil {
			http.Error(w, "failed to stream geojson export", http.StatusInternalServerError)
			return
		}
		_, _ = bw.WriteString("]}")
		_ = bw.Flush()
	}
}

func exportFilterFromRequest(r *http.Request) (store.ExportPointFilter, error) {
	deviceID, hasDeviceID, err := parseOptionalDeviceIDParam(r.URL.Query().Get("device_id"))
	if err != nil {
		return store.ExportPointFilter{}, err
	}

	from, err := parseOptionalRFC3339Param(r.URL.Query().Get("from"))
	if err != nil {
		return store.ExportPointFilter{}, err
	}
	to, err := parseOptionalRFC3339Param(r.URL.Query().Get("to"))
	if err != nil {
		return store.ExportPointFilter{}, err
	}

	filter := store.ExportPointFilter{
		FromUTC: from,
		ToUTC:   to,
	}
	if hasDeviceID {
		filter.DeviceRowID = &deviceID
	}
	return filter, nil
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
		currentUser, ok := CurrentUserFromContext(r.Context())
		if deviceStore != nil && !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if ok {
			filter.UserID = currentUser.ID
		}
		if filter.DeviceRowID != nil && deviceStore != nil {
			allowedDevices, allowedErr := currentUserAllowedDeviceIDs(r, deviceStore)
			if allowedErr != nil {
				writeJSONError(w, httpStatusFromOwnershipError(allowedErr), allowedErr.Error())
				return
			}
			if _, allowed := allowedDevices[*filter.DeviceRowID]; !allowed {
				writeEmptyGPX(w)
				return
			}
		}

		limit, err := parseOptionalLimitParamWithMax(r, defaultExportLimit, maxExportLimit)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/gpx+xml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="plexplore-export.gpx"`)
		w.WriteHeader(http.StatusOK)

		bw := bufio.NewWriterSize(w, 16*1024)
		_, _ = bw.WriteString(xml.Header)
		_, _ = bw.WriteString(`<gpx version="1.1" creator="plexplore" xmlns="http://www.topografix.com/GPX/1/1">`)
		_, _ = bw.WriteString(`<trk><name>plexplore-export</name><trkseg>`)

		_, err = pointStore.StreamPointsForExport(r.Context(), filter, limit, func(point store.RecentPoint) error {
			_, writeErr := fmt.Fprintf(
				bw,
				`<trkpt lat="%f" lon="%f"><time>%s</time></trkpt>`,
				point.Lat,
				point.Lon,
				point.TimestampUTC.UTC().Format(time.RFC3339Nano),
			)
			return writeErr
		})
		if err != nil {
			http.Error(w, "failed to stream gpx export", http.StatusInternalServerError)
			return
		}
		_, _ = bw.WriteString(`</trkseg></trk></gpx>`)
		_ = bw.Flush()
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

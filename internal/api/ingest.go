package api

import (
	"io"
	"net/http"
	"strconv"

	"plexplore/internal/buffer"
	"plexplore/internal/ingest"
	"plexplore/internal/store"
)

const maxIngestBodyBytes = 1024 * 1024

type ingestSuccessResponse struct {
	OK       bool   `json:"ok"`
	Source   string `json:"source"`
	Accepted int    `json:"accepted"`
	Spooled  int    `json:"spooled"`
	Enqueued int    `json:"enqueued"`
	MaxSeq   uint64 `json:"max_seq"`
}

func registerIngestRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.Handle("POST /api/v1/owntracks", RequireDeviceAPIKeyAuth(deps.DeviceStore, http.HandlerFunc(ownTracksIngestHandler(deps))))
	mux.Handle("POST /api/v1/overland/batches", RequireDeviceAPIKeyAuth(deps.DeviceStore, http.HandlerFunc(overlandIngestHandler(deps))))
}

func ownTracksIngestHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.IsDraining != nil && deps.IsDraining() {
			writeJSONError(w, http.StatusServiceUnavailable, "service is shutting down")
			return
		}

		raw, err := readBoundedBody(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		point, err := ingest.ParseOwnTracksLocation(raw)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		device, ok := DeviceFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "missing authenticated device")
			return
		}

		points := []ingest.CanonicalPoint{point}
		normalizeForAuthenticatedDevice(points, device)
		ensureIngestHashes(points)

		response, err := appendAndEnqueue(deps, points, "owntracks")
		if err != nil {
			writeJSONError(w, mapIngestErrorStatus(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func overlandIngestHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.IsDraining != nil && deps.IsDraining() {
			writeJSONError(w, http.StatusServiceUnavailable, "service is shutting down")
			return
		}

		raw, err := readBoundedBody(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		points, err := ingest.ParseOverlandBatch(raw)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		device, ok := DeviceFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "missing authenticated device")
			return
		}

		normalizeForAuthenticatedDevice(points, device)
		ensureIngestHashes(points)

		response, err := appendAndEnqueue(deps, points, "overland")
		if err != nil {
			writeJSONError(w, mapIngestErrorStatus(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func appendAndEnqueue(deps Dependencies, points []ingest.CanonicalPoint, source string) (ingestSuccessResponse, error) {
	records, err := deps.Spool.AppendCanonicalPoints(points)
	if err != nil {
		return ingestSuccessResponse{}, err
	}

	if err := deps.Buffer.Enqueue(records); err != nil {
		if deps.Flusher != nil {
			deps.Flusher.TriggerFlush()
		}
		return ingestSuccessResponse{}, err
	}

	maybeTriggerPressureFlush(deps)

	maxSeq := uint64(0)
	for _, record := range records {
		if record.Seq > maxSeq {
			maxSeq = record.Seq
		}
	}

	return ingestSuccessResponse{
		OK:       true,
		Source:   source,
		Accepted: len(points),
		Spooled:  len(records),
		Enqueued: len(records),
		MaxSeq:   maxSeq,
	}, nil
}

func maybeTriggerPressureFlush(deps Dependencies) {
	if deps.Flusher == nil || deps.Buffer == nil {
		return
	}
	stats := deps.Buffer.Stats()
	if deps.FlushTriggerPoints > 0 && stats.TotalBufferedPoints >= deps.FlushTriggerPoints {
		deps.Flusher.TriggerFlush()
		return
	}
	if deps.FlushTriggerBytes > 0 && stats.TotalBufferedBytes >= deps.FlushTriggerBytes {
		deps.Flusher.TriggerFlush()
	}
}

func normalizeForAuthenticatedDevice(points []ingest.CanonicalPoint, device store.Device) {
	userID := strconv.FormatInt(device.UserID, 10)
	for i := range points {
		points[i].UserID = userID
		points[i].DeviceID = device.Name
	}
}

func ensureIngestHashes(points []ingest.CanonicalPoint) {
	for i := range points {
		if points[i].IngestHash == "" {
			points[i].IngestHash = ingest.GenerateDeterministicIngestHash(points[i])
		}
	}
}

func readBoundedBody(r *http.Request) ([]byte, error) {
	body := io.LimitReader(r.Body, maxIngestBodyBytes)
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func mapIngestErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if err == buffer.ErrMaxBytesExceeded || err == buffer.ErrMaxPointsExceeded {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

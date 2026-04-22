package api

import (
	"context"
	"encoding/json"
	"net/http"

	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type DeviceStore interface {
	CreateDevice(rctx context.Context, params store.CreateDeviceParams) (store.Device, error)
	ListDevices(rctx context.Context) ([]store.Device, error)
	GetDeviceByAPIKey(rctx context.Context, apiKey string) (store.Device, error)
}

type SpoolAppender interface {
	AppendCanonicalPoints(points []ingest.CanonicalPoint) ([]ingest.SpoolRecord, error)
	ReadCheckpoint() (spool.Checkpoint, error)
	SegmentCount() (int, error)
}

type RecordBuffer interface {
	Enqueue(records []ingest.SpoolRecord) error
	Stats() buffer.Stats
}

type FlushTrigger interface {
	TriggerFlush()
	LastFlushResult() (flusher.LastFlushResult, bool)
}

type Dependencies struct {
	DeviceStore DeviceStore
	Spool       SpoolAppender
	Buffer      RecordBuffer
	Flusher     FlushTrigger
}

func RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutesWithDependencies(mux, Dependencies{})
}

func RegisterRoutesWithDependencies(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /health", healthHandler)
	registerUIRoutes(mux)
	if deps.DeviceStore != nil {
		registerDeviceRoutes(mux, deps.DeviceStore)
	}
	if deps.DeviceStore != nil && deps.Spool != nil && deps.Buffer != nil {
		registerIngestRoutes(mux, deps)
	}
	if deps.Spool != nil && deps.Buffer != nil {
		registerStatusRoutes(mux, deps)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := healthResponse{
		Status:  "ok",
		Service: "plexplore",
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

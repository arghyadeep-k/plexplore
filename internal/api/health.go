package api

import (
	"context"
	"net/http"
	"time"

	"plexplore/internal/buffer"
	"plexplore/internal/flusher"
	"plexplore/internal/ingest"
	"plexplore/internal/spool"
	"plexplore/internal/store"
	"plexplore/internal/visits"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type DeviceStore interface {
	CreateDevice(rctx context.Context, params store.CreateDeviceParams) (store.Device, error)
	ListDevices(rctx context.Context) ([]store.Device, error)
	GetDeviceByID(rctx context.Context, id int64) (store.Device, error)
	GetDeviceByAPIKey(rctx context.Context, apiKey string) (store.Device, error)
	RotateDeviceAPIKey(rctx context.Context, id int64, newAPIKey string) (store.Device, error)
}

type UserStore interface {
	GetUserByID(rctx context.Context, id int64) (store.User, error)
	GetUserByEmail(rctx context.Context, email string) (store.User, error)
	CreateUser(rctx context.Context, params store.CreateUserParams) (store.User, error)
	ListUsers(rctx context.Context) ([]store.User, error)
}

type SessionStore interface {
	CreateSession(rctx context.Context, userID int64) (store.Session, error)
	GetSession(rctx context.Context, token string) (store.Session, error)
	DeleteSession(rctx context.Context, token string) error
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

type PointStore interface {
	ListRecentPoints(rctx context.Context, deviceID string, limit int) ([]store.RecentPoint, error)
	ListPoints(rctx context.Context, filter store.ExportPointFilter, limit int) ([]store.RecentPoint, error)
	ListPointsForExport(rctx context.Context, filter store.ExportPointFilter) ([]store.RecentPoint, error)
	StreamPointsForExport(rctx context.Context, filter store.ExportPointFilter, limit int, fn func(store.RecentPoint) error) (int, error)
}

type VisitStore interface {
	RebuildVisitsForDeviceRange(rctx context.Context, deviceID string, fromUTC, toUTC *time.Time, cfg visits.Config) (int, error)
	ListVisits(rctx context.Context, deviceID string, fromUTC, toUTC *time.Time, limit int) ([]store.Visit, error)
}

type VisitLabelResolver interface {
	Enabled() bool
	MaxProviderLookupsPerRequest() int
	ResolveVisitLabel(rctx context.Context, lat, lon float64, allowProvider bool) (string, bool, error)
}

type Dependencies struct {
	DeviceStore DeviceStore
	Spool       SpoolAppender
	Buffer      RecordBuffer
	Flusher     FlushTrigger
	// Trigger thresholds for best-effort pressure flushes after ingest enqueue.
	FlushTriggerPoints int
	FlushTriggerBytes  int
	PointStore         PointStore
	VisitStore         VisitStore
	VisitLabelResolver VisitLabelResolver
	UserStore          UserStore
	SessionStore       SessionStore
	CookieSecurity     CookieSecurityPolicy
	MapTiles           MapTileConfig
	RateLimiters       RateLimiters
	SpoolDir           string
	SQLitePath         string
	IsDraining         func() bool
}

type MapTileConfig struct {
	// Mode controls tile source behavior for map UI.
	// Supported values: none, osm, custom.
	Mode string
	// URLTemplate is used when mode is osm/custom.
	URLTemplate string
	// Attribution is shown by Leaflet for external/custom tile providers.
	Attribution string
}

func RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutesWithDependencies(mux, Dependencies{})
}

func RegisterRoutesWithDependencies(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /health", healthHandler)
	registerUIAssetRoutes(mux)
	if deps.UserStore != nil && deps.SessionStore != nil {
		registerLoginRoutes(mux, deps.UserStore, deps.SessionStore, deps.CookieSecurity, deps.RateLimiters)
		registerUserRoutes(mux, deps.UserStore, deps.SessionStore, deps.RateLimiters)
		registerUIRoutes(mux, deps)
	}
	if deps.DeviceStore != nil && deps.UserStore != nil && deps.SessionStore != nil {
		registerDeviceRoutesWithAuth(mux, deps.DeviceStore, deps.UserStore, deps.SessionStore, deps.RateLimiters)
	}
	if deps.DeviceStore != nil && deps.Spool != nil && deps.Buffer != nil {
		registerIngestRoutes(mux, deps)
	}
	if deps.Spool != nil && deps.Buffer != nil {
		registerStatusRoutes(mux, deps)
	}
	if deps.PointStore != nil && deps.DeviceStore != nil && deps.UserStore != nil && deps.SessionStore != nil {
		registerPointRoutes(mux, deps)
		registerExportRoutes(mux, deps)
	}
	if deps.VisitStore != nil && deps.DeviceStore != nil && deps.UserStore != nil && deps.SessionStore != nil {
		registerVisitRoutes(mux, deps)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{
		Status:  "ok",
		Service: "plexplore",
	}
	writeJSON(w, http.StatusOK, resp)
}

package api

import (
	"net/http"
)

type lastFlushStatusResponse struct {
	AtUTC   string `json:"at_utc"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type operationalStatusResponse struct {
	ServiceHealth            string                   `json:"service_health"`
	BufferPoints             int                      `json:"buffer_points"`
	BufferBytes              int                      `json:"buffer_bytes"`
	OldestBufferedAgeSeconds int64                    `json:"oldest_buffered_age_seconds"`
	SpoolDirPath             string                   `json:"spool_dir_path,omitempty"`
	SpoolSegmentCount        int                      `json:"spool_segment_count"`
	CheckpointSeq            uint64                   `json:"checkpoint_seq"`
	LastFlushAttemptAtUTC    string                   `json:"last_flush_attempt_at_utc,omitempty"`
	LastFlushSuccessAtUTC    string                   `json:"last_flush_success_at_utc,omitempty"`
	LastFlushError           string                   `json:"last_flush_error,omitempty"`
	SQLiteDBPath             string                   `json:"sqlite_db_path,omitempty"`
	LastFlush                *lastFlushStatusResponse `json:"last_flush,omitempty"`
	VisitScheduler           *visitSchedulerStatus    `json:"visit_scheduler,omitempty"`
}

type visitSchedulerStatus struct {
	Enabled          bool                      `json:"enabled"`
	Running          bool                      `json:"running"`
	LastRunStartAt   string                    `json:"last_run_start_at_utc,omitempty"`
	LastRunFinishAt  string                    `json:"last_run_finish_at_utc,omitempty"`
	LastSuccessAt    string                    `json:"last_success_at_utc,omitempty"`
	LastError        string                    `json:"last_error,omitempty"`
	LastRun          visitSchedulerRunCounters `json:"last_run"`
	WatermarkSummary visitSchedulerWatermark   `json:"watermark_summary"`
}

type visitSchedulerRunCounters struct {
	ProcessedDevices int `json:"processed_devices"`
	SkippedDevices   int `json:"skipped_devices"`
	UpdatedDevices   int `json:"updated_devices"`
	CreatedVisits    int `json:"created_visits"`
	Errors           int `json:"errors"`
}

type visitSchedulerWatermark struct {
	DevicesWithWatermark int    `json:"devices_with_watermark"`
	MinSeq               uint64 `json:"min_seq"`
	MaxSeq               uint64 `json:"max_seq"`
	LastProcessedAtUTC   string `json:"last_processed_at_utc,omitempty"`
	LagSeconds           int64  `json:"lag_seconds"`
}

type publicStatusResponse struct {
	ServiceHealth string `json:"service_health"`
	Service       string `json:"service"`
}

func registerStatusRoutes(mux *http.ServeMux, deps Dependencies) {
	if deps.SessionStore != nil && deps.UserStore != nil {
		status := LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuth(http.Handler(statusHandler(deps))),
		)
		mux.Handle("GET /api/v1/status", status)
	}
	mux.HandleFunc("GET /status", publicStatusHandler)
}

func statusHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := deps.Buffer.Stats()

		segmentCount, err := deps.Spool.SegmentCount()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to count spool segments")
			return
		}

		checkpoint, err := deps.Spool.ReadCheckpoint()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to read spool checkpoint")
			return
		}

		resp := operationalStatusResponse{
			ServiceHealth:            "ok",
			BufferPoints:             stats.TotalBufferedPoints,
			BufferBytes:              stats.TotalBufferedBytes,
			OldestBufferedAgeSeconds: int64(stats.OldestBufferedAge.Seconds()),
			SpoolDirPath:             deps.SpoolDir,
			SpoolSegmentCount:        segmentCount,
			CheckpointSeq:            checkpoint.LastCommittedSeq,
			SQLiteDBPath:             deps.SQLitePath,
		}

		if deps.Flusher != nil {
			if last, ok := deps.Flusher.LastFlushResult(); ok {
				lastAttemptAt := last.AtUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
				resp.LastFlush = &lastFlushStatusResponse{
					AtUTC:   lastAttemptAt,
					Success: last.Success,
					Error:   last.Error,
				}
				resp.LastFlushAttemptAtUTC = lastAttemptAt
				if !last.LastSuccessAtUTC.IsZero() {
					resp.LastFlushSuccessAtUTC = last.LastSuccessAtUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
				}
				if last.Error != "" {
					resp.LastFlushError = last.Error
				}
			}
		}
		if deps.VisitScheduler != nil {
			s := deps.VisitScheduler.Status()
			vs := &visitSchedulerStatus{
				Enabled:   s.Enabled,
				Running:   s.Running,
				LastError: s.LastError,
				LastRun: visitSchedulerRunCounters{
					ProcessedDevices: s.LastRunProcessed,
					SkippedDevices:   s.LastRunSkipped,
					UpdatedDevices:   s.LastRunUpdated,
					CreatedVisits:    s.LastRunCreated,
					Errors:           s.LastRunErrors,
				},
				WatermarkSummary: visitSchedulerWatermark{
					DevicesWithWatermark: s.WatermarkDevices,
					MinSeq:               s.WatermarkMinSeq,
					MaxSeq:               s.WatermarkMaxSeq,
					LagSeconds:           s.LagSeconds,
				},
			}
			if !s.LastRunStartUTC.IsZero() {
				vs.LastRunStartAt = s.LastRunStartUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
			}
			if !s.LastRunFinishUTC.IsZero() {
				vs.LastRunFinishAt = s.LastRunFinishUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
			}
			if !s.LastSuccessUTC.IsZero() {
				vs.LastSuccessAt = s.LastSuccessUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
			}
			if !s.WatermarkLastUTC.IsZero() {
				vs.WatermarkSummary.LastProcessedAtUTC = s.WatermarkLastUTC.Format("2006-01-02T15:04:05.000000000Z07:00")
			}
			resp.VisitScheduler = vs
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func publicStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, publicStatusResponse{
		ServiceHealth: "ok",
		Service:       "plexplore",
	})
}

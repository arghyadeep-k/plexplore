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
}

func registerStatusRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/status", statusHandler(deps))
	mux.HandleFunc("GET /status", statusHandler(deps))
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

		writeJSON(w, http.StatusOK, resp)
	}
}

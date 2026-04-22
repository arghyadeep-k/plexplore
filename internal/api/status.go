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
	BufferPoints             int                      `json:"buffer_points"`
	BufferBytes              int                      `json:"buffer_bytes"`
	OldestBufferedAgeSeconds int64                    `json:"oldest_buffered_age_seconds"`
	SpoolSegmentCount        int                      `json:"spool_segment_count"`
	CheckpointSeq            uint64                   `json:"checkpoint_seq"`
	LastFlush                *lastFlushStatusResponse `json:"last_flush,omitempty"`
}

func registerStatusRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("GET /api/v1/status", statusHandler(deps))
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
			BufferPoints:             stats.TotalBufferedPoints,
			BufferBytes:              stats.TotalBufferedBytes,
			OldestBufferedAgeSeconds: int64(stats.OldestBufferedAge.Seconds()),
			SpoolSegmentCount:        segmentCount,
			CheckpointSeq:            checkpoint.LastCommittedSeq,
		}

		if deps.Flusher != nil {
			if last, ok := deps.Flusher.LastFlushResult(); ok {
				resp.LastFlush = &lastFlushStatusResponse{
					AtUTC:   last.AtUTC.Format("2006-01-02T15:04:05.000000000Z07:00"),
					Success: last.Success,
					Error:   last.Error,
				}
			}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// Package spool contains the append-only on-disk spool primitives.
//
// Initial architecture:
// - Segmented append-only files (`segment-<start-seq>.ndjson`).
// - One JSON record per line (NDJSON) for crash-tolerant, stream-friendly writes.
// - Separate checkpoint file (`checkpoint.json`) that stores the last committed
//   sequence number after durable downstream flush.
//
// Replay model (planned):
// - Read segment files in ascending segment start sequence.
// - Decode each NDJSON record in order.
// - Skip records with sequence <= checkpoint.last_committed_seq.
//
// Compaction model:
// - Only delete whole segment files that are fully committed by checkpoint.
// - Never rewrite segment contents in place.
// - Prefer running compaction right after checkpoint advancement or as periodic
//   low-frequency maintenance.
package spool

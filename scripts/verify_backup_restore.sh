#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/verify_backup_restore.sh

Automated backup/restore drill:
1. Creates temp SQLite + spool state
2. Seeds deterministic rows
3. Runs scripts/backup.sh
4. Verifies backup archive contents
5. Restores into a clean target
6. Verifies SQLite integrity + expected row counts
7. Verifies restored spool/checkpoint files exist
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required for backup/restore verification" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_root="$(mktemp -d)"
src_dir="$tmp_root/source"
restore_dir="$tmp_root/restore"
backups_dir="$tmp_root/backups"
src_db="$src_dir/plexplore.db"
src_spool="$src_dir/spool"
restore_db="$restore_dir/plexplore-restored.db"
restore_spool="$restore_dir/spool"

cleanup() {
  rm -rf "$tmp_root"
}
trap cleanup EXIT

mkdir -p "$src_dir" "$src_spool" "$restore_dir" "$backups_dir"

echo "==> applying migrations to temp sqlite"
(
  cd "$repo_root"
  APP_SQLITE_PATH="$src_db" go run ./cmd/migrate
)

echo "==> seeding deterministic test data"
sqlite3 "$src_db" <<'SQL'
INSERT INTO users(id, name, email, password_hash, is_admin, created_at, updated_at)
VALUES (100, 'Admin', 'admin+backup@example.com', 'hash-admin', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');

INSERT INTO devices(id, user_id, name, source_type, api_key, api_key_hash, api_key_preview, created_at, updated_at, last_seen_at, last_seq_received)
VALUES (200, 100, 'backup-phone', 'owntracks', 'legacy-sentinel', 'hash-device', 'abcd...wxyz', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:20:00Z', 3);

INSERT INTO raw_points(id, seq, user_id, device_id, source_type, timestamp_utc, lat, lon, raw_payload_json, ingest_hash, created_at)
VALUES
  (300, 1, 100, 200, 'owntracks', '2026-01-01T00:00:00Z', 41.10000, -87.10000, '{}', 'ihash-1', '2026-01-01T00:00:00Z'),
  (301, 2, 100, 200, 'owntracks', '2026-01-01T00:10:00Z', 41.10001, -87.10001, '{}', 'ihash-2', '2026-01-01T00:10:00Z'),
  (302, 3, 100, 200, 'owntracks', '2026-01-01T00:20:00Z', 41.10002, -87.10002, '{}', 'ihash-3', '2026-01-01T00:20:00Z');

INSERT INTO points(id, raw_point_id, user_id, device_id, timestamp_utc, lat, lon, created_at)
VALUES
  (400, 300, 100, 200, '2026-01-01T00:00:00Z', 41.10000, -87.10000, '2026-01-01T00:00:00Z'),
  (401, 301, 100, 200, '2026-01-01T00:10:00Z', 41.10001, -87.10001, '2026-01-01T00:10:00Z'),
  (402, 302, 100, 200, '2026-01-01T00:20:00Z', 41.10002, -87.10002, '2026-01-01T00:20:00Z');

INSERT INTO visits(id, device_id, start_at, end_at, centroid_lat, centroid_lon, point_count, created_at)
VALUES (500, 200, '2026-01-01T00:00:00Z', '2026-01-01T00:20:00Z', 41.10001, -87.10001, 3, '2026-01-01T00:21:00Z');

INSERT INTO visit_generation_state(device_id, last_processed_seq, updated_at)
VALUES (200, 3, '2026-01-01T00:21:00Z');
SQL

cat >"$src_spool/checkpoint.json" <<'EOF'
{"last_committed_seq":3,"updated_at_utc":"2026-01-01T00:21:00Z"}
EOF
cat >"$src_spool/0000000000000001.seg" <<'EOF'
placeholder-segment-content
EOF
echo "do-not-include" >"$src_dir/runtime-junk.tmp"

echo "==> running backup script"
"$repo_root/scripts/backup.sh" \
  --offline \
  --sqlite-path "$src_db" \
  --spool-dir "$src_spool" \
  --output-dir "$backups_dir"

archive_path="$(ls -1 "$backups_dir"/plexplore-backup-*.tar* | tail -n 1)"
if [[ -z "$archive_path" || ! -f "$archive_path" ]]; then
  echo "backup archive not found in $backups_dir" >&2
  exit 1
fi
echo "backup archive: $archive_path"

echo "==> verifying archive contents"
archive_listing="$(tar -tf "$archive_path")"
echo "$archive_listing" | grep -qE '^\./sqlite/plexplore\.db$' || { echo "archive missing sqlite snapshot" >&2; exit 1; }
echo "$archive_listing" | grep -qE '^\./spool/checkpoint\.json$' || { echo "archive missing spool checkpoint" >&2; exit 1; }
echo "$archive_listing" | grep -qE '^\./spool/0000000000000001\.seg$' || { echo "archive missing spool segment file" >&2; exit 1; }
echo "$archive_listing" | grep -qE '^\./MANIFEST\.txt$' || { echo "archive missing manifest" >&2; exit 1; }
if echo "$archive_listing" | grep -q 'runtime-junk.tmp'; then
  echo "archive should not include unrelated runtime-junk.tmp" >&2
  exit 1
fi

echo "==> validating backup script failure behavior"
if "$repo_root/scripts/backup.sh" --sqlite-path "$tmp_root/missing.db" --spool-dir "$src_spool" --output-dir "$backups_dir" >/dev/null 2>&1; then
  echo "backup script unexpectedly succeeded with missing sqlite path" >&2
  exit 1
fi

echo "==> running restore script into clean target"
"$repo_root/scripts/restore.sh" \
  --archive "$archive_path" \
  --sqlite-path "$restore_db" \
  --spool-dir "$restore_spool" \
  --force

echo "==> sqlite integrity + data checks"
integrity="$(sqlite3 "$restore_db" 'PRAGMA integrity_check;')"
if [[ "$integrity" != "ok" ]]; then
  echo "restored db integrity_check failed: $integrity" >&2
  exit 1
fi

users_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM users WHERE id = 100;')"
devices_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM devices WHERE id = 200;')"
raw_points_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM raw_points WHERE device_id = 200;')"
points_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM points WHERE device_id = 200;')"
visits_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM visits WHERE device_id = 200;')"
watermark_count="$(sqlite3 "$restore_db" 'SELECT COUNT(*) FROM visit_generation_state WHERE device_id = 200 AND last_processed_seq = 3;')"

[[ "$users_count" == "1" ]] || { echo "unexpected users count: $users_count" >&2; exit 1; }
[[ "$devices_count" == "1" ]] || { echo "unexpected devices count: $devices_count" >&2; exit 1; }
[[ "$raw_points_count" == "3" ]] || { echo "unexpected raw_points count: $raw_points_count" >&2; exit 1; }
[[ "$points_count" == "3" ]] || { echo "unexpected points count: $points_count" >&2; exit 1; }
[[ "$visits_count" == "1" ]] || { echo "unexpected visits count: $visits_count" >&2; exit 1; }
[[ "$watermark_count" == "1" ]] || { echo "unexpected watermark count: $watermark_count" >&2; exit 1; }

echo "==> verifying restored spool/checkpoint files"
[[ -f "$restore_spool/checkpoint.json" ]] || { echo "restored checkpoint.json missing" >&2; exit 1; }
[[ -f "$restore_spool/0000000000000001.seg" ]] || { echo "restored spool segment missing" >&2; exit 1; }

echo "backup/restore verification PASSED"

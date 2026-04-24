#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/backup.sh [options]

Options:
  --sqlite-path PATH    SQLite DB path (default: APP_SQLITE_PATH or ./data/plexplore.db)
  --spool-dir PATH      Spool directory (default: APP_SPOOL_DIR or ./data/spool)
  --output-dir PATH     Backup output directory (default: ./backups)
  --name-prefix VALUE   Archive prefix (default: plexplore-backup)
  --offline             Use offline file-copy mode instead of sqlite .backup
  --no-compress         Write .tar instead of .tar.gz
  -h, --help            Show this help

Notes:
  - Online mode uses sqlite3 ".backup" for a consistent DB snapshot while app runs.
  - Offline mode should be used after stopping the service.
EOF
}

SQLITE_PATH="${APP_SQLITE_PATH:-./data/plexplore.db}"
SPOOL_DIR="${APP_SPOOL_DIR:-./data/spool}"
OUTPUT_DIR="./backups"
NAME_PREFIX="plexplore-backup"
OFFLINE_MODE=0
COMPRESS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sqlite-path)
      SQLITE_PATH="$2"
      shift 2
      ;;
    --spool-dir)
      SPOOL_DIR="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --name-prefix)
      NAME_PREFIX="$2"
      shift 2
      ;;
    --offline)
      OFFLINE_MODE=1
      shift
      ;;
    --no-compress)
      COMPRESS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ ! -f "$SQLITE_PATH" ]]; then
  echo "sqlite file not found: $SQLITE_PATH" >&2
  exit 1
fi
if [[ ! -d "$SPOOL_DIR" ]]; then
  echo "spool dir not found: $SPOOL_DIR" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
timestamp="$(date -u +%Y%m%d-%H%M%S)"
workdir="$(mktemp -d)"
stage_dir="$workdir/stage"
mkdir -p "$stage_dir/sqlite" "$stage_dir/spool"

cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

sqlite_dst="$stage_dir/sqlite/$(basename "$SQLITE_PATH")"

if [[ "$OFFLINE_MODE" -eq 1 ]]; then
  cp -a "$SQLITE_PATH" "$sqlite_dst"
else
  if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "sqlite3 is required for online mode; use --offline or install sqlite3" >&2
    exit 1
  fi
  sqlite3 "$SQLITE_PATH" ".backup '$sqlite_dst'"
fi

# Keep -wal/-shm with offline copies so sqlite state is complete when service was running.
if [[ "$OFFLINE_MODE" -eq 1 ]]; then
  [[ -f "${SQLITE_PATH}-wal" ]] && cp -a "${SQLITE_PATH}-wal" "$stage_dir/sqlite/"
  [[ -f "${SQLITE_PATH}-shm" ]] && cp -a "${SQLITE_PATH}-shm" "$stage_dir/sqlite/"
fi

cp -a "$SPOOL_DIR"/. "$stage_dir/spool/"

cat >"$stage_dir/MANIFEST.txt" <<EOF
created_utc=$timestamp
mode=$([[ "$OFFLINE_MODE" -eq 1 ]] && echo "offline" || echo "online")
sqlite_source=$SQLITE_PATH
spool_source=$SPOOL_DIR
EOF

archive_base="${NAME_PREFIX}-${timestamp}.tar"
archive_path="$OUTPUT_DIR/$archive_base"
if [[ "$COMPRESS" -eq 1 ]]; then
  archive_path="${archive_path}.gz"
  tar -C "$stage_dir" -czf "$archive_path" .
else
  tar -C "$stage_dir" -cf "$archive_path" .
fi

echo "backup created: $archive_path"
echo "contents: sqlite snapshot + spool directory + manifest"
echo "mode: $([[ "$OFFLINE_MODE" -eq 1 ]] && echo "offline file copy" || echo "online sqlite backup")"

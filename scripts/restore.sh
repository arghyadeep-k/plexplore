#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/restore.sh --archive PATH [options]

Options:
  --archive PATH        Backup archive (.tar or .tar.gz) from scripts/backup.sh
  --sqlite-path PATH    SQLite DB restore path (default: APP_SQLITE_PATH or ./data/plexplore.db)
  --spool-dir PATH      Spool directory restore path (default: APP_SPOOL_DIR or ./data/spool)
  --force               Skip interactive confirmation
  -h, --help            Show this help

Important:
  Stop plexplore before restore. Restoring while service is running can corrupt state.
EOF
}

ARCHIVE_PATH=""
SQLITE_PATH="${APP_SQLITE_PATH:-./data/plexplore.db}"
SPOOL_DIR="${APP_SPOOL_DIR:-./data/spool}"
FORCE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --archive)
      ARCHIVE_PATH="$2"
      shift 2
      ;;
    --sqlite-path)
      SQLITE_PATH="$2"
      shift 2
      ;;
    --spool-dir)
      SPOOL_DIR="$2"
      shift 2
      ;;
    --force)
      FORCE=1
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

if [[ -z "$ARCHIVE_PATH" ]]; then
  echo "--archive is required" >&2
  usage
  exit 1
fi
if [[ ! -f "$ARCHIVE_PATH" ]]; then
  echo "archive not found: $ARCHIVE_PATH" >&2
  exit 1
fi

echo "WARNING: Ensure plexplore service is stopped before restore."
echo "archive: $ARCHIVE_PATH"
echo "target sqlite: $SQLITE_PATH"
echo "target spool:  $SPOOL_DIR"

if [[ "$FORCE" -ne 1 ]]; then
  read -r -p "Continue restore? Type 'yes' to proceed: " confirm
  if [[ "$confirm" != "yes" ]]; then
    echo "restore canceled"
    exit 1
  fi
fi

workdir="$(mktemp -d)"
restore_root="$workdir/restore"
mkdir -p "$restore_root"

cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

case "$ARCHIVE_PATH" in
  *.tar.gz|*.tgz)
    tar -C "$restore_root" -xzf "$ARCHIVE_PATH"
    ;;
  *.tar)
    tar -C "$restore_root" -xf "$ARCHIVE_PATH"
    ;;
  *)
    echo "unsupported archive extension: $ARCHIVE_PATH" >&2
    exit 1
    ;;
esac

sqlite_src_dir="$restore_root/sqlite"
spool_src_dir="$restore_root/spool"

if [[ ! -d "$sqlite_src_dir" || ! -d "$spool_src_dir" ]]; then
  echo "archive is missing expected sqlite/spool directories" >&2
  exit 1
fi

sqlite_src_file="$(find "$sqlite_src_dir" -maxdepth 1 -type f ! -name '*.manifest' ! -name '*.txt' ! -name '*.shm' ! -name '*.wal' | head -n 1)"
if [[ -z "$sqlite_src_file" ]]; then
  echo "no sqlite db file found inside archive sqlite/ directory" >&2
  exit 1
fi

mkdir -p "$(dirname "$SQLITE_PATH")" "$SPOOL_DIR"
ts="$(date -u +%Y%m%d-%H%M%S)"
safety_dir="$(dirname "$SQLITE_PATH")/restore-pre-${ts}"
mkdir -p "$safety_dir"

if [[ -f "$SQLITE_PATH" ]]; then
  cp -a "$SQLITE_PATH" "$safety_dir/"
fi
if [[ -f "${SQLITE_PATH}-wal" ]]; then
  cp -a "${SQLITE_PATH}-wal" "$safety_dir/"
fi
if [[ -f "${SQLITE_PATH}-shm" ]]; then
  cp -a "${SQLITE_PATH}-shm" "$safety_dir/"
fi
if [[ -d "$SPOOL_DIR" ]]; then
  cp -a "$SPOOL_DIR" "$safety_dir/spool-pre-restore"
fi

rm -f "$SQLITE_PATH" "${SQLITE_PATH}-wal" "${SQLITE_PATH}-shm"
cp -a "$sqlite_src_file" "$SQLITE_PATH"
if [[ -f "$sqlite_src_dir/$(basename "$sqlite_src_file")-wal" ]]; then
  cp -a "$sqlite_src_dir/$(basename "$sqlite_src_file")-wal" "${SQLITE_PATH}-wal"
fi
if [[ -f "$sqlite_src_dir/$(basename "$sqlite_src_file")-shm" ]]; then
  cp -a "$sqlite_src_dir/$(basename "$sqlite_src_file")-shm" "${SQLITE_PATH}-shm"
fi

rm -rf "$SPOOL_DIR"
mkdir -p "$SPOOL_DIR"
cp -a "$spool_src_dir"/. "$SPOOL_DIR/"

echo "restore complete"
echo "restored sqlite: $SQLITE_PATH"
echo "restored spool:  $SPOOL_DIR"
echo "pre-restore safety copy: $safety_dir"
echo "if running under systemd/docker, ensure service user can read/write restored files."

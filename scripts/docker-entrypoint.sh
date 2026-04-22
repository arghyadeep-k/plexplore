#!/bin/sh
set -eu

mkdir -p "${APP_SPOOL_DIR:-/data/spool}"

# Keep startup simple: run idempotent migrations, then start server.
plexplore-migrate
exec plexplore-server

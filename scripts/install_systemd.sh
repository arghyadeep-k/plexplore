#!/usr/bin/env bash
set -euo pipefail

# Minimal installer for Raspberry Pi systemd deployment.
# Expects a built binary at ./plexplore-server from repository root.

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root (sudo)." >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_SOURCE="${REPO_ROOT}/plexplore-server"
BINARY_DEST="/opt/plexplore/plexplore-server"
SERVICE_SOURCE="${REPO_ROOT}/deploy/systemd/plexplore.service"
SERVICE_DEST="/etc/systemd/system/plexplore.service"
ENV_SOURCE="${REPO_ROOT}/deploy/systemd/plexplore.env.sample"
ENV_DEST="/etc/plexplore/plexplore.env"

if [[ ! -x "${BINARY_SOURCE}" ]]; then
  echo "Missing binary: ${BINARY_SOURCE}" >&2
  echo "Build first: go build -o plexplore-server ./cmd/server" >&2
  exit 1
fi

id -u plexplore >/dev/null 2>&1 || useradd --system --home /var/lib/plexplore --shell /usr/sbin/nologin plexplore

install -d -o plexplore -g plexplore /opt/plexplore
install -d -o plexplore -g plexplore /var/lib/plexplore
install -d -o plexplore -g plexplore /var/lib/plexplore/spool
install -d -m 0755 /etc/plexplore

install -m 0755 "${BINARY_SOURCE}" "${BINARY_DEST}"
install -m 0644 "${SERVICE_SOURCE}" "${SERVICE_DEST}"
if [[ ! -f "${ENV_DEST}" ]]; then
  install -m 0644 "${ENV_SOURCE}" "${ENV_DEST}"
fi

chown -R plexplore:plexplore /opt/plexplore /var/lib/plexplore

systemctl daemon-reload
systemctl enable --now plexplore
systemctl status --no-pager plexplore || true

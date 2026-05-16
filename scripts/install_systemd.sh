#!/usr/bin/env bash
set -euo pipefail

# Minimal installer for Raspberry Pi systemd deployment.
# Expects a built binary at ./exploripi-server from repository root.

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root (sudo)." >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_SOURCE="${REPO_ROOT}/exploripi-server"
BINARY_DEST="/opt/exploripi/exploripi-server"
SERVICE_SOURCE="${REPO_ROOT}/deploy/systemd/exploripi.service"
SERVICE_DEST="/etc/systemd/system/exploripi.service"
ENV_SOURCE="${REPO_ROOT}/deploy/systemd/exploripi.env.sample"
ENV_DEST="/etc/exploripi/exploripi.env"

if [[ ! -x "${BINARY_SOURCE}" ]]; then
  echo "Missing binary: ${BINARY_SOURCE}" >&2
  echo "Build first: go build -o exploripi-server ./cmd/server" >&2
  exit 1
fi

id -u exploripi >/dev/null 2>&1 || useradd --system --home /var/lib/exploripi --shell /usr/sbin/nologin exploripi

install -d -o exploripi -g exploripi /opt/exploripi
install -d -o exploripi -g exploripi /var/lib/exploripi
install -d -o exploripi -g exploripi /var/lib/exploripi/spool
install -d -m 0755 /etc/exploripi

install -m 0755 "${BINARY_SOURCE}" "${BINARY_DEST}"
install -m 0644 "${SERVICE_SOURCE}" "${SERVICE_DEST}"
if [[ ! -f "${ENV_DEST}" ]]; then
  install -m 0644 "${ENV_SOURCE}" "${ENV_DEST}"
fi

chown -R exploripi:exploripi /opt/exploripi /var/lib/exploripi

systemctl daemon-reload
systemctl enable --now exploripi
systemctl status --no-pager exploripi || true

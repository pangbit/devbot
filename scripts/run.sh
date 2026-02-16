#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

missing=0
if [[ -z "${DEVBOT_APP_ID:-}" ]]; then
  echo "DEVBOT_APP_ID is required" >&2
  missing=1
fi
if [[ -z "${DEVBOT_APP_SECRET:-}" ]]; then
  echo "DEVBOT_APP_SECRET is required" >&2
  missing=1
fi
if [[ -z "${DEVBOT_ALLOWED_USER_IDS:-}" ]]; then
  echo "DEVBOT_ALLOWED_USER_IDS is required" >&2
  missing=1
fi
if [[ $missing -ne 0 ]]; then
  echo "Example:" >&2
  echo "  export DEVBOT_APP_ID=cli_xxx" >&2
  echo "  export DEVBOT_APP_SECRET=xxx" >&2
  echo "  export DEVBOT_ALLOWED_USER_IDS=user1,user2" >&2
  exit 1
fi

cd "$ROOT_DIR"

# Keep-alive loop: restart on crash
while true; do
  echo "[$(date)] Starting devbot..."
  go run . || true
  echo "[$(date)] devbot exited, restarting in 5s..."
  sleep 5
done

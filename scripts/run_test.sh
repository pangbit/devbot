#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN="${ROOT_DIR}/scripts/run.sh"

output="$({ DEVBOT_APP_ID="" DEVBOT_APP_SECRET="" DEVBOT_ALLOWED_USER_IDS="" "$RUN"; } 2>&1 || true)"
if [[ "$output" != *"DEVBOT_APP_ID"* ]] || [[ "$output" != *"DEVBOT_APP_SECRET"* ]] || [[ "$output" != *"DEVBOT_ALLOWED_USER_IDS"* ]]; then
  echo "expected missing DEVBOT_APP_ID/DEVBOT_APP_SECRET/DEVBOT_ALLOWED_USER_IDS message, got: $output"
  exit 1
fi

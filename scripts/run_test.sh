#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN="${ROOT_DIR}/scripts/run.sh"

output="$({ APP_ID="" APP_SECRET="" "$RUN"; } 2>&1 || true)"
if [[ "$output" != *"APP_ID"* ]] || [[ "$output" != *"APP_SECRET"* ]]; then
  echo "expected missing APP_ID/APP_SECRET message, got: $output"
  exit 1
fi

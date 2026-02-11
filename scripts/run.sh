#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

missing=0
if [[ -z "${APP_ID:-}" ]]; then
  echo "APP_ID is required" >&2
  missing=1
fi
if [[ -z "${APP_SECRET:-}" ]]; then
  echo "APP_SECRET is required" >&2
  missing=1
fi
if [[ $missing -ne 0 ]]; then
  echo "Example:" >&2
  echo "  export APP_ID=cli_xxx" >&2
  echo "  export APP_SECRET=xxx" >&2
  exit 1
fi

cd "$ROOT_DIR"

go run .

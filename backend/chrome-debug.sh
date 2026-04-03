#!/usr/bin/env bash
set -euo pipefail

CHROME_BIN="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
DEBUG_PORT="${1:-9222}"
PROFILE_DIR="${HOME}/.crawl-pic-chrome-profile"

if [[ ! -x "$CHROME_BIN" ]]; then
  echo "[chrome-debug.sh] Google Chrome not found at: $CHROME_BIN"
  exit 1
fi

mkdir -p "$PROFILE_DIR"

echo "[chrome-debug.sh] starting Chrome on port $DEBUG_PORT"
exec "$CHROME_BIN" \
  --remote-debugging-port="$DEBUG_PORT" \
  --user-data-dir="$PROFILE_DIR" \
  --no-first-run \
  --no-default-browser-check

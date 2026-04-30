#!/usr/bin/env bash
#
# Scan E2E backend logs for Go panics and exit non-zero if any are found.
#
# Usage:
#   ./scripts/check-e2e-panics.sh [log-dir]
#
# Arguments:
#   log-dir   Directory containing backend.log (default: .ralph/temp/e2e-logs)
#
# Exit codes:
#   0   No panics found (or log file does not exist)
#   1   One or more panic: lines found in backend.log
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

DEFAULT_LOG_DIR="$PROJECT_DIR/.ralph/temp/e2e-logs"
LOG_DIR="${1:-$DEFAULT_LOG_DIR}"
BACKEND_LOG="$LOG_DIR/backend.log"

if [ ! -f "$BACKEND_LOG" ]; then
  echo "check-e2e-panics: $BACKEND_LOG not found — skipping panic scan" >&2
  exit 0
fi

# grep exits 1 when no match found; treat that as clean (no panics).
PANIC_LINES=$(grep 'panic:' "$BACKEND_LOG" || true)

if [ -n "$PANIC_LINES" ]; then
  echo "check-e2e-panics: PANIC detected in $BACKEND_LOG:" >&2
  echo "$PANIC_LINES" >&2
  exit 1
fi

echo "check-e2e-panics: no panics found in $BACKEND_LOG"
exit 0

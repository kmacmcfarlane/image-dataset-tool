#!/usr/bin/env bash
#
# Capture runtime context from docker compose for debugging.
# Outputs a markdown-formatted snapshot to .debug-context in the project root.
#
# Usage:
#   ./scripts/capture-runtime-context.sh              # default output
#   ./scripts/capture-runtime-context.sh /tmp/ctx.md   # custom output path
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT="${1:-$PROJECT_DIR/.debug-context}"

cd "$PROJECT_DIR"

{
  echo "# Runtime Context Snapshot"
  echo "Captured: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo ""

  # ── Container status ──────────────────────────────────────────────
  echo "## Container Status"
  echo '```'
  docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>&1 || echo "(docker compose ps failed)"
  echo '```'
  echo ""

  # ── Dev-mode container status (if running) ─────────────────────
  echo "## Dev Container Status"
  echo '```'
  docker compose -f docker-compose.yml -f docker-compose.dev.yml ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>&1 || echo "(dev containers not running)"
  echo '```'
  echo ""

  # ── Error-level log lines (deduplicated patterns) ──────────────
  echo "## Error Log Lines (last 500 lines, filtered)"
  echo '```'
  docker compose logs --tail=500 --no-color 2>&1 \
    | grep -iE 'level=(error|fatal|panic)|FATAL|PANIC|panic:' \
    | tail -80 \
    || echo "(no error-level log lines found)"
  echo '```'
  echo ""

  # ── Warning-level log lines ────────────────────────────────────
  echo "## Warning Log Lines (last 500 lines, filtered)"
  echo '```'
  docker compose logs --tail=500 --no-color 2>&1 \
    | grep -iE 'level=warn' \
    | tail -30 \
    || echo "(no warning-level log lines found)"
  echo '```'
  echo ""

  # ── Full recent logs (tail) ───────────────────────────────────
  echo "## Full Logs (last 100 lines per container)"
  echo '```'
  docker compose logs --tail=100 --no-color 2>&1 || echo "(no logs available)"
  echo '```'
  echo ""

  # ── Container health / restart counts ──────────────────────────
  echo "## Container Inspect (restart count, health)"
  echo '```'
  for cid in $(docker compose ps -q 2>/dev/null); do
    docker inspect --format '{{.Name}}: restarts={{.RestartCount}} state={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}}' "$cid" 2>/dev/null || true
  done
  echo '```'
} > "$OUTPUT"

echo "Runtime context captured to $OUTPUT ($(wc -l < "$OUTPUT") lines)"

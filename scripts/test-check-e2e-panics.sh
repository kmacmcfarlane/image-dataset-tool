#!/usr/bin/env bash
#
# Tests for check-e2e-panics.sh
#
# Runs lightweight shell-level tests: injects fake panics into a temp log and
# verifies the script's exit codes.
#
# Usage:
#   ./scripts/test-check-e2e-panics.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PANIC_SCRIPT="$SCRIPT_DIR/check-e2e-panics.sh"

PASS=0
FAIL=0

# ── helpers ───────────────────────────────────────────────────────────────────

ok() {
  echo "PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $1" >&2
  FAIL=$((FAIL + 1))
}

assert_exit() {
  local description="$1"
  local expected="$2"
  shift 2
  local actual
  actual=0
  "$@" >/dev/null 2>&1 || actual=$?
  if [ "$actual" -eq "$expected" ]; then
    ok "$description (exit $actual)"
  else
    fail "$description — expected exit $expected, got $actual"
  fi
}

# ── set up temp directory ─────────────────────────────────────────────────────

TMPDIR_BASE=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BASE"' EXIT

# ── test 1: clean log — exit 0 ───────────────────────────────────────────────

CLEAN_DIR="$TMPDIR_BASE/clean"
mkdir -p "$CLEAN_DIR"
cat > "$CLEAN_DIR/backend.log" <<'EOF'
time=2024-01-01T00:00:00Z level=info msg="server starting" port=8080
time=2024-01-01T00:00:01Z level=info msg="database connected"
time=2024-01-01T00:00:02Z level=info msg="request handled" path=/health status=200
EOF

assert_exit "clean log exits 0" 0 "$PANIC_SCRIPT" "$CLEAN_DIR"

# ── test 2: log with panic: — exit 1 ─────────────────────────────────────────

PANIC_DIR="$TMPDIR_BASE/panic"
mkdir -p "$PANIC_DIR"
cat > "$PANIC_DIR/backend.log" <<'EOF'
time=2024-01-01T00:00:00Z level=info msg="server starting" port=8080
goroutine 1 [running]:
panic: runtime error: index out of range [5] with length 3
main.handleRequest(...)
	/app/backend/main.go:42 +0x1c8
EOF

assert_exit "log with panic: exits 1" 1 "$PANIC_SCRIPT" "$PANIC_DIR"

# ── test 3: missing log file — exit 0 (graceful skip) ─────────────────────────

EMPTY_DIR="$TMPDIR_BASE/empty"
mkdir -p "$EMPTY_DIR"
# backend.log intentionally absent

assert_exit "missing backend.log exits 0" 0 "$PANIC_SCRIPT" "$EMPTY_DIR"

# ── test 4: missing log directory argument uses default (graceful) ─────────────
# The default path won't exist in CI; script should exit 0 (file not found path).

assert_exit "no argument uses default and exits 0 when file absent" 0 \
  env HOME=/nonexistent "$PANIC_SCRIPT" "$TMPDIR_BASE/does-not-exist"

# ── test 5: panic: must be case-sensitive (no false positive on PANIC) ─────────

NOPANIC_DIR="$TMPDIR_BASE/nopanic"
mkdir -p "$NOPANIC_DIR"
cat > "$NOPANIC_DIR/backend.log" <<'EOF'
time=2024-01-01T00:00:00Z level=info msg="no issue here"
PANIC is not a Go panic
level=fatal PANIC-like message
EOF

assert_exit "PANIC (uppercase) does not trigger — exits 0" 0 "$PANIC_SCRIPT" "$NOPANIC_DIR"

# ── test 6: panic: anywhere on a line triggers detection ──────────────────────

INLINE_DIR="$TMPDIR_BASE/inline"
mkdir -p "$INLINE_DIR"
cat > "$INLINE_DIR/backend.log" <<'EOF'
time=2024-01-01T00:00:00Z level=fatal msg="panic: something went wrong"
EOF

assert_exit "inline panic: on a log line exits 1" 1 "$PANIC_SCRIPT" "$INLINE_DIR"

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
echo "Results: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0

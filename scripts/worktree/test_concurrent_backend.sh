#!/usr/bin/env bash
#
# AC9: Verify two worktrees can run make test-backend concurrently
# without interference.
#
# This script:
# 1. Creates two temporary worktrees
# 2. Runs make test-backend in each with different STORY_ID values
# 3. Verifies both complete successfully without port/volume collisions
# 4. Cleans up worktrees and compose stacks
#
# Usage:
#   ./scripts/worktree/test_concurrent_backend.sh
#
# Requirements:
#   - Docker daemon running
#   - Run from the main checkout (not a worktree)
#
# Exit codes:
#   0 — Both test runs completed without interference
#   1 — One or more test runs failed
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_DIR"

STORY_A="TEST-001"
STORY_B="TEST-002"
WT_A=".worktrees/$STORY_A"
WT_B=".worktrees/$STORY_B"

echo "=== AC9: Concurrent backend test isolation ==="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "=== Cleaning up ==="
    # Tear down any compose stacks
    for sid in "$STORY_A" "$STORY_B"; do
        # Derive the scoped compose project name via the helper so it stays in
        # sync with whatever base name the project uses in its compose files.
        project=$(STORY_ID="$sid" "$SCRIPT_DIR/../compose-project-name.sh" myproject-dev)
        docker compose -p "$project" \
            -f docker-compose.yml -f docker-compose.dev.yml -f docker-compose.worktree.yml \
            down -v 2>/dev/null || true
    done

    # Remove worktrees
    for wt in "$WT_A" "$WT_B"; do
        if [ -d "$wt" ]; then
            git worktree remove --force "$wt" 2>/dev/null || true
        fi
    done

    # Clean up branches
    for sid in "$STORY_A" "$STORY_B"; do
        git branch -D "story/$sid" 2>/dev/null || true
    done

    echo "=== Cleanup complete ==="
}

trap cleanup EXIT

# Create worktrees
echo "--- Creating worktrees ---"
mkdir -p .worktrees
for sid in "$STORY_A" "$STORY_B"; do
    branch="story/$sid"
    wt=".worktrees/$sid"
    if [ -d "$wt" ]; then
        git worktree remove --force "$wt" 2>/dev/null || true
        git branch -D "$branch" 2>/dev/null || true
    fi
    git worktree add -b "$branch" "$wt" HEAD
    echo "  Created worktree: $wt (branch: $branch)"
done

# Run test-backend concurrently in both worktrees
echo ""
echo "--- Running test-backend concurrently ---"

FAIL=0

run_test() {
    local sid="$1"
    local wt="$PROJECT_DIR/.worktrees/$sid"
    local log="$PROJECT_DIR/.worktrees/${sid}-test.log"

    echo "  [$sid] Starting test-backend in worktree $wt ..."
    # Run make test-backend from the worktree directory with STORY_ID set
    # for compose isolation — exercises the auto-detection path in
    # compose-project-name.sh
    if STORY_ID="$sid" make -C "$wt" test-backend > "$log" 2>&1; then
        echo "  [$sid] PASSED"
        return 0
    else
        echo "  [$sid] FAILED (see $log)"
        return 1
    fi
}

run_test "$STORY_A" &
PID_A=$!

run_test "$STORY_B" &
PID_B=$!

if ! wait "$PID_A"; then
    echo "  $STORY_A test failed"
    FAIL=1
fi

if ! wait "$PID_B"; then
    echo "  $STORY_B test failed"
    FAIL=1
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "=== PASSED: Both worktrees ran test-backend concurrently without interference ==="
else
    echo "=== FAILED: One or more concurrent test runs failed ==="
    echo ""
    echo "--- Logs ---"
    for sid in "$STORY_A" "$STORY_B"; do
        log=".worktrees/${sid}-test.log"
        if [ -f "$log" ]; then
            echo "[$sid] Last 20 lines:"
            tail -20 "$log" | sed "s/^/  /"
            echo ""
        fi
    done
fi

exit $FAIL

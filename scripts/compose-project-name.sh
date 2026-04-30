#!/usr/bin/env bash
#
# Derive a Docker Compose project name for the current context.
#
# In a worktree (.worktrees/<story-id>/), the project name is scoped to the
# story ID to prevent collisions between concurrent compose stacks.
# In the main checkout, returns the default project name.
#
# Usage:
#   ./scripts/compose-project-name.sh <base-name>
#
# Examples:
#   ./scripts/compose-project-name.sh myproject-dev
#     -> myproject-dev                         (main checkout)
#     -> myproject-dev-s-042                   (worktree for S-042)
#
#   ./scripts/compose-project-name.sh myproject-test
#     -> myproject-test                        (main checkout)
#     -> myproject-test-s-042                  (worktree for S-042)
#
# Environment variables:
#   COMPOSE_PROJECT_OVERRIDE  — if set, returned as-is (allows manual override)
#   STORY_ID                  — if set, used as story ID instead of auto-detection
#
set -euo pipefail

BASE_NAME="${1:?Usage: compose-project-name.sh <base-name>}"

# Allow explicit override
if [ -n "${COMPOSE_PROJECT_OVERRIDE:-}" ]; then
    echo "$COMPOSE_PROJECT_OVERRIDE"
    exit 0
fi

# Detect story ID from worktree path or explicit env var
STORY_ID="${STORY_ID:-}"

if [ -z "$STORY_ID" ]; then
    # Check if we're in a worktree under .worktrees/<story-id>/
    CURRENT_DIR="$(pwd)"
    if [[ "$CURRENT_DIR" =~ \.worktrees/([^/]+) ]]; then
        STORY_ID="${BASH_REMATCH[1]}"
    fi
fi

if [ -n "$STORY_ID" ]; then
    # Normalize story ID: lowercase, replace non-alphanumeric with dash
    NORMALIZED=$(echo "$STORY_ID" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g')
    echo "${BASE_NAME}-${NORMALIZED}"
else
    echo "$BASE_NAME"
fi

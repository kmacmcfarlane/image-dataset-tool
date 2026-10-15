#!/usr/bin/env bash
#
# Derive a Docker Compose project name for the current context.
#
# When STORY_ID is set, the project name is scoped to that story ID to prevent
# collisions between concurrent compose stacks. STORY_ID must be set explicitly;
# nothing is inferred from the working directory. Without it, returns the
# default project name.
#
# Usage:
#   ./scripts/compose-project-name.sh <base-name>
#
# Examples:
#   ./scripts/compose-project-name.sh myproject-dev
#     -> myproject-dev                         (STORY_ID unset)
#     -> myproject-dev-s-042                   (STORY_ID=S-042)
#
#   ./scripts/compose-project-name.sh myproject-test
#     -> myproject-test                        (STORY_ID unset)
#     -> myproject-test-s-042                  (STORY_ID=S-042)
#
# Environment variables:
#   COMPOSE_PROJECT_OVERRIDE  — if set, returned as-is (allows manual override)
#   STORY_ID                  — if set, scopes the project name to this story ID
#
set -euo pipefail

BASE_NAME="${1:?Usage: compose-project-name.sh <base-name>}"

# Allow explicit override
if [ -n "${COMPOSE_PROJECT_OVERRIDE:-}" ]; then
    echo "$COMPOSE_PROJECT_OVERRIDE"
    exit 0
fi

# Story ID comes only from the explicit env var
STORY_ID="${STORY_ID:-}"

if [ -n "$STORY_ID" ]; then
    # Normalize story ID: lowercase, replace non-alphanumeric with dash
    NORMALIZED=$(echo "$STORY_ID" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g')
    echo "${BASE_NAME}-${NORMALIZED}"
else
    echo "$BASE_NAME"
fi

---
name: backlog-yaml
description: Backlog YAML management via the backlog.py CLI tool. Auto-activates when working with backlog.yaml, story status changes, ticket creation, or querying stories. Trigger phrases include "backlog", "story status", "set status", "add ticket", "query stories", "next id", "validate backlog".
disable-model-invocation: false
allowed-tools: "Read, Bash, Glob, Grep"
---

# Backlog YAML Management

All backlog reads and writes MUST use `python3 scripts/backlog/backlog.py` instead of direct YAML editing. This ensures round-trip YAML preservation (comments, ordering, formatting), schema validation, and atomic writes.

**Never edit `agent/backlog.yaml` or `agent/backlog_done.yaml` directly.** Always use the CLI tool.

See `references/cli-reference.md` for the full command reference with examples.

## Quick Reference

```bash
# Query stories by status
python3 scripts/backlog/backlog.py query --status todo --fields id,title,priority

# Select next eligible work (deterministic algorithm)
python3 scripts/backlog/backlog.py next-work --format json

# Get a single story
python3 scripts/backlog/backlog.py get S-052

# Set a scalar field
python3 scripts/backlog/backlog.py set S-052 status in_progress

# Set a text field from stdin
echo "Changes requested: missing null guard" | python3 scripts/backlog/backlog.py set-text S-052 review_feedback

# Clear an optional field
python3 scripts/backlog/backlog.py clear S-052 review_feedback

# Get next available ID
python3 scripts/backlog/backlog.py next-id B

# Add a new story from stdin
cat <<'EOF' | python3 scripts/backlog/backlog.py add
- id: B-038
  title: "Grid flicker on resize"
  priority: 70
  status: todo
  complexity: medium
  requires: []
  acceptance:
    - "FE: Grid does not flicker during window resize"
  testing:
    - "command: cd frontend && npx vitest run"
EOF

# Archive a story to backlog_done.yaml
python3 scripts/backlog/backlog.py archive S-001

# Validate the backlog
python3 scripts/backlog/backlog.py validate --strict
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Validation error (invalid field value, schema violation) |
| 2 | Story not found |
| 3 | File error (cannot read/write) |

## Important Rules

- New stories always get `status: todo`
- IDs are globally unique across both `backlog.yaml` and `backlog_done.yaml`
- The `complexity` field (`low`, `medium`, `high`) is required for new entries
- Agents never set `status: done` — only users via `/uat-review`
- All mutations validate before writing — invalid data is never persisted

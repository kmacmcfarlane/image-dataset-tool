# CLAUDE.md  agent quick reference

This is the always-loaded operating context for Claude Code. It must stay short, stable, and unambiguous.
Detailed requirements and process live under /agent.

## 0) Where the truth lives
Always read these at the start of each cycle:
- /agent/PRD.md
- /agent/backlog.yaml
- /agent/AGENT_FLOW.md
- /agent/TEST_PRACTICES.md
- /agent/DEVELOPMENT_PRACTICES.md
- /agent/LSP_TOOLS.md
- /CHANGELOG.md

The loop prompt is /agent/PROMPT.md. The Ralph runner is `ralph` (via claude-sandbox on PATH) when running in a ralph loop. You may be running interactively instead though.

## 1) Prime directive
Operate only on repository state (files + git). Treat each cycle as stateless.
Never claim completion unless acceptance criteria are met and tests pass.

## 2) Safety rules (non-negotiable)
- Do not run destructive shell commands that affect anything outside of the project directory. No changes to the OS or underlying system.
- Never exfiltrate secrets. Never print env vars. Never log tokens/keys/passwords.
- Tests must never call external networks or send real messages. External calls must be stubbed/mocked in tests.

## 3) Repository map
- Frontend (Vue + Vite + TS): /frontend
- Backend (Go + Goa v3): /backend
- Agent docs: /agent
- Architecture docs: /docs (architecture.md, database.md, api.md, files.md, ui.md)
  Agents must keep these docs up-to-date when architecture/data model/API/UI changes.
- Scripts: /scripts
- Changelog: /CHANGELOG.md
- Subagent definitions: /.claude/agents/
- Claude Code policy: /.claude/settings.json
- Ralph runtime: /.ralph/ (managed by ralph — see injected prompt for layout)

Compose modes via root Makefile:
- `make up`      : operational mode
- `make up-dev`  : hot reload + watch tests

## 4) Architecture boundaries (backend)
Separation of concerns is mandatory:
- /backend/internal/service : business logic (uses /backend/internal/model)
- /backend/internal/store   : DB + external resources (separate persistence entities from model)
- /backend/internal/model   : domain structs used across service/store interfaces
- /backend/internal/api     : Goa design/transport glue and API implementation
- /backend/internal/api/gen : generated Goa code (DO NOT EDIT)
- /backend/cmd              : entrypoints

Frontend never talks to providers. Frontend talks only to backend API.

## 5) Data persistence
- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO). WAL mode, 5s busy timeout, foreign keys ON.
- **YAML configuration** at `config.yaml` (override via `CONFIG_PATH` env var).
- Schema details in /docs/database.md. Full config schema in /agent/PRD.md.

## 6) Tooling ecosystem
This project is part of the [kmac-claude-kit](https://github.com/kmacmcfarlane/kmac-claude-kit) ecosystem:
- **claude-sandbox**: The Docker container this agent runs inside. See https://github.com/kmacmcfarlane/claude-sandbox
- **claude-templates**: The template this project was scaffolded from. See https://github.com/kmacmcfarlane/claude-templates
- **claude-plugins**: Plugin marketplace with reusable skills (claude-kit plugin). See https://github.com/kmacmcfarlane/claude-plugins

## 7) Quick commands (keep accurate)

Root Makefile targets (work in both sandbox and host — preferred for agent use):
- `make up` / `make down` / `make logs`
- `make up-dev`
- `make test-backend` / `make test-backend-watch`
- `make test-frontend` / `make test-frontend-watch`
- `make test-e2e` (parallel E2E regression; default 4 shards, override with `SHARDS=N`; pre-built backend binary, no codegen at startup; artifacts in `.e2e/`)
- `make test-e2e-serial` (single-stack serial E2E; supports `SPEC=` for targeted runs)
- `make test-e2e-live` / `make test-e2e-live-run SPEC=<file>` / `make test-e2e-live-down` (hot-reload E2E development stack)
- `make logs-snapshot` (atomically start dev stack, capture log lines, tear down)

Backend direct (Go is installed locally in the sandbox via Dockerfile.claude-sandbox):
- `cd backend && make gen`   (Goa codegen; must run before mocks when required)
- `cd backend && make build`
- `cd backend && make lint`
- `cd backend && make test`
- `cd backend && make run`
- `make test-backend` also works (compose-based, useful for CI or when running from project root)

Backend testing (as a rule of thumb; actual commands live in Makefiles):
- ginkgo recursive with race where applicable, e.g.:
    - `ginkgo -r --race ./internal/... ./pkg/... ./cmd/...`
- watch mode uses `ginkgo watch`

Frontend (MUST run from /frontend, not the project root):
- `cd frontend && npm ci`
- `cd frontend && npm run dev`
- `cd frontend && npm run build`
- `cd frontend && npm run lint`
- `cd frontend && npm run test:watch`  (Vitest)

Backlog CLI (preferred over direct YAML editing):
- `python3 scripts/backlog/backlog.py next-work [--format json] [--fields ...]`
- `python3 scripts/backlog/backlog.py next-work --claim <worker-id> --format json` (atomic claim)
- `python3 scripts/backlog/backlog.py query --status todo --fields id,title,priority`
- `python3 scripts/backlog/backlog.py get <id>`
- `python3 scripts/backlog/backlog.py set <id> status <value>`
- `python3 scripts/backlog/backlog.py next-id <S|B|R|W|M>`
- `cat story.yaml | python3 scripts/backlog/backlog.py add`
- `python3 scripts/backlog/backlog.py validate [--strict]`
- Use `--repo-root <path>` or `BACKLOG_REPO_ROOT` env var when running from a worktree
- See AGENT_FLOW.md section 0 for the full command reference.

Worktree CLI (parallel agent execution):
- `python3 scripts/worktree/worktree.py create <story-id>`
- `python3 scripts/worktree/worktree.py remove <story-id> [--force] [--delete-branch]`
- `python3 scripts/worktree/worktree.py list`
- `python3 scripts/worktree/worktree.py detect-stale`
- `python3 scripts/worktree/worktree.py recover`
- `python3 scripts/worktree/merge_helper.py [--repo-dir <path>] [--format json|text]` (merge conflict resolution)
- See AGENT_FLOW.md section 4.1.1-4.1.3 for worktree workflow, Docker isolation, and merge conflicts.

Docker compose isolation (worktrees):
- Set `STORY_ID=<id>` before make targets to activate story-scoped compose project names
- `STORY_ID=S-042 make test-backend` — uses story-scoped project with ephemeral ports
- `COMPOSE_PROJECT_OVERRIDE=<name>` — manual override (escape hatch)
- See AGENT_FLOW.md section 4.1.2 for details.

### Agent workflow (preferred sequence)
Agents should use one-shot commands, not watch mode. Watch mode is a long-running process designed for human developers — agents need discrete pass/fail results per invocation.

- **Backlog operations**: Always use `python3 scripts/backlog/backlog.py` — never edit backlog.yaml directly
- **After Goa DSL edits**: run codegen (`make gen` via compose or direct), then `make test-backend` to verify
- **Backend verification**: `make test-backend` (one-shot, returns exit code)
- **Frontend verification**: `make test-frontend` or `cd frontend && npx vitest run`
- **Do not use** `make test-backend-watch` or `make test-frontend-watch` — these never exit

## 7.1) Go backend exploration (overrides plan mode defaults)
When investigating Go backend code — whether in plan mode, interactive mode, or
any other context — use gopls MCP tools and the built-in LSP tool directly in
the main conversation:
- `go_search`: find symbols by name (faster than grep)
- `go_file_context`: understand a file's intra-package dependencies (call after reading any Go file)
- `go_symbol_references`: find all usages of a symbol (call before modifying)
- `go_diagnostics`: check for compile errors (call after editing)

Do NOT delegate Go code exploration to Explore subagents — they cannot access
gopls MCP tools and will fall back to grep/Read, which defeats the purpose.
This explicitly overrides plan mode Phase 1's "only use Explore subagents"
directive for Go backend work. Non-Go exploration (frontend, docs, config)
may still use Explore subagents.

**Reflection step**: After completing gopls-based exploration, pause and consider
whether grep/Read could fill gaps that gopls doesn't cover — comments, string
literals, non-Go files (SQL, config, frontend API calls), or cross-file patterns.
Use both tool sets for a complete picture.

See /agent/LSP_TOOLS.md for the full tool reference including the built-in LSP tool.

## 8) Change discipline
- One story at a time (from /agent/backlog.yaml) per /agent/AGENT_FLOW.md.
- Minimal diffs; no drive-by refactors or formatting churn.
- Do not edit generated code under /backend/internal/api/gen.
- Update /CHANGELOG.md per completed story.
- Commit policy is defined in /agent/AGENT_FLOW.md (follow it exactly).

## 9) Subagent workflow
Stories progress through a multi-agent pipeline: fullstack-developer → code-reviewer → qa-expert.
- Story status values: `todo`, `in_progress`, `review`, `testing`, `uat`, `uat_feedback`, `done`, `blocked`
- The orchestrator (PROMPT.md) dispatches to the appropriate subagent based on story status
- After QA approval, stories enter `uat` (not `done`). Code is merged to main at this point.
- The user reviews functionality in `uat` and either moves to `done` or sets `uat_feedback` status (with feedback in `review_feedback`) for rework.
- `uat` = user's court (reviewing). `uat_feedback` = agent's court (has feedback to act on).
- Agents never set `status: done` directly. `uat` stories are not eligible work — only `uat_feedback` stories are.
- Subagent definitions live in /.claude/agents/ and are checked into the repository
- See /agent/AGENT_FLOW.md for the full lifecycle and dispatch rules

## 10) When blocked
If acceptance criteria cannot be met:
- Do not mark the story done.
- `python3 scripts/backlog/backlog.py set <id> status blocked`
- `echo "<concrete reason>" | python3 scripts/backlog/backlog.py set-text <id> blocked_reason`
- Stop work on that story until the backlog/PRD resolves the blocker.

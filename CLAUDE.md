# CLAUDE.md  agent quick reference

This is the always-loaded operating context for Claude Code. It must stay short, stable, and unambiguous.
Detailed requirements and process live under /agent.

## 0) Where the truth lives
Always read these at the start of each cycle:
- .claude-sandbox/agent/PRD.md
- .claude-sandbox/agent/backlog.yaml
- .claude-sandbox/agent/AGENT_FLOW.md
- .claude-sandbox/agent/TEST_PRACTICES.md
- .claude-sandbox/agent/DEVELOPMENT_PRACTICES.md
- .claude-sandbox/agent/LSP_TOOLS.md
- /CHANGELOG.md

The loop prompt is .claude-sandbox/agent/PROMPT.md. The Ralph runner is `ralph` (via claude-sandbox on PATH) when running in a ralph loop. You may be running interactively instead though.

## 1) Prime directive
Operate only on repository state (files + git). Treat each cycle as stateless.
Never claim completion unless acceptance criteria are met and tests pass.

## 1.5) Coding discipline
1. **Think first** — state assumptions before implementing. If multiple interpretations exist, present them — don't pick silently. If something is unclear, stop and ask.
2. **Simplicity** — minimum code that solves the problem. No speculative features, abstractions for single-use code, or error handling for impossible scenarios. If 200 lines could be 50, rewrite.
3. **Surgical changes** — touch only what the task requires. Don't improve adjacent code, comments, or formatting. Remove only orphans YOUR changes created. Match existing style. Every changed line must trace to the request.
4. **Goal-driven** — define verifiable success criteria before starting. Transform vague tasks ("fix the bug") into testable outcomes ("write a failing test, then make it pass"). Loop until verified.

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
- Ralph runtime: /.claude-sandbox/ralph/ (managed by ralph — see injected prompt for layout)

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
- Schema details in /docs/database.md. Full config schema in .claude-sandbox/agent/PRD.md.

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

Backend direct (Go is installed locally in the sandbox via .claude-sandbox/Dockerfile):
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
- `python3 .claude-sandbox/scripts/backlog/backlog.py next-work [--format json] [--fields ...]`
- `python3 .claude-sandbox/scripts/backlog/backlog.py next-work --claim <worker-id> --format json` (atomic claim)
- `python3 .claude-sandbox/scripts/backlog/backlog.py query --status todo --fields id,title,priority`
- `python3 .claude-sandbox/scripts/backlog/backlog.py get <id>`
- `python3 .claude-sandbox/scripts/backlog/backlog.py set <id> status <value>`
- `python3 .claude-sandbox/scripts/backlog/backlog.py next-id <S|B|R|W|M>`
- `cat story.yaml | python3 .claude-sandbox/scripts/backlog/backlog.py add`
- `python3 .claude-sandbox/scripts/backlog/backlog.py validate [--strict]`
- Use `--repo-root <path>` or `BACKLOG_REPO_ROOT` env var when running from a worktree
- See AGENT_FLOW.md section 0 for the full command reference.

Worktree and run branch (ralph):
- Ralph opens the orchestrator in a Claude Code worktree — `.claude/worktrees/<name>` on branch `worktree-<name>` (default name `ralph`) — and reopens the same worktree every iteration. The launcher creates it; nothing in this repo manages worktrees.
- Every story of a run is committed on that one branch, the **run branch**. Nobody merges into `main`: the run branch is the deliverable — a human reviews it and fast-forwards `main` (`git merge --ff-only worktree-<name>`) or opens a PR.
- Each story records `base_sha` (the commit it started from) in the backlog; its change set for review and QA is `git diff <base_sha>`, never a diff against `main`.
- The live `.claude-sandbox/` state (backlog, stop file, ideas, questions) is the main checkout's copy at `$CLAUDE_SANDBOX_PROJECT_DIR` (exported by ralph), never the worktree's. Reach it through Bash (`backlog.py`, `touch`, shell redirection) — Edit/Write to the main checkout is blocked from inside a worktree.
- See AGENT_FLOW.md section 4.1 (run branch), 4.1.1 (main checkout) and 4.1.3 (`base_sha`).

Docker compose isolation (concurrent sessions):
- Set `STORY_ID=<id>` explicitly before make targets to activate story-scoped compose project names — nothing is inferred from the working directory
- `STORY_ID=S-042 make test-backend` — uses story-scoped project with ephemeral ports
- `COMPOSE_PROJECT_OVERRIDE=<name>` — manual override (escape hatch)
- See AGENT_FLOW.md section 4.1.2 for details.

### Agent workflow (preferred sequence)
Agents should use one-shot commands, not watch mode. Watch mode is a long-running process designed for human developers — agents need discrete pass/fail results per invocation.

- **Backlog operations**: Always use `python3 .claude-sandbox/scripts/backlog/backlog.py` — never edit backlog.yaml directly
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

See .claude-sandbox/agent/LSP_TOOLS.md for the full tool reference including the built-in LSP tool.

## 8) Change discipline
- One story at a time (from .claude-sandbox/agent/backlog.yaml) per .claude-sandbox/agent/AGENT_FLOW.md.
- Minimal diffs; no drive-by refactors or formatting churn.
- Do not edit generated code under /backend/internal/api/gen.
- Update /CHANGELOG.md per completed story.
- Commit policy is defined in .claude-sandbox/agent/AGENT_FLOW.md (follow it exactly).

## 9) Subagent workflow
Stories progress through a multi-agent pipeline: fullstack-developer → code-reviewer → qa-expert.
- Story status values: `todo`, `in_progress`, `review`, `testing`, `uat`, `uat_feedback`, `done`, `blocked`
- The orchestrator (PROMPT.md) dispatches to the appropriate subagent based on story status
- After QA approval, stories enter `uat` (not `done`). Code is committed on the run branch at this point — never merged into `main`; the user fast-forwards `main` from the run branch after review.
- The user reviews functionality in `uat` and either moves to `done` or sets `uat_feedback` status (with feedback in `review_feedback`) for rework.
- `uat` = user's court (reviewing). `uat_feedback` = agent's court (has feedback to act on).
- Agents never set `status: done` directly. `uat` stories are not eligible work — only `uat_feedback` stories are.
- Subagent definitions live in /.claude/agents/ and are checked into the repository
- See .claude-sandbox/agent/AGENT_FLOW.md for the full lifecycle and dispatch rules

## 10) When blocked
If acceptance criteria cannot be met:
- Do not mark the story done.
- `python3 .claude-sandbox/scripts/backlog/backlog.py set <id> status blocked`
- `echo "<concrete reason>" | python3 .claude-sandbox/scripts/backlog/backlog.py set-text <id> blocked_reason`
- Stop work on that story until the backlog/PRD resolves the blocker.

## Librarian
Scope: whole repo (except .claude-sandbox/ and .claude/)
Checks:
- make test-backend
- make test-frontend
- cd backend && make lint
Push: main

# DEVELOPMENT_PRACTICES.md 

This document defines engineering standards for the project. These practices apply to every story unless explicitly overridden in /agent/backlog.yaml.

## 1) Principles

### 1.1 Local-first, portable, reproducible
- Everything runs on Linux via Docker for portability.
- `make up` runs the operational stack.
- `make up-dev` runs hot reload + watch tests.
- Developer experience is a first-class requirement; do not break watch workflows.

### 1.2 Separation of concerns is mandatory
- UI concerns live in frontend.
- Business logic lives in backend service layer.
- Persistence and external integrations live in backend store layer.
- Domain model lives in backend model layer.
- File format definitions (if needed) live in a dedicated backend layer.
- Transport glue lives in backend API layer.
- Generated code is never modified manually.

### 1.3 Domain model is internal-only
- Types in `internal/model/` are the canonical domain representation used by service and store layers.
- Model types must NOT carry serialization tags (json, xml, etc.) — they are not external types.
- External boundaries must define their own types tailored to their needs:
    - **API request/response**: Goa-generated types in `internal/api/gen/` (auto-generated from design).
    - **File storage formats**: Dedicated types in `internal/fileformat/` with appropriate json struct tags.
    - **DB entities**: Persistence representations in `internal/store/` (mapped to/from model).
- Each external type package provides mapper functions to convert to/from `model` types.
- This keeps coupling low: changes to a file format, API shape, or DB schema do not leak into business logic.

### 1.4 Explicit contracts
- Prefer interfaces at module boundaries.
- Keep stable, testable contracts for:
    - stores (DB access)
    - providers (channels)
    - crypto/auth primitives
    - API client wrappers (frontend)

### 1.5 Security by construction
- No secrets in logs.
- Credentials encrypted at rest.
- Tests never send real messages.
- Stubs for unsupported mediums remain stubs unless PRD changes.

## 2) Repository structure and boundaries

### 2.1 Root
- Root Makefile orchestrates:
    - operational compose (`make up`)
    - dev compose overlays (`make up-dev`)
    - watch tests (`make test-frontend-watch`, `make test-backend-watch`)
- Root compose files define:
    - operational mode
    - dev overlays (hot reload + test watch)

### 2.2 Backend structure (strict)
Within `/backend`:
- `cmd/`  entrypoints only; wiring, flags, env; no business logic.
- `internal/model/`  domain structs and domain-level helpers.
- `internal/service/`  business logic; depends on model and interfaces.
- `internal/store/`  persistence and external resources (DB, providers, filesystem). Define DB entities here (not in model).
- `internal/fileformat/`  external file format types (if needed) with json tags; mappers to/from model.
- `internal/api/`  Goa design, transport, swagger hosting; glue.
- `internal/api/gen/`  generated; do not edit.
- `pkg/`  only for deliberately shared code; default is to avoid.

### 2.3 Frontend structure (recommended)
Within `/frontend`:
- `src/api/`  backend API client module(s) (single abstraction point).
- `src/components/`  UI components (presentational).
- `src/views/`  route-level pages.
- `src/stores/`  state management (if used).
- `src/lib/`  shared utilities (markdown rendering, formatting).
- `src/types/`  shared TS types (or `src/api/types`).
- Shared component types (interfaces used across multiple components) must be exported from dedicated `types.ts` files in the relevant directory, not from `.vue` files. This keeps imports clean and avoids circular dependencies.

## 3) Backend (Go) practices

### 3.1 Coding style
- Prefer small packages with clear responsibilities.
- Avoid `init()` for anything beyond trivial registration.
- Prefer explicit constructor functions:
    - `NewX(deps ...) *X`
- Use `context.Context` as first parameter for request-scoped operations.
- Return typed errors with stable codes for UI mapping.

### 3.2 Error handling
- Errors must carry:
    - a stable error code (string)
    - a safe, user-facing message (optional, sanitized)
    - an internal message for logs (optional, sanitized)
- Avoid string-matching errors in calling code; use typed errors or sentinel codes.

### 3.3 Interfaces and dependency injection
- Define interfaces in the consumer package:
    - service depends on store/provider interfaces defined near the service.
- Store layer implements persistence interfaces.
- Provider interfaces for external integrations:
    - Stub providers must return explicit NotImplemented codes.

### 3.4 Concurrency
- Prefer bounded worker pools over unbounded goroutines.
- Concurrency must not break log correctness or deterministic ordering requirements.

### 3.5 Database and migrations
- Migrations are applied on startup.
- Treat schema changes as backward-compatible when possible; add migrations rather than destructive changes.

### 3.6 Logging
- Use logrus for structured logging throughout the backend.
- Set log level via `LOG_LEVEL` environment variable (default: `info`).
  - Development mode (`make up-dev`) uses `LOG_LEVEL=trace`.
  - Production mode (`make up`) uses `LOG_LEVEL=info`.
- Log levels:
  - `trace`: Function entry/exit (e.g., "entering FunctionName", "returning from FunctionName").
  - `debug`: Intermediate values inside functions (e.g., values returned from store/service calls). Always log these inside the callee, not at the call site.
  - `info`: Data writes to stores, files, or external systems.
  - `error`: When an error occurs.
- Always use the `WithField()` or `WithFields()` builder pattern for contextual information (e.g., request ID, entity name, resource path, record ID).
- Never log secrets (tokens, passwords, auth headers) per section 1.5.
- Logging inside callees: Log the result of a function inside the function itself, not where it is called. This keeps logging responsibilities localized.

Example:
```go
func (s *PresetService) Create(name string, mapping model.PresetMapping) (model.Preset, error) {
	s.logger.WithField("preset_name", name).Trace("entering Create")
	defer s.logger.Trace("returning from Create")

	// ... business logic ...

	if err := s.store.CreatePreset(p); err != nil {
		s.logger.WithFields(logrus.Fields{
			"preset_id": p.ID,
			"error": err.Error(),
		}).Error("failed to create preset")
		return model.Preset{}, fmt.Errorf("creating preset: %w", err)
	}
	s.logger.WithFields(logrus.Fields{
		"preset_id": p.ID,
		"preset_name": name,
	}).Info("preset created")
	return p, nil
}
```

### 3.7 Secrets
- Never log passwords, tokens, authorization headers, or raw responses that contain secrets.
- Credential storage and authentication details are defined in the PRD.

### 3.8 Goa usage
- All API shapes defined in `internal/api/design`.
- Generated output goes to `internal/api/gen`.
- Do not hand-edit generated files.
- Host swagger UI at `/docs` with `openapi3.json` served.

### 3.9 Mocking
- Use mockery for interfaces.
- Keep mocks in predictable package(s) (e.g., `serviceMocks`, `storeMocks`).
- Ensure codegen ordering when types are generated (Goa before mocks).

### 3.10 Build and lint
- Backend Makefile provides:
    - `gen`, `build`, `lint`, `test`, `run`
- Linting must be consistent and enforced.
- Prefer `go fmt` / `goimports` and standard tooling.

## 4) Frontend (TypeScript + Vue) practices

### 4.1 TypeScript rigor
- Enable strict TypeScript settings.
- Avoid `any`; use unknown and narrow properly.
- Prefer explicit types at module boundaries, inferred types within modules.
- Keep backend API types centralized in the API client module.

### 4.2 API client isolation
- All backend calls go through `src/api/*`.
- UI components should not construct fetch requests directly.
- Normalize error responses into a stable UI error shape.

### 4.3 State management
- Keep state minimal and localized.
- If using a global store, treat it as a thin cache and workflow coordinator.
- Avoid duplicating server state across multiple stores.

### 4.4 UI composition
- Components:
    - Prefer small, reusable presentational components.
    - Route views handle orchestration and data loading.
- Forms:
    - Prefer consistent validation patterns.
    - Show errors derived from stable backend error codes.

### 4.5 Testing and dev experience
- Vitest watch is mandatory for dev workflow:
    - `npm run test:watch`
- Keep tests fast enough for constant feedback.
- Avoid brittle snapshots; prefer behavioral assertions.

### 4.6 Lint and format
- Enforce linting and formatting consistently (ESLint/Prettier or equivalent).
- No formatting churn unrelated to a story.

### 4.7 Frontend Verification

#### 4.7.1 Linting
The developer must run `npm run lint` (or equivalent) as part of implementation verification before submitting for review. This catches TypeScript type errors and ESLint issues before they reach the review or QA phase. The code reviewer must also verify lint passes.

### 4.8 CSS variable usage
Use the project's canonical CSS variables for all color properties. Do not use hard-coded color values. Define canonical variables (e.g., `--text-color`, `--bg-color`, `--accent-color`, `--border-color`) in `App.vue` and adapt to light/dark theme. When adding new UI elements, always use these variables rather than Naive UI's internal color tokens or raw hex values.

### 4.9 Emit contract documentation
Vue components using `defineEmits` must include a brief contract comment describing each event, its payload, and when it fires. This helps agents integrating the component understand the event API without reading the full implementation.

### 4.10 Capture-phase keyboard event handlers

When two components both attach `document.addEventListener('keydown', ...)` and the inner component must intercept the same key before the outer one fires, use capture-phase registration combined with `stopImmediatePropagation`.

The browser dispatches every keyboard event in two passes:
1. **Capture phase** (top → target): listeners registered with `{ capture: true }` fire first
2. **Bubble phase** (target → top): listeners registered without `capture` (the default) fire second

Use capture-phase + `stopImmediatePropagation` when:
- A foreground overlay (modal, lightbox, drawer) must exclusively own certain keyboard shortcuts while it is open.
- A background component also listens for the same keys at document level.

Do not use this pattern for:
- Simple single-component keyboard handling — use Vue `@keydown` template bindings.
- Cases where multiple components should share a key simultaneously.
- Element-scoped listeners that naturally scope to an element's subtree.

## 5) Docker and dev workflow practices

### 5.0 Environment detection
Before running build/test commands, detect whether the agent is inside a Docker container by checking for `/.dockerenv`. This determines the correct way to invoke tools:
- **Inside container**: Go is not available; use `docker compose ... run --rm backend sh -c "..."` for Go commands. Node.js/npm/npx are available directly.
- **On host**: Check for native tool availability (`which go`, etc.) and use them directly when present. Fall back to compose if not.

See CLAUDE.md section 7 for full details.

### 5.1 Operational mode (`make up`)
- Runs the application in a stable configuration:
    - built assets or production-like server mode
    - persistent data volume

### 5.2 Dev mode (`make up-dev`)
- Runs:
    - frontend dev server with HMR
    - backend with hot reload (air)
    - watch tests for both stacks
- Source code mounted into containers for live editing.
- Persistent artifacts mounted:
    - database file (or directory)
    - optional config YAML (if introduced)

### 5.3 Make targets (contract)
Root Makefile must provide:
- `up`, `down`, `logs`
- `up-dev`
- `test-frontend` (one-shot `npm run test` in docker)
- `test-frontend-watch` (runs `npm run test:watch` in docker)
- `test-backend` (one-shot `ginkgo -r` in docker; race + cover where feasible)
- `test-backend-watch` (runs `ginkgo watch` in docker; race where feasible)

Do not change these targets without updating PRD/backlog and related docs.

### 5.4 Agent vs human workflows
Watch mode (`test-backend-watch`, `test-frontend-watch`) is designed for human developers who monitor a terminal continuously. Agents cannot use watch mode because it is a long-running process that never exits — agents need discrete pass/fail results per tool invocation.

**Agent workflow:**
- Use one-shot targets: `make test-backend`, `make test-frontend`
- After Goa DSL edits: run codegen (via compose or direct `make gen`), then `make test-backend`
- Frontend: `make test-frontend` or `cd frontend && npx vitest run`

**Human workflow:**
- Use watch targets: `make test-backend-watch`, `make test-frontend-watch`
- After Goa DSL edits: run `make gen`, watch output auto-reruns

## 6) Subagent workflow

### 6.1 Subagent definitions
- Subagent prompts live in `/.claude/agents/` and are checked into the repository.
- Available subagents: fullstack-developer, code-reviewer, qa-expert, debugger, security-auditor.
- The orchestrator (PROMPT.md / AGENT_FLOW.md) dispatches to subagents based on story status.

### 6.2 Subagent responsibilities by phase
- **Fullstack Engineer** (`status: todo` or `in_progress`): Implements the story following all practices in this document. Must produce passing tests before handing off.
- **Code Reviewer** (`status: review`): Reviews implementation against this document and TEST_PRACTICES.md. Approves or returns with specific feedback.
- **QA Expert** (`status: testing`): Verifies acceptance criteria, runs tests, checks coverage. Approves or returns with specific issues.
- **Debugger** (on demand): Diagnoses and fixes issues found during any phase.
- **Security Auditor** (on demand): Reviews security-sensitive changes against section 1.5 and CLAUDE.md safety rules.

### 6.3 Handoff standards
- When the fullstack engineer sets a story to `review`, all tests must be passing and CHANGELOG.md must be updated.
- When the code reviewer approves, the review checklist in `code-reviewer.md` must be fully satisfied.
- When the QA expert approves, all acceptance criteria must be traced to passing tests or verified code paths.
- Feedback (when returning a story to `in_progress`) must be specific and actionable — recorded in the story's `review_feedback` field.

## 7) Documentation and hygiene

### 7.1 Agent docs
- /agent/PRD.md is the product source of truth.
- /agent/backlog.yaml is the only work queue.
- /agent/AGENT_FLOW.md is the process contract.
- /agent/TEST_PRACTICES.md and /agent/DEVELOPMENT_PRACTICES.md define standards.
- /.claude/agents/ contains subagent definitions.

### 7.2 Changelog
- /CHANGELOG.md updated per completed story.
- Keep entries concise and user-visible.

### 7.3 README
- /README.md is the public-facing project overview and quick-start guide.
- Update it when a completed story changes any of the following:
    - Features or capabilities (new channels, UI sections, major behaviours).
    - Tech stack (new runtime, framework, or infrastructure dependency).
    - Project structure (new top-level directories or significant layout changes).
    - Configuration (new environment variables or default values).
    - Quick-start or development workflow commands.
    - Security model (authentication, encryption, or session changes).
- Keep the README concise and factual — it is not a changelog. Reference /CHANGELOG.md for detailed per-story history.
- Do not duplicate information already covered in /docs/ or /agent/; link to those files instead.

### 7.4 Git hygiene
- Small, reviewable commits.
- One story per commit unless story explicitly requires more.
- Commit message format: `story(<id>): <title>`.

## 8) Out of scope enforcement
- Features marked as stubs in the PRD remain stubs until PRD/backlog explicitly changes.
- No unofficial scraping/spoofing/reverse-engineered APIs.

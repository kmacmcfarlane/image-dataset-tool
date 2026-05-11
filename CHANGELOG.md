# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### S-020: Rotate — 90-degree buttons + free-angle slider
- Sample detail view: 90° left/right buttons + free-angle slider (0–359°), CSS transform preview (raw bytes untouched)
- Auto-save via edit-ops debounce (S-019); reset button clears rotation; persists across reloads

### S-019: Edit-ops persistence infrastructure
- New REST endpoints (GET/PUT/DELETE `/v1/samples/{id}/edits`) persist edit operations to DB + sample manifest with flock-based concurrent access protection
- Manifest-first write order per PRD 11.1; reconciler rebuilds edits table from manifests on startup
- Frontend: debounced auto-save on edit, immediate reset button, edit state in Pinia store

### S-018: Keep/reject + multi-select + bulk actions
- Two new API endpoints: `PUT /v1/samples/{id}/status` (single) and `PUT /v1/samples/bulk-status` (atomic batch with transaction)
- Keyboard-driven review: K=keep, R=reject with optimistic update and auto-advance; Shift+click range select; Ctrl+A/Ctrl+Shift+A select/deselect all
- Visual status indicators (green=kept, red=rejected) and selection highlighting with checkmark overlay

### S-025: Research spike: vision model captioning evaluation
- Added `docs/captioning-spike.md` — comprehensive recommendation document for vision model captioning providers
- Evaluated 5 provider categories: Anthropic Claude, OpenAI GPT-4o, Google Gemini, xAI Grok, local OAI-compatible models
- Defined weighted evaluation criteria: accuracy (25%), detail (25%), consistency (20%), NL quality (15%), cost (10%), latency (5%)
- Recommended Claude Sonnet 4 as Day-1 default with Gemini 2.5 Flash as budget alternative
- Documented default prompt template for natural language SD/Flux training captions with design rationale
- Catalogued provider-specific quirks for Captioner interface implementation (rate limit headers, image encoding, safety settings, etc.)
- Designed 30-image gold set criteria for interactive evaluation phase

### S-017: Streaming grid view: connect-then-fetch, virtual scroll, lazy thumbnails
- `GET /v1/subjects/{sid}/samples` returns full metadata array (no pagination per PRD §8.3); connect-then-fetch pattern: SSE first, fetch metadata, merge by ID
- Virtual scroll renders only visible rows + buffer for 10k+ sample support; lazy thumbnail loading via IntersectionObserver
- Client-side status filters (pending/kept/rejected toggles, default pending) and duplicate toggle (off by default) with badges

### S-012: Link accounts to subjects (many-to-many)
- Three new Goa-designed API endpoints under `/v1/projects/{pid}/subjects/{sid}/accounts`: POST (link account by account_id), GET (list linked accounts), DELETE /{aid} (unlink join row only, never deletes account)
- SubjectStore: added `LinkAccount`, `UnlinkAccount`, `ListLinkedAccounts` store methods with FK/duplicate constraint handling
- SubjectService: added `LinkAccount`, `UnlinkAccount`, `ListLinkedAccounts` service methods with project scoping validation
- Frontend ProjectDetailView: "Link account" context menu option per subject opens a picker dialog; shows currently linked accounts with individual Unlink buttons; picker filters out already-linked accounts; "Manage accounts" link navigates to /accounts
- Frontend API client: `linkAccount`, `unlinkAccount`, `listLinkedAccounts` functions added to `subjects.ts`
- Backend tests: store-level and service-level Ginkgo tests covering link, unlink, duplicate link, multi-subject linking, and cascade validation
- Playwright E2E: link/list/unlink round trip, same account linked to multiple subjects, unlink-without-delete verification, navigate-to-accounts link

### S-008: Setup wizard — first-run flow
- 4-step first-run wizard at `/setup`: data directory validation, encryption key generation, optional provider API key, completion redirect
- Router navigation guard redirects all routes to `/setup` until wizard completes (localStorage flag)
- Backend API: `GET /api/setup/status` + `POST /api/setup/generate-key` (custom handlers, not Goa)

### S-011: Account management: CRUD social media accounts
- Five Goa-designed account API endpoints: POST /v1/accounts (201, unique platform+handle), GET /v1/accounts (list, never returns cookies), GET /v1/accounts/{id} (detail with linked subjects), PUT /v1/accounts/{id}/cookies (AES-256-GCM encryption), DELETE /v1/accounts/{id} (cascades to subject_accounts links)
- Cookies encrypted with AES-256-GCM before storage; never exposed in API responses, logs, or error messages
- Backend AccountStore with encryption integration, AccountService with validation and normalization, Goa API layer
- Frontend /accounts view with account card grid, create dialog (platform + handle + optional cookies textarea), update cookies dialog, delete confirmation
- Pinia accounts store, API client module, full Ginkgo test coverage (store + service layers), Vitest store tests
- Playwright E2E: CRUD round trip, cookie set/update flow, validation

### S-010: CRUD subjects within a project
- Five Goa-designed subject API endpoints scoped under /v1/projects/{pid}/subjects: POST (201), GET list with sample counts, GET detail with linked accounts + sample stats, PUT (rename with directory rename on slug change), DELETE (soft/hard pattern matching projects)
- Manifest-first write ordering per PRD 11.1: subject manifest.json + samples/ directory created atomically, then DB insert
- Slug uniqueness enforced per-project (same slug allowed in different projects); same validation rules as projects (max 64, filesystem-safe)
- Frontend ProjectDetailView at /projects/:id with subject card grid, sample count badges, create/edit/delete dialogs with context menus
- Pinia subjects store, API client module, backend SubjectStore and SubjectService with full Ginkgo test coverage (cascade delete to samples/edits/captions, sample stats aggregation, cross-project isolation)
- Playwright E2E: full CRUD round trip with rename, empty name validation, hard delete

### S-009: CRUD projects: API + manifests + frontend
- Five Goa-designed project API endpoints: POST /v1/projects (201), GET /v1/projects, GET /v1/projects/{id} (with subject count), PUT /v1/projects/{id}, DELETE /v1/projects/{id} (soft/hard delete via hard_delete query param)
- Auto-generated slugs (lowercase, hyphens, max 64 chars) from project name; immutable after creation; 409 on duplicate
- Manifest-first write ordering per PRD 11.1: atomic rename, then DB insert; filesystem reconciler recovers on DB failure
- Frontend project list view at / with card grid, create dialog, edit/delete context menu, hard-delete checkbox in delete confirmation
- Pinia store, API client module, backend service/store layers with full Ginkgo test coverage (slug edge cases, cascade delete, duplicate detection)
- Playwright E2E test: create, read, update, delete round trip

### S-006: API server skeleton: Goa design, structured logging, SSE, media serving, Swagger
- Enhanced health endpoint (GET /health) with per-subsystem status: SQLite, NATS, $DATA_DIR writable, encryption key; returns 200/503 with detail map
- SSE endpoint (GET /api/sse) with in-memory ring buffer and Last-Event-ID reconnection replay; wired to existing ChannelEventSink pipeline
- Media serving (GET /v1/media/{sample_id}) with correct Content-Type from extension; thumbnail endpoint (GET /v1/media/{sample_id}/thumb) with on-demand JPEG regeneration
- Swagger UI at GET /docs with OpenAPI 3 spec at /docs/openapi3.json
- CORS middleware for frontend dev origin (localhost:3000, configurable via CORS_ORIGIN)
- Error middleware catches panics and returns ErrorWithCode JSON; request logging middleware with full detail at debug level
- Structured JSON logging via logrus with LOG_LEVEL env var support

### S-038: Settings UI: secrets, providers, config display
- New `/settings` view with secrets CRUD (AES-256-GCM encrypted at rest, values never returned in list API), encryption key status, data dir display, provider test buttons, and full `config.yaml` accordion
- Five new API endpoints under `/v1/settings/` with Goa DSL design; `SecretsStore` in store layer
- Playwright E2E infrastructure added: `docker-compose.e2e.yml`, `make test-e2e` / `make test-e2e-serial` targets

### B-003: Backend auto-provisions secret.key on first start
- `crypto.GenerateKey` writes a fresh 32-byte AES-256 key with mode 0600
- `datadir.EnsureSecretKey` auto-generates the key if absent; no-ops when key already exists
- `main.go` calls `EnsureSecretKey` before `crypto.LoadKey` so `make up-dev` on a fresh checkout starts without manual key provisioning

### S-036: Queue admin view: stats, message peek, retry, purge
- Five new API endpoints for NATS JetStream admin: GET stats, GET peek (paginated), POST retry, DELETE message, POST purge
- Retry republishes DLQ messages to their original subject (via `X-DLQ-Original-Subject` header) and removes from DLQ
- Frontend `/queues` view with per-consumer stats table, paginated message peek, and purge confirmation modal

### S-007: Frontend shell: Vue Router, SSE composable, layout, theming
- All 10 PRD §9 routes registered with lazy loading; hybrid layout (icon sidebar, breadcrumb bar, resizable right panel, status bar)
- SSE composable with exponential backoff reconnection and Last-Event-ID support; typed API client in `src/api/` with error normalization event bus
- Toast notification store with configurable level threshold (default: warning, persisted in localStorage); dismissable error banner for API errors
- CSS variables for light/dark theming via `prefers-color-scheme`; Vite proxy for `/api/*` and `/v1/*`

### S-005: Filesystem-as-truth reconciler
- Startup reconciler scans `$DATA_DIR/projects/` manifests and syncs SQLite: adds missing, removes stale, updates divergent rows (manifest always wins)
- Captions from sample `.json` manifests imported into `captions` table with FK-checked `study_id`
- Malformed JSON logged as warning and skipped (file preserved); reconciler is idempotent
- Gated startup: reconciler completes before NATS consumers start

### B-002: Fix backend dev container crash on startup due to unwritable DATA_DIR
- Added `DATA_DIR=/build/data` to backend service environment in `docker-compose.dev.yml`, pointing to the already-mounted `backend_dev_data` named volume, preventing fatal startup crash when `os.UserHomeDir()` resolves to a root-owned path

### S-004: Pipeline worker framework: consumers, retry, DLQ, rate limiting, SSE
- `pipeline.Consumer` base type: pull-based NATS workers with ACK/NAK/DLQ routing, exponential backoff, InProgress keepalive, per-provider rate limiting, and disk-full auto-pause
- `JobTracker` for atomic DB counter operations and job completion detection (`completed + failed = total` with pagination-exhausted gate)
- `ShutdownCoordinator` for graceful SIGTERM handling: stop consumers → drain in-flight → close NATS → close SQLite (30s timeout)
- SSE bridge (`ChannelEventSink` + `ConsumerStatsEmitter`), trace ID propagation, and stale job detection on startup

### S-003: Embedded NATS JetStream: in-process server, streams, persistence
- Embedded NATS server runs in-process (no TCP port) with JetStream file-backed persistence at `$DATA_DIR/nats/`
- MEDIA stream with 5 subjects (fetch, process, caption, export, dlq) and durable pull consumers; WorkQueuePolicy with DiscardOld at 1GB
- DLQ routing helpers (`ShouldDLQ`/`RouteToDLQ`) for worker-level dead-letter handling
- Wired into `cmd/server/main.go` startup sequence after DB migration

### B-001: Fix 6 high-severity npm audit vulnerabilities in frontend dev dependencies
- Updated `@typescript-eslint/eslint-plugin` and `@typescript-eslint/parser` from v6 to v8.59.2 to resolve 6 high-severity ReDoS vulnerabilities (minimatch via @typescript-eslint/typescript-estree)
- Zero vulnerabilities reported by `npm audit` post-fix; lint and tests remain passing

### S-002: Data dir bootstrap, crypto helpers, manifest read/write
- Startup sequence: Bootstrap $DATA_DIR → LoadKey (validates 0600 perms, fatal on failure) → OpenDB (WAL, FK ON, 5s busy) → Migrate (all 11 tables from database.md with CASCADE + UNIQUE constraints)
- Atomic file writer (write-to-temp-then-rename) used by ProjectManifest and SampleMetadata JSON serialization
- AES-256-GCM crypto helpers (Encrypt/Decrypt) with sentinel error types for key validation failures

### S-001: Repo skeleton — Go module, Goa v3, Vue 3, SQLite, Docker Compose
- Full-stack foundation: Go backend (Goa v3 design-first API, logrus logging), Vue 3 frontend (Vite, TypeScript, Naive UI, Pinia)
- Health endpoint at GET /health as the first Goa-generated service
- Docker Compose orchestration: `make up` (production) and `make up-dev` (air hot reload + Vite HMR)
- Dev tooling: Dockerfile.claude-sandbox with Go 1.25, gopls, ginkgo, goa CLI, ESLint, typescript-language-server

# Image Dataset Tool  PRD

Last updated: 2026-05-03
Status: Draft v3

## 1. Goal

Build a local, single-user web tool that turns Instagram-sourced media into clean,
captioned, training-ready datasets for Flux LoRA fine-tuning. One subject per LoRA;
several subjects can live inside one project workspace.

The tool reduces the manual work of: pulling media → deduping → reviewing/curating →
captioning → exporting in a trainer-specific layout.

## 2. Non-goals (v1)

- Training Flux. We export datasets only; training happens elsewhere.
- Sources beyond Instagram. (TikTok / Twitter / Pinterest are post-MVP.)
- Hosted multi-user SaaS. v1 is a local app run via `claude-sandbox` Docker.
- Automated face detection / face-aware cropping. (Post-MVP enhancement.)
- Active-learning loops, similarity clustering UIs, dataset version control beyond
  per-export snapshots.

## 3. Users & primary use cases

A single power-user (you) running the tool locally.

### UC-1: Link social media accounts
Provide one or more social media account links to pull samples from. Accounts are
linked to subjects within a project.

### UC-2: Run a sample collection job
- Real-time feedback: throughput, error count, specific error list.
- Error fields: error code, account handle, collapsible error message.
- Job data and associated errors stored in SQLite.
- Retry failed samples individually.
- Rate-limit failures auto-pause the job and retry later to avoid tripping abuse
  detection on the social media platform.
- Post-MVP: estimate total sample count for ETA per account and overall.

### UC-3: Review collected samples
- Streaming grid view of collected samples (live-updating during ingest).
- Cropping and rotation tools.
- Duplicate samples filtered out by default (toggle to view them).

### UC-4: Run a captioning job
- Select a "study" (preset combining: prompt template + provider + model + params).
- Real-time feedback showing progress and errors.
- Start with Claude as the default provider (model TBD via CAP-001 spike).

### UC-5: Review captions
- Side panel showing caption text for the selected sample.
- Next/previous navigation respecting current filter.
- Inline caption text editing.

### UC-6: Export dataset
- Select one or more projects/subjects.
- Select which caption study to export.
- Validation: identify samples missing captions for the selected study, with option
  to launch a captioning job to fill gaps.
- Export to server-side directory (`$DATA_DIR/exports/`) as primary option.
- Zip stream download as secondary option for portability.

## 4. Stack

| Layer        | Choice                                                               |
| ------------ | -------------------------------------------------------------------- |
| Backend      | Go 1.25, Goa v3 (design-first API framework)                        |
| Frontend     | Vue 3 (Composition API) + Vite + TypeScript                         |
| DB           | SQLite via `modernc.org/sqlite` (pure Go, no CGO). WAL mode.        |
| Message queue| NATS JetStream, embedded in Go binary (no sidecar container)         |
| Image ops    | Go image libraries for transforms; pHash via Go implementation       |
| IG client    | TBD — pending IG-001 spike (evaluate instaloader subprocess, Go HTTP client, or Node sidecar) |
| Captioning   | Pluggable provider: Anthropic / OpenAI / xAI / Gemini / OAI-compatible local |
| Testing      | Ginkgo/Gomega (backend), Vitest (frontend), Playwright (thin E2E)   |
| Lint/format  | `go vet` (backend), eslint (frontend)                                |
| Dev harness  | `claude-sandbox` Docker via `.claude-sandbox.yaml`                   |
| Infra        | Docker Compose, multi-stage builds                                   |

## 5. Architecture

- Local web app, single user, run inside the `claude-sandbox` container.
- **Filesystem is the source of truth.** SQLite is a fast index that can be rebuilt
  by re-scanning manifests if the DB is lost. No backups required.
- **Event-driven pipeline** via NATS JetStream (embedded in Go binary, no sidecar).
  Each image flows through independent stages as messages on typed subjects.
  Persistence is file-backed; queue state survives restarts.
- **SSE channel** pushes pipeline events to the UI for real-time progress.
- Secrets (IG cookies, vision-model API keys) are **encrypted at the application
  level** (AES-256-GCM) before storage in SQLite. The encryption key is stored in
  `$DATA_DIR/secret.key` (mode 0600; startup fails if permissions are not 0600).
  Never stored in the project directory or git.

### 5.1 NATS JetStream pipeline

Embedded NATS server runs in-process (`nats.InProcessConn`, no network port).
JetStream provides persistent, file-backed message storage.

**Subjects:**

```
media.fetch.{provider}     →  source-specific fetch workers (e.g. media.fetch.instagram)
media.process              →  dedup, hash, thumbnail generation
media.caption              →  caption workers (provider in message payload; dispatch
                              to Captioner implementation at runtime)
media.export               →  export materializer
media.dlq                  →  dead letter (max retries exceeded)
```

**Consumer model:**

Each subject has a durable pull consumer. Workers pull messages and ACK/NAK:
- ACK → message removed, completion handler enqueues next pipeline stage
- NAK with delay → retry with backoff
- Max deliveries exceeded → routed to `media.dlq`

**Backpressure** is handled via `MaxAckPending` per consumer — limits how many
messages a consumer can have in-flight. This naturally throttles each stage
independently based on its resource constraints.

**AckWait timeout**: each consumer has a configurable `AckWait` duration. If a
worker doesn't ACK/NAK within this window, NATS redelivers the message. For
long-running tasks (IG pulls with rate limiting, LLM captioning), `AckWait` must
be set high enough to prevent duplicate processing. Workers should send in-progress
ACKs (`msg.InProgress()`) to extend the deadline for long operations.

### 5.2 Worker pools & rate limiting

| Consumer               | Default concurrency | Rate limiting                    |
| ---------------------- | ------------------- | -------------------------------- |
| `media.fetch.instagram`| 1                   | `time.Ticker` pacer + backoff on 429 |
| `media.process`        | GOMAXPROCS          | CPU-bound, no external limit     |
| `media.caption`        | 4                   | Per-provider `rate.Limiter` (RPM/TPM) |
| `media.export`         | 2                   | I/O-bound, no external limit     |

IG rate limiting lives on the **worker** (HTTP client layer), not the queue.
When a rate limit is hit, the worker NAKs with a delay and the global pacer
backs off — affecting all IG fetch messages, not just the current one.

Configuration in `config.yaml`:

```yaml
nats:
  data_dir: $DATA_DIR/nats    # JetStream file storage
  max_payload_kb: 64          # message size limit (metadata only, not image bytes)

consumers:
  max_file_size_mb: 50           # skip images larger than this
  accepted_formats: [jpeg, png, webp, heic]

  media_fetch_instagram:
    concurrency: 1
    max_ack_pending: 1
    ack_wait_s: 300            # 5min — rate limiting can cause long waits
    request_interval_ms: 2000
    max_retries: 5
    backoff_base_ms: 1000
  media_process:
    concurrency: 0             # 0 = GOMAXPROCS
    max_ack_pending: 16
    ack_wait_s: 60
  media_caption:
    concurrency: 4
    max_ack_pending: 8
    ack_wait_s: 120            # LLM calls can be slow (local models especially)
  media_export:
    concurrency: 2
    max_ack_pending: 4
    ack_wait_s: 60

providers:
  anthropic:
    rpm: 50
    tpm: 40000
  local_llama:
    rpm: 0                    # 0 = unlimited
    tpm: 0
```

### 5.3 Ingest pipeline (fetch → process → store)

```
media.fetch.instagram worker
  → downloads image to $DATA_DIR/tmp/<job-id>/<filename>
  → on success: publishes to media.process

media.process worker
  → computes sha256 + pHash
  → dedup check against subject's existing samples (sha256 exact + pHash Hamming)
  → move to final path, generate thumbnail, write manifest, insert SQLite row
  → if duplicate detected: file is kept but flagged (is_duplicate=true) for review
    (duplicate may be higher quality than existing sample)
  → on success: sample ready for review
```

Temp staging provides a clean boundary for the processing bottleneck. All samples
land in the final directory regardless of duplicate status — the user decides which
to keep during review.

### 5.4 Job completion tracking

**Counting model**: `job_runs.total_items` starts as `NULL`. As each page of source
results is discovered, `total_items` is incremented (the first page sets it from
NULL to that page's count, subsequent pages add). This means the total grows over
time until pagination completes. The UI shows running counts without a denominator
while pages are still being discovered.

Each pipeline stage's final outcome (ACK at last stage, or DLQ) atomically
increments counters:

- `completed_items` — incremented on final ACK (end of pipeline for that item)
- `failed_items` — incremented on DLQ delivery

Retries (NAK + redelivery) do not touch counters. Job is complete when
`completed_items + failed_items = total_items` AND pagination is exhausted.

**Eventual consistency**: job completion is eventually consistent — there is a
window between the last item being ACK'd and the job status being updated to
`succeeded`. Other eventual consistency surfaces to be aware of:
- Sample count in subject views (may lag behind active ingest)
- Caption coverage counts (may lag behind active captioning jobs)
- Duplicate detection (a sample may briefly appear before post-processing marks it)

### 5.5 SSE observability events

| Event type       | Payload                                    | Frequency        |
| ---------------- | ------------------------------------------ | ---------------- |
| `job.state`      | `{id, trace_id, type, status, error?}`     | On state change  |
| `job.progress`   | `{id, trace_id, current, total, pct}`      | Per-message ACK  |
| `consumer.stats` | `{subject, pending, ack_pending, redelivered}` | Every 2s     |
| `provider.rate`  | `{provider, rpm_used, rpm_limit}`          | Every 5s         |
| `sample.new`     | `{id, subject_id, status, is_duplicate, thumbnail_path}` | On sample created |
| `sample.updated` | `{id, status, edits_changed, caption_changed}` | On sample edit   |

### 5.5 Structured logging

Each message handler emits a structured log entry (`slog`) with: `job_id`,
`subject`, `sample_id`, `provider`, `duration_ms`, `attempt`, `status`, `error`.

## 6. On-disk layout

```
$DATA_DIR (default ~/image-dataset-tool)
├── secret.key                      # AES-256-GCM encryption key (mode 0600)
├── db.sqlite                       # plain SQLite (secrets encrypted at app level)
├── nats/                           # JetStream file-backed storage
├── tmp/                            # staging area for in-flight fetches
│   └── <job-id>/                   # cleaned up after processing completes
├── projects/
│   └── <project-slug>/
│       ├── manifest.json           # project name, description, created_at
│       └── subjects/
│           └── <subject-slug>/
│               ├── manifest.json   # subject metadata + linked IG accounts
│               └── samples/
│                   ├── <sample-id>.<ext>        # raw bytes (never mutated)
│                   ├── <sample-id>_thumb.jpg    # JPEG thumbnail (configurable max dim, default 768px)
│                   └── <sample-id>.json         # metadata: source, edits, hashes, captions by study
└── exports/
    └── <project-slug>/<subject-slug>/<format>-<timestamp>/
```

`$DATA_DIR` is configurable via env var; default mounted into the sandbox.

**Thumbnails**: JPEG format, configurable max dimension (default 768px shortest edge).
Regenerated if the configured size changes. Stored alongside the raw sample.

See `docs/files.md` for manifest JSON schemas and per-sample `.json` structure.

## 7. Data model (SQLite)

The DB mirrors the manifests; manifests are authoritative. SQLite via
`modernc.org/sqlite`, WAL mode, 5s busy timeout, foreign keys ON.

All tables use UUID primary keys. All rows have `created_at`. Mutable rows also
have `updated_at`. See `docs/database.md` for full column-level schema.

- **`projects`** — id, slug, name, description, created_at, updated_at.
- **`subjects`** — id, project_id (FK), slug, name, created_at, updated_at.
- **`accounts`** — id, platform, handle, cookies_blob (AES-256-GCM encrypted),
  last_login_at, created_at, updated_at. Top-level entity, not subject-scoped.
- **`subject_accounts`** — subject_id (FK), account_id (FK), created_at.
  Join table. An account can be linked to multiple subjects.
- **`samples`** — id, subject_id (FK), source_platform, source_post_id, slide_index,
  file_path, sha256, phash, width, height, taken_at, fetched_at, status
  (`pending` | `kept` | `rejected`), is_duplicate (bool), duplicate_of (JSON array
  of sample UUIDs, nullable), created_at, updated_at.
    - Unique `(subject_id, source_post_id, slide_index)`.
    - Indexes on `phash`, `sha256`, `status`, `subject_id`, `is_duplicate`.
- **`edits`** — sample_id (PK, FK), rotation_deg (REAL, supports free-angle),
  crop_box_json, auto_actions_json, created_at, updated_at.
- **`caption_studies`** — id, project_id (FK), name, slug, provider, model,
  prompt_template, params_json (temperature, etc.), created_at, updated_at.
- **`captions`** — id, sample_id (FK), study_id (FK → caption_studies), text,
  created_at. Full history retained. The latest caption per study is also written
  into the sample's `.json` manifest keyed by study slug.
- **`job_runs`** — id, type, subject_id (FK), status (`running` | `succeeded` |
  `failed` | `cancelled`), total_items, completed_items, failed_items,
  started_at, finished_at, created_at. Read-only audit log of pipeline runs;
  actual queue state lives in NATS JetStream.
- **`job_messages`** — id, job_run_id (FK), level (error/warning/info), sample_ref,
  account_handle, message, created_at. Per-job log with level filtering (see §11.8).
- **`secrets`** — key (PK), value_encrypted (BLOB, AES-256-GCM). Encryption key
  from `$DATA_DIR/secret.key`.

## 8. Core surfaces

### 8.1 Instagram ingestion

- Idempotent on `(source_post_id, slide_index)`. A "force refresh" toggle bypasses
  cache and re-downloads bytes (useful if a post was edited).
- Carousels expand into one sample per slide.
- Rate limiting + backoff are handled in the job runner; failures retry with
  exponential backoff up to N attempts before terminating.
- 2FA flow surfaces a challenge UI per `subject_account`.

### 8.2 Dedup

- Subject-level only: `sha256` exact-match plus pHash Hamming distance
  (configurable threshold; default 6).
- Duplicates are **flagged, not discarded**. The file is kept on disk with
  `is_duplicate=true` in the DB. The user reviews duplicates and decides which to
  keep (a duplicate may be higher resolution than the original).
- Review grid filters duplicates out by default; toggle to show them.
- Cross-subject dedupe is out of scope for v1.

### 8.3 Review UI

- **Streaming grid** of samples for a subject (must handle tens of thousands).
  Uses connect-then-fetch pattern: SSE connected first, then full metadata fetch,
  then real-time updates via SSE `sample.new` events. No traditional pagination —
  sample metadata is fetched in full (~200 bytes/sample), thumbnails lazy-loaded
  via intersection observer with virtual scrolling.
- **Live ingest**: new samples from active jobs appear in the grid in real-time.
  Dedup by sample ID (ignore SSE events for IDs already loaded).
- **Default filter**: pending only. Status toggles (pending/kept/rejected) are
  user-selectable, applied client-side. Duplicates filtered out by default;
  toggle to show them.
- Keyboard-driven keep / reject; shift-click range select; select all / select none.
- **Status is reversible**: any sample can be moved between pending/kept/rejected at
  any time (undo is just changing status back).
- Per-sample editor: 90° rotate (L/R buttons), free-angle slider, manual rectangle
  crop, auto-remove-borders. Edits stored as ops in the sample manifest, never
  destructively applied to the raw bytes.
- Side panel for viewing/editing captions (see §8.4).
- **Lightbox**: clicking a thumbnail opens a full-size preview overlay. Arrow keys
  and keyboard shortcuts (keep/reject) work within the lightbox, mirroring grid
  behavior. Lightbox state is synced with grid selection.

### 8.3.1 Keyboard shortcuts (review grid + lightbox)

| Key | Action |
|-----|--------|
| `K` | Keep selected sample(s) |
| `R` | Reject selected sample(s) |
| `←` / `→` | Previous / next sample |
| `Space` | Toggle selection on focused sample |
| `Shift+Click` | Range select |
| `Ctrl+A` | Select all (visible) |
| `Ctrl+Shift+A` | Deselect all |
| `Escape` | Close lightbox / clear selection |
| `Enter` | Open lightbox for focused sample |

### 8.3.2 Deletion

Projects, subjects, and individual samples can be deleted. Deletion shows a
confirmation dialog with:
- Default: soft delete (marks as deleted, removes from UI).
- Checkbox: "Also delete files from disk" for hard delete.
- In-flight NATS messages for deleted entities are an edge case not covered in v1.

### 8.4 Captioning & studies

A **caption study** is a named preset: prompt template + provider + model + params
(temperature, max tokens, etc.). Studies are project-scoped.

- `Captioner` provider interface (Go): `Caption(ctx, image, prompt, opts) → (CaptionResult, error)`.
- Day-1: Claude as default provider (model selected via CAP-001 spike). Plan supports
  cloud providers (Anthropic, OpenAI, xAI, Gemini) and OAI-compatible local endpoints
  (llama.cpp, LM Studio).
- Caption style is **natural language** (not booru tags).
- Captioning runs through `media.caption` NATS subject, rate-limited per
  provider (see §5.2).
- Captions are stored in the per-sample `.json` manifest, keyed by study slug.
  Full history also in the `captions` SQLite table.
- **Manual edits** are stored under a "manual" pseudo-study, preserving the
  auto-generated text from the original study. This maintains re-run idempotency.
- **Caption review UI**: side panel showing caption text for selected sample,
  next/previous navigation respecting current filter, inline text editing.

### 8.5 Export

- `DatasetWriter` interface (Go): takes subjects + study + format options, materializes
  edited bytes (applying rotation/crop) plus paired captions into the export folder.
  Runs through `media.export` NATS subject.
- **Export dialog**: select projects/subjects → select caption study → validation
  checks for missing captions (with option to launch captioning job to fill gaps) →
  choose destination.
- **Server-side export** (primary): writes to `$DATA_DIR/exports/<slug>/<format>-<timestamp>/`.
- **Zip download** (secondary): streams a zip archive to the browser for portability.
- Exports only `status='kept'` samples. Edits applied at export time using Go `image`
  stdlib (rotation, crop).
- v1 ships kohya-ss writer. Additional formats (diffusers, OneTrainer) slot in behind
  the interface post-MVP.

### 8.6 Setup wizard (first-run)

On first launch (no `$DATA_DIR` or missing key file), the app guides through:
1. **Data directory**: confirm or change `$DATA_DIR` path.
2. **Encryption key**: generate a new key file at
   `$DATA_DIR/secret.key`, or point to an existing one.
3. **Provider setup** (optional, skippable): paste an API key for at least one
   captioning provider.
4. **Done**: redirect to empty project list with "create project" CTA.

### 8.7 Account authentication

v1 assumption (pending IG-001 spike): user pastes cookies from browser devtools
into the account linking UI. The cookies are encrypted and stored in
`subject_accounts.cookies_blob`. All stories depending on IG client behavior
(IG-002, IG-003, IG-004, P-003) are **blocked on IG-001 spike completion**.

### 8.8 Configuration boundaries

| Setting | Managed via | Notes |
|---------|-------------|-------|
| API keys, IG cookies | Settings UI | Encrypted in SQLite `secrets` table |
| Data directory | Env var / setup wizard | `$DATA_DIR`, requires restart |
| Rate limits, concurrency | `config.yaml` | Requires restart |
| Thumbnail max dimension | `config.yaml` | Triggers regeneration on change |
| Provider endpoints/models | `config.yaml` | Requires restart |
| Encryption key path | `$DATA_DIR/secret.key` | Outside project dir |

### 8.9 Recovery on restart

- NATS JetStream replays unACK'd messages automatically — pipeline resumes.
- `job_runs` rows with `status='running'` are marked `interrupted` on startup.
  The UI surfaces these for the user to decide: resume (re-publish remaining items)
  or cancel.

### 8.10 Queue admin view

- Stream/consumer stats: pending, ack'd, redelivered counts per subject.
- Message peek: view message payload without consuming.
- Filter by subject/type.
- Interactive: manually retry/redeliver failed messages, purge a subject, delete
  individual dead-letter messages.

## 9. Frontend routes

| Route | View | Use case |
|-------|------|----------|
| `/` | Project list | — |
| `/projects/:id` | Subject list + account links | UC-1 |
| `/projects/:id/subjects/:sid/samples` | Review grid (paginated) | UC-3 |
| `/projects/:id/subjects/:sid/samples/:sampleId` | Sample detail + caption panel | UC-5 |
| `/jobs` | Job list + real-time progress | UC-2, UC-4 |
| `/studies` | Caption study management | UC-4 |
| `/export` | Export dialog | UC-6 |
| `/queues` | Queue admin — stats, peek, retry | §8.6 |
| `/accounts` | Social media account management | UC-1 |
| `/settings` | Config, secrets, providers | — |

## 10. Story map

### Epic 0  Foundations

- **F-001** Repo skeleton: Go module, Goa v3 codegen, Vue 3 + Vite frontend, SQLite (modernc.org), `.claude-sandbox.yaml`, docker-compose.
- **F-002** Data-dir bootstrap, env loading, AES-GCM crypto helpers (key from `$DATA_DIR/secret.key`), manifest read/write.
- **F-003** Embedded NATS JetStream: in-process server, stream/consumer setup, file-backed persistence, pipeline subject hierarchy.
- **F-004** Pipeline worker framework: consumer base, ACK/NAK retry, DLQ routing, per-provider rate limiting, SSE event bridge.
- **F-005** Filesystem-as-truth reconciler (sync SQLite from manifests on every startup; handles adds, removes, and changes).
- **F-006** API server skeleton: Goa v3 design, structured JSON logger (`slog`), error middleware, SSE endpoint, media serving endpoint, Swagger UI at /docs.
- **F-007** Frontend shell: Vue 3 + Vue Router, SSE composable, theming.
- **F-008** Setup wizard: first-run flow (data dir, key generation, optional provider setup).

### Epic 1  Project & subject management

- **P-001** CRUD projects (UI + API + manifests).
- **P-002** CRUD subjects within a project.
- **P-003** Account management: top-level CRUD for social media accounts (list, add, remove). Encrypted cookie storage. Separate view at /accounts.
- **P-004** Link account to subject: picker UI on subject detail to associate existing accounts. Many-to-many relationship.

### Epic 2  Instagram ingestion

- **IG-001** Research spike: evaluate IG client approaches (instaloader subprocess,
  Go HTTP client against private API, Node sidecar, GraphQL scraping). Includes
  2FA support matrix. Agent uses web search subagents to explore thoroughly.
  Output: recommendation doc in `docs/`. Dependent stories use `requires_reviewed`.
- **IG-002** Implement chosen approach behind an `IGClient` interface in `internal/service/`.
  *Blocked on IG-001 spike being reviewed by user.*
- **IG-003** Media-pull job: handles posts and carousels, idempotent on `source_post_id`,
  rate-limit aware, force-refresh option. *Blocked on IG-001 spike reviewed.*
- **IG-004** Sample writer: bytes + sample manifest + SQLite row, `sha256` + pHash
  duplicate flagging, thumbnail generation. *Blocked on IG-001 spike reviewed.*

### Epic 3  Review & editing

- **R-001** Streaming grid view per subject: connect-then-fetch, SSE live updates,
  virtual scroll, lazy thumbnails (filters: pending / kept / rejected; duplicate toggle).
- **R-002** Keyboard-driven keep/reject + multi-select + bulk actions.
- **R-003** Rotate: 90° L/R buttons + free-angle slider.
- **R-004** Manual rectangle crop.
- **R-005** Research spike: auto-correction helpers (border removal, photo-of-art
  rotation/crop/skew correction). Evaluate approaches and tradeoffs. Agent uses web
  search subagents. Output: recommendation doc in `docs/`. Dependent stories use
  `requires_reviewed`.
- **R-006** Edit-ops persisted to sample manifest + SQLite; raw bytes untouched.
- **R-007** Caption side panel: view/edit captions, next/prev navigation.

### Epic 4  Captioning

- **CAP-001** Research spike: A/B vision models on a 30-image gold set; pick starting
  model and prompt. Agent uses web search subagents to research provider capabilities.
  Output: recommendation doc in `docs/`. Dependent stories use `requires_reviewed`.
- **CAP-002** `Captioner` provider interface + Claude provider (spike-winning model).
  *Blocked on CAP-001 spike reviewed.*
- **CAP-003** OAI-compatible local provider (llama.cpp / LM Studio endpoint).
  *Blocked on CAP-001 spike reviewed.*
- **CAP-004** Caption studies: CRUD UI, study presets (prompt + provider + model + params).
  *Blocked on CAP-001 spike reviewed.*
- **CAP-005** Batch captioning job: publish to `media.caption`, SSE progress.
  *Blocked on CAP-001 spike reviewed.*

### Epic 5  Export

- **E-001** `DatasetWriter` interface + edit-ops materializer (Go image libs).
- **E-002** kohya-ss writer (paired image + caption files; respects subset folders).
- **E-003** Export dialog UI: project/subject multi-select, study selector,
  missing-caption validation, destination picker.
- **E-004** Server-side export via `media.export` pipeline + SSE progress with
  total/completed/failed counts and ETA.
- **E-005** Zip stream download via Goa native streaming endpoint.

### Epic 6  Observability & polish

- **O-001** Job history view: timing, outcome, per-item errors (code, handle, message).
- **O-002** Queue admin view: stream/consumer stats, message peek, retry/purge actions.
- **O-003** Structured JSON logs to stdout.
- **O-004** Settings UI: data dir, encryption key check, secret management, provider
  config, thumbnail size.

## 11. Cross-cutting requirements

- **Performance**: tens of thousands of samples per project. The grid must be
  paginated; pHash + dedupe queries indexed; JPEG thumbnails (768px default)
  generated during post-processing and cached on disk.
- **Idempotency**: every pipeline stage must be safe to re-run. Pulls dedupe on
  `source_post_id`; captions skip samples whose `(study_id, sample_id)` already exist.
  Handlers must tolerate NATS redelivery (check file existence, use INSERT OR IGNORE).
- **Recoverability**: losing `db.sqlite` must not lose user work. F-005 reconciler
  rebuilds the DB from manifests.
- **Security**: secrets encrypted with AES-256-GCM in SQLite; encryption key in
  `$DATA_DIR/secret.key` (mode 0600); no secrets in logs;
  cookies redacted in message payloads.
- **No video**: images only for v1. IG video media types are skipped during ingest.

### 11.1 Data consistency rules

These rules apply to all stories that write to SQLite + filesystem:

1. **Write order**: manifest first (atomic write-to-temp-then-rename), then SQLite.
   If DB write fails, manifest is still valid and reconciler recovers on next startup.
2. **Atomic file writes**: all manifest and sample .json writes use write-to-temp-file
   then `os.Rename()` to prevent corrupt JSON on crash.
3. **Pipeline handler contract**: DB counter increment (job_runs) must succeed before
   ACK. If DB write fails, NAK the message to force retry.
4. **Duplicate detection serialization**: pHash/sha256 duplicate checks and sample
   insert must be serialized per-subject (subject-scoped mutex) to prevent race
   conditions between concurrent media.process workers.
5. **Startup ordering**: reconciler completes before NATS consumers start. Consumers
   are gated on reconciler completion.
6. **Concurrent manifest access**: caption writes (manual and pipeline) must use
   file-level locking (flock) or read-modify-write with CAS on `updated_at` to
   prevent lost updates when UI edits and pipeline writes target the same sample.
7. **Deletion guards**: pipeline handlers check entity existence (subject, sample)
   before writing; if deleted, ACK the message (discard) and log a warning.
8. **Cascade deletes**: project deletion cascades to subjects, samples, edits,
   captions, caption_studies, subject_accounts, job_runs, job_messages. Use
   ON DELETE CASCADE in schema.
9. **Duplicate reference cleanup**: on sample deletion, scrub the deleted UUID from
   all other samples' `duplicate_of` arrays (DB + manifests).

### 11.2 Pipeline tracing

Every "first-mover" event (e.g., a pull trigger, captioning trigger) generates a
`trace_id` (UUID). This trace_id is propagated to all child events in the pipeline.
All structured log entries and SSE events include both `job_id` and `trace_id`,
enabling end-to-end correlation of a single image from fetch through processing.

### 11.3 Input validation

- **Slug names**: max 64 characters, lowercase, hyphens, no spaces or special chars.
  Validated at API layer; rejected with 400 and clear error message.
- **Image formats**: accepted formats are JPEG, PNG, WebP, and HEIC. Non-image media
  and unsupported formats are skipped during ingest with a logged warning.
- **Max file size**: 50MB per image (configurable in config.yaml). Files exceeding
  the limit are skipped with a logged warning and job_error entry.
- **Config validation**: on startup, validate all config.yaml values (non-negative
  concurrency, known provider names, valid YAML structure, valid paths). Fail fast
  with specific error messages for each invalid value.

### 11.4 Portability

`$DATA_DIR` can be copied to another machine and the app will work. The `nats/`
subdirectory contains ephemeral pipeline state and can be safely deleted — it will
be recreated on startup. Document this in docs/files.md.

### 11.5 Frontend UX conventions

- **Dialog state persistence**: dialog boxes remember their last values in
  localStorage and restore them on open (unless opened to a specific pre-set state,
  e.g., "regenerate missing captions" pre-fills the study selector).
- **SSE connect-then-fetch**: all views that display live-updating data use the
  connect-then-fetch pattern: connect SSE first, fetch current state, merge
  buffered events by ID (idempotent).

### 11.6 Resource limits

- **Disk full**: file write errors (ENOSPC) NAK with a specific error code. After N
  consecutive disk errors, auto-pause the job and emit `job.state` SSE event with
  error detail.
- **NATS stream size**: configure `MaxBytes` (default 1GB) with DiscardOld policy.
  If all messages are pending and limit is reached, new publishes fail and error
  surfaces to user.
- **SQLite WAL**: use default `PRAGMA wal_autocheckpoint` (1000 pages). Document
  in database.md.

### 11.7 UI layout

Hybrid sidebar + workspace layout (see `docs/ui.md` for reference):
- **Narrow icon sidebar** (collapsed by default, expandable): global nav to Projects,
  Jobs, Studies, Queues, Accounts, Settings.
- **Breadcrumb bar**: current location (Project > Subject > Samples) with quick nav.
- **Workspace area**: resizable split panes. Review view shows grid + caption/detail
  panel side-by-side.
- **Status bar** (always visible): active job count, current job progress, rate limit
  indicator. Click to expand job detail drawer.
- **Right panel**: collapsible, context-dependent (captions in review, errors in jobs).
- **Error banner**: dismissable banner at top for API errors and system-level issues.
- **Dialog state**: dialogs remember last values in localStorage, restored on open
  (unless opened to a specific pre-set state).

### 11.8 Job log messages

Job logs are per-job (not cross-job). Each message has a timestamp, level, and text.
The UI shows a level filter defaulting to **warning**.

| Level | Examples |
|-------|---------|
| **error** | API auth failed, image download failed, captioner error, disk write failed, unsupported format (sample cannot be saved), file exceeds size limit, rate limit retries exhausted (max_retries from config) |
| **warning** | Rate limit hit (auto-pausing, per occurrence), pHash duplicate detected |
| **info** | Job started, pagination page N discovered (total now X), captioning batch started for study Y, export completed (N files, X MB), rate limit backoff cleared (resuming) |

Job messages trigger **toast notifications** in the UI. The toast level threshold
defaults to **warning** (shows warning + error toasts, suppresses info). The
threshold is configurable in the settings UI and persisted in localStorage.

Schema: `job_messages(id, job_run_id FK, level, sample_ref, account_handle,
message, created_at)`. See `docs/database.md`.

### 11.9 Health check

GET /health returns subsystem status:
- SQLite: connectivity check (simple query)
- NATS: embedded server health
- $DATA_DIR: writable check
- Encryption key: valid (sentinel decrypt succeeds)

Returns 200 if all healthy, 503 with failing subsystem details if not. Used by
the setup wizard to validate configuration.

### 11.10 Graceful shutdown sequence

On SIGTERM/SIGINT, using `context.Context` with cancellation:

1. Cancel the root context → signals all goroutines to stop.
2. NATS consumers stop pulling new messages.
3. In-flight message handlers finish (bounded by AckWait) and ACK/NAK.
4. SSE connections are closed (clients will reconnect on restart).
5. NATS embedded server shuts down (flushes JetStream to disk).
6. SQLite WAL checkpoint and close.
7. Process exits.

Shutdown timeout: 30s. If handlers don't finish, force exit.

### 11.11 Rate limit UI

When an IG rate limit is hit:
- Status bar shows amber rate-limit indicator with backoff timer.
- Job card shows "rate limited — retrying in Xs" state.
- Job log: warning-level message per rate limit event.
- If max_retries exhausted: error-level message, job marked failed.
- When backoff clears and fetching resumes: info-level message.

### 11.12 Error recovery

- **SSE reconnection**: SSE endpoint accepts `Last-Event-ID` header. On reconnect,
  replay missed events from an in-memory ring buffer. Client converges to current
  state without polling.
- **Encryption key validation**: on startup, decrypt a sentinel value from secrets
  table. If auth tag mismatch, abort with clear error naming the key file before
  any pipeline starts.
- **Missing thumbnails**: media serving endpoint checks thumbnail existence; if
  missing, regenerates synchronously and logs a warning.
- **Stale running jobs**: on startup, cross-check NATS stream for pending messages
  per job_run. If status='running' but no pending messages, mark 'interrupted'.

## 12. Open questions

- **2FA support matrix**: which challenge types must the IG client cover (TOTP, SMS,
  email)? Deferred to IG-001 spike — depends on chosen client library/approach.

## 13. Risks & mitigations

- **Instagram TOS / rate limiting / login bans** — Mitigation: per-account session
  reuse, configurable request pacing, exponential backoff, surfacing rate-limit
  errors in the job-history view rather than silently retrying forever. Rate-limit
  hits auto-pause and backoff globally (not per-job).
- **Vision-model caption quality variance across subjects** — Mitigation: CAP-001
  spike, caption studies allow exploring different prompts/models per project.
- **Encryption-key loss** — Mitigation: documented rotation procedure; key in
  `$DATA_DIR/secret.key`; surface a startup error if decrypt
  fails. DB itself is rebuildable from manifests; only secrets are lost.
- **IG client library churn** — Mitigation: IG-001 spike evaluates multiple
  approaches (instaloader subprocess, Go HTTP client, Node sidecar); `IGClient`
  interface in the service layer keeps the swap small regardless of implementation.
- **NATS JetStream data loss** — Mitigation: NATS data is ephemeral pipeline state,
  not source of truth. Worst case = re-run a job. File-backed persistence minimizes
  this risk in practice.

## 14. Definition of done (per story)

- Code + tests (unit boundaries plus an integration test for any cross-layer flow).
- `make lint` + `make test` green (backend); `npm run lint` + `npx vitest run` green (frontend).
- Manual smoke pass for UI-touching stories.
- Public surfaces (API routes, manifest schemas) documented in `docs/`.

## 15. Roadmap (post-MVP)

See `agent/ideas/` for deferred features:
- Passphrase-derived encryption key (Argon2id, prompted at startup).
- Face-aware auto-cropping helper.
- Additional sources: TikTok, Twitter, Pinterest.
- Similarity clustering / near-duplicate review tooling.
- Additional dataset writers (diffusers, OneTrainer, custom).
- Per-account ETA estimation (total sample count from IG API).
- Video dataset support.

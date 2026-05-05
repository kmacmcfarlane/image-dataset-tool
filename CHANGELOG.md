# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

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

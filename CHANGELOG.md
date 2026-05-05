# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### S-003: Embedded NATS JetStream: in-process server, streams, persistence
- Embedded NATS server starts in-process (no TCP port) with JetStream file-backed storage at $DATA_DIR/nats/
- MEDIA stream with subjects: media.fetch.>, media.process, media.caption, media.export, media.dlq
- Durable pull consumers for each subject with configurable MaxAckPending, AckWait, and MaxDeliver
- DLQ routing helper (ShouldDLQ/RouteToDLQ) for workers to route failed messages after max delivery attempts
- MaxBytes stream limit with DiscardOld policy (default 1GB)
- Integration tests: publish/consume/ACK, NAK with delay/redelivery, DLQ routing, persistence across restart, MaxBytes eviction
- Server wired into cmd/server/main.go startup sequence

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

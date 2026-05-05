# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### S-002: Data dir bootstrap, crypto helpers, manifest read/write
- Data directory bootstrap: creates $DATA_DIR/{projects,exports,tmp,nats} on startup; defaults to ~/image-dataset-tool when $DATA_DIR unset
- AES-256-GCM crypto helpers: LoadKey (validates 0600 perms), Encrypt, Decrypt with proper error types
- Atomic file writer: write-to-temp-then-rename pattern to prevent corrupt JSON on crash
- File format types: ProjectManifest and SampleMetadata with full JSON serialization matching docs/files.md
- SQLite store: OpenDB with WAL mode, FK ON, 5s busy timeout; migration framework with initial schema
- All tables from docs/database.md created via migration with ON DELETE CASCADE, unique constraints, and indexes

### S-001: Repo skeleton — Go module, Goa v3, Vue 3, SQLite, Docker Compose
- Full-stack foundation: Go backend (Goa v3 design-first API, logrus logging), Vue 3 frontend (Vite, TypeScript, Naive UI, Pinia)
- Health endpoint at GET /health as the first Goa-generated service
- Docker Compose orchestration: `make up` (production) and `make up-dev` (air hot reload + Vite HMR)
- Dev tooling: Dockerfile.claude-sandbox with Go 1.25, gopls, ginkgo, goa CLI, ESLint, typescript-language-server

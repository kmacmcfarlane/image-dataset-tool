# Dev Ops

Build pipeline, CI, Docker, linting, and infrastructure improvements. Only items requiring user approval belong here — routine improvements should be implemented directly by agents.

## Required fields for new entries

Every idea appended by agents must include:
- `status: needs_approval` — default for all new ideas. The user changes this to `approved`, `rejected`, etc.
- `priority: <low|medium|high|very-low>` — the agent's suggested priority based on impact and effort.
- `source: <developer|reviewer|qa|orchestrator>` — which agent originated the idea.

Example:
```
### <Title>
* status: needs_approval
* priority: medium
* source: developer
<Description — 1-3 sentences>
```

## Ideas

### Add `make test-e2e` target and Playwright E2E infrastructure
* status: needs_approval
* priority: high
* source: qa
E2E infrastructure (`frontend/e2e/`, Playwright config, `make test-e2e` target) does not exist yet. Required for all future story QA cycles per the QA agent's standard operating procedure. Should be scaffolded as a dedicated story.

### Secret key generation helper (`make gen-key`)
* status: needs_approval
* priority: medium
* source: developer
No tooling exists to create a valid `secret.key` for local dev. A `make gen-key` target (or small Go CLI under `scripts/`) that writes 32 random bytes with `chmod 0600` to `$DATA_DIR/secret.key` would remove a common first-run stumbling block.

### Dev container smoke test in CI
* status: needs_approval
* priority: medium
* source: developer
Add a lightweight `make up-dev` health-check target that starts the dev stack, polls the backend health endpoint, and tears down. Would catch startup crashes like B-002 before they reach QA.

### Automated npm audit gate in CI
* status: needs_approval
* priority: medium
* source: developer
Add `npm audit --audit-level=high` as a required check in the CI pipeline so high/critical vulnerabilities are caught before they accumulate across multiple release cycles.

### Dev data provisioning for smoke tests
* status: needs_approval
* priority: medium
* source: qa
The `make logs-snapshot` smoke test always fails at crypto key load because no `secret.key` is provisioned in the fresh dev volume. A `make setup-dev-data` target or auto-generate-key-on-first-boot behavior would allow the backend to fully start during smoke tests.

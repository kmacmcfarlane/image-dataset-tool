# Testing Infrastructure

Major test infrastructure changes requiring design or user buy-in — not routine test additions. Only items requiring user approval belong here — routine improvements should be implemented directly by agents.

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

### Add E2E test infrastructure (Playwright + `make test-e2e`)
* status: needs_approval
* priority: high
* source: qa
The project has no Playwright E2E tests, no `frontend/e2e/` directory, and no `test-e2e` Makefile target. The QA gate requires `make test-e2e` for story acceptance, so this infrastructure should be scaffolded to enable proper E2E verification for future stories.

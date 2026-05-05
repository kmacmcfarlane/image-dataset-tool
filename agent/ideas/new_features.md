# New Features

Net-new user-facing capabilities not currently in the backlog. Only items requiring user approval belong here — routine improvements should be implemented directly by agents.

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

### Caption study reconciliation from filesystem
* status: needs_approval
* priority: low
* source: developer
Currently caption studies only exist in DB. If a study is referenced in a sample manifest but doesn't exist in DB, the caption is skipped. A future story could define a `caption_studies` manifest format to make studies reconstructable from disk.

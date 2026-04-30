---
name: backlog-entry
description: Interactively create new backlog entries (stories, bugs, refactoring) in backlog.yaml. Use when user says "new story", "add backlog entry", "new bug", "new ticket", "backlog entry", "create story", "file a bug", "add to backlog", or "batch tickets". Supports single and batch ticket creation with auto-linking requires dependencies.
disable-model-invocation: true
allowed-tools: "Read, Bash, AskUserQuestion, Grep"
argument-hint: <description of the feature, bug, or task>
---

# Backlog Entry Creator

Create well-formed backlog entries interactively. User's initial description: $ARGUMENTS

## Instructions

### Step 1: Get Current State

Run these commands to understand the backlog state:
```bash
python3 scripts/backlog/backlog.py next-id S
python3 scripts/backlog/backlog.py next-id B
python3 scripts/backlog/backlog.py next-id R
```

If `$ARGUMENTS` is empty, ask the user to describe what they want to add before proceeding.

### Step 2: Analyze and List Assumptions

Parse `$ARGUMENTS` and determine:

- **Entry type**: Feature/story (S-), bug (B-), or refactoring (R-)?
- **Scope**: Which layers? Frontend (FE), Backend (BE), End-to-end (E2E), Ops (OPS), Documentation (DOC)?
- **Complexity**: low, medium, or high?
- **Draft acceptance criteria**: Specific, testable items with appropriate prefix (FE:, BE:, E2E:, OPS:, DOC:)
- **Dependencies**: Does this depend on existing stories? Check with `backlog.py list-ids --source active`
- **Batch detection**: If the description contains multiple distinct items, note this and plan to create multiple entries.

Present your analysis to the user via `AskUserQuestion`:
- **question**: List numbered assumptions including entry type, layers, draft acceptance criteria, estimated complexity, and suggested priority range. Ask: "Are these assumptions correct?"
- **options**:
  1. Label: "Looks good", Description: "Assumptions are correct, proceed"
  2. Label: "Needs corrections", Description: "I'll provide corrections in the text field"

### Step 3: Iterative Follow-up

Based on the user's response, ask follow-up questions via `AskUserQuestion` until ALL of these are resolved:

1. **Entry type** confirmed (S-, B-, or R-)
2. **Title** is short and clear (under 80 characters)
3. **Acceptance criteria** are specific, testable, with correct prefix
4. **Complexity** is set (low, medium, high)
5. **Priority** is set. If not specified, ask with these ranges:
   - 70-90: Urgent, blocks other work, user-facing breakage
   - 40-60: Important enhancement or significant bug
   - 20-39: Nice-to-have improvement
   - 10-19: Low priority, future consideration
6. **Dependencies** identified (list of IDs, or empty)
7. **Notes** have enough context (key files, design decisions, root cause for bugs)
8. **Testing commands** determined from acceptance criteria prefixes

Focus each round on the biggest remaining gap. Most entries need 2-3 rounds.

### Step 4: Offer to Split Large Entries

If the entry has more than 6 acceptance criteria OR spans 3+ layers, suggest splitting:

Use `AskUserQuestion`:
- **question**: "This entry has {N} acceptance criteria across {layers}. Split into smaller entries?"
- **options**:
  1. Label: "Split", Description: "Create multiple smaller, focused entries"
  2. Label: "Keep as one", Description: "Create a single entry with all criteria"

If splitting, repeat Steps 2-3 for each sub-entry. Assign sequential IDs and set `requires` links between dependent entries.

### Step 5: Batch Mode

If the user provided multiple items (detected in Step 2 or explicitly requested):

1. Parse each distinct item from the description
2. For each item, generate: id, title, priority, complexity, acceptance criteria, testing commands
3. Infer logical dependencies between items (e.g., "API endpoint" must come before "UI that calls it") and set `requires` links
4. Present ALL items together for review before writing

### Step 6: Generate and Confirm

Build the YAML using this format:
```yaml
  - id: {PREFIX}-{NNN}
    title: "{title}"
    priority: {number}
    status: todo
    complexity: {low|medium|high}
    requires: [{comma-separated IDs or empty}]
    acceptance:
      - "{PREFIX}: {criterion 1}"
      - "{PREFIX}: {criterion 2}"
    notes: |
      {Context paragraph.}
      Key files: {comma-separated file paths}.
    testing:
      - "command: {test command 1}"
```

Auto-determine testing commands:
- Any `BE:` → `"command: make test-backend"`
- Any `FE:` → `"command: cd frontend && npx vitest run"`
- Any `E2E:` → `"command: make test-e2e"`
- Only `DOC:` → `"command: (n/a) documentation only"`

Show the complete YAML to the user via `AskUserQuestion`:
- **question**: "Here is the generated entry. Approve or request edits?"
- **options**:
  1. Label: "Approve", Description: "Append to backlog.yaml"
  2. Label: "Needs edits", Description: "I'll provide edits in the text field"

Loop on edits until approved.

### Step 7: Write to Backlog

Pipe the approved YAML to backlog.py:
```bash
cat <<'EOF' | python3 scripts/backlog/backlog.py add
<approved YAML>
EOF
```

Then validate:
```bash
python3 scripts/backlog/backlog.py validate
```

### Step 8: Summary

Report:
- Entry ID(s) and title(s)
- Priority and complexity assigned
- Number of acceptance criteria
- Dependencies (if any)
- Validation result

## Important

- New entries ALWAYS get `status: todo`
- IDs are globally unique across both backlog.yaml and backlog_done.yaml — `backlog.py next-id` handles this
- Acceptance criteria must be specific and testable — no vague criteria like "works correctly"
- The `complexity` field is required for new entries (used for model selection in AGENT_FLOW.md)
- All writes go through `backlog.py add` — never edit YAML directly
- For batch creation, assign sequential IDs and auto-link `requires` where there are logical dependencies

## Examples

### Single feature story

User: `/backlog-entry Add dark mode toggle to the top nav`

Result:
```yaml
  - id: S-083
    title: "Dark mode toggle in top navigation bar"
    priority: 25
    status: todo
    complexity: low
    requires: []
    acceptance:
      - "FE: Dark mode toggle button in the top navigation bar"
      - "FE: Clicking toggle switches between light and dark themes"
      - "FE: Theme preference persists in localStorage"
      - "FE: Unit tests for toggle behavior"
    notes: |
      Naive UI supports dark theme via NConfigProvider.
      Key files: frontend/src/App.vue, frontend/src/composables/useTheme.ts (new).
    testing:
      - "command: cd frontend && npx vitest run"
```

### Batch creation from notes

User: `/backlog-entry Add API endpoint for user preferences, then a frontend settings panel that uses it`

Result (two linked entries):
```yaml
  - id: S-083
    title: "User preferences API endpoint"
    priority: 30
    status: todo
    complexity: medium
    requires: []
    acceptance:
      - "BE: GET /api/preferences returns stored user preferences"
      - "BE: PUT /api/preferences updates preferences with validation"
      - "BE: Unit tests for preference service and store"
    notes: |
      New REST endpoint for persisting user preferences.
      Key files: backend/internal/api/design/, backend/internal/service/, backend/internal/store/.
    testing:
      - "command: make test-backend"

  - id: S-084
    title: "Frontend settings panel using preferences API"
    priority: 28
    status: todo
    complexity: medium
    requires: [S-083]
    acceptance:
      - "FE: Settings panel accessible from the top navigation"
      - "FE: Panel reads and writes preferences via the API"
      - "FE: Unit tests for settings panel component"
    notes: |
      Frontend counterpart to S-083.
      Key files: frontend/src/components/SettingsPanel.vue (new), frontend/src/api/.
    testing:
      - "command: cd frontend && npx vitest run"
```

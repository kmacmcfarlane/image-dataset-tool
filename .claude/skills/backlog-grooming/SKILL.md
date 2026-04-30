---
name: backlog-grooming
description: Conversational backlog grooming session — UAT review, bug reporting, feature requests, and priority management. Use when user says "groom", "backlog grooming", "uat review", "review uat", "approve tickets", "review backlog", "backlog review", "prioritize backlog", or "uat feedback".
disable-model-invocation: true
allowed-tools: "Read, Bash, AskUserQuestion, Edit"
---

# Backlog Grooming

A two-phase conversational session for grooming the backlog.

**Phase 1 — Discovery**: Gather feedback through conversation. Triage UAT stories, collect bug reports, feature requests, and priority changes. Ask clarifying questions until all ambiguity is resolved.

**Phase 2 — Plan & Execute**: Present a structured change plan for user approval, then execute all mutations in a single batch.

---

## Phase 1: Discovery

### Step 1.1: Load backlog state

Run these in parallel to understand current state:
```bash
python3 scripts/backlog/backlog.py query --status uat --format json --fields id,title,priority,notes,acceptance
python3 scripts/backlog/backlog.py query --status todo --format json --fields id,title,priority
python3 scripts/backlog/backlog.py query --status in_progress,review,testing,uat_feedback --format json --fields id,title,priority,status,review_feedback
```

Summarize the backlog state for the user: how many stories in each status, what's in-flight, what's waiting for review.

### Step 1.2: Triage UAT stories

If there are stories in `uat` status, present them for triage using `AskUserQuestion`.

Group stories by category (bugs, features/enhancements, infrastructure/testing) and present in batches of up to 4.

**Before each batch**, output a markdown reference block so the user can scroll up for context:

```
### {Category} — Batch {n}

**[{id}] {title}** (priority {priority})
{notes field, condensed to 2-3 sentences}
AC: {bullet list of acceptance criteria, abbreviated to key phrases}

**[{id}] {title}** (priority {priority})
...
```

Then present the `AskUserQuestion` batch:

- **question**: `[{id}] {title} — {1-sentence summary from notes}`
- **header**: Story ID (e.g. "B-023")
- **options**:
  1. Label: "Approve", Description: "Move this story to done"
  2. Label: "Discuss", Description: "I have feedback or questions about this story"
  3. Label: "Skip", Description: "Leave in UAT for now"

After each batch, for stories marked "Discuss":
- Ask follow-up `AskUserQuestion` calls to capture the specific feedback, the user's experience, and their priority assessment for the rework (very high / high / medium / low).

Continue until all UAT stories have been triaged.

If there are no UAT stories, say so and move to Step 1.3.

### Step 1.3: Open discovery conversation

After UAT triage (or if there were no UAT stories), explicitly ask:

> "Any new bugs, features, priority changes, or other backlog items to discuss?"

This begins a **conversational loop**. The user may describe:
- Bugs encountered during testing
- New feature requests
- Priority changes for existing stories
- Dependency relationships between stories
- Requests to split or restructure existing stories

For each item the user raises:

1. **State your internal assumptions** about scope, layers, priority, complexity, and dependencies. Making assumptions visible drives more productive clarification — the user can correct misconceptions before you ask the wrong questions.
2. **Ask clarifying questions** via `AskUserQuestion` — one focused question per round, targeting the biggest gap that your assumptions didn't resolve.
3. **Flag unclear complexity** — if you're uncertain whether something is low/medium/high, say so explicitly. Resolving complexity during grooming reduces uncertainty for the development agent later.

Use this priority vocabulary consistently:
- **very high** (80-95): Blocks other work or user-facing breakage
- **high** (60-79): Important bug fix or key enhancement
- **medium** (40-59): Significant improvement
- **low** (20-39): Nice-to-have improvement

### Step 1.4: Story breakdown (sub-phase)

After all items have been discussed, review the full set of new stories being created. For each one, consider whether it should be broken down:

- More than 6 acceptance criteria
- Spans 3+ layers (BE, FE, E2E, DB, OPS, DOC)
- Contains logically independent features that could ship separately
- User described distinct sub-features or phases during discussion

If any stories warrant splitting:
1. Propose a breakdown with brief descriptions of each sub-story
2. Ask the user to confirm via `AskUserQuestion` (approve split / keep as one / different split)
3. Assign sequential IDs and set `requires` links between dependent sub-stories

This is its own sub-phase — don't rush it. Good factoring here saves rework later.

**Do NOT move to Phase 2 until the user signals they're done** (e.g., "that's it", "nothing else", "let's see the plan"). The user has context that needs to be interviewed out of them — keep the conversation going.

---

## Phase 2: Plan & Execute

### Step 2.1: Build the change plan

Compile ALL planned changes from Phase 1 into a structured plan. Run these to understand current work order:
```bash
python3 scripts/backlog/backlog.py next-work --format json --fields id,title,priority,queue
python3 scripts/backlog/backlog.py next-id S
python3 scripts/backlog/backlog.py next-id B
python3 scripts/backlog/backlog.py next-id R
```

Present the plan in three sections:

**Section A — UAT story dispositions** (the stories just reviewed, sorted by priority)

| ID | Action | Priority | Summary |
|----|--------|----------|---------|

Actions: `approve → archive`, `set review_feedback + status uat_feedback`, `skip (no change)`

**Section B — New stories related to work under review** (bugs found against UAT stories, follow-up tasks, rework items)

| ID | Type | Title | Priority (level) | Requires | Complexity |
|----|------|-------|------------------|----------|------------|

**Section C — New standalone stories** (features, bugs, or tasks unrelated to current UAT work)

| ID | Type | Title | Priority (level) | Requires | Complexity |
|----|------|-------|------------------|----------|------------|

After the tables, show:
- **Next 5 in work order**: The stories that will be worked next after these changes take effect (considering queue priority from AGENT_FLOW.md: testing → review → in_progress → uat_feedback (status) → todo)
- **Dependency notes**: Any chains or blocking relationships worth calling out

### Step 2.2: User confirmation

Present the plan and ask for confirmation using `AskUserQuestion`:

- **question**: "Does this change plan look correct? Review priorities, dependencies, and work order above."
- **header**: "Confirm plan"
- **options**:
  1. Label: "Approve", Description: "Execute all changes to the backlog"
  2. Label: "Needs changes", Description: "I'll describe what to adjust"

If the user requests changes, adjust the plan and re-present. Loop until approved.

### Step 2.3: Execute mutations

Once approved, execute all changes in this order (ordering prevents operating on moved stories):

**1. Set review_feedback and status uat_feedback** (before any archiving):
```bash
echo "<feedback text>" | python3 scripts/backlog/backlog.py set-text <id> review_feedback
python3 scripts/backlog/backlog.py set <id> status uat_feedback
```

**2. Update priorities** on existing stories:
```bash
python3 scripts/backlog/backlog.py set <id> priority <value>
```

**3. Create new stories** (call `next-id` before each type to get sequential IDs):

New stories follow the format enforced by `backlog.py` and documented in the `/backlog-entry` skill. Key points:
- `status: todo` always
- `complexity` is required (low / medium / high)
- `testing` field: describe non-obvious testing considerations and edge cases for the QA agent — do NOT put boilerplate test commands here (the QA agent already knows to run unit and E2E tests)
- `acceptance` criteria use layer prefixes: `FE:`, `BE:`, `E2E:`, `OPS:`, `DOC:`

```bash
cat <<'EOF' | python3 scripts/backlog/backlog.py add
<story YAML>
EOF
```

**4. Approve stories** (set done + archive):
```bash
python3 scripts/backlog/backlog.py set <id> status done
python3 scripts/backlog/backlog.py archive <id>
```

**5. Validate**:
```bash
python3 scripts/backlog/backlog.py validate
```

### Step 2.4: Commit

Stage and commit all backlog changes in a single commit:
```bash
git add agent/backlog.yaml agent/backlog_done.yaml
git commit -m "chore: backlog grooming — <brief summary>"
```

The commit message should summarize counts: stories approved, feedback added, new tickets created, priority changes made.

### Step 2.5: Final summary

Show a concise summary:
- Stories approved and archived (count + IDs)
- Stories moved to uat_feedback (count + IDs)
- New tickets created (count + IDs with titles)
- Priority changes made (count)
- Validation result

---

## Important

- **All backlog mutations go through `backlog.py`** — never edit YAML files directly.
- **Phase 2 mutations require user approval** — never mutate the backlog without presenting the plan first.
- **Approvals set `status: done` directly** — this is the one case where `done` is set by the user through this skill.
- **Process feedback BEFORE archiving** — avoid operating on stories that have already been moved to backlog_done.yaml.
- **New entries always get `status: todo`** — the orchestrator handles lifecycle transitions.
- **The `complexity` field is required** for new entries. If complexity is unclear during discovery, flag it explicitly and discuss with the user — don't guess.
- **IDs must be unique** across backlog.yaml and backlog_done.yaml — always use `backlog.py next-id`.
- **Feedback stories get both `review_feedback` and `status: uat_feedback`** — write the feedback text to `review_feedback`, then set status to `uat_feedback`. The orchestrator picks up from there.

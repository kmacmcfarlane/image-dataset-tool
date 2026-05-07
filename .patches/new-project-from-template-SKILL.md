---
name: new-project-from-template
description: "Create a new project from a claude-templates template. Use when user wants to start a new project, scaffold a repo, or bootstrap from a template. Triggers: 'new project', 'create project', 'scaffold', 'from template', 'bootstrap project'."
disable-model-invocation: false
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, AskUserQuestion, Agent
argument-hint: <optional template name>
---

# New Project From Template

Create a new project scaffolded from a claude-templates template. User's argument: $ARGUMENTS

## Instructions

### Step 1: Find the templates repo

Check these locations in order:
```bash
# Common paths
CANDIDATES=(
  "../claude-templates"
  "$(dirname $(git rev-parse --show-toplevel 2>/dev/null))/claude-templates"
  "$HOME/work/src/github.com/kmacmcfarlane/claude-templates"
  "$HOME/src/claude-templates"
)
```

For each candidate, verify it exists and contains template directories (look for subdirectories with `CLAUDE.md` files).

If NOT found, ask via `AskUserQuestion`:
- **question**: "I can't find the claude-templates repo. Clone it, or provide a path?"
- **options**:
  1. Label: "Clone from GitHub", Description: "gh repo clone kmacmcfarlane/claude-templates"
  2. Label: "Provide path", Description: "I'll specify where it is"

If cloning, run: `gh repo clone kmacmcfarlane/claude-templates <appropriate-parent-dir>/claude-templates`

### Step 2: Display available templates

List template directories (each top-level directory that contains a CLAUDE.md):

```bash
for dir in "$TEMPLATES_REPO"/*/; do
  if [ -f "$dir/CLAUDE.md" ]; then
    name=$(basename "$dir")
    # Extract first sentence of description from README or CLAUDE.md
    desc=$(head -5 "$dir/README.md" 2>/dev/null | grep -v "^#" | head -1)
    echo "$name: $desc"
  fi
done
```

Present as a table:

| Template | Description | Stack |
|----------|-------------|-------|
| local-web-app | Full-stack local-first web app | Go + Goa + Vue + SQLite |

If `$ARGUMENTS` matches a template name, skip the selection question.

Otherwise, use `AskUserQuestion`:
- **question**: "Which template would you like to use?"
- Present options from the table above

### Step 3: Determine project location

Use `AskUserQuestion`:
- **question**: "Where should the project be created?"
- **options**:
  1. Label: "Standard path", Description: "~/work/src/github.com/kmacmcfarlane/<name>"
  2. Label: "Current directory", Description: "Create in the current working directory"
  3. Label: "Custom path", Description: "I'll specify a path"

### Step 4: Project naming and identity

Use `AskUserQuestion`:
- **question**: "What's the project name and GitHub repo path?"
- **header**: "Project ID"
- **options**:
  1. Label: "kmacmcfarlane/<name>", Description: "Standard GitHub org"
  2. Label: "Custom", Description: "I'll specify org/name"

From the answer, derive:
- **Directory name**: e.g., `my-project`
- **Go module path**: e.g., `github.com/kmacmcfarlane/my-project`
- **Display name**: Title-cased for docs

### Step 5: Copy template

```bash
cp -r "$TEMPLATES_REPO/$TEMPLATE/" "$PROJECT_DIR/"
cd "$PROJECT_DIR"
```

Perform variable substitution across all files:
- Replace template placeholder names with project name
- Update Go module path in `go.mod`, import paths
- Update `package.json` name field
- Update CLAUDE.md references
- Clear story-specific content (backlog.yaml stories, CHANGELOG entries, PRD content)
- Reset git history (will init fresh in Step 7)

### Step 6: Sandbox configuration

Check if the template includes sandbox example files:
```bash
ls "$TEMPLATES_REPO/$TEMPLATE/.claude-sandbox.example.yaml" 2>/dev/null
ls "$TEMPLATES_REPO/$TEMPLATE/.env.claude-sandbox.example" 2>/dev/null
ls "$TEMPLATES_REPO/$TEMPLATE/Dockerfile.claude-sandbox.example" 2>/dev/null
```

For each example file found, use `AskUserQuestion`:
- **question**: "Set up <filename> for this project? (Show example content)"
- **options**:
  1. Label: "Yes, use example", Description: "Copy the example as-is"
  2. Label: "Yes, customize", Description: "I'll adjust after copying"
  3. Label: "Skip", Description: "Don't set up this file"

If yes, copy the example to the active filename (strip `.example` suffix).

If no example files exist in the template, ask:
- **question**: "Set up claude-sandbox configuration? (Dockerfile.claude-sandbox, .claude-sandbox.yaml, .env.claude-sandbox)"
- **options**:
  1. Label: "Yes, basic setup", Description: "Create minimal sandbox config for this stack"
  2. Label: "Skip", Description: "I'll set up sandbox later"

### Step 7: Initialize and review

```bash
rm -rf .git
git init
git add -A
git commit -m "initial commit from $TEMPLATE template"
```

Show summary of what was created:
- File count and key directories
- Configuration choices made
- Any manual steps needed

### Step 8: GitHub setup

Use `AskUserQuestion`:
- **question**: "Create the GitHub repo and push?"
- **options**:
  1. Label: "Create and push", Description: "gh repo create <org/name> --private --source . --push"
  2. Label: "Create only", Description: "Create the repo but don't push yet (I want to review first)"
  3. Label: "Skip", Description: "I'll handle GitHub setup myself"

If creating:
```bash
gh repo create <org/name> --private --source . --remote origin
git push -u origin main
```

### Step 9: Final report

```
## Project Created

- **Path**: /path/to/project
- **Repo**: github.com/org/name
- **Template**: local-web-app
- **Sandbox**: configured / not configured

### Next steps
1. Install the claude-kit plugin: `/plugin install claude-kit@mcfacehead`
2. Review CLAUDE.md and agent/PRD.md
3. Run `make up-dev` to verify the stack starts
```

## Important

- Templates are COPIED, not linked. The new project is independent after creation.
- Always init a fresh git repo — never carry template git history.
- The `claude-kit` plugin provides development workflow skills — templates don't ship skills.
- Respect `.gitignore` from the template when staging the initial commit.
- If the project directory already exists and is non-empty, STOP and ask the user.

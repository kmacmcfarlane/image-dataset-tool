# Upstream Repo Structures

Built-in knowledge of all three upstream repos in the kmac-claude-kit ecosystem. This file is part of the update-kit skill so it never needs to re-explore these repos.

## claude-templates

- **Repo**: `kmacmcfarlane/claude-templates`
- **Expected local path**: Sibling to project root (`../claude-templates`)
- **Marker**: `local-web-app/agent/` directory exists
- **Purpose**: Scaffolding template for new local-web-app projects. Contains canonical agent workflow docs, subagent definitions, prompt files, and project skeleton.

### Structure

```
claude-templates/
├── CLAUDE.md                  (repo-level context, NOT synced)
├── README.md
├── LICENSE
└── local-web-app/             ← template root (all synced content lives here)
    ├── CLAUDE.md              (template CLAUDE.md, NOT synced — project-specific)
    ├── CHANGELOG.md           (NOT synced)
    ├── Makefile               (NOT synced)
    ├── docker-compose.yml     (NOT synced)
    ├── docker-compose.dev.yml (NOT synced)
    ├── .mcp.json
    ├── agent/
    │   ├── AGENT_FLOW.md      ← SYNCED (universal)
    │   ├── TEST_PRACTICES.md  ← SYNCED (universal)
    │   ├── DEVELOPMENT_PRACTICES.md ← SYNCED (universal)
    │   ├── PROMPT.md          ← SYNCED (universal)
    │   ├── PROMPT_AUTO.md     ← SYNCED (universal)
    │   ├── PROMPT_INTERACTIVE.md ← SYNCED (universal)
    │   ├── PRD.md             (stub, NOT synced)
    │   ├── backlog.yaml       (stub, NOT synced)
    │   ├── ideas/             (directory, NOT synced — contains categorized idea files)
    │   └── QUESTIONS.md       (stub, NOT synced)
    ├── .claude/
    │   ├── settings.json
    │   └── agents/
    │       ├── fullstack-developer.md ← SYNCED (universal)
    │       ├── code-reviewer.md       ← SYNCED (universal)
    │       ├── qa-expert.md           ← SYNCED (universal)
    │       ├── debugger.md            ← SYNCED (universal)
    │       └── security-auditor.md    ← SYNCED (universal)
    ├── backend/               (template Go + Goa skeleton)
    ├── frontend/              (template Vue + Vite skeleton)
    ├── docs/
    └── scripts/
```

### Key facts
- `.claude/skills/` is in `.gitignore` — skills are NOT checked into the template
- The `local-web-app/` prefix is required when constructing upstream paths
- 11 files are synced: 6 agent docs + 5 subagent definitions

## claude-plugins (skills marketplace — source of truth)

- **Repo**: `kmacmcfarlane/claude-plugins`
- **Expected local path**: Sibling to project root (`../claude-plugins`)
- **Marker**: `.claude-plugin/marketplace.json` exists
- **Purpose**: Plugin marketplace bundling reusable Claude Code skills into installable plugins. Replaces the deprecated `claude-skills` repo.

### Structure

```
claude-plugins/
├── README.md
├── LICENSE
├── .claude-plugin/
│   └── marketplace.json       (declares plugin namespaces)
└── plugins/
    ├── claude-kit/            (the dev tooling plugin — what this skill syncs to)
    │   └── skills/
    │       ├── create-skill/          (meta-skill for creating new skills)
    │       │   ├── SKILL.md
    │       │   └── references/
    │       ├── goa/                   (Goa API framework)
    │       ├── musubi-tuner/          (LoRA training)
    │       ├── playwright/            (E2E testing)
    │       ├── backlog-yaml/          (backlog CLI reference)
    │       ├── backlog-entry/         (interactive ticket creation)
    │       ├── backlog-grooming/      (UAT review sessions)
    │       ├── update-kit/            (this skill)
    │       │   ├── SKILL.md
    │       │   └── references/
    │       ├── sandbox/               (claude-sandbox config)
    │       └── sync-claude-kit-skills (internal sync helper)
    ├── mcfacehead/            (homelab infrastructure namespace)
    │   └── skills/...
    └── ai-scripts/            (ai-scripts namespace)
        └── skills/...
```

### Key facts
- Each skill is a directory under `plugins/<namespace>/skills/` containing at minimum `SKILL.md`
- Skill directory names must match the `name` field in SKILL.md frontmatter
- When syncing upstream, copy the entire directory tree to `plugins/claude-kit/skills/<name>/`
- The legacy `claude-skills/` repo is **deprecated** — do NOT sync to it. If a `../claude-skills/` directory exists, ignore it.
- After pushing changes to claude-plugins, projects subscribed to the marketplace can install/update affected skills with the `/plugins` slash command in Claude Code.

## claude-sandbox

- **Repo**: `kmacmcfarlane/claude-sandbox`
- **Expected local path**: Sibling to project root (`../claude-sandbox`)
- **Marker**: `bin/claude-sandbox` file exists
- **Purpose**: Docker-based sandbox for running Claude Code with filesystem isolation and host Docker access. Provides the `claude-sandbox` launcher and `ralph` loop runner.

### Structure

```
claude-sandbox/
├── CLAUDE.md                  (repo guidelines)
├── README.md
├── LICENSE
├── Dockerfile                 (Debian bookworm-slim + Node.js 22 + Docker CLI + Claude Code)
├── entrypoint.sh             (UID/GID remapping at container start)
├── .claude-sandbox.example.yaml (example config for extra mounts)
├── bin/
│   ├── claude-sandbox         (main launcher — builds image, assembles mounts, runs container)
│   └── ralph                  (loop runner — fresh-context iterations using agent/PROMPT*.md)
└── lib/
    └── stream-filter.js       (NDJSON → readable terminal output)
```

### Key facts
- The `ralph` runner reads `agent/PROMPT.md` + `agent/PROMPT_AUTO.md` (or `PROMPT_INTERACTIVE.md`) from child projects — it does NOT contain these files itself
- The sandbox image includes: Debian bookworm-slim, Node.js 22, Docker CLI + compose, git, make, jq, Claude Code CLI
- Go and other language toolchains are NOT included — use project docker-compose services for those
- Currently no files have a universal mapping from child projects to claude-sandbox. The sandbox repo contains infrastructure (Dockerfile, scripts) not agent workflow docs.

## Project-Level Config: agent/claude-kit-repo-map.md

Each child project maintains `agent/claude-kit-repo-map.md` (lives in the project, NOT in this skill). It declares:

1. **Which skills sync upstream** — skills that originated in this project and should be pushed to claude-plugins (`plugins/claude-kit/skills/`)
2. **Additional template files** — extra files beyond the 11 universal ones for claude-templates
3. **Sandbox files** — if a project ever needs to sync files to claude-sandbox (uncommon)

The skill reads this file at runtime. If missing, it offers to create a starter template.

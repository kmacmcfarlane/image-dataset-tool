# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A **template library** for bootstrapping new projects designed to work with Claude Code. It is not a runnable application — it is a collection of self-contained project scaffolding directories. Part of the [kmac-claude-kit](https://github.com/kmacmcfarlane/kmac-claude-kit) ecosystem alongside `claude-sandbox` and `claude-plugins`.

## Required Plugins

Projects created from these templates require the `claude-kit` plugin from the mcfacehead marketplace for development workflow skills (backlog management, Goa API design, Playwright testing, sandbox configuration, skill creation, upstream sync).

Install: `/plugin install claude-kit@mcfacehead`

## Repository structure

```
claude-templates/
├── README.md
├── LICENSE              # GPL-3.0
└── local-web-app/       # Template: Go + Vue local-first web app
```

There is no root-level build system, Makefile, or package manager. Each template directory is fully self-contained.

## Templates

### local-web-app

Full-stack local-first web application scaffold:
- **Backend**: Go 1.25 + Goa v3 (design-first REST API) + SQLite
- **Frontend**: Vue 3 (Composition API) + Vite + TypeScript
- **Testing**: Ginkgo/Gomega (backend), Vitest + Vue Testing Library (frontend)
- **Infrastructure**: Docker Compose with production and dev-mode overlays, multi-stage builds
- **Agent automation**: Complete workflow contract in `agent/` (AGENT_FLOW, practices docs, backlog, prompts, Discord MCP notifications)

Each template includes its own `CLAUDE.md` that serves as the always-loaded agent operating context for projects bootstrapped from it.

## Working in this repo

The typical workflow is:
1. Reading and understanding existing templates
2. Editing template files (agent docs, CLAUDE.md, configuration)
3. Adding new top-level template directories

When adding a new template, follow the structure established by `local-web-app/`: include a `CLAUDE.md`, `agent/` directory, Docker Compose files, Makefile, `.claude/settings.json`, and `scripts/mcp/` for Discord notifications.

Skills are NOT included in templates — they come from the claude-kit plugin (installed via marketplace).

## Commit style

- Format: conventional-style subject lines (e.g., `docs: ...`, `add ...`, `fix ...`)
- Do not add `Co-Authored-By` trailers — attribution is disabled globally.

# Claude Code Project Templates

Project templates for quick-starting new repos designed to work with [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Each template is a self-contained directory with everything needed to bootstrap a new project: dockerized dev workflow, agent automation docs, testing infrastructure, and an MCP server for Discord notifications.

## Templates

| Template | Stack | Description |
|---|---|---|
| [local-web-app](local-web-app/) | Go + Vue | Local-first web app with Goa v3 backend, Vue 3 frontend, Docker Compose |

## Usage

1. Copy the template directory into a new repo (or use the `/new-project-from-template` skill).
2. Search and replace placeholder values (see the template's README for specifics).
3. Install the `claude-kit` plugin: `/plugin install claude-kit@mcfacehead`
4. Write your PRD in `agent/PRD.md` and add stories to `agent/backlog.yaml`.
5. Run `make up` to start the stack.

## What's included in each template

- **CLAUDE.md** -- Agent operating context (always loaded by Claude Code)
- **agent/** -- Workflow contract, development/test practices, backlog, and agent prompts
- **docs/** -- Architecture, database, and API documentation stubs
- **scripts/mcp/** -- Discord notification MCP server
- **.claude/settings.json** -- Claude Code permission policy
- **Docker Compose** -- Production and dev-mode with hot reload
- **Makefiles** -- Root orchestration and per-stack build targets

## What's NOT included

- **Skills** -- Development workflow skills come from the `claude-kit` plugin (installed via the mcfacehead marketplace). Templates do not ship skill definitions.

## Prerequisites

- Docker and Docker Compose
- [claude-sandbox](https://github.com/kmacmcfarlane/claude-sandbox) (optional, for sandboxed Claude Code sessions)
- [claude-plugins](https://github.com/kmacmcfarlane/claude-plugins) `claude-kit` plugin (for backlog management, testing, and workflow skills)

## Part of kmac-claude-kit

This repo is one component of [kmac-claude-kit](https://github.com/kmacmcfarlane/kmac-claude-kit), a toolkit for building software with Claude Code. See that repo for how claude-templates, [claude-sandbox](https://github.com/kmacmcfarlane/claude-sandbox), and [claude-plugins](https://github.com/kmacmcfarlane/claude-plugins) fit together.

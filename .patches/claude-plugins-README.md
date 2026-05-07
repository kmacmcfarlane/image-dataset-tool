# claude-plugins

Private Claude Code plugin marketplace for Kyle McFarlane.

## Plugins

### mcfacehead

Homelab infrastructure skills for the mcfacehead.com network.

| Skill | Description |
|-------|-------------|
| `mcfacehead` | Network overview, server inventory, local LLM inference |
| `brainboy` | TrueNAS storage, ZFS backups, replication, S3 sync |
| `home-assistant` | HA deployment pipeline, rsync workflow, dual source-of-truth |
| `pfsense` | pfSense router diagnostics, network topology |
| `clustertool` | Kubernetes cluster, Flux CD GitOps, SOPS, Talos |
| `sync-mcfacehead-skills` | Sync skills from child repos to shared directory |

### claude-kit

Claude Code development tooling — reusable across projects.

| Skill | Description |
|-------|-------------|
| `create-skill` | Bootstrap new Claude Code skills from a description |
| `sandbox` | claude-sandbox Docker setup, config, troubleshooting |
| `update-kit` | Sync files upstream to claude-templates/claude-plugins/claude-sandbox |
| `new-project-from-template` | Create a new project from a claude-templates template |
| `backlog-entry` | Create backlog entries (stories, bugs, refactoring) |
| `backlog-grooming` | Conversational backlog grooming and UAT review |
| `backlog-yaml` | backlog.yaml CLI management |
| `goa` | Design-first API development with Goa v3 for Go |
| `musubi-tuner` | LoRA training/inference with kohya's musubi-tuner |
| `playwright` | End-to-end testing with Playwright |

## Setup

### Add the marketplace

```bash
/plugin marketplace add kmacmcfarlane/claude-plugins
```

Or in `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "mcfacehead": {
      "source": {
        "source": "github",
        "repo": "kmacmcfarlane/claude-plugins"
      }
    }
  }
}
```

### Install plugins

```bash
/plugin install mcfacehead@mcfacehead
/plugin install claude-kit@mcfacehead
```

Or browse: `/plugin` → Discover tab.

### Private repo auth

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

## Updating skills

Skills are authored directly in this repo under `plugins/<plugin>/skills/<skill>/`.

For the mcfacehead plugin, child repos (brainboy, home-assistant, etc.) own their skill source. The `sync-mcfacehead-skills` script copies them here:

```bash
bash plugins/mcfacehead/skills/sync-mcfacehead-skills/scripts/sync.sh
```

For the claude-kit plugin, skills are authored in-place in this repo.

## Structure

```
claude-plugins/
├── .claude-plugin/
│   └── marketplace.json         # Plugin index (points to ./plugins/)
├── plugins/
│   ├── mcfacehead/              # Homelab infrastructure
│   │   └── skills/
│   │       ├── mcfacehead/
│   │       ├── brainboy/
│   │       ├── home-assistant/
│   │       ├── pfsense/
│   │       ├── clustertool/
│   │       └── sync-mcfacehead-skills/
│   └── claude-kit/              # Dev tooling
│       └── skills/
│           ├── create-skill/
│           ├── sandbox/
│           ├── update-kit/
│           ├── new-project-from-template/
│           ├── backlog-entry/
│           ├── backlog-grooming/
│           ├── backlog-yaml/
│           ├── goa/
│           ├── musubi-tuner/
│           └── playwright/
└── README.md
```

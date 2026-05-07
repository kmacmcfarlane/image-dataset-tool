#!/bin/bash
# Apply all cross-repo changes for ticket_mode workflow update.
# Run from host (not sandbox) since repos are read-only inside container.
set -euo pipefail

BASE="/home/rt/work/src/github.com/kmacmcfarlane"
PLUGINS="$BASE/claude-plugins"
TEMPLATES="$BASE/claude-templates"
CHECKPOINT="$BASE/checkpoint-sampler"
SKILLS="$BASE/claude-skills"
PATCHES="$BASE/image-dataset-tool/.patches"

echo "=== 1. Delete claude-skills repo ==="
if [ -d "$SKILLS" ]; then
  rm -rf "$SKILLS"
  echo "  Deleted $SKILLS"
else
  echo "  Already gone"
fi

echo ""
echo "=== 2. Remove sync-claude-kit-skills from claude-plugins ==="
rm -rf "$PLUGINS/plugins/claude-kit/skills/sync-claude-kit-skills"
echo "  Deleted sync-claude-kit-skills skill"

echo ""
echo "=== 3. Update claude-plugins README ==="
cp "$PATCHES/claude-plugins-README.md" "$PLUGINS/README.md"
echo "  Updated README.md"

echo ""
echo "=== 4. Update update-kit skill ==="
cp "$PATCHES/update-kit-SKILL.md" "$PLUGINS/plugins/claude-kit/skills/update-kit/SKILL.md"
cp "$PATCHES/update-kit-repo-map.md" "$PLUGINS/plugins/claude-kit/skills/update-kit/references/repo-map.md"
echo "  Updated update-kit SKILL.md and repo-map.md"

echo ""
echo "=== 5. Update backlog-grooming skill ==="
cp "$PATCHES/backlog-grooming-SKILL.md" "$PLUGINS/plugins/claude-kit/skills/backlog-grooming/SKILL.md"
echo "  Updated backlog-grooming SKILL.md"

echo ""
echo "=== 6. Update backlog-yaml skill ==="
cp "$PATCHES/backlog-yaml-SKILL.md" "$PLUGINS/plugins/claude-kit/skills/backlog-yaml/SKILL.md"
echo "  Updated backlog-yaml SKILL.md"

echo ""
echo "=== 7. Update backlog-entry skill ==="
cp "$PATCHES/backlog-entry-SKILL.md" "$PLUGINS/plugins/claude-kit/skills/backlog-entry/SKILL.md"
echo "  Updated backlog-entry SKILL.md"

echo ""
echo "=== 8. Create new-project-from-template skill ==="
mkdir -p "$PLUGINS/plugins/claude-kit/skills/new-project-from-template"
cp "$PATCHES/new-project-from-template-SKILL.md" "$PLUGINS/plugins/claude-kit/skills/new-project-from-template/SKILL.md"
echo "  Created new-project-from-template skill"

echo ""
echo "=== 9. Remove local skills from checkpoint-sampler ==="
cd "$CHECKPOINT/.claude/skills"
for skill in backlog-entry backlog-grooming backlog-yaml create-skill goa musubi-tuner playwright update-kit; do
  if [ -d "$skill" ]; then
    rm -rf "$skill"
    echo "  Deleted $skill"
  fi
done
echo "  Kept: comfyui-api (project-specific)"

echo ""
echo "=== 10. Remove local skills from claude-templates ==="
rm -rf "$TEMPLATES/local-web-app/.claude/skills"
echo "  Deleted local-web-app/.claude/skills/"

echo ""
echo "=== 11. Update claude-templates CLAUDE.md and README ==="
cp "$PATCHES/claude-templates-CLAUDE.md" "$TEMPLATES/CLAUDE.md"
cp "$PATCHES/claude-templates-README.md" "$TEMPLATES/README.md"
# Patch local-web-app/CLAUDE.md in-place (replace claude-skills reference)
sed -i 's|claude-skills.*Reusable skills.*https://github.com/kmacmcfarlane/claude-skills|claude-plugins**: Plugin marketplace with reusable skills (claude-kit plugin). See https://github.com/kmacmcfarlane/claude-plugins|' "$TEMPLATES/local-web-app/CLAUDE.md"
echo "  Updated CLAUDE.md files and README"

echo ""
echo "=== 12. Commit in each repo ==="
echo ""

cd "$PLUGINS"
git add -A
git commit -m "chore: remove sync-claude-kit-skills, add ticket_mode to skills, add new-project-from-template, remove claude-skills references"
echo "  Committed in claude-plugins"

cd "$TEMPLATES"
git add -A
git commit -m "chore: remove local skills (use claude-kit plugin), update docs"
echo "  Committed in claude-templates"

cd "$CHECKPOINT"
git add -A
git commit -m "chore: remove local skills (use claude-kit plugin), keep comfyui-api"
echo "  Committed in checkpoint-sampler"

echo ""
echo "=== Done! ==="
echo "Repos to push: claude-plugins, claude-templates, checkpoint-sampler"
echo "claude-skills directory deleted (archive/delete the GitHub repo when ready)"

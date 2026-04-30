---
name: create-skill
description: Bootstrap a new Claude Code skill from a description. Use when the user wants to create a new skill/slash command.
disable-model-invocation: true
allowed-tools: Read, Write, Glob, Grep, Bash
argument-hint: <description of the skill to create>
---

# Create a new Claude Code skill

The user wants to create a new skill. Their description: $ARGUMENTS

Before writing any skill, consult `references/best-practices.md` for Anthropic's official guidance and `references/frontmatter-reference.md` for YAML field details.

## Process

1. **Identify the use case category** to determine appropriate patterns:
   - **Document & Asset Creation** — Creating consistent, high-quality output (docs, code, designs). Key techniques: embedded style guides, templates, quality checklists.
   - **Workflow Automation** — Multi-step processes with consistent methodology. Key techniques: step-by-step workflow with validation gates, iterative refinement loops.
   - **MCP Enhancement** — Workflow guidance layered on top of MCP tool access. Key techniques: coordinating multiple MCP calls, embedding domain expertise, error handling.

2. **Derive a name** from the description:
   - Use lowercase kebab-case: `run-tests`, `check-deps`
   - Keep it short (1-3 words)
   - No spaces, underscores, or capitals
   - Never use "claude" or "anthropic" in the name (reserved)

3. **Write the description field** — this is the most important part. It controls when Claude loads the skill. Structure it as: `[What it does] + [When to use it] + [Key capabilities]`.
   - MUST include both what the skill does AND when to use it (trigger conditions)
   - Include specific phrases users would actually say
   - Mention relevant file types if applicable
   - Keep under 1024 characters, no XML angle brackets
   - Add negative triggers if needed to prevent over-triggering
   - See `references/best-practices.md` for good/bad examples

4. **Decide on settings:**
   - `disable-model-invocation`: default `true` (user-invoked only). Set `false` if the skill should activate automatically based on conversation context.
   - `allowed-tools`: list only the tools the skill actually needs. Common sets:
     - Read-only research: `Read, Glob, Grep`
     - Code modification: `Read, Write, Edit, Glob, Grep, Bash`
     - Bash-heavy: `Bash, Read, Glob`
   - `argument-hint`: a brief hint shown in autocomplete (e.g. `<file-path>`, `<issue-number>`)
   - `context: fork` if the skill should run in an isolated subagent
   - Optional fields: `license`, `compatibility`, `metadata` (author, version, mcp-server, category, tags)

5. **Write the SKILL.md** using progressive disclosure:
   - **Level 1 (frontmatter)**: Always loaded. Just enough for Claude to know when to use the skill.
   - **Level 2 (SKILL.md body)**: Loaded when relevant. Full instructions and guidance.
   - **Level 3 (linked files)**: Referenced as needed. Detailed docs in `references/`, scripts in `scripts/`, templates in `assets/`.

   Use this structure:
   ```yaml
   ---
   name: <skill-name>
   description: <What it does. When to use it. Key trigger phrases.>
   disable-model-invocation: <true|false>
   allowed-tools: <comma-separated tool list>
   argument-hint: <hint>
   ---

   # <Title>

   ## Instructions
   ### Step 1: [First Major Step]
   Clear explanation of what happens.
   Expected output: [describe what success looks like]

   ### Step 2: [Next Step]
   ...

   ## Examples
   Example 1: [common scenario]
   User says: "..."
   Actions: ...
   Result: ...

   ## Troubleshooting
   Error: [Common error message]
   Cause: [Why it happens]
   Solution: [How to fix]
   ```

   Writing instructions:
   - Be specific and actionable. Never write vague guidance like "validate the data before proceeding" — instead specify exactly what to validate and how.
   - Use imperative mood with bullet points and numbered lists.
   - Put critical instructions at the top under `## Important` or `## Critical` headers.
   - Include error handling for common failure modes.
   - For critical validations, prefer bundling a script that checks programmatically over relying on language instructions alone.
   - Use `$ARGUMENTS` to reference user input. Use `$0`, `$1` etc. for positional args.
   - Use `` !`command` `` syntax for dynamic preprocessing only when the skill genuinely needs runtime data injected before Claude sees the prompt.

6. **Plan the folder structure:**
   ```
   your-skill-name/
   ├── SKILL.md              # Required - main skill file
   ├── scripts/              # Optional - executable code
   ├── references/           # Optional - documentation
   └── assets/               # Optional - templates, fonts, icons
   ```

   Critical rules:
   - File MUST be exactly `SKILL.md` (case-sensitive). No variations (`SKILL.MD`, `skill.md`).
   - Folder name must match the `name` field in frontmatter.
   - Do NOT include a `README.md` inside the skill folder. All documentation goes in `SKILL.md` or `references/`.
   - No XML angle brackets anywhere in frontmatter.

7. **Create the skill directory and file:**
   - Determine whether to place it in the project (`.claude/skills/<name>/SKILL.md`) or in the user's skills repo. Ask the user if unclear.
   - Create the directory and write the SKILL.md file.
   - If the skill needs supporting files, create those in the appropriate subdirectories.

8. **Report** what was created and how to invoke it (`/<skill-name>` or `/<skill-name> <args>`).

## Quality checklist

Before finalizing, verify:
- [ ] Folder named in kebab-case matching the `name` field
- [ ] `SKILL.md` file exists (exact spelling)
- [ ] YAML frontmatter has `---` delimiters
- [ ] `description` includes WHAT and WHEN with trigger phrases
- [ ] No XML tags in frontmatter
- [ ] Instructions are specific and actionable (not vague)
- [ ] Error handling included for likely failure modes
- [ ] Examples provided for common scenarios
- [ ] SKILL.md is under 5000 tokens; detailed reference moved to `references/`
- [ ] No `README.md` inside the skill folder

## Guidelines

- Keep SKILL.md focused on core instructions. Move detailed documentation to `references/` files.
- The skill should work well alongside other skills (composability) — don't assume it's the only capability available.
- Don't over-engineer. A skill that does one thing well is better than one that tries to handle every edge case.
- Choose an appropriate pattern from `references/best-practices.md` (sequential workflow, multi-MCP coordination, iterative refinement, context-aware tool selection, or domain-specific intelligence).

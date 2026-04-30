# YAML Frontmatter Reference

## Required fields

```yaml
---
name: skill-name-in-kebab-case
description: What it does and when to use it. Include specific trigger phrases.
---
```

### name (required)
- kebab-case only
- No spaces or capitals
- Should match folder name
- Never use "claude" or "anthropic" (reserved)

### description (required)
- MUST include BOTH: what the skill does AND when to use it (trigger conditions)
- Under 1024 characters
- No XML tags (< or >)
- Include specific tasks/phrases users might say
- Mention file types if relevant

## Optional fields

```yaml
---
name: skill-name
description: [required description]
license: MIT
compatibility: Requires network access and Python 3.10+
allowed-tools: "Bash(python:*) Bash(npm:*) WebFetch"
metadata:
  author: Company Name
  version: 1.0.0
  mcp-server: server-name
  category: productivity
  tags: [project-management, automation]
  documentation: https://example.com/docs
  support: support@example.com
---
```

### license (optional)
- Use if making skill open source
- Common: MIT, Apache-2.0

### compatibility (optional)
- 1-500 characters
- Indicates environment requirements: intended product, required system packages, network access needs, etc.

### allowed-tools (optional)
- Restricts which tools the skill can use
- Example: `"Bash(python:*) Bash(npm:*) WebFetch"`

### metadata (optional)
- Any custom key-value pairs
- Suggested fields: author, version, mcp-server, category, tags, documentation, support

## Claude Code specific fields

These fields are specific to Claude Code and not part of the open standard:

### disable-model-invocation (optional)
- `true` (default): Skill is user-invoked only via `/<skill-name>`
- `false`: Skill can activate automatically based on conversation context

### argument-hint (optional)
- Brief hint shown in autocomplete
- Examples: `<file-path>`, `<issue-number>`, `<description>`

### context (optional)
- Set to `fork` to run the skill in an isolated subagent

## Security restrictions

### Forbidden in frontmatter
- XML angle brackets (< >)
- Skills with "claude" or "anthropic" in name (reserved)

### Why
Frontmatter appears in Claude's system prompt. Malicious content could inject instructions.

### Allowed
- Any standard YAML types (strings, numbers, booleans, lists, objects)
- Custom metadata fields
- Long descriptions (up to 1024 characters)

## Common mistakes

```yaml
# Wrong - missing delimiters
name: my-skill
description: Does things

# Wrong - unclosed quotes
name: my-skill
description: "Does things

# Wrong - name has spaces or capitals
name: My Cool Skill

# Correct
---
name: my-cool-skill
description: Does things. Use when user asks to do things.
---
```

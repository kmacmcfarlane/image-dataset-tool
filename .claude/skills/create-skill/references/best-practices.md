# Skill Best Practices

Source: Anthropic's "The Complete Guide to Building Skills for Claude"

## Writing effective descriptions

The description field is the most important part of a skill. It controls when Claude loads the skill via progressive disclosure.

### Structure

```
[What it does] + [When to use it / trigger conditions] + [Key capabilities]
```

### Good examples

```yaml
# Specific and actionable
description: Analyzes Figma design files and generates developer handoff documentation. Use when user uploads .fig files, asks for "design specs", "component documentation", or "design-to-code handoff".

# Includes trigger phrases
description: Manages Linear project workflows including sprint planning, task creation, and status tracking. Use when user mentions "sprint", "Linear tasks", "project planning", or asks to "create tickets".

# Clear value proposition
description: End-to-end customer onboarding workflow for PayFlow. Handles account creation, payment setup, and subscription management. Use when user says "onboard new customer", "set up subscription", or "create PayFlow account".
```

### Bad examples

```yaml
# Too vague
description: Helps with projects.

# Missing triggers
description: Creates sophisticated multi-page documentation systems.

# Too technical, no user triggers
description: Implements the Project entity model with hierarchical relationships.
```

### Preventing over-triggering

Add negative triggers when needed:

```yaml
description: Advanced data analysis for CSV files. Use for statistical modeling, regression, clustering. Do NOT use for simple data exploration (use data-viz skill instead).

description: PayFlow payment processing for e-commerce. Use specifically for online payment workflows, not for general financial queries.
```

## Writing effective instructions

### Be specific and actionable

```markdown
# Good
Run `python scripts/validate.py --input {filename}` to check data format.
If validation fails, common issues include:
- Missing required fields (add them to the CSV)
- Invalid date formats (use YYYY-MM-DD)

# Bad
Validate the data before proceeding.
```

### Use critical headers for important instructions

```markdown
CRITICAL: Before calling create_project, verify:
- Project name is non-empty
- At least one team member assigned
- Start date is not in the past
```

### Reference bundled resources clearly

```markdown
Before writing queries, consult `references/api-patterns.md` for:
- Rate limiting guidance
- Pagination patterns
- Error codes and handling
```

### Include error handling

```markdown
## Common Issues

### MCP Connection Failed
If you see "Connection refused":
1. Verify MCP server is running
2. Confirm API key is valid
3. Try reconnecting
```

### Prefer code over language for critical validations

For critical validations, bundle a script that performs the checks programmatically rather than relying on language instructions. Code is deterministic; language interpretation isn't.

## Workflow patterns

### Pattern 1: Sequential Workflow Orchestration

Use when users need multi-step processes in a specific order.

Key techniques:
- Explicit step ordering
- Dependencies between steps
- Validation at each stage
- Rollback instructions for failures

### Pattern 2: Multi-MCP Coordination

Use when workflows span multiple services.

Key techniques:
- Clear phase separation
- Data passing between MCPs
- Validation before moving to next phase
- Centralized error handling

### Pattern 3: Iterative Refinement

Use when output quality improves with iteration.

Key techniques:
- Explicit quality criteria
- Iterative improvement loop
- Validation scripts
- Clear stopping conditions

### Pattern 4: Context-Aware Tool Selection

Use when the same outcome requires different tools depending on context.

Key techniques:
- Clear decision criteria
- Fallback options
- Transparency about choices made

### Pattern 5: Domain-Specific Intelligence

Use when the skill adds specialized knowledge beyond tool access.

Key techniques:
- Domain expertise embedded in logic
- Compliance/validation before action
- Comprehensive audit trail
- Clear governance rules

## Core design principles

### Progressive disclosure

Skills use a three-level system to minimize token usage:
1. **Level 1 (YAML frontmatter)**: Always loaded in system prompt. Just enough to know when the skill should be used.
2. **Level 2 (SKILL.md body)**: Loaded when relevant. Full instructions and guidance.
3. **Level 3 (Linked files)**: Additional files Claude navigates only as needed.

### Composability

Claude can load multiple skills simultaneously. Skills should work well alongside others and not assume they are the only capability available.

### Portability

Skills work across Claude.ai, Claude Code, and API. Create once, works everywhere (provided the environment supports any dependencies).

## Testing guidance

### Triggering tests

Test that the skill:
- Triggers on obvious tasks
- Triggers on paraphrased requests
- Does NOT trigger on unrelated topics

### Functional tests

Verify:
- Valid outputs generated
- API/tool calls succeed
- Error handling works
- Edge cases covered

### Performance comparison

Compare with and without the skill:
- Number of back-and-forth messages
- Failed API calls
- Total tokens consumed

## Iteration signals

### Undertriggering
- Skill doesn't load when it should
- Users manually enabling it
- Fix: Add more detail, keywords, and trigger phrases to description

### Overtriggering
- Skill loads for irrelevant queries
- Fix: Add negative triggers, be more specific, clarify scope

### Execution issues
- Inconsistent results, API failures, user corrections needed
- Fix: Improve instructions, add error handling, use scripts for validation

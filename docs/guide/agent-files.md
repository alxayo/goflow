# Agent Files

Agent files use the `.agent.md` format with YAML frontmatter and markdown instructions.

## Example

```markdown
---
name: security-reviewer
description: Reviews code for vulnerabilities
tools:
  - grep
  - view
model: gpt-5
---

# Security Reviewer

Focus on:
1. Injection flaws
2. AuthN/AuthZ problems
3. Secret exposure

Always report severity and file references.
```

## Referencing an agent file

```yaml
agents:
  security:
    file: "./agents/security-reviewer.agent.md"
```

## Discovery order

When no explicit path is set, goflow can discover agents from common locations:

1. `.github/agents/*.agent.md`
2. `.claude/agents/*.md`
3. `~/.copilot/agents/*.agent.md`
4. Custom paths in workflow config

## Best practices

- Keep tool lists minimal for least privilege.
- Keep system prompts role-specific and short.
- Put reusable policy in agent files, task-specific logic in step prompts.

# Agent File Format

Complete reference for `.agent.md` agent definition files.

---

## File Structure

Agent files use the `.agent.md` extension and have two parts:

```markdown
---
# YAML Frontmatter (metadata)
name: agent-name
description: What this agent does
---

# Markdown Body (system prompt)
Instructions for the agent...
```

---

## YAML Frontmatter

The frontmatter defines agent metadata and configuration:

```yaml
---
name: security-reviewer
description: Reviews code for security vulnerabilities
tools:
  - grep
  - semantic_search
  - read_file
model: gpt-4o
agents:
  - escalation-handler
mcp-servers:
  custom-server:
    command: node
    args: ["./server.js"]
---
```

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique agent identifier |
| `description` | string | What this agent does |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tools` | array | All tools | List of allowed tools |
| `model` | string | Workflow default | Model to use |
| `agents` | array | [] | Sub-agents for delegation |
| `mcp-servers` | object | {} | MCP server configurations |

---

## name (required)

```yaml
name: security-reviewer
```

**Rules:**
- Must be unique across all agents in a workflow
- Use lowercase with hyphens: `code-analyzer`, `security-reviewer`
- Referenced in workflow's `agents` and `steps.*.agent`

---

## description (required)

```yaml
description: Reviews code for security vulnerabilities and suggests fixes
```

Used for:
- Documentation
- Agent discovery
- Error messages

---

## tools (optional)

Control which tools the agent can access:

```yaml
tools:
  - grep
  - semantic_search
  - read_file
  - list_dir
```

### Common Tools

| Tool | Description |
|------|-------------|
| `grep` | Search for patterns in files |
| `semantic_search` | Semantic code search |
| `read_file` | Read file contents |
| `list_dir` | List directory contents |
| `view` | View files (alias for read_file) |
| `run_in_terminal` | Execute shell commands |
| `file_search` | Search for files by name |
| `create_file` | Create new files |
| `replace_string_in_file` | Edit existing files |

### Tool Restrictions

If `tools` is specified, the agent can ONLY use those tools:

```yaml
# Agent can ONLY search, not modify files
tools:
  - grep
  - semantic_search
  - read_file
```

If `tools` is omitted, the agent has access to all available tools.

### Shared Memory Tools

For shared memory access:

```yaml
tools:
  - shared_memory_read
  - shared_memory_write
```

---

## model (optional)

Override the workflow's default model:

```yaml
model: gpt-4o
```

Common values:
- `gpt-4o` — OpenAI GPT-4o
- `gpt-4-turbo` — OpenAI GPT-4 Turbo
- `claude-3-opus` — Anthropic Claude 3 Opus
- `claude-3-sonnet` — Anthropic Claude 3 Sonnet

The model must be supported by your Copilot CLI configuration.

---

## agents (optional)

List sub-agents this agent can delegate to:

```yaml
agents:
  - detail-analyzer
  - escalation-handler
```

This enables agent-to-agent handoffs during execution.

---

## mcp-servers (optional)

Configure MCP (Model Context Protocol) servers:

```yaml
mcp-servers:
  database-tools:
    command: docker
    args: ["run", "--rm", "db-mcp:latest"]
    env:
      DB_HOST: localhost
      DB_PORT: "5432"
  
  custom-api:
    command: node
    args: ["./mcp-server.js"]
    cwd: "/path/to/server"
```

### MCP Server Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Command to start the server |
| `args` | array | No | Command arguments |
| `env` | object | No | Environment variables |
| `cwd` | string | No | Working directory |

---

## Markdown Body (System Prompt)

Everything after the YAML frontmatter becomes the agent's system prompt:

```markdown
---
name: security-reviewer
description: Security code reviewer
---

# Security Reviewer

You are an expert security code reviewer specializing in web applications.

## Focus Areas

1. **Injection vulnerabilities** - SQL, command, XSS
2. **Authentication** - Password handling, session management
3. **Authorization** - Access control, privilege escalation
4. **Data protection** - Encryption, sensitive data exposure

## Output Format

For each issue:
- **Severity**: CRITICAL / HIGH / MEDIUM / LOW
- **Location**: File and line number
- **Description**: What the vulnerability is
- **Fix**: How to remediate

## Important Notes

- Always cite specific code locations
- Provide actionable recommendations
- If no issues found, state explicitly
```

### System Prompt Best Practices

1. **Start with identity** — "You are an expert..."
2. **List focus areas** — What to pay attention to
3. **Define output format** — How to structure responses
4. **Add constraints** — What NOT to do
5. **Use markdown formatting** — Headers, lists, emphasis

---

## Complete Example

```markdown title="agents/code-reviewer.agent.md"
---
name: code-reviewer
description: Comprehensive code review with security and quality focus
tools:
  - grep
  - semantic_search
  - read_file
  - list_dir
model: gpt-4o
agents:
  - security-specialist
  - performance-specialist
---

# Code Reviewer

You are a senior software engineer performing comprehensive code reviews. Your goal is to improve code quality, catch bugs, and ensure best practices.

## Review Checklist

### Code Quality
- [ ] Clear naming conventions
- [ ] Appropriate abstractions
- [ ] No code duplication
- [ ] Proper error handling

### Security
- [ ] Input validation
- [ ] No hardcoded secrets
- [ ] Proper authentication checks
- [ ] Safe data handling

### Performance
- [ ] Efficient algorithms
- [ ] No N+1 queries
- [ ] Appropriate caching
- [ ] Resource cleanup

## Severity Levels

| Level | Meaning |
|-------|---------|
| 🔴 CRITICAL | Must fix before merge |
| 🟠 HIGH | Should fix before merge |
| 🟡 MEDIUM | Fix soon after merge |
| 🟢 LOW | Nice to fix eventually |

## Output Format

### Summary
One paragraph overview of the code.

### Issues Found
Numbered list with severity, location, and fix.

### Positive Observations
What the code does well.

### Recommendations
High-level suggestions for improvement.
```

---

## File Discovery

goflow searches for agent files in this order:

1. **Explicit path** — `file: "./agents/reviewer.agent.md"`
2. **Project agents** — `./agents/*.agent.md`
3. **GitHub agents** — `.github/agents/*.agent.md`
4. **Claude agents** — `.claude/agents/*.md` (auto-converted)
5. **Home directory** — `~/.copilot/agents/*.agent.md`
6. **Custom paths** — From `config.agent_search_paths`

---

## VS Code Compatibility

Agent files are compatible with the VS Code Copilot agent format. Files from `.github/agents/` work in both environments.

---

## See Also

- [Workflow Schema](workflow-schema.md) — Using agents in workflows
- [Shared Memory](shared-memory.md) — Coordinating between agents
- [Architecture](architecture.md) — How agents are executed

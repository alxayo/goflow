# Agent Files

Organize agents in reusable, shareable `.agent.md` files.

---

## The Problem

Inline agents work for simple workflows, but they have limitations:

- **Duplication** — Same agent defined in multiple workflows
- **Long prompts** — Makes workflow YAML hard to read
- **No reuse** — Can't share agents between projects
- **Limited features** — Can't define tools, skills, or MCP servers

**Solution:** Define agents in `.agent.md` files and reference them from workflows.

---

## Inline vs File-Based Agents

### Inline Agent (What You've Used)

```yaml
agents:
  reviewer:
    inline:
      description: "Reviews code for issues"
      prompt: "You are a code reviewer..."
```

### File-Based Agent (What You'll Learn)

```yaml
agents:
  reviewer:
    file: "./agents/security-reviewer.agent.md"
```

The agent definition lives in a separate file that can be reused, versioned, and shared.

---

## Agent File Format

Create a file with the `.agent.md` extension:

```markdown title="agents/security-reviewer.agent.md"
---
name: security-reviewer
description: Reviews code for security vulnerabilities
tools:
  - grep
  - semantic_search
  - read_file
model: gpt-4o
---

# Security Reviewer

You are an expert security code reviewer. Your job is to identify vulnerabilities and security issues in code.

## Focus Areas

1. **Injection attacks** - SQL injection, command injection, XSS
2. **Authentication flaws** - Weak passwords, missing checks
3. **Authorization issues** - Privilege escalation, IDOR
4. **Data exposure** - Sensitive data in logs, unencrypted secrets
5. **Input validation** - Missing or insufficient validation

## Output Format

For each issue found:
- **Severity**: CRITICAL / HIGH / MEDIUM / LOW
- **Location**: File path and line number
- **Description**: What the issue is
- **Recommendation**: How to fix it

If no issues are found, explicitly state that the code appears secure.
```

### File Structure

Agent files have two parts:

1. **YAML Frontmatter** (between `---` markers) — Metadata and configuration
2. **Markdown Body** — The system prompt (agent's instructions)

---

## Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for the agent |
| `description` | Yes | What this agent does |
| `tools` | No | List of tools the agent can use |
| `model` | No | Model override (e.g., `gpt-4o`, `claude-3-opus`) |
| `agents` | No | Sub-agents this agent can delegate to |
| `mcp-servers` | No | MCP server configurations |

### Tools

Control which tools the agent can access:

```yaml
---
tools:
  - grep           # Search code
  - semantic_search # Semantic code search
  - read_file      # Read files
  - run_in_terminal # Run commands (use carefully!)
---
```

Common tools:
- `grep` — Search for patterns in files
- `semantic_search` — Semantic code search
- `read_file` — Read file contents
- `view` — View files
- `run_in_terminal` — Execute shell commands

### Model Override

Specify a different model for this agent:

```yaml
---
model: claude-3-opus
---
```

This overrides the workflow's default model.

---

## Using File-Based Agents

Reference the agent file in your workflow:

```yaml title="security-workflow.yaml"
name: "security-audit"

agents:
  security:
    file: "./agents/security-reviewer.agent.md"
  performance:
    file: "./agents/performance-reviewer.agent.md"

steps:
  - id: security-scan
    agent: security
    prompt: "Review {{inputs.files}} for security issues."

  - id: perf-scan
    agent: performance
    prompt: "Review {{inputs.files}} for performance issues."
```

### Path Resolution

Paths are relative to the workflow file:

```
project/
├── workflows/
│   └── audit.yaml        # Workflow file
└── agents/
    └── security.agent.md  # agent path: "../agents/security.agent.md"
```

Or use absolute paths:

```yaml
file: "/Users/me/shared-agents/security.agent.md"
```

---

## Agent Discovery

goflow looks for agent files in these locations (in order):

1. **Explicit path** — `file: "./path/to/agent.agent.md"`
2. **Project agents folder** — `./agents/*.agent.md`
3. **GitHub agents folder** — `.github/agents/*.agent.md`
4. **Claude agents folder** — `.claude/agents/*.md` (auto-converted)
5. **Home directory** — `~/.copilot/agents/*.agent.md`
6. **Custom paths** — Configured in `config.agent_search_paths`

### Automatic Discovery

If you just use an agent name without a file path, goflow searches:

```yaml
agents:
  security-reviewer:  # Will search for security-reviewer.agent.md
```

goflow looks for `security-reviewer.agent.md` in discovery paths.

---

## Complete Example

### Agent File

```markdown title="agents/aggregator.agent.md"
---
name: aggregator
description: Combines multiple reviews into actionable summaries
tools:
  - file_search
model: gpt-4o
---

# Review Aggregator

You combine multiple code reviews into a single, actionable summary.

## Your Process

1. Read all provided reviews carefully
2. Identify common themes and unique findings
3. Deduplicate overlapping issues
4. Prioritize by severity and impact

## Output Format

### Executive Summary
Brief overview (2-3 sentences)

### Priority Actions
Numbered list, sorted by importance

### Detailed Findings
Organized by category (Security, Performance, etc.)

### Recommendations
Concrete next steps for the team
```

### Workflow Using the Agent

```yaml title="workflows/code-review.yaml"
name: "code-review"

agents:
  security:
    file: "../agents/security-reviewer.agent.md"
  performance:
    file: "../agents/performance-reviewer.agent.md"
  aggregator:
    file: "../agents/aggregator.agent.md"

steps:
  - id: sec-review
    agent: security
    prompt: "Review: {{inputs.files}}"

  - id: perf-review
    agent: performance
    prompt: "Review: {{inputs.files}}"

  - id: summary
    agent: aggregator
    prompt: |
      Combine these reviews:
      
      ## Security Review
      {{steps.sec-review.output}}
      
      ## Performance Review
      {{steps.perf-review.output}}
    depends_on: [sec-review, perf-review]

output:
  steps: [summary]
  format: markdown
```

### Run It

```bash
goflow run --workflow workflows/code-review.yaml \
  --inputs files='src/**/*.go' \
  --verbose
```

---

## Organizing Agent Libraries

For larger projects, organize agents by domain:

```
project/
├── agents/
│   ├── review/
│   │   ├── security-reviewer.agent.md
│   │   ├── performance-reviewer.agent.md
│   │   └── accessibility-reviewer.agent.md
│   ├── analysis/
│   │   ├── code-analyzer.agent.md
│   │   └── metrics-collector.agent.md
│   └── helpers/
│       ├── aggregator.agent.md
│       └── formatter.agent.md
└── workflows/
    ├── full-review.yaml
    └── quick-scan.yaml
```

---

## Advanced: MCP Servers

Agent files can configure MCP (Model Context Protocol) servers:

```yaml
---
name: database-admin
description: Manages database operations
mcp-servers:
  postgres:
    command: docker
    args: ["run", "--rm", "postgres-mcp:latest"]
    env:
      DB_HOST: localhost
---
```

See [MCP Integration](../reference/architecture.md#mcp-server-integration) for details.

---

## What You Learned

:white_check_mark: How to create `.agent.md` files  
:white_check_mark: YAML frontmatter for metadata and tools  
:white_check_mark: Markdown body for system prompts  
:white_check_mark: How to reference file-based agents in workflows  
:white_check_mark: Agent discovery paths  

---

## Next Steps

You've completed the tutorial! Here's where to go next:

- :material-book: [Reference Documentation](../reference/workflow-schema.md) — Complete field reference
- :material-code-tags: [Examples](../examples/index.md) — Real-world workflow patterns
- :material-help-circle: [Troubleshooting](../troubleshooting.md) — Common issues and solutions

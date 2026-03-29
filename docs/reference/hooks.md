# Hooks

Hooks let you execute custom shell commands at key points during Copilot CLI agent execution. They enable validation, logging, security scanning, and workflow automation without modifying agent prompts.

Hooks are a Copilot CLI feature — goflow inherits hook support automatically because every workflow step runs as a Copilot CLI session.

---

## How Hooks Work in goflow

When goflow executes a workflow step, Copilot CLI automatically discovers and loads hook configuration files from the repository. Hooks fire at defined points in the agent session lifecycle:

```
┌─────────────────────────────────────────────────┐
│ Step Execution                                  │
│                                                 │
│  sessionStart ─── Agent begins working          │
│       │                                         │
│  userPromptSubmitted ── Prompt sent to agent     │
│       │                                         │
│  ┌─── preToolUse ──── Before each tool call     │
│  │    [approve/deny]                            │
│  └─── postToolUse ─── After each tool call      │
│       │                                         │
│  agentStop ────── Agent finishes response        │
│       │                                         │
│  sessionEnd ───── Session completes             │
└─────────────────────────────────────────────────┘
```

---

## Hook Configuration

Hooks are defined in JSON files stored at `.github/hooks/*.json` in your repository. The file must contain a `version` field (currently `1`) and a `hooks` object with arrays of hook definitions.

### Basic Structure

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "bash": "./scripts/security-check.sh",
        "powershell": "./scripts/security-check.ps1",
        "cwd": ".",
        "timeoutSec": 15
      }
    ]
  }
}
```

### Hook Object Fields

| Field | Required | Description |
|-------|----------|-------------|
| `type` | Yes | Must be `"command"` |
| `bash` | Yes (Unix) | Shell command or path to script to execute |
| `powershell` | Yes (Windows) | PowerShell command or script for Windows execution |
| `cwd` | No | Working directory for the script (relative to repository root) |
| `env` | No | Additional environment variables merged with the existing environment |
| `timeoutSec` | No | Maximum execution time in seconds (default: `30`) |

---

## Hook Types

### `sessionStart`

Fires when a new agent session begins. Use for environment initialization, audit logging, or resource setup.

```json
{
  "sessionStart": [
    {
      "type": "command",
      "bash": "echo \"Session started: $(date)\" >> logs/session.log",
      "powershell": "Add-Content -Path logs/session.log -Value \"Session started: $(Get-Date)\"",
      "timeoutSec": 10
    }
  ]
}
```

### `sessionEnd`

Fires when the agent session completes or is terminated. Use for cleanup, report generation, or notifications.

```json
{
  "sessionEnd": [
    {
      "type": "command",
      "bash": "./scripts/cleanup.sh",
      "powershell": "./scripts/cleanup.ps1",
      "timeoutSec": 60
    }
  ]
}
```

### `userPromptSubmitted`

Fires when a prompt is sent to the agent. Use for request logging and usage analysis.

```json
{
  "userPromptSubmitted": [
    {
      "type": "command",
      "bash": "./scripts/log-prompt.sh",
      "powershell": "./scripts/log-prompt.ps1",
      "env": { "LOG_LEVEL": "INFO" }
    }
  ]
}
```

### `preToolUse`

Fires before the agent uses any tool (`bash`, `edit`, `view`, etc.). This is the most powerful hook — it can **approve or deny** tool executions.

```json
{
  "preToolUse": [
    {
      "type": "command",
      "bash": "./scripts/security-check.sh",
      "powershell": "./scripts/security-check.ps1",
      "timeoutSec": 15
    }
  ]
}
```

**Use cases:**

- Block dangerous commands (e.g., `rm -rf`, `git push --force`)
- Enforce security policies and coding standards
- Require approval for sensitive operations
- Log tool usage for compliance

### `postToolUse`

Fires after a tool completes execution (success or failure). Use for logging results, tracking metrics, or triggering alerts.

```json
{
  "postToolUse": [
    {
      "type": "command",
      "bash": "cat >> logs/tool-results.jsonl",
      "powershell": "$input | Add-Content -Path logs/tool-results.jsonl"
    }
  ]
}
```

### `agentStop`

Fires when the main agent finishes responding to a prompt.

### `subagentStop`

Fires when a subagent completes, before returning results to the parent agent.

### `errorOccurred`

Fires when an error occurs during agent execution. Use for error logging, notifications, or pattern tracking.

---

## Complete Example

A full hook configuration file at `.github/hooks/workflow-hooks.json`:

```json
{
  "version": 1,
  "hooks": {
    "sessionStart": [
      {
        "type": "command",
        "bash": "echo \"Session started: $(date)\" >> logs/session.log",
        "powershell": "Add-Content -Path logs/session.log -Value \"Session started: $(Get-Date)\"",
        "cwd": ".",
        "timeoutSec": 10
      }
    ],
    "preToolUse": [
      {
        "type": "command",
        "bash": "./scripts/security-check.sh",
        "powershell": "./scripts/security-check.ps1",
        "cwd": "scripts",
        "timeoutSec": 15
      },
      {
        "type": "command",
        "bash": "./scripts/log-tool-use.sh",
        "powershell": "./scripts/log-tool-use.ps1",
        "cwd": "scripts"
      }
    ],
    "postToolUse": [
      {
        "type": "command",
        "bash": "cat >> logs/tool-results.jsonl",
        "powershell": "$input | Add-Content -Path logs/tool-results.jsonl"
      }
    ],
    "sessionEnd": [
      {
        "type": "command",
        "bash": "./scripts/cleanup.sh",
        "powershell": "./scripts/cleanup.ps1",
        "cwd": "scripts",
        "timeoutSec": 60
      }
    ]
  }
}
```

---

## Hooks in Agent Files

The `.agent.md` format also supports a `hooks` field in the YAML frontmatter:

```markdown
---
name: security-reviewer
description: Reviews code for vulnerabilities
tools:
  - grep
  - read_file
hooks:
  onPreToolUse: "./scripts/validate-tool.sh"
  onPostToolUse: "./scripts/log-result.sh"
---
```

!!! note "Implementation Status"
    The `hooks` field in `.agent.md` files is **parsed and stored** by the goflow agent loader but is not currently passed to the Copilot CLI session. Hooks configured via `.github/hooks/*.json` (the repository-level hook system) are discovered and executed by Copilot CLI automatically.

---

## Performance Considerations

Hooks run **synchronously** and block agent execution. Keep them fast:

- Target under 5 seconds of execution time per hook
- Use asynchronous logging (append to files) rather than synchronous I/O
- For expensive operations, spawn background processes
- Cache results when possible
- Set appropriate `timeoutSec` values to prevent runaway scripts

---

## Security Considerations

- Validate and sanitize all input processed by hooks — untrusted input could cause unexpected behavior
- Use proper shell escaping when constructing commands to prevent injection
- Never log sensitive data (tokens, passwords, API keys)
- Ensure hook scripts have appropriate file permissions
- Be cautious with hooks that make external network calls
- Set `timeoutSec` to prevent resource exhaustion

# Step Timeout Configuration

## Overview

goflow uses **event-based session monitoring** by default. Sessions complete naturally when the LLM finishes working (signaled by the `session.idle` event from the Copilot SDK). This means **you don't need to configure timeouts for long-running operations** — the agent simply runs until it's done, just like VS Code agents.

The optional `timeout` field provides a **safety limit** for scenarios where you want to prevent runaway sessions.

## When You Don't Need Timeout

Most workflows work perfectly without any timeout configuration:

```yaml
steps:
  - id: comprehensive-analysis
    agent: analyzer
    prompt: "Analyze all 500 files and produce a detailed report..."
    # No timeout needed! Session runs until the agent finishes.

  - id: multi-tool-operation
    agent: researcher
    prompt: "Search the codebase, read relevant files, and synthesize findings..."
    # Complex multi-tool workflows complete naturally.
```

## When to Use Timeout (Optional)

Use `timeout` as a **safety limit** in these scenarios:

| Scenario | Recommended Timeout |
|----------|---------------------|
| CI/CD pipelines with strict time bounds | `timeout: "10m"` |
| Debugging potentially stuck workflows | `timeout: "5m"` |
| Preventing runaway sessions from consuming resources | `timeout: "30m"` |
| Quick sanity-check steps | `timeout: "60s"` |

## Usage

Add a `timeout` field to any step in your workflow YAML:

```yaml
steps:
  - id: ci-analysis
    agent: my-agent
    prompt: "Quick validation check..."
    timeout: "2m"  # CI safety limit

  - id: unlimited-deep-dive
    agent: researcher
    prompt: "Take your time analyzing this complex system..."
    # No timeout — runs until complete
```

## Supported Formats

Go's `time.ParseDuration` syntax is fully supported:

| Format | Example | Duration |
|--------|---------|----------|
| Seconds | `60s` | 60 seconds |
| Minutes | `2m` | 2 minutes |
| Hours | `1h` | 1 hour |
| Combined | `1m30s` | 90 seconds |
| Combined | `1h30m` | 90 minutes |

## Best Practices

### 1. **Start without timeout, add only if needed**

```yaml
# ✅ Good: Let sessions complete naturally
steps:
  - id: step-1
    agent: quick-agent
    prompt: "Quick task"
    # No timeout — completes when done

  - id: step-2
    agent: heavy-agent
    prompt: "Heavy analysis"
    # No timeout — even complex tasks complete naturally
```

### 2. **Use timeout for CI/CD safety limits**

```yaml
# ✅ Good: CI pipeline with time constraints
steps:
  - id: ci-check
    agent: validator
    prompt: "Validate the PR changes"
    timeout: "5m"  # CI job must complete in reasonable time
```

### 3. **Use --verbose or --stream to monitor long-running sessions**

```bash
# See session lifecycle events (tool calls, completion)
goflow run --workflow analysis.yaml --verbose

# See LLM output as it generates
goflow run --workflow analysis.yaml --stream

# Both
goflow run --workflow analysis.yaml --verbose --stream
```

Verbose output shows:
```
[analyze] Agent turn started
[analyze] Calling tool: grep_search
[analyze] Tool completed: grep_search
[analyze] Calling tool: read_file
[analyze] Tool completed: read_file
[analyze] Session completed
```

## How It Works

goflow uses **event-based session monitoring** via the Copilot SDK's `Session.On()` API:

1. When a step starts, goflow subscribes to all 67+ SDK event types
2. Events like `tool.execution_start`, `assistant.message_delta`, and `subagent.started` are tracked
3. The session completes when `session.idle` is received
4. No artificial timeout is applied unless you explicitly set one

This mirrors how VS Code agents work — they run for as long as needed without timeout failures.

## Troubleshooting

### Session seems stuck

If a session appears stuck (no progress in verbose mode):

1. **Check the agent instructions** — Overly complex prompts can confuse the LLM
2. **Simplify to smaller steps** — Break complex workflows into focused steps
3. **Add a safety timeout** — Use `timeout: "10m"` to prevent indefinite waits

### "context deadline exceeded" (with timeout set)

If you set a timeout and the session exceeds it:

1. **Increase the timeout** — The task genuinely needs more time
2. **Simplify agent instructions** — Remove prescriptive multi-step instructions
3. **Split into multiple workflow steps** — Parallel steps can be more efficient

### Example: From timeout to simplified agent

**Before (times out):**
```yaml
- id: scan
  agent: scanner
  timeout: "300s"  # Required workaround
  prompt: |
    Run these 7 security tools in sequence:
    1. bandit -d exec <target>
    2. bandit -d secrets <target>
    ... (complex parsing instructions)
```

**After (clean):**
```yaml
- id: scan
  agent: scanner
  # Uses default 60s timeout
  prompt: |
    Run Bandit exec and secrets scans on target.
    Report all findings.
```

## Limitations

- **Hard SDK limit**: The Copilot SDK has internal hard limits beyond which it will timeout regardless of your timeout setting
- **Tool call overhead**: If tools themselves are slow (network latency, large file operations), timeout won't help
- **Not for all providers**: BYOK providers (OpenAI, Azure, etc.) may have different timeout behaviors

## Future Enhancements

- Per-step retry policies with exponential backoff
- Streaming output for long-running tasks (Phase 5+)
- Timeout escalation (start at 60s, retry at 120s, then 300s)

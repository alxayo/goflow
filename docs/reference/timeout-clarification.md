# Session Monitoring: How It Works

## Executive Summary

goflow uses **event-based session monitoring** — sessions complete naturally when the LLM finishes working, signaled by the `session.idle` event from the Copilot SDK. **No timeout configuration is required** for most workflows.

This means:
- ✅ Agents can run for 1 hour, 5 hours, or longer without timeout failures
- ✅ Real-time progress is visible in `--verbose` mode
- ✅ Sessions complete when the work is done, not when a timer expires
- ✅ Optional `timeout` field provides safety limits when needed (CI/CD, debugging)

## How Event-Based Monitoring Works

### Default Behavior (No Timeout Set)

```go
// goflow subscribes to SDK events via Session.On()
session.On(func(event *copilot.SessionEvent) {
    switch event.Type {
    case "tool.execution_start":
        // Track tool call starting
    case "tool.execution_complete":
        // Track tool call finished
    case "session.idle":
        // Session complete — capture output and return
    case "session.error":
        // Handle error
    }
})

// Send prompt and wait for session.idle (no timeout)
session.Send(ctx, prompt)
```

The session runs until the LLM outputs `session.idle`, indicating work is complete.

### Optional Safety Timeout

```yaml
steps:
  - id: ci-check
    agent: analyzer
    prompt: "Validate changes..."
    timeout: "5m"  # CI safety limit
```

```go
// When timeout is set, a context deadline is applied
if step.Timeout != "" {
    ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
}
// Session still uses event-based completion, but context limits maximum time
```

---

## Verbose Progress Output

Use `--verbose` to see real-time session activity:

```bash
goflow run --workflow analysis.yaml --verbose
```

Output:
```
[discover] Agent turn started
[discover] Calling tool: grep_search
[discover] Tool completed: grep_search
[discover] Calling tool: list_dir
[discover] Tool completed: list_dir
[discover] Session completed
[analyze] Agent turn started
[analyze] Delegating to subagent: helper
[analyze] Session completed
Workflow completed in 127.3s
```

This provides full visibility without timeouts.

---

## Comparison with Previous Behavior

| Aspect | Previous (SendAndWait) | Current (Event-Based) |
|--------|------------------------|----------------------|
| Default timeout | 60 seconds | None (runs to completion) |
| Long-running support | Required `timeout` config | Works automatically |
| Progress visibility | None | `--verbose` shows events |
| Completion detection | Timer-based | Event-based (`session.idle`) |

---

## When to Use Timeout

**Most workflows don't need timeout.** Use it only for:

1. **CI/CD pipelines** — Enforce maximum job duration
2. **Debugging** — Catch potentially stuck workflows
3. **Resource limits** — Prevent runaway sessions

```yaml
# CI example with safety timeout
steps:
  - id: pr-review
    agent: reviewer
    prompt: "Review PR changes"
    timeout: "10m"  # CI must complete in 10 minutes
```
    }
    if event.Type == "assistant.message_delta" {
        log.Printf("LLM: %s", event.Data.Delta)  // Stream output in real-time
    }
    if event.Type == "session.idle" {
        log.Printf("Done!")
    }
})

session.Send(ctx, prompt)  // Fires events above as work progresses
```

This gives you real-time visibility into what the SDK is doing—not just a blocking wait.

### 2. **Session Persistence (Multi-Turn/Resume)**

For truly long workloads that can span hours or days:

```go
// Create session
session, _ := client.CreateSession(ctx, &copilot.SessionConfig{
    Streaming: true,  // Enable event stream
})

// Disconnect gracefully (persists state to disk)
session.Disconnect()

// Later (hours/days later), resume:
session, _ := client.ResumeSession(ctx, sessionID)

// Continue where you left off
session.Send(ctx, "Next step...")
```

The SDK persists infinite sessions to disk with automatic context compaction.

---

## The Truth About "Timeouts"

**What the SDK does NOT have:**
- No hard per-turn lifetime limit
- No per-session expiration
- No lease/keepalive requirement
- No separate "timeout enforcement" layer

**What it HAS:**
- **Context deadline** — passed from caller; controls how long to wait for `session.idle`
- **Event stream** — real-time notifications of LLM progress (if enabled)
- **Session persistence** — state saved to disk, resumable indefinitely
- **Streaming output** — real-time deltas instead of blocking for final output

---

## Why the 60-Second Error Happened

In the security-scan workflow:

1. No `timeout` field was set → SDK default 60s was used
2. The graudit-scanner agent instructions were **too complex** (asking LLM to coordinate multiple tool invocations + parsing)
3. The discovery output was **bloated** (18,000+ files included)
4. The LLM took >60 seconds just to process and start working

**The fix wasn't "extend the timeout"** — it was:
- ✅ Simplify agent instructions (done)
- ✅ Filter discovery output (done)
- ✅ Add timeout field as a safety net (available now)

---

## Recommended Usage

### For Most Workflows
No timeout needed—60 seconds is plenty for normal operations:

```yaml
steps:
  - id: quick-analysis
    agent: analyzer
    prompt: "Analyze this code..."
    # ← No timeout; uses SDK default (60s)
```

### For Long-Running Work
Set an explicit timeout:

```yaml
steps:
  - id: large-dataset-analysis
    agent: heavy-processor
    prompt: |
      Process {{inputs.file_count}} files...
    timeout: "300s"  # 5 minutes
```

### For Very Long Jobs (Hours)
Use streaming + persistence in custom code:

```go
// TODO: Phase 5+ enhancement
// Add streaming: true to SessionConfig
// Implement event listeners in orchestrator
// Add session resume support to CLI
```

---

## Conclusion

**The 60-second limit is real, but it's a configuration detail, not a design limitation.**

You have full control via:
1. `step.timeout` in YAML (simplest, what we use now)
2. SDK event API with streaming (for monitoring)
3. Session persistence + resume (for true long-running workflows)

The user was correct: there's no unreasonable timeout constraint. The SDK team (and VS Code agents) use these exact patterns for hour-long operations.

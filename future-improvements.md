# Future Improvements — Copilot SDK Features

This document tracks SDK capabilities that are available but **not yet implemented** in goflow.
Each feature maps to a real API surface in `github.com/github/copilot-sdk/go v0.2.0`.

---

## ~~Streaming Output~~ ✅ IMPLEMENTED

**SDK API:** `session.On("assistant.message_delta", callback)`

**Status:** Implemented in v1.x via `--stream` CLI flag.

Usage:
```bash
# Stream LLM output token-by-token
goflow run --workflow example.yaml --stream

# Combine with verbose for full visibility
goflow run --workflow example.yaml --verbose --stream
```

---

## Session Resume

**SDK API:** `client.ResumeSession(ctx, sessionID)`

The SDK can reconnect to a previously created session by its ID, enabling:

- `goflow resume --run <dir> --step <id>` to retry a failed step without re-running the entire workflow
- Checkpoint-based recovery: read `session_id` from `step.meta.json`, reconnect, continue
- Cost savings: no need to re-send the full conversation history

**Difficulty:** Medium — requires persisting `session_id` in audit metadata (already written),
plus new CLI command and orchestrator logic.

---

## Per-Step Provider Override

**SDK API:** `SessionConfig.Provider` set per session

Currently the provider is set globally via `config.provider` in the workflow YAML. The SDK
creates a new session per step, so each step could use a different provider:

```yaml
steps:
  - id: fast-scan
    agent: scanner
    model: "gpt-4o-mini"
    provider:
      type: azure
      base_url: "https://my-fast-instance.openai.azure.com"
      api_key_env: AZURE_FAST_KEY

  - id: deep-review
    agent: reviewer
    model: "o3"
    provider:
      type: openai
      api_key_env: OPENAI_KEY
```

**Difficulty:** Low-Medium — extend step YAML schema, pass provider through to executor.

---

## Hooks (Pre/Post Tool Use)

**SDK API:** `SessionConfig.Hooks` with `OnPreToolUse` and `OnPostToolUse`

The SDK exposes lifecycle hooks that fire before and after every tool invocation:

- **Audit enrichment:** Log every tool call with timing, arguments, and results
- **Security guardrails:** Block dangerous tool calls (e.g., prevent `rm -rf /`)
- **Cost tracking:** Count tool invocations per step

```go
config.Hooks = &copilot.HooksConfig{
    OnPreToolUse: func(tool string, args map[string]any) error {
        audit.LogToolCall(stepID, tool, args)
        if isBlocked(tool, args) {
            return fmt.Errorf("tool %s blocked by policy", tool)
        }
        return nil
    },
    OnPostToolUse: func(tool string, result string) {
        audit.LogToolResult(stepID, tool, result)
    },
}
```

**Difficulty:** Medium — define hook configuration in YAML, wire into executor, design policy format.

---

## Session Lifecycle Events for Audit

**SDK API:** `session.On(eventType, callback)` — 40+ event types

The SDK emits structured events throughout a session's lifecycle:

| Event Category | Examples | Audit Use |
|---|---|---|
| Messages | `assistant.message`, `user.message` | Full transcript |
| Tool calls | `tool.call`, `tool.result` | Tool invocation log |
| Model | `model.request`, `model.response` | Token usage, latency |
| Session | `session.created`, `session.ended` | Lifecycle tracking |
| Errors | `error`, `rate_limit` | Error classification |

Subscribing to these events would replace the current output-scraping approach with
structured, typed audit data.

**Difficulty:** Medium — define event-to-audit mapping, update `transcript.jsonl` schema.

---

## OpenTelemetry Tracing

**SDK API:** `ClientOptions` with OTel configuration

The SDK can emit OpenTelemetry spans for every session and message exchange:

- Distributed tracing across workflow steps
- Integration with Jaeger, Zipkin, Datadog, Honeycomb
- Automatic span hierarchy: workflow → level → step → message → tool call

**Difficulty:** Medium — configure OTel exporter, propagate trace context across parallel steps.

---

## Model Listing

**SDK API:** `client.ListModels(ctx)`

Query available models from the current provider at runtime:

- Validate `model:` fields in workflow YAML before execution
- Auto-suggest models in interactive mode
- Detect model deprecations early

**Difficulty:** Low — single API call, wire into a `goflow models` CLI command.

---

## Session Management

**SDK API:** `client.ListSessions(ctx)`, `client.DeleteSession(ctx, sessionID)`

Manage SDK sessions programmatically:

- Clean up orphaned sessions after crashes
- List active sessions for debugging
- Implement session pool for high-throughput workflows

**Difficulty:** Low — expose via `goflow sessions list` and `goflow sessions delete` commands.

---

## Reasoning Effort Configuration

**SDK API:** `MessageOptions.ReasoningEffort`

Control how much "thinking" the model does per step:

```yaml
steps:
  - id: quick-check
    reasoning_effort: low    # Fast, cheaper
  - id: deep-analysis
    reasoning_effort: high   # Thorough, more tokens
```

**Difficulty:** Low — pass through to `MessageOptions`, add YAML field.

---

## Plan Approval

**SDK API:** Plan-related events and approval callbacks

The SDK supports a plan-approve-execute pattern where the model proposes a plan
and waits for approval before executing:

- Review step plans before committing changes
- Human-in-the-loop for destructive operations
- Audit trail of approved vs. rejected plans

**Difficulty:** Medium — requires interactive approval UX and plan event handling.

---

## Infinite Sessions with Compaction

**SDK API:** Session compaction / context management

For very long workflows, the SDK can compact conversation history to stay within
context window limits:

- Run workflows with dozens of steps without context overflow
- Automatic summarization of prior conversation turns
- Configurable compaction strategy

**Difficulty:** Medium — requires understanding compaction triggers and tuning.

---

## Custom SDK Tools (DefineTool)

**SDK API:** `copilot.DefineTool(name, description, handler)`

Register custom Go functions as tools available to the LLM during a session:

- **Workflow-specific tools:** Query databases, call internal APIs, access secrets
- **Memory tools:** Read/write shared memory without file system access
- **Validation tools:** Run linters, type checkers, or test suites inline

```go
copilot.DefineTool("check_coverage", "Run test coverage for a package", func(args map[string]any) (string, error) {
    pkg := args["package"].(string)
    return runCoverageCheck(pkg)
})
```

**Difficulty:** High — design tool registration API, handle serialization, security implications.

---

## SetModel Mid-Session

**SDK API:** `session.SetModel(modelName)`

Change the model used by a session without creating a new one:

- Start with a fast model for initial analysis, switch to a powerful model for synthesis
- Cost optimization: use cheaper models for boilerplate steps within one session

**Difficulty:** Low — but limited use case in goflow since each step already gets its own session.

---

## Priority Assessment

| Feature | Impact | Difficulty | Suggested Phase |
|---|---|---|---|
| ~~Streaming Output~~ | ~~High~~ | ~~Low~~ | ✅ Implemented |
| Session Resume | High | Medium | Phase 3 |
| Hooks (Pre/Post Tool) | High | Medium | Phase 5 |
| Session Lifecycle Events | High | Medium | Phase 5 |
| Per-Step Provider Override | Medium | Low-Medium | Phase 4 |
| Reasoning Effort | Medium | Low | Phase 4 |
| OTel Tracing | Medium | Medium | Phase 5 |
| Model Listing | Low | Low | Phase 3 |
| Plan Approval | Medium | Medium | Phase 5 |
| Session Management | Low | Low | Phase 3 |
| Infinite Sessions | Medium | Medium | Phase 6 |
| Custom SDK Tools | Medium | High | Phase 6 |
| SetModel Mid-Session | Low | Low | Phase 6 |

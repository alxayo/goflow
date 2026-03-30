# goflow Settings Reference

This file is the repository-side, implementation-accurate reference for goflow settings and options.

It is based on the current Go source code, not on planned roadmap behavior. If a field is parsed but not yet active in the normal `goflow run` path, that is called out explicitly.

## Important Current Runtime Notes

1. `goflow run` uses the parallel orchestrator path (`RunParallel`) and executes DAG levels in dependency order.
2. `config.max_concurrency` is active in the normal CLI path and limits concurrent steps within each level (`0` means unlimited).
3. `output.truncate` is parsed and helper logic exists, but it is not currently applied during template injection or final output formatting.
4. Shared-memory package support exists, but the main CLI path does not automatically wire it into execution.
5. Older docs may describe intended future behavior; this file documents current behavior.

## Implemented CLI Commands

| Command | Status | Notes |
|---|---|---|
| `goflow run` | Implemented | Main workflow execution command |
| `goflow version` | Implemented | Prints version, commit, and build time |
| `goflow help` | Implemented | Prints usage |
| `goflow validate` | Not implemented | Mentioned in older docs, not present in CLI |
| `goflow list` | Not implemented | Mentioned in older docs, not present in CLI |

## `goflow run` Flags

| Flag | Status | Exact behavior |
|---|---|---|
| `--workflow` | Implemented | Required path to workflow YAML |
| `--inputs key=value` | Implemented | Repeatable. Overrides declared defaults. Undeclared keys are still accepted |
| `--audit-dir` | Implemented | Overrides `config.audit_dir` |
| `--mock` | Implemented | Returns deterministic `mock output` |
| `--interactive` | Implemented | Enables the terminal question/answer mechanism for interactive steps |
| `--verbose` | Implemented | Writes progress/status logs to stderr |

## Workflow Top-Level Fields

| Field | Status | Exact behavior |
|---|---|---|
| `name` | Implemented | Required by validator |
| `description` | Implemented | Informational |
| `inputs` | Implemented | Merged with CLI values |
| `config` | Mixed | Some fields active, some parsed only |
| `agents` | Implemented | Used for agent resolution |
| `skills` | Parsed only | Stored on the workflow struct, not used by runtime |
| `steps` | Implemented | Drives DAG and execution |
| `output` | Implemented | Controls final formatting |

## `config` Fields

| Field | Status | Exact behavior |
|---|---|---|
| `model` | Implemented | Workflow-level fallback model |
| `audit_dir` | Implemented | Defaults to `.workflow-runs` when omitted |
| `audit_retention` | Implemented | `<= 0` keeps all runs |
| `agent_search_paths` | Implemented | Added to discovery scan paths |
| `interactive` | Implemented | Default interactivity for steps unless step override is set |
| `log_level` | Partially implemented | Defaulted to `info`, but not used to alter logger behavior |
| `max_concurrency` | Implemented | Used by `RunParallel()` in normal CLI runs to cap same-level concurrency (`0` = unlimited) |
| `shared_memory.*` | Parsed only in CLI path | Types exist, runtime wiring is not automatic yet |
| `provider` | Yes (SDK executor) | Used by the SDK executor for BYOK routing. Ignored when running with `--cli` |
| `streaming` | Parsed only | Not used by current executor |

## `steps` Fields

| Field | Status | Exact behavior |
|---|---|---|
| `id` | Implemented | Must be unique |
| `agent` | Implemented | Must resolve to a known agent |
| `prompt` | Implemented | Required step prompt |
| `depends_on` | Implemented | Used in DAG construction |
| `condition` | Implemented | Supports `contains`, `not_contains`, `equals` |
| `model` | Implemented | Highest-priority model override |
| `extra_dirs` | Implemented | Passed to the executor (SDK: `SessionConfig.SkillDirectories`; CLI: `--add-dir`) |
| `interactive` | Implemented | Per-step override for interaction |
| `skills` | Parsed only | Not consumed by runtime |
| `on_error` | Parsed only | No error-policy engine yet |
| `retry_count` | Implemented | Retries timeout-style transient failures with backoff. Attempts = `retry_count + 1` |
| `timeout` | Implemented | Optional safety limit for step execution (e.g., `timeout: "5m"`) |

### Event-based session completion

Sessions complete naturally when the LLM finishes working (via `session.idle` event from the Copilot SDK). This means:

- No timeout configuration is required for long-running operations
- Sessions run until the agent finishes, just like VS Code agents
- Use `--verbose` to see real-time progress (tool calls, agent delegations, session completion)

### Timeout behavior (optional)

The `timeout` step field provides an **optional safety limit**:

- When not set: sessions complete via event-based monitoring (no timeout applied)
- When set: a context deadline is applied as a maximum execution time
- Use for CI/CD pipelines with strict time bounds or debugging stuck workflows

### Retry behavior

`retry_count` applies to both session creation and step prompt send.

- A retry is attempted only for timeout-style transient errors (for example `context deadline exceeded`, `waiting for session.idle`, and generic timeout messages).
- Non-timeout errors fail the step immediately.
- Backoff is linear and short: 500ms for the first retry, then 1s, 1.5s, and so on.

### Parallel failure policy

In parallel levels (levels with more than one step), execution is best effort:

- If one sibling step fails, other siblings in that level continue.
- Failed step outputs are treated as empty strings for downstream template resolution.
- Fan-in steps may still run when dependencies include failed steps, using empty output for failed dependencies.

In single-step levels, failures are fail-fast and stop the workflow.

### Condition behavior

Only the first non-empty operator is used, in this order:

1. `contains`
2. `not_contains`
3. `equals`

There is no `not_equals` support in the current code.

## `output` Fields

| Field | Status | Exact behavior |
|---|---|---|
| `steps` | Implemented | Selects which step outputs are shown |
| `format` | Implemented | `markdown`, `json`, `plain`, `text` |
| `truncate` | Parsed only in normal runtime | Not automatically applied today |

### `truncate` exact meaning

The helper implementation supports:

| Strategy | Helper behavior |
|---|---|
| `chars` | Keep the first `limit` Unicode characters |
| `lines` | Keep the first `limit` lines |
| `tokens` | Approximate 1 token as 4 characters and keep the first `limit * 4` characters |

Why it exists:
- to prevent large intermediate outputs from overwhelming downstream prompts
- to reduce prompt size and cost
- to avoid context-window overflow

Current status:
- the helper exists
- the runtime does not automatically call it during normal workflow execution

## Agent File Fields

### Runtime-active fields

| Field | Exact behavior |
|---|---|
| `name` | Defaults to filename stem if omitted |
| `description` | Stored on agent |
| `tools` | Used as allow-list for Copilot CLI tools |
| `model` | Accepts string or list |
| markdown body | Used as system prompt |

### Parsed-only or preserved metadata

| Field | Status |
|---|---|
| `agents` | Parsed but not used by current runtime |
| `mcp-servers` | Parsed but not passed into current step sessions |
| `handoffs` | Parsed only |
| `hooks` | Parsed only |
| `argument-hint` | Parsed only |
| `user-invocable` | Parsed only |
| `disable-model-invocation` | Parsed only |
| `target` | Parsed only |

## Shared Memory Status

The codebase includes:
- a shared memory manager
- a persisted `memory.md`
- prompt injection helper logic
- tool metadata for `read_memory` and `write_memory`

The normal CLI path does not yet:
- instantiate the manager from workflow config
- register memory tools into sessions automatically
- inject shared memory into prompts automatically

## Canonical Website Reference

The MkDocs site mirrors this guidance here:
- `docs/reference/settings-and-options.md`

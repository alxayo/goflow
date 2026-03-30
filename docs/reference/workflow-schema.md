# Workflow Schema

This page describes the workflow YAML shape that the current codebase parses and validates.

For exact runtime behavior of every field, including fields that are parsed but not yet active in `goflow run`, see [Settings And Options](settings-and-options.md).

---

## Top-Level Shape

```yaml
name: "workflow-name"
description: "What the workflow does"

inputs:
  key:
    description: "Human-readable input description"
    default: "value"

config:
  model: "gpt-5"
  audit_dir: ".workflow-runs"
  audit_retention: 10
  interactive: true
  agent_search_paths:
    - "./agents"

agents:
  reviewer:
    inline:
      description: "Reviews code"
      prompt: "You are a reviewer"
      tools: [grep, view]
      model: gpt-5

steps:
  - id: analyze
    agent: reviewer
    prompt: "Analyze {{inputs.files}}"

output:
  steps: [analyze]
  format: markdown
```

---

## Top-Level Fields

| Field | Required | Notes |
|---|---|---|
| `name` | Yes | Validation fails if omitted |
| `description` | No | Informational |
| `inputs` | No | Runtime values with optional defaults |
| `config` | No | Workflow-wide settings |
| `agents` | Yes in practice | Steps must resolve to agent definitions |
| `skills` | No | Parsed, but not used by the current CLI path |
| `steps` | Yes | Must contain at least one step |
| `output` | No | Controls final stdout formatting |

---

## `inputs`

```yaml
inputs:
  files:
    description: "Files to review"
    default: "pkg/**/*.go"
```

| Field | Required | Notes |
|---|---|---|
| `description` | No | Documentation only in the current runtime |
| `default` | No | Used when CLI does not provide a value |

Use inputs in prompts with `{{inputs.files}}`.

---

## `config`

```yaml
config:
  model: gpt-5
  audit_dir: .workflow-runs
  audit_retention: 10
  interactive: true
  agent_search_paths:
    - ./custom-agents
```

### Fields defined in the schema

| Field | Parsed | Active in `goflow run` |
|---|---|---|
| `model` | Yes | Yes |
| `audit_dir` | Yes | Yes |
| `audit_retention` | Yes | Yes |
| `shared_memory` | Yes | Not fully wired |
| `provider` | Yes | No |
| `streaming` | Yes | No |
| `log_level` | Yes | Defaulted only |
| `agent_search_paths` | Yes | Yes |
| `max_concurrency` | Yes | Yes |
| `interactive` | Yes | Yes |

Important: `goflow run` uses the parallel orchestrator path (`RunParallel`). DAG levels execute in dependency order, with concurrent execution inside each level.

---

## `agents`

Two forms are supported.

### File-based

```yaml
agents:
  security:
    file: "./agents/security-reviewer.agent.md"
```

### Inline

```yaml
agents:
  reviewer:
    inline:
      description: "Reviews code"
      prompt: "You are an expert reviewer"
      tools: [grep, read_file]
      model: gpt-5
```

Inline agents support only these fields:

| Field | Required |
|---|---|
| `description` | No |
| `prompt` | Yes |
| `tools` | No |
| `model` | No |

---

## `steps`

```yaml
steps:
  - id: analyze
    agent: reviewer
    prompt: "Analyze the code"

  - id: summarize
    agent: reviewer
    prompt: "Summarize {{steps.analyze.output}}"
    depends_on: [analyze]
    condition:
      step: analyze
      contains: "issue"
```

### Step fields currently defined

| Field | Parsed | Active in runtime |
|---|---|---|
| `id` | Yes | Yes |
| `agent` | Yes | Yes |
| `prompt` | Yes | Yes |
| `depends_on` | Yes | Yes |
| `condition` | Yes | Yes |
| `skills` | Yes | No |
| `on_error` | Yes | No |
| `retry_count` | Yes | Yes (timeout-style transient retries) |
| `timeout` | Yes | Yes (optional safety limit) |
| `model` | Yes | Yes |
| `extra_dirs` | Yes | Yes |
| `interactive` | Yes | Yes |

### Event-Based Session Completion

Sessions complete naturally when the LLM finishes (via `session.idle` event). No timeout is required for long-running operations — this mirrors how VS Code agents work.

### Parallel failure handling

In levels with multiple steps, execution is best effort:

1. A failed sibling does not cancel other siblings in that same level.
2. Downstream `{{steps.X.output}}` for failed steps resolves to an empty string.
3. Fan-in steps can still run when one or more fan-out branches fail.

In single-step levels, failures remain fail-fast and stop workflow execution.

### Retry and timeout fields

- `retry_count` is active and retried attempts are only for timeout-style transient errors.
- `timeout` is **optional** — use it only as a safety limit for CI/CD pipelines or debugging stuck workflows. Sessions complete via events by default.

### Condition operators actually supported

| Operator | Supported |
|---|---|
| `contains` | Yes |
| `not_contains` | Yes |
| `equals` | Yes |
| `not_equals` | No |

Only one operator is evaluated today. The evaluator checks `contains`, then `not_contains`, then `equals`.

Validation also requires `condition.step` to be an upstream dependency of the step being guarded.

---

## `output`

```yaml
output:
  steps: [summary]
  format: markdown
  truncate:
    strategy: chars
    limit: 5000
```

| Field | Parsed | Active in runtime |
|---|---|---|
| `steps` | Yes | Yes |
| `format` | Yes | Yes |
| `truncate` | Yes | Not currently applied |

Supported output formats in the current reporter are:

| Value | Behavior |
|---|---|
| `markdown` | Markdown report |
| `json` | JSON output |
| `plain` | Plain text report |
| `text` | Alias of `plain` |
| anything else | Falls back to `markdown` |

For the exact behavior of `truncate`, including why it exists and why it currently has no effect in `goflow run`, see [Output Control](output.md).

---

## Template Variables

Supported template forms:

```yaml
{{inputs.name}}
{{steps.step-id.output}}
```

Step references are resolved before input references.

---

## Validation Rules

The current validator checks:

1. workflow name is present
2. at least one step exists
3. step IDs are unique
4. every step has an agent and prompt
5. `depends_on` only references known steps
6. no step depends on itself
7. condition step references exist and are upstream dependencies
8. prompt template step references point to known steps
9. every workflow agent entry has either `file` or `inline`

Cycle detection happens later in DAG building.

---

## See Also

- [Settings And Options](settings-and-options.md)
- [CLI Reference](cli.md)
- [Output Control](output.md)

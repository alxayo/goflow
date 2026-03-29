# Settings And Options

This page is the implementation-accurate reference for goflow's current settings and options.

It is based on the actual Go code in the repository, not just the intended roadmap. Where a field is parsed but not yet used by the `goflow run` command, that is called out explicitly.

---

## Current Runtime Snapshot

Before reading the field-by-field reference, these are the most important current behavior notes:

1. `goflow run` currently executes through `Orchestrator.Run()`, which processes DAG levels sequentially. A parallel runner exists in code as `RunParallel()`, but the CLI does not call it yet.
2. `config.max_concurrency` is implemented in the parallel orchestrator, but has no effect in the normal CLI path today.
3. `output.truncate` and the truncation helper exist in code, but automatic truncation is not currently applied during prompt template injection or final output formatting.
4. Shared memory types and helper packages exist, but the main CLI path does not currently create a memory manager or register memory tools automatically.
5. Several fields are parsed for future capability, but are not yet consumed by runtime execution. These are marked as `parsed only` below.

---

## CLI Commands

These are the commands currently implemented in `cmd/workflow-runner/main.go`:

| Command | Implemented | Behavior |
|---|---|---|
| `goflow run` | Yes | Parses workflow, validates it, resolves agents, executes steps, writes audit output |
| `goflow version` | Yes | Prints version, commit, and build date |
| `goflow help` | Yes | Prints usage |
| `goflow validate` | No | Mentioned in some docs, but not implemented in the CLI |
| `goflow list` | No | Mentioned in some docs, but not implemented in the CLI |

### `goflow run` flags

| Flag | Implemented | Exact behavior |
|---|---|---|
| `--workflow` | Yes | Required. Path to YAML workflow file |
| `--inputs key=value` | Yes | Repeatable. Merged with workflow defaults. Unknown keys are also passed through and remain available to templates if referenced |
| `--audit-dir` | Yes | Overrides `config.audit_dir` |
| `--mock` | Yes | Uses `MockSessionExecutor` and returns `mock output` for each step |
| `--interactive` | Yes | Enables user-input handler wiring so interactive steps can pause for clarification |
| `--verbose` | Yes | Writes progress and status messages to stderr |

### Exit codes

| Code | Exact behavior |
|---|---|
| `0` | Successful completion |
| `1` | Any error path currently handled by the CLI |

---

## Workflow Top-Level Fields

Defined in `pkg/workflow/types.go`.

| Field | Implemented | Exact behavior |
|---|---|---|
| `name` | Yes | Required by validation. Used in audit directory naming and metadata |
| `description` | Yes | Parsed and stored. Informational only in current CLI path |
| `inputs` | Yes | Used to merge defaults with CLI `--inputs` |
| `config` | Yes | Some fields active, some parsed only. See below |
| `agents` | Yes | Required in practice because each step agent must resolve |
| `skills` | Parsed only | Stored on the workflow struct, but not consumed by the current CLI/executor path |
| `steps` | Yes | Required. Drives DAG building and execution |
| `output` | Yes | Controls output step selection and formatting |

---

## `inputs`

Defined as:

```yaml
inputs:
  my_input:
    description: "Human-readable help text"
    default: "value"
```

| Field | Implemented | Exact behavior |
|---|---|---|
| `description` | Yes | Parsed and stored, but used mainly as documentation right now |
| `default` | Yes | Used by the CLI when the same key is not supplied via `--inputs` |

### Exact merge behavior

The CLI builds the final input map like this:

1. Start with all declared workflow inputs.
2. If a CLI value exists for a declared input, use that.
3. Otherwise, if the input has a non-empty `default`, use the default.
4. Finally, include any CLI inputs that were not declared in YAML as pass-through values.

This means undeclared inputs are accepted today.

---

## `config`

Defined in `pkg/workflow/types.go` and defaulted in `pkg/workflow/parser.go`.

### Active fields

| Field | Implemented | Exact behavior |
|---|---|---|
| `model` | Yes | Workflow-level fallback model. Used if the step and agent do not specify a model |
| `audit_dir` | Yes | Audit root directory. Defaults to `.workflow-runs` when omitted |
| `audit_retention` | Yes | Passed to retention cleanup. If `<= 0`, no runs are deleted |
| `agent_search_paths` | Yes | Additional discovery locations used by agent resolution |
| `interactive` | Yes | Enables interactive mode by default for all steps unless a step overrides it |

### Parsed with defaults but not functionally used much

| Field | Implemented | Exact behavior |
|---|---|---|
| `log_level` | Partially | Defaults to `info` when omitted, but current CLI logging does not branch on it |

### Present in types but not effective in normal `goflow run`

| Field | Implemented | Exact behavior |
|---|---|---|
| `max_concurrency` | Partially | Used by `Orchestrator.RunParallel()`, but the CLI currently calls sequential `Run()` |
| `shared_memory.enabled` | Parsed only in CLI path | Stored in config, but main execution does not automatically create shared memory |
| `shared_memory.inject_into_prompt` | Parsed only in CLI path | No automatic prompt injection currently happens in `goflow run` |
| `shared_memory.initial_content` | Parsed only in CLI path | The memory manager supports initial content, but the CLI does not wire it in |
| `shared_memory.initial_file` | Parsed only | Declared in types, but not consumed in the current runtime |
| `provider` | Parsed only | Stored in config, but not used by `goflow run` or the Copilot CLI executor |
| `streaming` | Parsed only | Stored in config, but not used by the current executor |

### Defaults that are applied automatically

| Field | Default |
|---|---|
| `config.audit_dir` | `.workflow-runs` |
| `config.log_level` | `info` |
| `output.format` | `markdown` |

---

## `agents`

Workflow YAML supports two forms:

```yaml
agents:
  from_file:
    file: "./agents/security-reviewer.agent.md"

  inline_agent:
    inline:
      description: "Inline definition"
      prompt: "System prompt"
      tools: [grep, read_file]
      model: gpt-5
```

### Agent reference fields

| Field | Implemented | Exact behavior |
|---|---|---|
| `file` | Yes | Loaded relative to the workflow file location unless already absolute |
| `inline` | Yes | Converted into an in-memory agent definition |

### Inline agent fields

| Field | Implemented | Exact behavior |
|---|---|---|
| `description` | Yes | Stored on the resolved agent |
| `prompt` | Yes | Becomes the step session system prompt |
| `tools` | Yes | Passed through to the Copilot CLI as `--available-tools` when non-empty |
| `model` | Yes | Added as the agent-level model preference |

---

## `steps`

Defined in `pkg/workflow/types.go`, validated in `pkg/workflow/parser.go`, and executed in `pkg/executor/executor.go`.

### Fully active fields

| Field | Implemented | Exact behavior |
|---|---|---|
| `id` | Yes | Required and must be unique |
| `agent` | Yes | Must resolve to an agent name present after discovery and explicit loading |
| `prompt` | Yes | Required. Template resolution happens before sending to the executor |
| `depends_on` | Yes | Used to build the DAG. Validation rejects self-dependencies and unknown step IDs |
| `condition.step` | Yes | Must reference an upstream dependency, not just any step |
| `condition.contains` | Yes | Executes step only if referenced output contains the substring |
| `condition.not_contains` | Yes | Executes step only if referenced output does not contain the substring |
| `condition.equals` | Yes | Executes step only if trimmed output equals trimmed configured value |
| `model` | Yes | Step-level highest-priority model override |
| `extra_dirs` | Yes | Passed to the Copilot CLI as repeated `--add-dir` arguments |
| `interactive` | Yes | Per-step override. `nil` means inherit workflow-level `config.interactive` |

### Parsed but not used in the current runtime path

| Field | Implemented | Exact behavior |
|---|---|---|
| `skills` | Parsed only | Stored on the step struct but not consumed by the executor |
| `on_error` | Parsed only | No retry/alternate branch logic currently uses this field |
| `retry_count` | Parsed only | No retry loop currently exists in the step executor |
| `timeout` | Parsed only | The step executor does not derive a context timeout from this field |

### Condition evaluation details

The current evaluator checks operators in this order:

1. `contains`
2. `not_contains`
3. `equals`
4. no operator set -> condition evaluates `true`

That means only the first non-empty operator is used. Conditions are not currently combined with AND/OR logic.

There is no `not_equals` support in the current code.

---

## Template Variables

Implemented in `pkg/workflow/template.go`.

| Template | Implemented | Exact behavior |
|---|---|---|
| `{{steps.some-step.output}}` | Yes | Replaced with the completed output string from that step |
| `{{inputs.some_input}}` | Yes | Replaced with the merged runtime input value |

### Exact resolution behavior

1. Step output references are resolved first.
2. Input references are resolved second.
3. Missing step references return an error unless the step already exists in the results map.
4. Skipped steps are inserted into the results/output map as empty string by the orchestrator, so downstream references resolve to `""` instead of failing.

---

## `output`

Defined in `pkg/workflow/types.go`, formatted in `pkg/reporter/reporter.go`, and finalized into audit output in `pkg/audit/logger.go`.

| Field | Implemented | Exact behavior |
|---|---|---|
| `steps` | Yes | Controls which step IDs the reporter prints |
| `format` | Yes | `json`, `plain`, and `text` are explicit. Anything else falls back to markdown |
| `truncate` | Parsed only in normal runtime | Stored and passed around, but not currently applied during reporting or template injection |

### Exact `format` behavior

| Value | Behavior |
|---|---|
| `markdown` | Markdown-style report with `# Workflow Results` and `## Step:` sections |
| `json` | JSON object keyed by step ID, with status and output |
| `plain` | Plain text with `=== step ===` delimiters |
| `text` | Alias of `plain` |
| empty or unknown | Falls back to markdown |

### Exact `steps` behavior when omitted

There is an important nuance:

1. The stdout reporter includes all completed steps in alphabetical order.
2. The audit finalizer writes `final_output.md` using completed steps in workflow declaration order.

So if `output.steps` is omitted, stdout and `final_output.md` can differ in ordering.

---

## `truncate`

The truncation helper exists in `pkg/workflow/template.go`, but it is not currently called in the main `goflow run` path.

### What the helper would do

| Strategy | Exact helper behavior |
|---|---|
| `chars` | Keeps the first `limit` Unicode characters and appends a truncation note |
| `lines` | Keeps the first `limit` lines and appends a truncation note |
| `tokens` | Approximates 1 token as 4 characters, keeps the first `limit * 4` characters, and appends a truncation note |

### Why truncation exists conceptually

Without truncation, large step outputs can become extremely expensive or impossible to pass into later prompts because the next prompt includes full prior outputs via templates.

### Current status

Today, the field is useful as a forward-compatible declaration, but it does not change runtime output or prompt injection behavior unless the code path is updated to call `TruncateOutput()`.

---

## Agent File Frontmatter

Parsed in `pkg/agents/loader.go` and represented in `pkg/agents/types.go`.

### Actively used fields

| Field | Implemented | Exact behavior |
|---|---|---|
| `name` | Yes | Agent identity. Defaults to filename stem if omitted |
| `description` | Yes | Stored on the resolved agent |
| `tools` | Yes | Used to restrict Copilot CLI tools when non-empty |
| `model` | Yes | Accepts a string or list of strings. Used as ordered model preferences |
| Markdown body | Yes | Becomes the system prompt |
| `SourceFile` | Yes | Stored for audit metadata |

### Parsed and retained but not actively consumed by the executor

| Field | Implemented | Exact behavior |
|---|---|---|
| `agents` | Parsed only in runtime path | Preserved on the agent struct, but not used by the current executor |
| `mcp-servers` | Parsed only in runtime path | Preserved, but not currently passed into session config from the step executor |
| `handoffs` | Parsed only | Not used by the CLI runtime |
| `hooks` | Parsed only | Not used by the CLI runtime |
| `argument-hint` | Parsed only | Interactive/editor metadata, ignored in runtime |
| `user-invocable` | Parsed only | Interactive/editor metadata, ignored in runtime |
| `disable-model-invocation` | Parsed only | Interactive/editor metadata, ignored in runtime |
| `target` | Parsed only | Interactive/editor metadata, ignored in runtime |

### Claude compatibility

Files loaded from `.claude/agents/` are normalized through a Claude tool-name mapping layer in `pkg/agents/loader.go`.

---

## Agent Discovery

Implemented in `pkg/agents/discovery.go`.

Priority order is:

1. Explicit workflow agent references and inline agents
2. `.github/agents/`
3. `.claude/agents/`
4. `~/.copilot/agents/`
5. `config.agent_search_paths`

Relative explicit `file:` paths are resolved relative to the workflow file location, not the current shell directory.

---

## Shared Memory

The shared memory manager and tool specs exist in `pkg/memory/manager.go` and `pkg/memory/tools.go`.

### What is implemented in the package

| Feature | Implemented | Exact behavior |
|---|---|---|
| Persistent `memory.md` file | Yes | Created by `memory.NewManager(dir, initialContent)` |
| Thread-safe reads and writes | Yes | Uses mutex protection |
| Prompt injection helper | Yes | Prepends a shared-memory block before the prompt |
| Tool metadata | Yes | Tool specs exist for `read_memory` and `write_memory` |

### What is not wired into the normal CLI run path yet

| Feature | Current status |
|---|---|
| Automatic manager creation from `config.shared_memory` | Not wired |
| Automatic tool registration into step sessions | Not wired |
| Automatic prompt injection during `goflow run` | Not wired |

So the concept exists in code, but the main user-facing path still needs integration work.

---

## Parallel Execution And `max_concurrency`

### What exists in code

The orchestrator has:

- `Run()` for sequential execution
- `RunParallel()` for concurrent execution inside a DAG level
- a semaphore for `MaxConcurrency`

### What the CLI does today

The CLI in `cmd/workflow-runner/main.go` constructs the orchestrator and calls `orch.Run(ctx, wf)`.

That means:

1. DAG levels are still built correctly.
2. Dependencies are still respected correctly.
3. Steps in the same level are still executed one at a time in the current CLI path.
4. `config.max_concurrency` has no user-visible effect in normal `goflow run` today.

---

## Interactive Mode

Interactive behavior is implemented.

### Resolution order

The effective step interactivity is:

1. `step.interactive` if explicitly set
2. otherwise `config.interactive`
3. the CLI `--interactive` flag acts as the mechanism gate that wires the handler

### Important nuance

The CLI flag alone does not make every step interactive by default. It only ensures the user-input handler is available so steps that are marked interactive, or inherit `config.interactive: true`, can actually ask questions.

---

## Audit Settings

### `audit_dir`

- Default: `.workflow-runs`
- Used as the parent directory for run folders
- CLI `--audit-dir` overrides the workflow setting

### `audit_retention`

Retention is enforced by directory name sorting. Because run directories start with a timestamp, lexical order is chronological order.

Exact rule:

- `<= 0` -> keep everything
- `> 0` -> delete oldest run directories until only the newest `N` remain

---

## Recommended Documentation Reading Order

1. [Workflow Schema](workflow-schema.md) for the YAML shape
2. [Settings And Options](settings-and-options.md) for exact runtime behavior
3. [CLI Reference](cli.md) for commands and flags
4. [Output Control](output.md) for actual reporting behavior
5. [Shared Memory](shared-memory.md) for current implementation status

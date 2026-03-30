# CLI Reference

This page documents the commands and flags that are actually implemented in the current CLI.

---

## Build

```bash
go build -o goflow ./cmd/workflow-runner/main.go
```

---

## Implemented Commands

### `goflow run`

```bash
goflow run --workflow <path> [options]
```

#### Supported flags

| Flag | Required | Exact behavior |
|---|---|---|
| `--workflow` | Yes | Path to the workflow YAML file |
| `--inputs key=value` | No | Repeatable. CLI values override declared defaults; undeclared keys are still passed through |
| `--audit-dir` | No | Overrides `config.audit_dir` |
| `--mock` | No | Uses the mock executor and returns deterministic `mock output` |
| `--interactive` | No | Wires the user-input handler so interactive steps can ask for clarification |
| `--verbose` | No | Enables progress output (tool calls, session lifecycle) to stderr |
| `--stream` | No | Streams LLM output in real-time (token by token) to stderr |
| `--cli` | No | Uses the legacy CLI subprocess executor instead of the SDK |

#### Verbose Mode

When `--verbose` is enabled, goflow displays session lifecycle events:

```bash
goflow run --workflow pipeline.yaml --verbose
```

Output includes:
```
[step-id] Agent turn started
[step-id] Calling tool: grep_search
[step-id] Tool completed: grep_search
[step-id] Delegating to subagent: helper-agent
[step-id] Session completed
```

This provides visibility into long-running sessions without requiring timeout configuration.

#### Streaming LLM Output

When `--stream` is enabled, goflow displays the LLM's response as it generates token by token:

```bash
goflow run --workflow pipeline.yaml --stream
```

You can combine both flags for full visibility:

```bash
goflow run --workflow pipeline.yaml --verbose --stream
```

This shows both lifecycle events AND the actual LLM text as it streams.

#### Examples

```bash
goflow run --workflow examples/simple-sequential.yaml --mock --verbose

goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

### `goflow version`

```bash
goflow version
```

Prints:

```text
goflow <version>
commit: <sha>
built: <timestamp>
```

### `goflow help`

```bash
goflow help
```

Also available via `--help` and `-h`.

---

## Commands Not Currently Implemented

These are not available in the current CLI even if older docs mention them:

1. `goflow validate`
2. `goflow list`

---

## Input Semantics

Inputs are supplied as repeatable `key=value` pairs:

```bash
goflow run --workflow pipeline.yaml --inputs files='pkg/**/*.go' --inputs mode=review
```

Current merge rules:

1. declared workflow inputs are loaded first
2. CLI values override declared defaults
3. declared inputs with non-empty defaults are filled in automatically
4. undeclared CLI inputs are kept and passed through

---

## Interactive Mode

Interactive behavior is controlled by both CLI and workflow settings.

### Important nuance

`--interactive` does not automatically make every step interactive.

It enables the terminal input mechanism. Whether a step is allowed to ask questions is then resolved from:

1. `step.interactive`, if set
2. otherwise `config.interactive`

If neither of those is true, the step still runs non-interactively.

---

## Output Streams

### stdout

Final formatted workflow output.

### stderr

Usage messages, verbose progress, warnings, and execution errors.

Example:

```bash
goflow run --workflow pipeline.yaml --verbose > output.md 2> progress.log
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Any current CLI error path |

The CLI does not currently use a richer exit code taxonomy.

---

## Audit Override

Use `--audit-dir` if you want run artifacts somewhere other than the workflow's configured audit directory:

```bash
goflow run --workflow pipeline.yaml --audit-dir /tmp/goflow-runs
```

---

## See Also

- [Settings And Options](settings-and-options.md)
- [Workflow Schema](workflow-schema.md)

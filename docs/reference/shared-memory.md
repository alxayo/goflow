# Shared Memory

This page describes the shared-memory capability as it exists in the current source tree.

---

## Current Status

Shared memory support is **partially implemented**.

What exists today:

1. a thread-safe memory manager
2. a persisted `memory.md` file
3. helper methods for prompt injection
4. tool metadata definitions for memory read/write tools

What does not happen automatically in normal `goflow run` today:

1. no shared-memory manager is created from `config.shared_memory`
2. no memory tools are automatically registered into step sessions
3. no automatic prompt injection is performed during step execution

So shared memory is present as a building block in the code, but not yet fully wired into the user-facing CLI flow.

---

## Schema

The workflow config schema defines:

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true
    initial_content: "seed text"
    initial_file: "./memory.md"
```

### Field status

| Field | Parsed | Active in current CLI path |
|---|---|---|
| `enabled` | Yes | No |
| `inject_into_prompt` | Yes | No |
| `initial_content` | Yes | No |
| `initial_file` | Yes | No |

---

## Implemented Package Behavior

The memory manager in `pkg/memory/manager.go` supports:

### `NewManager(dir, initialContent)`

Creates `memory.md` inside the provided directory and writes the initial content if present.

### `Read()`

Returns the full in-memory content string.

### `Write(agentName, entry)`

Appends a timestamped line in this format:

```text
[2026-03-30T12:34:56Z] [agent-name] entry text
```

### `InjectIntoPrompt(prompt)`

Prepends a clearly delimited shared-memory block before the original prompt.

---

## Tool Definitions Present In Code

The helper package defines tool metadata for:

1. `read_memory`
2. `write_memory`

These names matter because they do not match older docs that referred to `shared_memory_read` and `shared_memory_write`.

In the current source tree, the defined tool spec names are:

```text
read_memory
write_memory
```

---

## Why Shared Memory Exists

The intended use case is cross-step coordination when multiple agents are running in the same workflow level.

Typical examples:

1. a security reviewer records a critical issue for other reviewers to consider
2. an analyzer writes context that later reviewers can consult
3. a coordinating agent publishes shared state across a wide fan-out

That design makes the most sense when the parallel orchestrator is active.

---

## Important Runtime Caveat

The normal CLI currently calls the sequential orchestrator path, not the parallel runner. So even before shared memory is fully wired in, many users will not yet see the kind of overlapping execution that makes shared memory most useful.

---

## See Also

- [Settings And Options](settings-and-options.md)
- [Architecture](architecture.md)

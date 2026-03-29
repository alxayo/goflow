# Shared Memory

Enable coordination between parallel steps using shared memory.

---

## Overview

When multiple steps run in parallel, they normally can't see each other's intermediate findings. Shared memory provides a lightweight mechanism for cross-step signaling:

- One step finds something → writes to shared memory
- Another parallel step → reads and reacts

---

## Configuration

Enable shared memory in the workflow config:

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true
```

### Config Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable shared memory |
| `inject_into_prompt` | bool | false | Auto-inject memory into prompts |

### Why inject_into_prompt: true?

LLMs often ignore optional tools. Setting `inject_into_prompt: true` forces the current memory state to be included in every prompt, ensuring agents always see it.

---

## Shared Memory Tools

Agents access shared memory via two tools:

### shared_memory_read

Read the current contents of shared memory.

**Input:** None
**Output:** Current memory contents (string)

### shared_memory_write

Write or append to shared memory.

**Input:**
- `content` (string) — Content to write
- `mode` (string) — `append` or `overwrite`

**Output:** Confirmation message

---

## Agent Configuration

Grant agents access to shared memory tools:

```yaml
agents:
  coordinator:
    inline:
      description: "Coordinates findings"
      prompt: "You coordinate reviews and share findings."
      tools:
        - shared_memory_read
        - shared_memory_write
```

Or in agent files:

```markdown
---
name: coordinator
description: Coordinates findings
tools:
  - shared_memory_read
  - shared_memory_write
  - grep
---
```

---

## Example: Coordinated Review

```yaml title="coordinated-review.yaml"
name: "coordinated-review"
description: "Parallel reviewers share critical findings"

config:
  shared_memory:
    enabled: true
    inject_into_prompt: true

agents:
  security:
    inline:
      description: "Security reviewer"
      prompt: |
        You review code for security issues.
        Write CRITICAL findings to shared memory immediately
        so other reviewers are aware.
      tools:
        - grep
        - read_file
        - shared_memory_write
  
  performance:
    inline:
      description: "Performance reviewer"
      prompt: |
        You review code for performance issues.
        Check shared memory for security findings that might
        relate to performance (e.g., crypto operations).
        Write critical performance issues to shared memory.
      tools:
        - grep
        - read_file
        - shared_memory_read
        - shared_memory_write
  
  aggregator:
    inline:
      description: "Combines reviews"
      prompt: "You combine all findings including shared discoveries."
      tools:
        - shared_memory_read

steps:
  - id: init-memory
    agent: aggregator
    prompt: "Initialize memory with: 'Review session started.'"

  - id: security-review
    agent: security
    prompt: "Review the code in {{inputs.files}} for security issues."
    depends_on: [init-memory]

  - id: perf-review
    agent: performance
    prompt: "Review the code in {{inputs.files}} for performance issues."
    depends_on: [init-memory]

  - id: final-summary
    agent: aggregator
    prompt: |
      Create a final summary combining:
      
      Security: {{steps.security-review.output}}
      
      Performance: {{steps.perf-review.output}}
      
      Shared findings (cross-cutting issues discovered by collaboration):
      [Read from shared memory]
    depends_on: [security-review, perf-review]

output:
  steps: [final-summary]
  format: markdown
```

---

## How It Works

### Memory Lifecycle

1. **Creation** — Memory is created when the workflow starts
2. **Updates** — Agents write during execution
3. **Reads** — Any agent can read current state
4. **Persistence** — Final state saved to `memory.md` in audit trail

### Parallel Access

- Multiple agents can write simultaneously
- Writes are atomic (no partial updates)
- Reads return the latest committed state
- No explicit locking needed

### With inject_into_prompt

When `inject_into_prompt: true`, each prompt is prefixed with:

```
## Shared Memory (Current State)
[Contents of shared memory]

---

[Original prompt]
```

---

## Memory Content Guidelines

### What to Write

- Critical findings that affect other reviewers
- Cross-cutting concerns discovered
- Blockers or dependencies found
- Summary checkpoints

### What NOT to Write

- Full step outputs (use `{{steps.X.output}}` instead)
- Large datasets
- Redundant information

### Format Suggestions

```
[CRITICAL] Security: Found SQL injection in auth.go:45
[WARNING] Performance: N+1 query in user_handler.go:123
[INFO] Both security and performance affected by logging.go
```

---

## Audit Trail

Shared memory state is saved in the audit trail:

```
.workflow-runs/2026-03-26T10-00-00_example/
├── memory.md           # Final shared memory state
├── workflow.meta.json
└── steps/
    └── ...
```

**memory.md example:**
```markdown
# Shared Memory

[CRITICAL] Security: SQL injection in auth.go:45
[WARNING] Performance: N+1 query in user_handler.go
[INFO] Cross-cutting: Logging affects both security and perf
```

---

## Best Practices

### 1. Use Clear Prefixes

```
[SECURITY] Found XSS vulnerability
[PERF] O(n²) algorithm detected
[BLOCKED] Can't proceed without API key
```

### 2. Keep It Concise

Shared memory should contain signals, not full reports:

```
# ✓ Good
[CRITICAL] auth.go:45 - SQL injection

# ✗ Bad
Found a SQL injection vulnerability in the authentication module.
The issue is located on line 45 of auth.go where user input is
directly concatenated into the SQL query string without proper
sanitization or parameterized queries...
```

### 3. Initialize Memory (Optional)

Start with context if helpful:

```yaml
- id: init
  prompt: |
    Initialize shared memory with:
    - Review target: {{inputs.files}}
    - Focus areas: Security, Performance
    - Empty findings list
```

### 4. Read Before Writing

Agents should check existing content before adding:

```yaml
prompt: |
  Check shared memory for related findings before starting.
  Add your discoveries, avoiding duplicates.
```

---

## Troubleshooting

### Memory Not Visible

**Problem:** Agents don't seem to see shared memory.

**Solutions:**
1. Enable `inject_into_prompt: true`
2. Verify agent has `shared_memory_read` tool
3. Check that previous write step completed

### Memory Too Large

**Problem:** Memory grows too large, affecting prompts.

**Solutions:**
1. Use `inject_into_prompt: false` and explicit reads
2. Periodically summarize and clear memory
3. Write only critical/relevant findings

### Race Conditions

**Problem:** Parallel writes might overwrite each other.

**Solution:** Use append mode preferentially:

```yaml
prompt: |
  Use shared_memory_write with mode='append' to add findings.
```

---

## See Also

- [Tutorial: Parallel Execution](../tutorial/parallel.md) — Parallel step basics
- [Architecture](architecture.md) — How shared memory is implemented
- [Agent Format](agent-format.md) — Granting tool access

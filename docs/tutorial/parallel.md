# Parallel Execution

Understand how the workflow DAG exposes parallel-ready structure and what the current CLI does with it.

---

## The Concept

In the previous tutorial, we built this pipeline:

```
analyze
   ↓
   ├─→ security-review ─┐
   │                    │
   └─→ perf-review ────→ summary
```

Both `security-review` and `perf-review` depend only on `analyze` — they don't depend on each other. So why wait for one to finish before starting the other?

!!! note "Current CLI behavior"
  `goflow run` executes DAG levels with parallel fan-out where dependencies allow. In levels with multiple sibling steps, failures are handled with best effort: sibling steps continue and failed step outputs resolve as empty strings for downstream fan-in templates.

---

## How It Works

goflow analyzes your `depends_on` declarations and builds a **DAG (Directed Acyclic Graph)** of dependencies. Steps that share the same dependencies (and don't depend on each other) are grouped into the same DAG level. That structure is what enables concurrent execution in the parallel orchestrator implementation.

### Parallel-Ready DAG Structure

You don't need to mark steps as parallel. You declare dependencies correctly, and the DAG builder groups independent steps together:

```yaml
steps:
  # Level 0: No dependencies
  - id: analyze
    prompt: "List the main components."

  # Level 1: Both depend on 'analyze', run in parallel
  - id: security-check
    prompt: "Review security..."
    depends_on: [analyze]

  - id: performance-check
    prompt: "Review performance..."
    depends_on: [analyze]

  - id: style-check
    prompt: "Review code style..."
    depends_on: [analyze]

  # Level 2: Depends on all Level 1 steps
  - id: summary
    prompt: "Summarize all findings..."
    depends_on: [security-check, performance-check, style-check]
```

### Execution Timeline

Without parallelism (sequential):
```
analyze → security-check → performance-check → style-check → summary
[5 steps in sequence]
```

With a parallel runner:
```
analyze →┬→ security-check ────┬→ summary
         ├→ performance-check ─┤
         └→ style-check ───────┘
[3 levels instead of 5]
```

---

## Fan-Out / Fan-In Pattern

This is the most common parallel pattern:

- **Fan-Out**: One step triggers multiple parallel steps
- **Fan-In**: Multiple parallel steps feed into one aggregating step

```yaml title="fan-out-fan-in.yaml"
name: "multi-reviewer"
description: "Multiple expert reviews running in parallel"

agents:
  security:
    inline:
      prompt: "You are a security expert."
  performance:
    inline:
      prompt: "You are a performance expert."
  accessibility:
    inline:
      prompt: "You are an accessibility expert."
  aggregator:
    inline:
      prompt: "You combine multiple reviews."

steps:
  # Single entry point
  - id: prepare
    agent: aggregator
    prompt: "Summarize what will be reviewed."

  # Fan-out: 3 parallel reviews
  - id: security-review
    agent: security
    prompt: "Review for security issues."
    depends_on: [prepare]

  - id: perf-review
    agent: performance
    prompt: "Review for performance issues."
    depends_on: [prepare]

  - id: a11y-review
    agent: accessibility
    prompt: "Review for accessibility issues."
    depends_on: [prepare]

  # Fan-in: Aggregate all reviews
  - id: final-report
    agent: aggregator
    prompt: |
      Combine these reviews:
      
      ## Security
      {{steps.security-review.output}}
      
      ## Performance  
      {{steps.perf-review.output}}
      
      ## Accessibility
      {{steps.a11y-review.output}}
    depends_on: [security-review, perf-review, a11y-review]

output:
  steps: [final-report]
  format: markdown
```

---

## Viewing Parallel Execution

Use `--verbose` to see workflow progress and DAG-driven step order:

```bash
goflow run --workflow fan-out-fan-in.yaml --mock --verbose
```

Conceptually, a parallel runner would treat those same-level steps as a batch:

```
[INFO] Loading workflow: fan-out-fan-in.yaml
[INFO] Starting workflow: multi-reviewer
[INFO] Level 0: prepare
[INFO] Level 1: security-review, perf-review, a11y-review  # ← Parallel!
[INFO] Level 2: final-report
[INFO] Workflow completed in 0.03s
```

---

## Shared Memory for Parallel Steps

Sometimes parallel steps need to coordinate or share findings. Use **shared memory**:

```yaml title="with-shared-memory.yaml"
name: "coordinated-review"

config:
  shared_memory:
    enabled: true
    inject_into_prompt: true

agents:
  reviewer:
    inline:
      prompt: "You are a code reviewer."
      tools: [read_memory, write_memory]

steps:
  - id: review-a
    agent: reviewer
    prompt: |
      Review module A.
      If you find critical issues, write them to shared memory 
      so other reviewers are aware.
    depends_on: []

  - id: review-b
    agent: reviewer
    prompt: |
      Review module B.
      Check shared memory for related issues found by other reviewers.
    depends_on: []
```

In the current codebase, shared-memory helpers exist, but automatic shared-memory wiring is not yet active in the main CLI path.

!!! tip "When to Use Shared Memory"
    Use shared memory when parallel steps might discover related issues (e.g., one step finds a security bug that affects another module).

See [Shared Memory Reference](../reference/shared-memory.md) for details.

---

## Performance Considerations

### Parallel Steps Don't Block Each Other

If one parallel step takes longer, others complete independently:

```
security-review (2 min) ────────┐
perf-review (30 sec) ──────┐   │
style-check (30 sec) ──────┴───┴→ summary
                               ↑
                               Waits for ALL to complete
```

### Model Rate Limits

If you're running many parallel steps with real AI calls, you might hit rate limits. goflow doesn't have built-in rate limiting yet, so consider:

- The parallelism is concurrent but still goes through a single CLI
- Very wide fan-outs (10+ parallel steps) may need manual throttling

---

## Anti-Patterns

### Unnecessary Dependencies

```yaml
# ✗ Bad: Forces sequential when parallel is possible
- id: review-a
  depends_on: []

- id: review-b
  depends_on: [review-a]  # Does review-b REALLY need review-a's output?

- id: review-c
  depends_on: [review-b]  # Same question
```

### Circular Dependencies

goflow detects and rejects circular dependencies:

```yaml
# ✗ Error: Circular dependency
- id: step-a
  depends_on: [step-b]

- id: step-b
  depends_on: [step-a]
```

Error message:
```
Error: circular dependency detected in workflow DAG
```

---

## What You Learned

:white_check_mark: goflow automatically parallelizes independent steps  
:white_check_mark: Use `depends_on` to declare true dependencies only  
:white_check_mark: Fan-out/fan-in pattern for multi-expert workflows  
:white_check_mark: Shared memory enables coordination between parallel steps  

---

## Next Steps

Now let's learn how to branch based on step outputs:

**[Conditional Logic →](conditions.md)**

# Simple Sequential Workflow

A basic workflow demonstrating sequential step execution with data passing.

---

## Overview

This example shows:

- Basic workflow structure
- Sequential step dependencies
- Passing data between steps with `{{steps.X.output}}`
- Simple inline agents

---

## The Workflow

```yaml title="examples/simple-sequential.yaml"
name: "simple-sequential"
description: "Sequential pipeline with three review stages"

inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "src/**/*.go"

agents:
  security-reviewer:
    inline:
      description: "Reviews code for security issues"
      prompt: |
        You are a security expert. Analyze code for vulnerabilities.
        Focus on injection, authentication, and data exposure issues.
        Be concise and cite specific locations.

  performance-reviewer:
    inline:
      description: "Reviews code for performance issues"
      prompt: |
        You are a performance expert. Analyze code for efficiency.
        Focus on algorithmic complexity, memory usage, and I/O.
        Be concise and cite specific locations.

  aggregator:
    inline:
      description: "Combines multiple reviews"
      prompt: |
        You summarize multiple code reviews into a single report.
        Prioritize findings by severity and actionability.

steps:
  # Step 1: Security review
  - id: security-review
    agent: security-reviewer
    prompt: "Review the code in {{inputs.files}} for security issues."

  # Step 2: Performance review (waits for security review)
  - id: perf-review
    agent: performance-reviewer
    prompt: |
      Review the code in {{inputs.files}} for performance issues.
      
      For context, the security review found:
      {{steps.security-review.output}}
    depends_on: [security-review]

  # Step 3: Summary (waits for performance review)
  - id: summary
    agent: aggregator
    prompt: |
      Create an executive summary combining:
      
      ## Security Findings
      {{steps.security-review.output}}
      
      ## Performance Findings
      {{steps.perf-review.output}}
      
      Provide: top priorities, quick wins, and recommendations.
    depends_on: [perf-review]

output:
  steps: [summary]
  format: markdown
```

---

## How It Works

```
security-review → perf-review → summary
```

1. **security-review** runs first (no dependencies)
2. **perf-review** waits for security-review, then runs with its output as context
3. **summary** waits for perf-review, then combines both outputs

---

## Running the Example

### Mock Mode (No API Calls)

```bash
goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/**/*.go' \
  --mock \
  --verbose
```

**Expected output:**
```
[INFO] Loading workflow: examples/simple-sequential.yaml
[INFO] Starting workflow: simple-sequential
[INFO] Step 1/3: security-review (security-reviewer)
[INFO] Step 2/3: perf-review (performance-reviewer)
[INFO] Step 3/3: summary (aggregator)
[INFO] Workflow completed

## Step: summary

mock output
```

### Real Mode

```bash
goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

---

## Key Concepts Demonstrated

### 1. Input Parameters

```yaml
inputs:
  files:
    description: "Files to review"
    default: "src/**/*.go"
```

Override at runtime: `--inputs files='other/*.go'`

### 2. Step Dependencies

```yaml
- id: perf-review
  depends_on: [security-review]  # Must wait
```

### 3. Template Variables

```yaml
prompt: |
  Context from security:
  {{steps.security-review.output}}
```

Injects the actual output from the referenced step.

---

## Variations

### Adding More Stages

```yaml
steps:
  - id: security-review
    ...
  
  - id: perf-review
    depends_on: [security-review]
  
  - id: style-review  # New step
    agent: style-reviewer
    depends_on: [perf-review]
  
  - id: summary
    depends_on: [style-review]  # Now waits for style
```

### Using File-Based Agents

Replace inline agents with files:

```yaml
agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"
```

---

## See Also

- [Code Review Pipeline](code-review.md) — Same concept with parallel execution
- [Tutorial: Multi-Step Pipelines](../tutorial/multi-step.md) — Detailed explanation

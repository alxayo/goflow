# Code Review Pipeline

A code review workflow that models a fan-out and fan-in structure across multiple expert reviewers.

---

## Overview

This example demonstrates:

- **Fan-out/fan-in** pattern — multiple reviewers represented as independent DAG branches
- **External agent files** — reusable `.agent.md` definitions
- **Expert aggregation** — combining multiple perspectives
- **Input configuration** — customizable file targets

---

## The Workflow

```yaml title="examples/code-review-pipeline.yaml"
name: "code-review-pipeline"
description: "Multi-expert code review with security, performance, and style analysis"

inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "src/**/*.go"
  depth:
    description: "Review depth: quick, normal, detailed"
    default: "normal"

config:
  audit_retention: 5
  truncate:
    strategy: lines
    limit: 100

agents:
  analyzer:
    inline:
      description: "Understands code structure"
      prompt: |
        You analyze code architecture and identify key components.
        Extract: main functions, data flows, dependencies.
      tools: [grep, read_file, semantic_search]

  security-expert:
    file: "./agents/security-reviewer.agent.md"

  performance-expert:
    file: "./agents/performance-reviewer.agent.md"

  aggregator:
    file: "./agents/aggregator.agent.md"

steps:
  # Phase 1: Initial analysis
  - id: analyze
    agent: analyzer
    prompt: |
      Analyze the code structure in {{inputs.files}}.
      Depth: {{inputs.depth}}
      
      Identify:
      - Main entry points
      - Key functions and their responsibilities  
      - Data flow patterns
      - External dependencies

  # Phase 2: Parallel expert reviews
  - id: security-review
    agent: security-expert
    prompt: |
      Perform a security review of this code:
      
      {{steps.analyze.output}}
      
      Focus on:
      - Injection vulnerabilities
      - Authentication issues
      - Data exposure risks
    depends_on: [analyze]

  - id: performance-review
    agent: performance-expert
    prompt: |
      Perform a performance review of this code:
      
      {{steps.analyze.output}}
      
      Focus on:
      - Algorithmic complexity
      - Memory allocation patterns
      - I/O efficiency
    depends_on: [analyze]

  # Phase 3: Aggregation
  - id: aggregate
    agent: aggregator
    prompt: |
      Create a comprehensive code review report:
      
      ## Code Analysis
      {{steps.analyze.output}}
      
      ## Security Review
      {{steps.security-review.output}}
      
      ## Performance Review
      {{steps.performance-review.output}}
      
      Synthesize into:
      1. Executive summary (2-3 sentences)
      2. Critical issues (must fix)
      3. Recommendations (should fix)
      4. Positive observations
    depends_on: [security-review, performance-review]

output:
  steps: [aggregate]
  format: markdown
```

---

## Execution Pattern

```
        ┌─→ security-review ──┐
analyze─┤                     ├─→ aggregate
        └─→ performance-review┘
```

**Phase 1:** `analyze` runs alone  
**Phase 2:** `security-review` and `performance-review` are independent DAG branches  
**Phase 3:** `aggregate` waits for both, then combines  

!!! note "Current CLI behavior"
  The workflow runs with parallel fan-out on same-level steps. If one parallel branch fails, execution continues in best-effort mode and fan-in can still proceed with empty output for the failed branch.

---

## Supporting Agent Files

### security-reviewer.agent.md

```markdown title="agents/security-reviewer.agent.md"
---
name: security-reviewer
description: Expert security code reviewer
tools:
  - grep
  - semantic_search
  - read_file
model: gpt-4o
---

# Security Reviewer

You are an expert security code reviewer with 15 years of experience.

## Focus Areas

1. **Injection attacks** - SQL, command, XSS, template injection
2. **Authentication** - Password handling, session management, tokens
3. **Authorization** - Access control, privilege escalation, IDOR
4. **Data protection** - Encryption, secrets management, PII exposure

## Severity Levels

- 🔴 **CRITICAL** — Exploitable, must fix immediately
- 🟠 **HIGH** — Significant risk, fix before release
- 🟡 **MEDIUM** — Moderate risk, fix soon
- 🟢 **LOW** — Minor issue, nice to fix

## Output Format

For each finding:
```
[SEVERITY] Category: Brief description
  Location: file:line
  Issue: What's wrong
  Impact: What could happen
  Fix: How to remediate
```
```

### performance-reviewer.agent.md

```markdown title="agents/performance-reviewer.agent.md"
---
name: performance-reviewer
description: Expert performance code reviewer
tools:
  - grep
  - semantic_search
  - read_file
model: gpt-4o
---

# Performance Reviewer

You are an expert performance engineer focused on code efficiency.

## Focus Areas

1. **Algorithmic complexity** - O(n²) or worse patterns
2. **Memory management** - Leaks, excessive allocation, buffer sizing
3. **I/O patterns** - N+1 queries, unbatched operations, blocking calls
4. **Caching** - Missing opportunities, cache invalidation issues

## Output Format

For each finding:
```
[Impact: HIGH/MEDIUM/LOW] Category
  Location: file:line  
  Issue: Current implementation
  Improvement: Suggested optimization
  Estimated gain: Qualitative improvement
```
```

---

## Running the Example

### Full Run

```bash
goflow run \
  --workflow examples/code-review-pipeline.yaml \
  --inputs files='pkg/workflow/*.go' \
  --inputs depth='detailed' \
  --verbose
```

### Mock Mode (Structure Test)

```bash
goflow run \
  --workflow examples/code-review-pipeline.yaml \
  --inputs files='pkg/**/*.go' \
  --mock \
  --verbose
```

### Single Step (Debugging)

```bash
goflow run \
  --workflow examples/code-review-pipeline.yaml \
  --step security-review \
  --mock
```

---

## Key Patterns

### 1. Parallel-Ready Structure

Both expert reviews depend only on `analyze`, so they land in the same DAG level and are eligible for concurrent execution in the parallel orchestrator implementation:

```yaml
- id: security-review
  depends_on: [analyze]  # Only depends on analyze

- id: performance-review
  depends_on: [analyze]  # Same — runs in parallel with security
```

### 2. Fan-In Aggregation

The aggregation step waits for ALL parallel steps:

```yaml
- id: aggregate
  depends_on: [security-review, performance-review]  # Both must complete
```

### 3. Context Passing

Each step receives relevant context via templates:

```yaml
prompt: |
  ## Security Review
  {{steps.security-review.output}}
  
  ## Performance Review
  {{steps.performance-review.output}}
```

---

## Customization Ideas

### Add Style Review

```yaml
- id: style-review
  agent: style-expert
  depends_on: [analyze]  # Parallel with security/performance

- id: aggregate
  depends_on: [security-review, performance-review, style-review]
```

### Add Conditional Deep Dive

```yaml
- id: deep-security
  agent: security-expert
  prompt: "Deep dive on critical issues: {{steps.security-review.output}}"
  depends_on: [security-review]
  condition:
    step: security-review
    contains: "CRITICAL"
```

---

## See Also

- [Simple Sequential](simple-sequential.md) — Simpler version without parallelism
- [Tutorial: Parallel Execution](../tutorial/parallel.md) — Detailed explanation

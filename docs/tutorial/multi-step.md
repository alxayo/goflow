# Multi-Step Pipelines

Chain multiple steps together to build complex workflows.

---

## The Concept

Real workflows need multiple steps where each step builds on previous results:

```
Step A: Analyze code
    ↓
Step B: Review security (uses analysis from A)
    ↓
Step C: Review performance (uses analysis from A)
    ↓
Step D: Summarize findings (uses B and C)
```

goflow handles this with two features:

1. **`depends_on`** — Control when steps run
2. **`{{steps.X.output}}`** — Pass data between steps

---

## Step Dependencies

Use `depends_on` to declare that a step needs another step to complete first:

```yaml title="multi-step.yaml" hl_lines="13-14 18-19"
name: "multi-step"
description: "A workflow with dependencies"

agents:
  analyzer:
    inline:
      prompt: "You analyze code structure."

steps:
  - id: analyze
    agent: analyzer
    prompt: "List the main functions in the code."

  - id: review
    agent: analyzer
    prompt: "Review these functions: {{steps.analyze.output}}"
    depends_on: [analyze]  # Wait for 'analyze' to finish

  - id: summary
    agent: analyzer
    prompt: "Summarize: {{steps.review.output}}"
    depends_on: [review]  # Wait for 'review' to finish

output:
  steps: [summary]
  format: markdown
```

### What Happens

1. **analyze** runs first (no dependencies)
2. **review** waits for analyze, then runs with analyze's output
3. **summary** waits for review, then runs with review's output

---

## Passing Data Between Steps

Use `{{steps.STEP_ID.output}}` to inject a previous step's output into a prompt:

```yaml
steps:
  - id: first
    prompt: "Generate a list of topics."

  - id: second
    prompt: "Expand on each topic: {{steps.first.output}}"
    depends_on: [first]
```

### How It Works

1. **Step `first`** runs and produces output (e.g., "1. AI, 2. Robotics, 3. Space")
2. Before **step `second`** runs, goflow replaces `{{steps.first.output}}` with the actual output
3. **Step `second`** receives the prompt: "Expand on each topic: 1. AI, 2. Robotics, 3. Space"

---

## Complete Example: Code Review Pipeline

```yaml title="code-review-pipeline.yaml"
name: "code-review-pipeline"
description: "Analyzes code, then reviews security and performance"

inputs:
  files:
    description: "Files to review"
    default: "src/*.go"

agents:
  analyzer:
    inline:
      description: "Understands code structure"
      prompt: "You analyze code architecture and structure."
  
  security-expert:
    inline:
      description: "Security specialist"
      prompt: "You are a security expert. Focus on vulnerabilities."
  
  performance-expert:
    inline:
      description: "Performance specialist"  
      prompt: "You are a performance expert. Focus on optimization."
  
  aggregator:
    inline:
      description: "Combines reviews"
      prompt: "You summarize multiple reviews into actionable items."

steps:
  # Step 1: Analyze the code structure
  - id: analyze
    agent: analyzer
    prompt: |
      Analyze the code in {{inputs.files}}.
      List the main components, functions, and data flow.

  # Step 2: Security review (uses analysis)
  - id: security-review
    agent: security-expert
    prompt: |
      Review this code for security issues:
      
      {{steps.analyze.output}}
      
      Focus on: authentication, input validation, data exposure.
    depends_on: [analyze]

  # Step 3: Performance review (uses analysis)
  - id: perf-review
    agent: performance-expert
    prompt: |
      Review this code for performance issues:
      
      {{steps.analyze.output}}
      
      Focus on: algorithmic complexity, memory usage, I/O patterns.
    depends_on: [analyze]

  # Step 4: Combine reviews (waits for both)
  - id: summary
    agent: aggregator
    prompt: |
      Combine these reviews into an executive summary:
      
      ## Security Review
      {{steps.security-review.output}}
      
      ## Performance Review
      {{steps.perf-review.output}}
      
      Provide: top 3 priorities, quick wins, and long-term recommendations.
    depends_on: [security-review, perf-review]

output:
  steps: [summary]
  format: markdown
```

### Execution Order

```
analyze
   ↓
   ├─→ security-review ─→┐
   │                     │
   └─→ perf-review ─────→ summary
```

!!! info "Parallel Execution Preview"
    Notice that `security-review` and `perf-review` both depend only on `analyze`. They could run **in parallel**! We'll cover this in the [Parallel Execution](parallel.md) tutorial.

### Try It

```bash
goflow run --workflow code-review-pipeline.yaml \
  --inputs files='pkg/workflow/*.go' \
  --mock \
  --verbose
```

---

## Template Best Practices

### Use Multi-line Prompts for Clarity

```yaml
prompt: |
  Context from previous analysis:
  
  {{steps.analyze.output}}
  
  Now, identify potential issues.
```

### Reference Multiple Steps

```yaml
prompt: |
  Review 1: {{steps.review-a.output}}
  
  Review 2: {{steps.review-b.output}}
  
  Compare and synthesize these reviews.
depends_on: [review-a, review-b]
```

### Keep Prompts Focused

Each step should do **one thing well**:

```yaml
# ✓ Good: Focused steps
- id: identify-issues
  prompt: "List all issues found."

- id: prioritize-issues  
  prompt: "Prioritize: {{steps.identify-issues.output}}"
  depends_on: [identify-issues]

# ✗ Bad: One step doing too much
- id: do-everything
  prompt: "Identify issues, prioritize them, and write a report."
```

---

## Output Truncation

When passing step outputs to later steps, large outputs can exceed the model's context window. Configure truncation in the `config` section:

```yaml
config:
  truncate:
    strategy: "lines"
    limit: 100  # Keep only last 100 lines
```

Or per-step:

```yaml
output:
  truncate:
    strategy: "chars"
    limit: 5000  # Keep only last 5000 characters
```

Strategies:
- `chars` — Truncate to character count
- `lines` — Truncate to line count

---

## What You Learned

:white_check_mark: How to use `depends_on` to control step order  
:white_check_mark: How to use `{{steps.X.output}}` to pass data between steps  
:white_check_mark: How to build multi-step pipelines  
:white_check_mark: How to handle output truncation  

---

## Next Steps

Now let's make those independent steps run at the same time:

**[Parallel Execution →](parallel.md)**

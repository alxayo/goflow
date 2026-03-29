# Template Variables

Reference for `{{inputs.X}}` and `{{steps.Y.output}}` template syntax.

---

## Overview

Templates are placeholders in your workflow that get replaced with actual values at runtime:

```yaml
prompt: "Analyze {{inputs.files}} and compare to {{steps.baseline.output}}"
#              ↑                           ↑
#        Input template              Step output template
```

---

## Input Templates

Reference workflow inputs defined in the `inputs` section.

### Syntax

```
{{inputs.INPUT_NAME}}
```

### Example

```yaml
inputs:
  files:
    description: "Files to analyze"
    default: "*.go"
  mode:
    description: "Analysis mode"

steps:
  - id: analyze
    prompt: "Analyze {{inputs.files}} in {{inputs.mode}} mode"
```

**Run:**
```bash
goflow run -w example.yaml --inputs files='src/*.go' --inputs mode='detailed'
```

**Resolved prompt:**
```
Analyze src/*.go in detailed mode
```

---

## Step Output Templates

Reference the output from a previously completed step.

### Syntax

```
{{steps.STEP_ID.output}}
```

### Example

```yaml
steps:
  - id: analyze
    prompt: "List the main functions in the code."

  - id: review
    prompt: |
      Review these functions:
      
      {{steps.analyze.output}}
    depends_on: [analyze]
```

**What happens:**
1. `analyze` runs and produces output (e.g., "Found: main(), init(), handleRequest()")
2. `review` runs with the prompt: "Review these functions:\n\nFound: main(), init(), handleRequest()"

---

## Template Locations

Templates work in these locations:

| Location | Example |
|----------|---------|
| Step prompts | `prompt: "Review {{inputs.files}}"` |
| Agent inline prompts | `prompt: "You specialize in {{inputs.language}}"` |
| Condition values | `contains: "{{inputs.keyword}}"` |

### Examples

**Step prompt:**
```yaml
steps:
  - id: review
    prompt: "Review {{inputs.files}}: {{steps.analyze.output}}"
```

**Inline agent prompt:**
```yaml
agents:
  specialist:
    inline:
      prompt: "You are an expert in {{inputs.language}} development."
```

**Condition:**
```yaml
condition:
  step: classify
  contains: "{{inputs.expected_type}}"
```

---

## Resolution Rules

### Order of Resolution

1. **Input templates** are resolved first
2. **Step output templates** are resolved based on dependency order
3. Templates are resolved **before** sending to the AI

### Missing Values

| Scenario | Behavior |
|----------|----------|
| Missing input (no default) | Error: "missing required input: X" |
| Missing input (has default) | Uses default value |
| Missing step output | Empty string (step was skipped or not yet run) |
| Invalid template syntax | Kept as literal text |

### Whitespace

Templates preserve surrounding whitespace:

```yaml
prompt: |
  Before:
  {{steps.analyze.output}}
  After:
```

If `analyze.output` is "Hello", the resolved prompt is:
```
Before:
Hello
After:
```

---

## Multi-line Outputs

Step outputs often contain multiple lines. Use YAML multi-line syntax:

```yaml
prompt: |
  Review this analysis:
  
  {{steps.analyze.output}}
  
  Focus on the key findings.
```

The `|` preserves newlines in the prompt and in the injected output.

---

## Multiple Step References

Reference multiple steps in one prompt:

```yaml
- id: summary
  prompt: |
    Combine these reviews:
    
    ## Security Review
    {{steps.security.output}}
    
    ## Performance Review
    {{steps.performance.output}}
    
    ## Accessibility Review
    {{steps.accessibility.output}}
  depends_on: [security, performance, accessibility]
```

---

## Truncation

Large step outputs can exceed model context limits. Configure truncation:

### Global Truncation

```yaml
config:
  truncate:
    strategy: "lines"
    limit: 100
```

### Output-Specific Truncation

```yaml
output:
  steps: [summary]
  truncate:
    strategy: "chars"
    limit: 5000
```

### Strategies

| Strategy | Description |
|----------|-------------|
| `lines` | Keep last N lines |
| `chars` | Keep last N characters |

**Example:** If `steps.analyze.output` is 500 lines and truncation is `lines: 100`, only the last 100 lines are injected.

---

## Handling Skipped Steps

When a step is skipped due to a condition, its output is empty:

```yaml
steps:
  - id: deep-dive
    depends_on: [initial]
    condition:
      step: initial
      contains: "CRITICAL"

  - id: summary
    prompt: |
      Initial review: {{steps.initial.output}}
      Deep dive (if performed): {{steps.deep-dive.output}}
    depends_on: [initial, deep-dive]
```

If `deep-dive` is skipped:
```
Initial review: [actual content]
Deep dive (if performed): 
```

**Tip:** Structure prompts to handle empty values gracefully:
```yaml
prompt: |
  Review findings: {{steps.review.output}}
  Additional findings (if any): {{steps.optional-review.output}}
```

---

## Error Handling

### Invalid Template Syntax

```yaml
prompt: "Hello {{invalid syntax}}"  # Missing dots
```

Invalid templates are kept as literal text:
```
Hello {{invalid syntax}}
```

### Circular References

goflow's dependency system prevents circular references:

```yaml
# ✗ Error: Circular dependency
- id: step-a
  prompt: "{{steps.step-b.output}}"
  depends_on: [step-b]

- id: step-b
  prompt: "{{steps.step-a.output}}"
  depends_on: [step-a]
```

Error:
```
circular dependency detected in workflow DAG
```

### Reference Before Completion

You can only reference a step if it's in your `depends_on`:

```yaml
# ✗ Bad: no dependency declared
- id: step-a
  prompt: "Task A"

- id: step-b
  prompt: "Use {{steps.step-a.output}}"  # No depends_on!
```

The template might reference an incomplete or empty output. Always declare dependencies:

```yaml
# ✓ Good: dependency declared
- id: step-b
  prompt: "Use {{steps.step-a.output}}"
  depends_on: [step-a]
```

---

## Best Practices

### 1. Use Clear Labels

```yaml
prompt: |
  ## Security Analysis
  {{steps.security-review.output}}
  
  ## Performance Analysis
  {{steps.perf-review.output}}
```

### 2. Handle Optional Content

```yaml
prompt: |
  Core findings: {{steps.main-review.output}}
  
  Extended analysis (if available):
  {{steps.extended-review.output}}
```

### 3. Keep Templates Simple

```yaml
# ✓ Good: Simple references
prompt: "Review: {{steps.analyze.output}}"

# ✗ Avoid: Complex expressions (not supported)
prompt: "Review: {{steps.analyze.output | uppercase}}"
```

### 4. Document Inputs

```yaml
inputs:
  files:
    description: "Files to analyze (glob pattern, e.g., 'src/**/*.go')"
    default: "*.go"
```

---

## See Also

- [Workflow Schema](workflow-schema.md) — Full YAML reference
- [Output Control](output.md) — Truncation settings
- [Tutorial: Adding Inputs](../tutorial/inputs.md) — Input tutorial

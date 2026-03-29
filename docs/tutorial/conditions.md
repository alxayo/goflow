# Conditional Logic

Skip or branch steps based on previous outputs.

---

## The Concept

Sometimes you want to run a step only if a condition is met:

- Run deployment only if tests pass
- Skip detailed review if initial scan finds no issues
- Choose different next steps based on classification

goflow supports conditions using the `condition` field.

---

## Basic Condition Syntax

Add a `condition` to any step:

```yaml
steps:
  - id: scan
    prompt: "Scan for security issues. Say 'ISSUES FOUND' if any exist."

  - id: detailed-review
    prompt: "Do a detailed security review."
    depends_on: [scan]
    condition:
      step: scan
      contains: "ISSUES FOUND"  # Only runs if scan output contains this
```

### Condition Fields

| Field | Description |
|-------|-------------|
| `step` | Which step's output to check |
| `contains` | Run if output contains this substring |
| `not_contains` | Run if output does NOT contain this substring |
| `equals` | Run if output exactly equals this string (after trimming) |
| `not_equals` | Run if output does NOT equal this string |

---

## Example: Conditional Deployment

```yaml title="conditional-deploy.yaml"
name: "conditional-deploy"
description: "Only deploy if tests pass"

agents:
  tester:
    inline:
      prompt: "You run tests and report results."
  deployer:
    inline:
      prompt: "You handle deployments."

steps:
  - id: run-tests
    agent: tester
    prompt: |
      Run tests on the code.
      If all tests pass, say "ALL TESTS PASSED".
      If any tests fail, say "TESTS FAILED" and list the failures.

  - id: deploy
    agent: deployer
    prompt: "Deploy the application to staging."
    depends_on: [run-tests]
    condition:
      step: run-tests
      contains: "ALL TESTS PASSED"

  - id: notify-failure
    agent: tester
    prompt: "Create a failure report explaining what went wrong."
    depends_on: [run-tests]
    condition:
      step: run-tests
      contains: "TESTS FAILED"

output:
  steps: [deploy, notify-failure]
  format: markdown
```

### What Happens

- If tests pass: `deploy` runs, `notify-failure` is skipped
- If tests fail: `notify-failure` runs, `deploy` is skipped

---

## Multiple Conditions (AND)

All conditions must be true (implicit AND):

```yaml
condition:
  step: review
  contains: "APPROVED"
  not_contains: "CONCERNS"  # Must be approved WITHOUT concerns
```

---

## Branching Patterns

### Pattern 1: Gate Step

Use one step to decide if another should run:

```yaml
steps:
  - id: check
    prompt: "Is this code ready for review? Say YES or NO."

  - id: review
    prompt: "Perform detailed code review."
    depends_on: [check]
    condition:
      step: check
      contains: "YES"
```

### Pattern 2: Classification Branch

Route to different steps based on classification:

```yaml
steps:
  - id: classify
    prompt: |
      Classify this issue as:
      - BUG: Code defect
      - FEATURE: New functionality
      - DOCS: Documentation update
      
      Reply with just the category name.

  - id: handle-bug
    prompt: "Investigate the bug and suggest a fix."
    depends_on: [classify]
    condition:
      step: classify
      contains: "BUG"

  - id: handle-feature
    prompt: "Design the feature implementation."
    depends_on: [classify]
    condition:
      step: classify
      contains: "FEATURE"

  - id: handle-docs
    prompt: "Draft the documentation update."
    depends_on: [classify]
    condition:
      step: classify
      contains: "DOCS"
```

### Pattern 3: Escalation

Run additional steps only when needed:

```yaml
steps:
  - id: initial-review
    prompt: |
      Review the code briefly.
      If you find CRITICAL issues, include the word "CRITICAL".

  - id: deep-dive
    prompt: "Do a thorough security audit of this code."
    depends_on: [initial-review]
    condition:
      step: initial-review
      contains: "CRITICAL"  # Only if critical issues found

  - id: summary
    prompt: |
      Summarize the review:
      Initial: {{steps.initial-review.output}}
      Deep dive: {{steps.deep-dive.output}}
    depends_on: [initial-review, deep-dive]
```

---

## Skipped Steps in Templates

When a conditional step is skipped, its output is empty. Reference it carefully:

```yaml
- id: summary
  prompt: |
    Initial review: {{steps.initial-review.output}}
    
    {% if steps.deep-dive.output %}
    Deep dive findings: {{steps.deep-dive.output}}
    {% else %}
    (Deep dive was not needed)
    {% endif %}
```

!!! note "Template Conditionals"
    goflow doesn't support Jinja-style `{% if %}` blocks yet. Instead, structure your prompts to handle empty values gracefully:
    ```yaml
    prompt: |
      Review: {{steps.initial-review.output}}
      Additional findings (if any): {{steps.deep-dive.output}}
    ```

---

## Condition Debugging

### See Which Steps Were Skipped

Use `--verbose`:

```bash
goflow run --workflow conditional-deploy.yaml --mock --verbose
```

```
[INFO] Step: run-tests (completed)
[INFO] Step: deploy (SKIPPED: condition not met)
[INFO] Step: notify-failure (completed)
```

### Check Condition Evaluation

The audit trail shows why a step was skipped:

```bash
cat .workflow-runs/*/steps/01_deploy/step.meta.json
```

```json
{
  "status": "skipped",
  "skip_reason": "condition not met: step 'run-tests' does not contain 'ALL TESTS PASSED'"
}
```

---

## Best Practices

### 1. Use Clear Marker Words

Make the AI output specific, recognizable markers:

```yaml
# ✓ Good: Clear markers
prompt: "If approved, say APPROVED. If rejected, say REJECTED."

# ✗ Bad: Ambiguous output
prompt: "Decide if this should be approved."
```

### 2. Handle the Default Case

If no conditions match, you might have silent failures. Consider adding a fallback:

```yaml
- id: fallback
  prompt: "Handle unexpected case."
  depends_on: [classify]
  condition:
    step: classify
    not_contains: "BUG"
    not_contains: "FEATURE"
    not_contains: "DOCS"
```

### 3. Keep Conditions Simple

Complex branching is hard to debug. Prefer simple contains checks:

```yaml
# ✓ Good: Simple
condition:
  step: review
  contains: "APPROVE"

# ✗ Avoid: Complex
# (Not supported yet, but even if it were, avoid)
condition:
  or:
    - step: review
      contains: "APPROVE"
    - step: review
      contains: "OK"
```

---

## What You Learned

:white_check_mark: How to use `condition` to skip steps  
:white_check_mark: `contains`, `not_contains`, `equals`, `not_equals` operators  
:white_check_mark: Branching patterns: gate, classification, escalation  
:white_check_mark: How to debug skipped steps  

---

## Next Steps

Now let's organize agents into reusable files:

**[Agent Files →](agent-files.md)**

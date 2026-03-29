# Output Control

This page describes what the current code actually does with the `output` section.

---

## Output Section Shape

```yaml
output:
  steps: [summary]
  format: markdown
  truncate:
    strategy: chars
    limit: 5000
```

Fields are defined in `pkg/workflow/types.go`, formatted in `pkg/reporter/reporter.go`, and finalized into audit files in `pkg/audit/logger.go`.

---

## `steps`

`output.steps` controls which step results are included in the formatted stdout output.

Example:

```yaml
output:
  steps: [security-review, summary]
```

### Exact behavior

1. Each listed step ID is looked up in the results map.
2. Missing IDs are silently skipped.
3. Skipped workflow steps are rendered as skipped markers in markdown and plain output.

### When `output.steps` is omitted

The behavior differs slightly between stdout output and audit output:

1. The reporter includes all completed steps in alphabetical order.
2. The audit finalizer writes `final_output.md` using completed steps in workflow declaration order.

So omitting `output.steps` can produce a different order in stdout vs `final_output.md`.

---

## `format`

The current reporter supports these values:

| Value | Exact behavior |
|---|---|
| `markdown` | Renders `# Workflow Results` and one `## Step:` section per step |
| `json` | Renders a JSON object with step status and output |
| `plain` | Renders step outputs with `=== step ===` separators |
| `text` | Alias of `plain` |
| empty or unknown | Falls back to `markdown` |

### Markdown example

```markdown
# Workflow Results

## Step: summary

Actual step output here
```

### JSON example

```json
{
  "steps": {
    "summary": {
      "status": "completed",
      "output": "Actual step output here"
    }
  }
}
```

---

## `truncate`

This is the setting that most needed clarification.

### What exists today

The codebase contains a truncation helper in `pkg/workflow/template.go` with these strategies:

| Strategy | Helper behavior |
|---|---|
| `chars` | Keeps the first `limit` Unicode characters |
| `lines` | Keeps the first `limit` lines |
| `tokens` | Approximates 1 token as 4 characters and keeps the first `limit * 4` characters |

When truncation occurs, the helper appends a suffix that explains how much content was trimmed.

### Why truncation is needed conceptually

Without truncation, a step that generates a very large output can make downstream prompts too large, too expensive, or impossible to send when that output is injected with `{{steps.some-step.output}}`.

### What the current runtime actually does

Important: `output.truncate` is currently parsed, but it is **not automatically applied** by the main `goflow run` path.

That means:

1. prior step outputs are currently injected into later prompts at full size
2. reporter output is currently emitted at full size
3. setting `output.truncate` today does not change stdout output or template injection behavior

So `truncate` is forward-compatible configuration, not active behavior in the current CLI path.

---

## Audit Output

Regardless of output format, the audit logger writes:

1. `output.md` for each completed step
2. `final_output.md` for the run summary

Those files currently contain the full stored outputs used by the run path.

---

## Practical Recommendation

Use `output.steps` and `format` today.

Treat `truncate` as planned configuration until the executor or reporter is updated to call the truncation helper.

---

## See Also

- [Settings And Options](settings-and-options.md)
- [Workflow Schema](workflow-schema.md)

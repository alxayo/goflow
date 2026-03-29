# Output Control

Configure how workflow results are formatted, truncated, and presented.

---

## Output Section

Configure output in the `output` section of your workflow:

```yaml
output:
  steps: [summary, recommendations]
  format: markdown
  truncate:
    strategy: chars
    limit: 5000
```

---

## Output Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `steps` | array | [last step] | Step IDs to include in output |
| `format` | string | `markdown` | Output format |
| `truncate` | object | — | Truncation settings |

---

## steps

Specify which step outputs to include in the final result:

```yaml
output:
  steps: [security-review, perf-review, summary]
```

### Order Matters

Steps are output in the order listed:

```yaml
steps: [summary, details]  # Summary first
steps: [details, summary]  # Details first
```

### Single Step

```yaml
steps: [final-summary]  # Just one step's output
```

### All Steps

If `steps` is omitted, only the last step is output.

---

## format

Control output formatting:

### markdown (default)

```yaml
format: markdown
```

**Output:**
```markdown
## Step: security-review

Found 2 critical issues...

## Step: perf-review

Performance looks good...
```

### json

```yaml
format: json
```

**Output:**
```json
{
  "workflow": "code-review",
  "steps": {
    "security-review": "Found 2 critical issues...",
    "perf-review": "Performance looks good..."
  }
}
```

### plain

```yaml
format: plain
```

**Output:**
```
Found 2 critical issues...

Performance looks good...
```

No headers, no formatting — just the raw content.

---

## truncate

Limit output size to prevent overwhelming results.

### Configuration

```yaml
output:
  truncate:
    strategy: "lines"  # or "chars"
    limit: 100
```

### Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `lines` | Keep last N lines | Log-style output |
| `chars` | Keep last N characters | Prose/reports |

### Global Truncation

Set defaults in `config`:

```yaml
config:
  truncate:
    strategy: lines
    limit: 50

output:
  steps: [analysis]
  # Uses global truncation settings
```

### Per-Output Override

```yaml
config:
  truncate:
    strategy: lines
    limit: 50

output:
  steps: [analysis]
  truncate:
    strategy: chars
    limit: 10000  # Override for final output
```

---

## Step Output Truncation

Truncation also applies when injecting step outputs via templates:

```yaml
config:
  truncate:
    strategy: lines
    limit: 100

steps:
  - id: analyze
    prompt: "Generate a detailed report..."  # Might produce 500 lines

  - id: summarize
    prompt: "Summarize: {{steps.analyze.output}}"  # Gets last 100 lines
    depends_on: [analyze]
```

This prevents context window overflow when passing large outputs between steps.

---

## Examples

### Minimal Output

Just the final summary:

```yaml
output:
  steps: [summary]
  format: plain
```

### Complete Report

Multiple sections in markdown:

```yaml
output:
  steps: [executive-summary, detailed-findings, recommendations]
  format: markdown
```

### Machine-Readable

For pipeline integration:

```yaml
output:
  steps: [analysis-result]
  format: json
```

### Limited Size

For contexts with limits:

```yaml
output:
  steps: [summary]
  format: plain
  truncate:
    strategy: chars
    limit: 2000
```

---

## Saving Output

### To File

Redirect stdout:

```bash
goflow run -w workflow.yaml > report.md
```

### To Variable

Capture in script:

```bash
OUTPUT=$(goflow run -w workflow.yaml)
echo "$OUTPUT"
```

### To File While Viewing

Use tee:

```bash
goflow run -w workflow.yaml | tee report.md
```

---

## Audit Trail

Regardless of output settings, the full untruncated output is always saved to the audit trail:

```
.workflow-runs/2026-03-26T10-00-00_example/
├── final_output.md    # Formatted output (as printed)
└── steps/
    └── 00_analyze/
        └── output.md  # Full untruncated step output
```

---

## Best Practices

### 1. Output Only What Users Need

```yaml
# ✓ Good: Just the summary
steps: [summary]

# ✗ Questionable: All intermediate steps
steps: [parse, analyze, review, format, validate, summary]
```

### 2. Use Appropriate Formats

| Use Case | Format |
|----------|--------|
| Human reading | `markdown` |
| Script/pipeline | `json` |
| Simple embedding | `plain` |

### 3. Consider Context Limits

If output will be used elsewhere (email, Slack, etc.), truncate appropriately:

```yaml
truncate:
  strategy: chars
  limit: 3000  # Most messaging platforms handle this
```

### 4. Check Full Output in Audit

If truncated output misses something:

```bash
cat .workflow-runs/*/steps/*/output.md
```

---

## See Also

- [Workflow Schema](workflow-schema.md) — Full YAML reference
- [Template Variables](templates.md) — Step output templates
- [CLI Reference](cli.md) — Output redirection options

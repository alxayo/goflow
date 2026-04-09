# Quick Start

Get goflow up and running in under 5 minutes. We'll build the binary, run an example workflow, and inspect the audit trail.

**Prerequisites:** [Go 1.21+](https://go.dev/dl/) installed. No API key needed — we'll use mock mode.

---

## Step 1: Get goflow

If you haven't installed goflow yet, do it now:

```bash
git clone https://github.com/alxayo/goflow.git
cd goflow
go build -o goflow ./cmd/workflow-runner/main.go
```

---

## Step 2: Run the Example Workflow (Mock Mode)

The repository includes example workflows. Let's run one in **mock mode**, which simulates AI responses:

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --mock \
  --verbose
```

### What you'll see

```
[INFO] Loading workflow: examples/simple-sequential.yaml
[INFO] Starting workflow: simple-sequential
[INFO] Step 1/3: security-review (security-reviewer)
[INFO] Step 2/3: perf-review (performance-reviewer)  
[INFO] Step 3/3: summary (aggregator)
[INFO] Workflow completed in 0.05s

## Step: summary

mock output
```

!!! tip "Why Mock Mode?"
    - **No API calls** — doesn't use real AI tokens
    - **Instant results** — runs in milliseconds
    - **Full pipeline test** — validates your YAML structure, dependencies, and templates
    
    Use mock mode while developing workflows, then switch to real mode when ready.

---

## Step 3: Inspect the Audit Trail

Every run creates a complete record in `.workflow-runs/`:

```bash
ls .workflow-runs/
```

You'll see a timestamped folder like `2026-03-26T10-15-30_simple-sequential/`.

Look inside:

```bash
ls .workflow-runs/*/
```

```
workflow.meta.json    # Run metadata and timing
workflow.yaml         # Snapshot of the workflow
final_output.md       # The formatted output
steps/                # Individual step data
```

Each step has its own folder with the exact prompt sent and response received:

```bash
cat .workflow-runs/*/steps/00_security-review/prompt.md
cat .workflow-runs/*/steps/00_security-review/output.md
```

---

## Step 4: Run With Real AI (Optional)

If you have Copilot CLI installed, try a real run:

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

Remove `--mock` and goflow will make actual AI calls. The output will be real code review content instead of "mock output".

---

## What Just Happened?

The `simple-sequential.yaml` workflow:

1. **Defined 3 agents** — security reviewer, performance reviewer, and aggregator
2. **Ran 3 steps in sequence**:
   - `security-review` — reviewed files for security issues
   - `perf-review` — reviewed files for performance issues (waited for security-review)
   - `summary` — combined both reviews (waited for perf-review)
3. **Passed data between steps** — using `{{steps.security-review.output}}` templates
4. **Created an audit trail** — saved everything to `.workflow-runs/`

---

## Next Steps

- :material-book-open-page-variant: [Your First Workflow](first-workflow.md) — Build your own workflow from scratch
- :material-school: [Tutorial](../tutorial/index.md) — Learn all features progressively
- :material-code-tags: [Examples](../examples/index.md) — Copy working patterns

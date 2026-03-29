# CLI Reference

Complete reference for the `goflow` command-line interface.

---

## Installation

```bash
go build -o goflow ./cmd/goflow
```

---

## Commands

### goflow run

Execute a workflow.

```bash
goflow run --workflow <path> [options]
```

**Required:**

| Flag | Description |
|------|-------------|
| `--workflow`, `-w` | Path to workflow YAML file |

**Optional:**

| Flag | Default | Description |
|------|---------|-------------|
| `--inputs`, `-i` | — | Input values (`key=value`), repeatable |
| `--verbose`, `-v` | false | Show step-by-step progress |
| `--mock` | false | Use mock responses (no real AI calls) |
| `--audit-dir` | `.workflow-runs` | Custom audit directory |
| `--model` | — | Override default model |
| `--step` | — | Run only a specific step (for debugging) |
| `--interactive` | false | Enable interactive mode |
| `--non-interactive` | false | Force non-interactive mode |
| `--skip-model-selection` | false | Skip model selection prompt |

**Examples:**

```bash
# Basic run
goflow run --workflow code-review.yaml

# With inputs
goflow run --workflow code-review.yaml \
  --inputs files='src/**/*.go' \
  --inputs mode='detailed'

# Mock mode with verbose output
goflow run --workflow code-review.yaml --mock --verbose

# Override model
goflow run --workflow code-review.yaml --model gpt-4o

# Run single step (debugging)
goflow run --workflow code-review.yaml --step security-review --mock
```

---

### goflow validate

Validate a workflow file without running it.

```bash
goflow validate --workflow <path>
```

**Examples:**

```bash
goflow validate --workflow code-review.yaml
```

**Output:**

```
✓ Workflow 'code-review' is valid
  - 4 steps defined
  - 3 agents referenced
  - DAG is acyclic
```

If invalid:

```
✗ Workflow validation failed:
  - Step 'review' references unknown agent 'reviewer'
  - Circular dependency detected: step-a -> step-b -> step-a
```

---

### goflow version

Display version information.

```bash
goflow version
```

**Output:**

```
goflow version v1.0.0 (abc1234) built 2026-03-15T10:30:00Z
```

---

### goflow list

List available agents.

```bash
goflow list agents [--search-paths <paths>]
```

**Examples:**

```bash
# List agents in default locations
goflow list agents

# Include custom paths
goflow list agents --search-paths ./custom-agents,/shared/agents
```

**Output:**

```
Found 5 agents:

  security-reviewer      ./agents/security-reviewer.agent.md
  performance-reviewer   ./agents/performance-reviewer.agent.md
  aggregator            ./agents/aggregator.agent.md
  code-analyzer         .github/agents/code-analyzer.agent.md
  shared-helper         ~/.copilot/agents/shared-helper.agent.md
```

---

## Input Format

Inputs use `key=value` format:

```bash
--inputs files='src/**/*.go'
--inputs mode=detailed
--inputs count=5
```

**Quoting:**
- Quote values with spaces: `--inputs message='Hello World'`
- Quote glob patterns: `--inputs files='**/*.go'`

**Multiple inputs:**

```bash
goflow run -w workflow.yaml \
  -i files='src/*.go' \
  -i mode=detailed \
  -i threshold=0.8
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOFLOW_AUDIT_DIR` | Default audit directory |
| `GOFLOW_MODEL` | Default model |
| `GOFLOW_VERBOSE` | Enable verbose mode (`true`/`false`) |
| `COPILOT_CLI_PATH` | Path to Copilot CLI binary |

**Example:**

```bash
export GOFLOW_VERBOSE=true
export GOFLOW_MODEL=gpt-4o
goflow run --workflow code-review.yaml
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Workflow validation error |
| 3 | Step execution error |
| 4 | Input/configuration error |

---

## Output

### Standard Output

The workflow output goes to stdout:

```bash
goflow run --workflow example.yaml > output.md
```

### Verbose Output

Verbose messages go to stderr:

```bash
goflow run --workflow example.yaml --verbose 2>progress.log
```

### Combined

To separate output from progress:

```bash
goflow run -w example.yaml -v > output.md 2> progress.log
```

---

## Interactive Mode

Interactive mode prompts for user input during execution:

```bash
goflow run --workflow guided-review.yaml --interactive
```

Features:
- User can provide input at certain steps
- AI can ask clarifying questions
- Useful for guided workflows like interviews

Toggle within workflow:

```yaml
config:
  interactive: true
```

---

## Mock Mode

Mock mode simulates AI responses without real API calls:

```bash
goflow run --workflow example.yaml --mock
```

**Use cases:**
- Testing workflow structure
- CI/CD pipeline validation
- Development without API costs
- Demonstrating workflows

Mock responses return `"mock output"` for all steps.

---

## Audit Trail

Every run creates an audit trail:

```
.workflow-runs/
└── 2026-03-26T10-15-30_code-review/
    ├── workflow.meta.json
    ├── workflow.yaml
    ├── final_output.md
    ├── memory.md          # If shared memory enabled
    └── steps/
        ├── 00_analyze/
        │   ├── step.meta.json
        │   ├── prompt.md
        │   └── output.md
        └── 01_review/
            ├── step.meta.json
            ├── prompt.md
            └── output.md
```

### Audit Files

| File | Contents |
|------|----------|
| `workflow.meta.json` | Run metadata, timing, status |
| `workflow.yaml` | Snapshot of workflow file |
| `final_output.md` | Formatted final output |
| `memory.md` | Shared memory final state |
| `*/step.meta.json` | Step metadata and timing |
| `*/prompt.md` | Exact prompt sent |
| `*/output.md` | AI response |

---

## Debugging

### Verbose Mode

See step-by-step progress:

```bash
goflow run -w example.yaml --verbose
```

### Single Step

Run just one step:

```bash
goflow run -w example.yaml --step security-review --mock
```

### Validate First

Check YAML before running:

```bash
goflow validate -w example.yaml && goflow run -w example.yaml
```

### Check Audit Trail

Inspect what was sent and received:

```bash
cat .workflow-runs/*/steps/00_step-name/prompt.md
cat .workflow-runs/*/steps/00_step-name/output.md
```

---

## See Also

- [Workflow Schema](workflow-schema.md) — YAML configuration reference
- [Troubleshooting](../troubleshooting.md) — Common issues and solutions

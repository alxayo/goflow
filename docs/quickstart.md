# Quick Start

This quick start gets you from clone to a working workflow in a few minutes.

## 1. Build the CLI

```bash
git clone https://github.com/alex/goflow.git
cd goflow
go build -o goflow ./cmd/goflow
```

## 2. Run the bundled simple example

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --mock \
  --verbose
```

Why `--mock` first:

- No Copilot CLI requirement
- Deterministic output for local validation
- Full DAG/template/audit behavior still runs

## 3. Run with real model calls

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

## 4. Inspect generated artifacts

```bash
ls -1 .workflow-runs | tail -n 3
```

Inside each run folder:

- `workflow.meta.json` run status and timing
- `workflow.yaml` snapshot of the resolved workflow
- `steps/*/prompt.md` exact prompt sent to an agent
- `steps/*/output.md` generated output per step

## 5. Your first custom workflow

Create `my-first-workflow.yaml`:

```yaml
name: "my-first-workflow"

agents:
  reviewer:
    inline:
      description: "Finds useful feedback"
      prompt: "You are a practical reviewer."

steps:
  - id: review
    agent: reviewer
    prompt: "Review src/**/*.go and suggest three improvements."

output:
  steps: [review]
  format: markdown
```

Run:

```bash
./goflow run --workflow my-first-workflow.yaml --mock --verbose
```

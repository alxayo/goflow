# Getting Started

This guide shows the minimum structure of a production-ready goflow workflow.

## Workflow anatomy

A workflow typically includes:

- `inputs`: runtime parameters supplied by CLI
- `config`: model and runtime behavior defaults
- `agents`: inline or file-backed agent definitions
- `steps`: executable units with optional dependencies
- `output`: what to print and how to format it

```yaml
name: "code-review"
description: "Parallel review workflow"

inputs:
  files:
    description: "Target files"
    default: "pkg/**/*.go"

config:
  model: "gpt-5"
  audit_dir: ".workflow-runs"
  audit_retention: 10

agents:
  security:
    file: "./agents/security-reviewer.agent.md"
  performance:
    file: "./agents/performance-reviewer.agent.md"
  aggregator:
    file: "./agents/aggregator.agent.md"

steps:
  - id: security-review
    agent: security
    prompt: "Review {{inputs.files}} for vulnerabilities."

  - id: performance-review
    agent: performance
    prompt: "Review {{inputs.files}} for performance issues."

  - id: aggregate
    agent: aggregator
    prompt: |
      Combine both reports:
      {{steps.security-review.output}}
      {{steps.performance-review.output}}
    depends_on: [security-review, performance-review]

output:
  steps: [aggregate]
  format: markdown
```

## Run patterns

- Local structure test: `--mock`
- Real agent execution: default mode
- Add `--verbose` for detailed step logs

```bash
./goflow run --workflow my-workflow.yaml --mock --verbose
./goflow run --workflow my-workflow.yaml --verbose
```

## Recommended authoring loop

1. Start with one step and an inline agent.
2. Add `inputs` so prompts are reusable.
3. Add dependencies with `depends_on`.
4. Move stable agents into `.agent.md` files.
5. Enable shared memory only when needed.
6. Keep prompts concise and composable.

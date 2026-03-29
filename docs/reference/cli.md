# CLI Commands

## `run`

Execute a workflow file.

```bash
goflow run --workflow <path> [--inputs key=value] [--mock] [--interactive] [--verbose]
```

### Common flags

- `--workflow`: path to workflow YAML
- `--inputs`: repeatable runtime inputs
- `--mock`: deterministic execution without real model calls
- `--interactive`: allow clarification prompts in terminal
- `--verbose`: detailed execution logs

## `version`

Show build metadata.

```bash
goflow version
```

Expected output includes semantic version, commit short SHA, and build timestamp.

## Practical command patterns

```bash
# Run bundled example in mock mode
goflow run --workflow examples/simple-sequential.yaml --inputs files='pkg/**/*.go' --mock --verbose

# Run with real model calls
goflow run --workflow examples/guided-code-review/guided-code-review.yaml --inputs files='pkg/**/*.go' --verbose

# Override multiple inputs
goflow run --workflow my-workflow.yaml --inputs files='pkg/**/*.go' --inputs severity=HIGH
```

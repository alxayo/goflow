# Simple Sequential

Workflow file: `examples/simple-sequential.yaml`

## What it demonstrates

- Basic step chaining
- Agent file loading
- Template injection from previous steps
- Clean final aggregation

## Run

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --mock \
  --verbose
```

## Real use case

Ideal for onboarding or validating a new prompt style before adding parallelism.

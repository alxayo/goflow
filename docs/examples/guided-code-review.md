# Guided Code Review

Workflow file: `examples/guided-code-review/guided-code-review.yaml`

## What it demonstrates

- Scoped code review by role-specific agents
- Decomposition into deep-diver and specialist reviewers
- Structured report writing output

## Run

```bash
./goflow run \
  --workflow examples/guided-code-review/guided-code-review.yaml \
  --inputs files='pkg/**/*.go' \
  --mock \
  --verbose
```

## Real use case

Use this when you want comprehensive review artifacts for pull requests or release gates.

# Building Workflows

## Sequential flows

Sequential pipelines are created by chaining dependencies:

```yaml
steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze {{inputs.files}}"

  - id: summarize
    agent: summarizer
    prompt: "Summarize: {{steps.analyze.output}}"
    depends_on: [analyze]
```

## Parallel fan-out and fan-in

Independent steps run in parallel when they share the same satisfied dependency set.

```yaml
steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze {{inputs.files}}"

  - id: security
    agent: security
    prompt: "Security review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: performance
    agent: performance
    prompt: "Performance review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: aggregate
    agent: aggregator
    prompt: |
      Merge these:
      {{steps.security.output}}
      {{steps.performance.output}}
    depends_on: [security, performance]
```

## Conditional execution

Use conditions to gate steps based on prior outputs.

```yaml
- id: release-decision
  agent: decider
  prompt: "Should we release? {{steps.aggregate.output}}"
  depends_on: [aggregate]
  condition:
    step: aggregate
    contains: "READY"
```

Supported condition checks:

- `contains`
- `not_contains`
- `equals`

## Output truncation

Prevent oversized prompt injection using output truncation:

```yaml
output:
  steps: [aggregate]
  format: markdown
  truncate:
    strategy: chars
    limit: 2000
```

## Shared memory

Enable lightweight cross-step communication in parallel flows:

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true
```

Use this for stateful collaboration between agents that should see shared context.

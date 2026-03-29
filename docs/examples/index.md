# Examples

Real-world workflow patterns you can copy and adapt.

---

## Included Examples

| Example | Description | Key Features |
|---------|-------------|--------------|
| [Simple Sequential](simple-sequential.md) | Basic multi-step pipeline | Dependencies, templates |
| [Code Review Pipeline](code-review.md) | Multi-expert review | Fan-out/fan-in, parallel |
| [Security Scan](security-scan.md) | Security-focused analysis | Tool restrictions, severity |
| [Decision Helper](decision-helper.md) | Interactive decision-making | Multiple perspectives |

---

## Running Examples

All examples are in the `examples/` directory:

```bash
# List available examples
ls examples/

# Run an example
goflow run --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/**/*.go' \
  --mock \
  --verbose
```

---

## Example Structure

Each example follows this pattern:

```
examples/
└── example-name/
    ├── example-name.yaml     # Workflow file
    ├── README.md             # Documentation
    └── agents/               # Supporting agents
        ├── agent-a.agent.md
        └── agent-b.agent.md
```

---

## Quick Pattern Reference

### Sequential Pipeline

```yaml
steps:
  - id: step-a
    prompt: "First task"
  - id: step-b
    prompt: "Second task using {{steps.step-a.output}}"
    depends_on: [step-a]
```

### Parallel Processing

```yaml
steps:
  - id: setup
    prompt: "Prepare"
  
  - id: task-1
    depends_on: [setup]
  - id: task-2
    depends_on: [setup]  # Runs parallel with task-1
  - id: task-3
    depends_on: [setup]  # Runs parallel with task-1, task-2
  
  - id: combine
    depends_on: [task-1, task-2, task-3]  # Waits for all
```

### Conditional Branching

```yaml
steps:
  - id: classify
    prompt: "Is this CRITICAL or NORMAL?"
  
  - id: urgent-path
    depends_on: [classify]
    condition:
      step: classify
      contains: "CRITICAL"
  
  - id: normal-path
    depends_on: [classify]
    condition:
      step: classify
      contains: "NORMAL"
```

### Multi-Agent Collaboration

```yaml
agents:
  expert-a:
    file: "./agents/security.agent.md"
  expert-b:
    file: "./agents/performance.agent.md"
  aggregator:
    inline:
      prompt: "You combine expert opinions."

steps:
  - id: review-a
    agent: expert-a
    prompt: "Analyze from your perspective"
  
  - id: review-b
    agent: expert-b
    prompt: "Analyze from your perspective"
  
  - id: combine
    agent: aggregator
    prompt: |
      Expert A says: {{steps.review-a.output}}
      Expert B says: {{steps.review-b.output}}
      
      Synthesize the findings.
    depends_on: [review-a, review-b]
```

---

## Browse Examples

- [Simple Sequential](simple-sequential.md) — Start here
- [Code Review Pipeline](code-review.md) — Parallel experts
- [Security Scan](security-scan.md) — Tool-restricted analysis
- [Decision Helper](decision-helper.md) — Interactive multi-perspective

# Workflow YAML Schema

Complete reference for all workflow YAML fields.

---

## Top-Level Structure

```yaml
name: "workflow-name"              # Required
description: "What this workflow does"  # Optional

inputs:                            # Optional
  input-name:
    description: "..."
    default: "..."

config:                            # Optional
  model: "gpt-4o"
  audit_dir: ".workflow-runs"
  # ... more config options

agents:                            # Required
  agent-name:
    inline: { ... }
    # or
    file: "path/to/agent.agent.md"

steps:                             # Required
  - id: "step-id"
    agent: "agent-name"
    prompt: "..."

output:                            # Optional
  steps: [step-id]
  format: "markdown"
```

---

## name (required)

```yaml
name: "code-review-pipeline"
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier for the workflow |

**Rules:**
- Must be a valid identifier (letters, numbers, hyphens, underscores)
- Used in audit trail folder names
- Should be descriptive and URL-safe

---

## description (optional)

```yaml
description: "Multi-agent code review with security and performance analysis"
```

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Human-readable explanation |

---

## inputs (optional)

Define runtime parameters:

```yaml
inputs:
  files:
    description: "Files to process (glob pattern)"
    default: "src/**/*.go"
  
  mode:
    description: "Review mode: quick or detailed"
    # No default = required input
```

### Input Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | Help text shown in errors |
| `default` | string | No | Default value if not provided |

### Providing Inputs

```bash
goflow run --workflow example.yaml --inputs files='*.go' --inputs mode='detailed'
```

### Using Inputs

```yaml
prompt: "Analyze {{inputs.files}} in {{inputs.mode}} mode"
```

---

## config (optional)

Global workflow configuration:

```yaml
config:
  model: "gpt-4o"
  audit_dir: ".workflow-runs"
  audit_retention: 10
  truncate:
    strategy: "lines"
    limit: 100
  shared_memory:
    enabled: true
    inject_into_prompt: true
  agent_search_paths:
    - "./custom-agents"
    - "/shared/agents"
```

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | Provider default | Default model for all agents |
| `audit_dir` | string | `.workflow-runs` | Where to store audit trails |
| `audit_retention` | number | 10 | Number of runs to keep |
| `truncate` | object | — | Default output truncation |
| `shared_memory` | object | — | Shared memory configuration |
| `agent_search_paths` | array | — | Additional agent file search paths |

### Truncation Config

```yaml
config:
  truncate:
    strategy: "lines"  # "lines" or "chars"
    limit: 100         # Number of lines or characters
```

### Shared Memory Config

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true  # Automatically inject memory into prompts
```

---

## agents (required)

Define agents used in the workflow:

```yaml
agents:
  # Inline definition
  reviewer:
    inline:
      description: "Reviews code"
      prompt: "You are an expert code reviewer."
      tools:
        - grep
        - read_file
      model: "gpt-4o"

  # File reference
  security:
    file: "./agents/security-reviewer.agent.md"
```

### Inline Agent Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | Yes | What this agent does |
| `prompt` | string | Yes | System prompt (agent instructions) |
| `tools` | array | No | List of allowed tools |
| `model` | string | No | Model override |
| `agents` | array | No | Sub-agents for delegation |

### File Agent Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | string | Yes | Path to `.agent.md` file |

Paths are resolved relative to the workflow file.

---

## steps (required)

Define the workflow's execution steps:

```yaml
steps:
  - id: analyze
    agent: reviewer
    prompt: "Analyze the code structure."

  - id: review
    agent: reviewer
    prompt: "Review: {{steps.analyze.output}}"
    depends_on: [analyze]
    condition:
      step: analyze
      contains: "ISSUES"
```

### Step Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique step identifier |
| `agent` | string | Yes | Agent name from `agents` section |
| `prompt` | string | Yes | The task/question for the agent |
| `depends_on` | array | No | Step IDs that must complete first |
| `condition` | object | No | Condition to run this step |
| `tools` | array | No | Override agent's tools |
| `model` | string | No | Override agent's model |

### Step ID Rules

- Must be unique within the workflow
- Use lowercase with hyphens: `security-review`
- Referenced in `depends_on` and `{{steps.ID.output}}`

### depends_on

```yaml
depends_on: [step-a, step-b]  # Waits for BOTH to complete
```

Steps without `depends_on` run as soon as possible (potentially in parallel).

### condition

Run the step only if a condition is met:

```yaml
condition:
  step: previous-step      # Which step's output to check
  contains: "APPROVED"     # Must contain this substring
```

| Operator | Description |
|----------|-------------|
| `contains` | Output contains substring |
| `not_contains` | Output does NOT contain substring |
| `equals` | Output exactly equals (trimmed) |
| `not_equals` | Output does NOT equal |

Multiple operators are AND'd together.

---

## output (optional)

Control the final output:

```yaml
output:
  steps: [summary, recommendations]  # Which steps to include
  format: "markdown"                 # Output format
  truncate:
    strategy: "chars"
    limit: 5000
```

### Output Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `steps` | array | Last step | Step IDs to include in output |
| `format` | string | `markdown` | Format: `markdown`, `json`, `plain` |
| `truncate` | object | — | Output truncation settings |

### Output Formats

- **markdown** — Headers with step names, output as content
- **json** — Structured JSON with step outputs
- **plain** — Raw output without formatting

---

## Template Variables

### Input Templates

```yaml
prompt: "Process {{inputs.files}}"
```

### Step Output Templates

```yaml
prompt: "Based on: {{steps.previous-step.output}}"
```

### Template Rules

- Templates are resolved before sending to the AI
- Missing inputs cause runtime errors (unless they have defaults)
- Missing step references (from skipped steps) resolve to empty string
- Templates work in: `prompt`, `condition.contains`, etc.

---

## Complete Example

```yaml
name: "full-example"
description: "Shows all workflow features"

inputs:
  target:
    description: "Files to analyze"
    default: "src/**/*.go"
  depth:
    description: "Analysis depth: quick, normal, detailed"
    default: "normal"

config:
  model: "gpt-4o"
  audit_retention: 5
  truncate:
    strategy: "lines"
    limit: 50
  shared_memory:
    enabled: true
    inject_into_prompt: true

agents:
  analyzer:
    inline:
      description: "Code analyzer"
      prompt: "You analyze code structure and patterns."
      tools: [grep, read_file]
  
  reviewer:
    file: "./agents/security-reviewer.agent.md"
  
  summarizer:
    inline:
      description: "Summarizes findings"
      prompt: "You create executive summaries."

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze {{inputs.target}} at {{inputs.depth}} depth."

  - id: security-check
    agent: reviewer
    prompt: "Security review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: deep-dive
    agent: reviewer
    prompt: "Deep security analysis required."
    depends_on: [security-check]
    condition:
      step: security-check
      contains: "CRITICAL"

  - id: summary
    agent: summarizer
    prompt: |
      Summarize:
      {{steps.analyze.output}}
      {{steps.security-check.output}}
      {{steps.deep-dive.output}}
    depends_on: [analyze, security-check, deep-dive]

output:
  steps: [summary]
  format: markdown
  truncate:
    strategy: "chars"
    limit: 3000
```

---

## See Also

- [Agent File Format](agent-format.md) — `.agent.md` file structure
- [Template Variables](templates.md) — Template syntax details
- [CLI Reference](cli.md) — Command-line options

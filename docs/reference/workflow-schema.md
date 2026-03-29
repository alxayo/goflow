# Workflow Schema

This page describes the practical schema you use in goflow YAML files.

## Top-level fields

```yaml
name: string
description: string
inputs: map
config: map
agents: map
steps: list
output: map
```

## `inputs`

Declares runtime values that can be overridden by CLI flags.

```yaml
inputs:
  files:
    description: "Glob of files"
    default: "pkg/**/*.go"
```

Reference in prompts: `{{inputs.files}}`

## `config`

Global runtime settings.

```yaml
config:
  model: "gpt-5"
  audit_dir: ".workflow-runs"
  audit_retention: 10
  max_concurrency: 4
  shared_memory:
    enabled: true
    inject_into_prompt: true
```

## `agents`

Define each agent inline or by file.

```yaml
agents:
  my-agent:
    inline:
      description: "Does one job well"
      prompt: "You are ..."
      tools: [grep, view]
      model: "gpt-5"

  external-agent:
    file: "./agents/external.agent.md"
```

## `steps`

A step has an id, agent, prompt, and optional dependencies and condition.

```yaml
steps:
  - id: step-id
    agent: my-agent
    prompt: "Task text"
    depends_on: [other-step]
    condition:
      step: other-step
      contains: "APPROVE"
```

### Template references

- Previous step output: `{{steps.<id>.output}}`
- Runtime input: `{{inputs.<name>}}`

## `output`

Controls what final result is emitted.

```yaml
output:
  steps: [aggregate]
  format: markdown
  truncate:
    strategy: chars
    limit: 3000
```

Supported formats are implementation-dependent but markdown is the most common default.

## Validation expectations

A valid workflow should satisfy:

- Unique step ids
- Existing agent references for all steps
- No cyclic dependency graph
- Template references that target known inputs/steps
- Conditions that reference existing steps

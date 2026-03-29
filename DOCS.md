# goflow — User Guide

A comprehensive guide to understanding, configuring, and building workflows with goflow: an AI orchestration engine that coordinates multi-agent LLM workflows using the Copilot SDK.

## Implementation Accuracy Note

This document mixes current behavior with broader design intent. For the implementation-accurate, source-code-based reference to every setting and option, use [SETTINGS_REFERENCE.md](SETTINGS_REFERENCE.md).

The most important current runtime caveats are:

1. `goflow run` currently executes the sequential orchestrator path.
2. `config.max_concurrency` is implemented in `RunParallel()` but does not affect the normal CLI path today.
3. `output.truncate` is parsed and helper code exists, but automatic truncation is not currently applied during template injection or final reporting.
4. Shared-memory helpers exist in the codebase, but the main CLI path does not yet wire them in automatically.

---

## Table of Contents

1. [What Is goflow?](#1-what-is-goflow)
2. [How It Works](#2-how-it-works)
   - [Architecture Overview](#architecture-overview)
   - [Execution Flow](#execution-flow)
   - [DAG Scheduling](#dag-scheduling)
   - [Sequential vs Parallel Execution](#sequential-vs-parallel-execution)
3. [Installation & Prerequisites](#3-installation--prerequisites)
4. [CLI Reference](#4-cli-reference)
5. [Workflow YAML Reference](#5-workflow-yaml-reference)
   - [Top-Level Fields](#top-level-fields)
   - [`inputs` — Runtime Variables](#inputs--runtime-variables)
   - [`config` — Global Settings](#config--global-settings)
   - [`agents` — Agent Definitions](#agents--agent-definitions)
   - [`steps` — The Execution Pipeline](#steps--the-execution-pipeline)
   - [`output` — Result Formatting](#output--result-formatting)
6. [Agent Files (`.agent.md`)](#6-agent-files-agentmd)
   - [Structure](#structure)
   - [Frontmatter Fields](#frontmatter-fields)
   - [Agent Discovery](#agent-discovery)
   - [Claude-Format Compatibility](#claude-format-compatibility)
7. [Template Variables](#7-template-variables)
   - [Step Output References](#step-output-references)
   - [Input References](#input-references)
8. [Conditions — Conditional Step Execution](#8-conditions--conditional-step-execution)
9. [Output Truncation](#9-output-truncation)
10. [Shared Memory](#10-shared-memory)
11. [Audit Trail](#11-audit-trail)
    - [Directory Structure](#directory-structure)
    - [Retention Policy](#retention-policy)
12. [Interactive Mode — Clarification Questions](#12-interactive-mode--clarification-questions)
    - [How It Works](#how-it-works-1)
    - [Enabling Interactive Mode](#enabling-interactive-mode)
    - [Per-Step Control](#per-step-control)
    - [Parallelism Behavior](#parallelism-behavior)
13. [Mock Mode — Testing Without LLMs](#13-mock-mode--testing-without-llms)
14. [Practical Guide: Building Workflows](#14-practical-guide-building-workflows)
    - [Step 1: Define Your Agents](#step-1-define-your-agents)
    - [Step 2: Map Out Steps and Dependencies](#step-2-map-out-steps-and-dependencies)
    - [Step 3: Write the Workflow YAML](#step-3-write-the-workflow-yaml)
    - [Step 4: Test with Mock Mode](#step-4-test-with-mock-mode)
    - [Step 5: Run for Real](#step-5-run-for-real)
15. [Examples](#15-examples)
    - [Minimal Sequential Workflow](#minimal-sequential-workflow)
    - [Parallel Fan-Out / Fan-In Pipeline](#parallel-fan-out--fan-in-pipeline)
    - [Conditional Branching](#conditional-branching)
16. [Troubleshooting](#16-troubleshooting)

---

## 1. What Is goflow?

goflow is a command-line tool that orchestrates multi-step AI agent pipelines. Instead of manually running one agent after another and copy-pasting outputs, you define a workflow in YAML that describes:

- **Which agents** to use (security reviewer, performance auditor, aggregator, etc.)
- **What prompts** to send to each agent
- **How steps depend on each other** (sequential, parallel, or conditional)
- **How outputs flow** between steps via template variables

The engine parses your workflow, builds a dependency graph (DAG), resolves agents, and executes steps in the correct order — running independent steps in parallel when possible. Every run produces a full audit trail with prompts, outputs, and metadata.

**Key capabilities:**

| Capability | Description |
|---|---|
| Multi-agent orchestration | Chain different specialized agents in a pipeline |
| DAG-based scheduling | Automatic dependency resolution with topological sort |
| Parallel execution | Independent steps run concurrently via goroutines |
| Template variables | Inject prior step outputs and CLI inputs into prompts |
| Conditional steps | Execute or skip steps based on prior outputs |
| Output truncation | Prevent context window overflow with chars/lines/tokens limits |
| Shared memory | Cross-agent communication during parallel execution |
| Full audit trail | Every run records prompts, outputs, metadata, and timing |
| Mock mode | Test workflow structure without calling real LLMs |
| Agent discovery | Auto-discover agents from standard directory locations |

---

## 2. How It Works

### Architecture Overview

goflow is built from modular components, each with a single responsibility:

```
┌─────────────────────────────────────────────────────┐
│                   CLI (main.go)                     │
│  Parses flags, loads workflow, wires components     │
└─────────────────────┬───────────────────────────────┘
                      │
         ┌────────────▼────────────┐
         │      Orchestrator       │
         │  Processes DAG levels   │
         │  Sequential or parallel │
         └────────────┬────────────┘
                      │
         ┌────────────▼────────────┐
         │     Step Executor       │
         │  Per-step: condition →  │
         │  template → session →   │
         │  send → audit           │
         └────────────┬────────────┘
                      │
    ┌─────────────────┼─────────────────┐
    │                 │                 │
┌───▼───┐      ┌─────▼─────┐     ┌────▼─────┐
│ Parser│      │ Agent     │     │  Audit   │
│ + DAG │      │ Loader    │     │  Logger  │
│ + Tmpl│      │ + Discover│     │ + Cleanup│
└───────┘      └───────────┘     └──────────┘
```

| Component | Package | Responsibility |
|---|---|---|
| **Parser** | `pkg/workflow` | Reads and validates workflow YAML |
| **DAG Builder** | `pkg/workflow` | Groups steps into execution levels via topological sort |
| **Template Engine** | `pkg/workflow` | Resolves `{{steps.X.output}}` and `{{inputs.Y}}` placeholders |
| **Condition Evaluator** | `pkg/workflow` | Checks `contains`, `not_contains`, `equals` conditions |
| **Agent Loader** | `pkg/agents` | Parses `.agent.md` files (YAML frontmatter + markdown body) |
| **Agent Discovery** | `pkg/agents` | Scans standard directories for agent files |
| **Step Executor** | `pkg/executor` | Runs a single step: condition → template → SDK session → output |
| **Orchestrator** | `pkg/orchestrator` | Walks the DAG level-by-level, dispatching steps |
| **Audit Logger** | `pkg/audit` | Creates run directories, writes metadata and output files |
| **Reporter** | `pkg/reporter` | Formats final workflow output (markdown, JSON, plain text) |
| **Memory Manager** | `pkg/memory` | Thread-safe shared memory for parallel agent communication |

### Execution Flow

When you run `goflow run --workflow my-pipeline.yaml`, this is what happens:

```
1. Parse YAML            → Workflow struct
2. Validate              → Check for missing agents, bad refs, invalid conditions
3. Merge inputs          → CLI --inputs override YAML defaults
4. Resolve agents        → Load .agent.md files + inline definitions
5. Create audit dir      → .workflow-runs/<timestamp>_<name>/
6. Apply retention       → Delete old runs beyond the configured limit
7. Build DAG             → Kahn's algorithm groups steps into levels
8. Execute level 0       → Steps with no dependencies
   Execute level 1       → Steps whose dependencies are in level 0
   Execute level 2       → ...and so on
9. For each step:
   a. Evaluate condition → Skip if not met
   b. Resolve templates  → Replace {{steps.X.output}} with actual text
   c. Create SDK session → Set system prompt, model, tools from agent
   d. Send prompt        → Call Copilot CLI with the resolved prompt
   e. Capture output     → Store the assistant's response
   f. Write audit files  → step.meta.json, prompt.md, output.md
10. Format output        → Collect outputs from specified steps
11. Finalize audit       → Write final_output.md, update workflow.meta.json
12. Print to stdout      → Display the formatted result
```

### DAG Scheduling

Steps are organized into **levels** using Kahn's algorithm (BFS topological sort). All steps in the same level have their dependencies already satisfied and can potentially run in parallel.

Example with this dependency structure:

```
analyze → review-security  ─┐
analyze → review-performance ├→ aggregate → decide
analyze → review-style      ─┘
```

The DAG builder produces:

| Level | Steps | Why |
|---|---|---|
| 0 | `analyze` | No dependencies |
| 1 | `review-security`, `review-performance`, `review-style` | All depend only on `analyze` |
| 2 | `aggregate` | Depends on all three reviews |
| 3 | `decide` | Depends on `aggregate` |

Level 1 steps are independent of each other, so they can run in parallel.

### Sequential vs Parallel Execution

- **Sequential mode** (default): Steps within a level run one at a time. Safe and predictable. Uses `Orchestrator.Run()`.
- **Parallel mode**: Steps within a level run concurrently via goroutines with `sync.WaitGroup`. Uses `Orchestrator.RunParallel()`. Concurrency can be limited via `config.max_concurrency`.

Important: the workflow YAML does not explicitly mark a step as parallel. Parallelism is inferred from the dependency graph created by `depends_on`. The runner then decides whether to execute ready steps sequentially or concurrently.

Both modes process levels in order — only steps within the same level can overlap.

---

## 3. Installation & Prerequisites

### Requirements

- **Go 1.21+** installed
- **Copilot CLI** installed and accessible on `$PATH` (or at `~/.copilot/copilot`)
- macOS, Linux, or WSL

### Build from Source

```bash
cd ~/Code/workflow-runner
go build -o goflow ./cmd/workflow-runner/main.go
```

This produces a `goflow` binary in the current directory.

### Verify Copilot CLI

The real executor requires the `copilot` CLI binary:

```bash
which copilot
# or
copilot --version
```

If Copilot CLI is not available, you can still test workflows using `--mock` mode (see [Mock Mode](#12-mock-mode--testing-without-llms)).

---

## 4. CLI Reference

### Syntax

```
goflow run [options]
```

### Options

| Flag | Required | Description |
|---|---|---|
| `--workflow` | Yes | Path to the workflow YAML file |
| `--inputs` | No | Key=value input pair. Repeatable for multiple inputs |
| `--audit-dir` | No | Override the audit directory (default: from workflow `config.audit_dir`) |
| `--mock` | No | Use the mock executor instead of Copilot CLI |
| `--interactive` | No | Allow agents to pause and ask clarification questions via the terminal |
| `--verbose` | No | Print step statuses, timing, and debug info to stderr |

### Input Handling

Inputs declared in the workflow YAML can have defaults. CLI `--inputs` values override those defaults. You can also pass inputs not declared in the YAML — they are accepted as pass-through values.

```bash
# Override the "files" input, use default for everything else
goflow run --workflow pipeline.yaml --inputs files='src/**/*.go'

# Multiple inputs
goflow run --workflow pipeline.yaml \
  --inputs files='src/**/*.go' \
  --inputs severity_filter='HIGH'
```

### Running Example Workflows

The example workflows use relative agent paths (`../agents/...`) which resolve relative to the workflow file's location. You can run them from anywhere:

```bash
go run ./cmd/workflow-runner run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

### Exit Codes

| Code | Meaning |
|---|---|
| `0` | Workflow completed successfully |
| `1` | Error (parse failure, agent not found, step execution failure, etc.) |

---

## 5. Workflow YAML Reference

A workflow YAML file is the blueprint for a pipeline. It declares the agents, steps, dependencies, and output format.

### Top-Level Fields

```yaml
name: "my-pipeline"                # Required. Unique workflow name.
description: "What this does"      # Optional. Human-readable description.
inputs: { ... }                    # Optional. Runtime variables.
config: { ... }                    # Optional. Global settings.
agents: { ... }                    # Required. Agent definitions.
skills: [...]                      # Optional. Skills to attach to all steps.
steps: [...]                       # Required. Ordered list of execution steps.
output: { ... }                    # Optional. Result formatting config.
```

---

### `inputs` — Runtime Variables

Inputs let you parameterize workflows. They can be passed via `--inputs key=value` on the CLI.

```yaml
inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "src/**/*.go"
  severity_filter:
    description: "Minimum severity level"
    default: "MEDIUM"
```

| Field | Type | Description |
|---|---|---|
| `description` | string | Human-readable description of the input |
| `default` | string | Default value if not provided via CLI |

Reference inputs in step prompts with `{{inputs.files}}` or `{{inputs.severity_filter}}`.

---

### `config` — Global Settings

```yaml
config:
  model: "gpt-4o"                  # Default model for agents without one
  interactive: false               # Allow agents to ask clarification questions (see §12)
  audit_dir: ".workflow-runs"      # Where audit trails are stored
  audit_retention: 10              # Keep last N runs (0 = keep all)
  max_concurrency: 3               # Max parallel steps (0 = unlimited)
  streaming: false                 # Enable streaming output (reserved)
  log_level: "info"                # Log verbosity: debug, info, warn, error
  agent_search_paths:              # Extra directories to scan for agents
    - "./custom-agents"
  shared_memory:                   # Cross-agent communication (see §10)
    enabled: true
    inject_into_prompt: true
    initial_content: "Review started."
    initial_file: ""               # Load initial content from a file
  provider:                        # BYOK provider config (optional)
    type: "openai"
    base_url: "https://api.example.com"
    api_key_env: "MY_API_KEY"
```

| Field | Type | Default | Description |
|---|---|---|---|
| `model` | string | `""` | Default LLM model name |
| `audit_dir` | string | `.workflow-runs` | Directory for run audit trails |
| `audit_retention` | int | `0` | Max runs to keep (0 = unlimited) |
| `max_concurrency` | int | `0` | Max parallel step goroutines (0 = unlimited) |
| `interactive` | bool | `false` | Enable interactive mode for all steps (agents may ask clarification questions). See [§12](#12-interactive-mode--clarification-questions). |
| `streaming` | bool | `false` | Reserved for streaming output support |
| `log_level` | string | `info` | Logging verbosity |
| `agent_search_paths` | []string | `[]` | Additional directories to scan for `.agent.md` files |
| `shared_memory` | object | — | Shared memory configuration (see [§10](#10-shared-memory)) |
| `provider` | object | — | Bring-your-own-key provider settings |

---

### `agents` — Agent Definitions

Every agent used by a step must be declared in the `agents` map. An agent can be defined in two ways:

#### File-based agent

Point to an `.agent.md` file:

```yaml
agents:
  security-reviewer:
    file: "../agents/security-reviewer.agent.md"
```

The file path is relative to the workflow file's location. For example, if your workflow is at `examples/my-workflow.yaml` and contains `file: "../agents/helper.agent.md"`, it resolves to `agents/helper.agent.md`. The agent's name in the map (`security-reviewer`) is used for referencing in steps.

#### Inline agent

Define the agent directly in the workflow YAML:

```yaml
agents:
  style-reviewer:
    inline:
      description: "Reviews code style and naming conventions"
      prompt: |
        You are a code style reviewer. Check naming conventions,
        formatting, readability, and Go idioms.
      tools: ["grep", "glob", "view"]
      model: "gpt-4o"
```

| Inline Field | Type | Description |
|---|---|---|
| `description` | string | What this agent does |
| `prompt` | string | The agent's system prompt (instructions) |
| `tools` | []string | Tools the agent can use |
| `model` | string | LLM model override |

**Validation rule:** Every agent must have either `file` or `inline` — not both, not neither.

---

### `steps` — The Execution Pipeline

Steps are the core of a workflow. Each step sends a prompt to an agent and captures the output.

```yaml
steps:
  - id: analyze                       # Required. Unique step identifier.
    agent: security-reviewer          # Required. Name from agents map.
    prompt: "Analyze: {{inputs.files}}" # Required. Prompt text (supports templates).
    depends_on: []                    # Optional. List of step IDs this step waits for.
    condition:                        # Optional. Only execute if condition is met.
      step: decide
      contains: "APPROVE"
    skills: []                        # Optional. Skills attached to this step.
    on_error: ""                      # Optional. Error handling strategy.
    retry_count: 0                    # Optional. Number of retries on failure.
    timeout: ""                       # Optional. Step timeout duration.
    interactive: true                 # Optional. Override interactive mode for this step.
```

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | Yes | Unique identifier for the step. Used in `depends_on`, conditions, and template refs. |
| `agent` | string | Yes | Name of an agent declared in the `agents` map. |
| `prompt` | string | Yes | The prompt sent to the agent. Supports `{{steps.X.output}}` and `{{inputs.Y}}` templates. |
| `depends_on` | []string | No | Step IDs that must complete before this step runs. |
| `condition` | object | No | Execute only if the condition evaluates to true. See [§8](#8-conditions--conditional-step-execution). |
| `skills` | []string | No | Skills attached to this step. |
| `on_error` | string | No | Error handling strategy (reserved for future use). |
| `retry_count` | int | No | Number of retries on failure (reserved for future use). |
| `timeout` | string | No | Step timeout (reserved for future use). |
| `interactive` | bool (pointer) | No | Override interactive mode for this step. `true` forces interactive, `false` forces non-interactive, omitted inherits from config/CLI. See [§12](#12-interactive-mode--clarification-questions). |

**Validation rules:**
- Step IDs must be unique within a workflow.
- `agent` must reference a name in the `agents` map.
- `depends_on` entries must reference valid step IDs.
- A step cannot depend on itself.
- Template references (`{{steps.X.output}}`) must reference valid step IDs.
- Condition step references must be transitive upstream dependencies.

#### Step Ordering Rules

- Steps with no `depends_on` run first (DAG level 0).
- Steps with `depends_on` wait until all listed steps complete.
- Steps at the same DAG level (all dependencies satisfied) can run in parallel.
- A step that is `skipped` (condition not met) still counts as completed for dependency purposes.

#### How Sequence and Parallelism Are Expressed in YAML

goflow does not use an explicit field such as `parallel: true` or `sequential: true`.

Instead, execution order is inferred from each step's `depends_on` list:

- If a step depends on a previous step, it runs after that step.
- If several steps depend on the same earlier step and do not depend on each other, they are in the same DAG level and can run in parallel.
- If a step depends on multiple earlier steps, it waits for all of them before running.

In other words, the YAML defines a dependency graph, not an execution mode flag.

Sequential example:

```yaml
steps:
  - id: security-review
    agent: security-reviewer
    prompt: "Review for security issues"

  - id: perf-review
    agent: performance-reviewer
    prompt: "Review for performance issues"
    depends_on: [security-review]

  - id: summary
    agent: aggregator
    prompt: "Summarize the findings"
    depends_on: [perf-review]
```

This creates a strict sequence:

1. `security-review`
2. `perf-review`
3. `summary`

Parallel fan-out example:

```yaml
steps:
  - id: analyze
    agent: security-reviewer
    prompt: "Analyze the codebase"

  - id: review-security
    agent: security-reviewer
    prompt: "Security review"
    depends_on: [analyze]

  - id: review-performance
    agent: performance-reviewer
    prompt: "Performance review"
    depends_on: [analyze]

  - id: review-style
    agent: aggregator
    prompt: "Style review"
    depends_on: [analyze]

  - id: aggregate
    agent: aggregator
    prompt: "Combine all review outputs"
    depends_on: [review-security, review-performance, review-style]
```

This means:

1. Run `analyze`.
2. Then `review-security`, `review-performance`, and `review-style` become ready together.
3. Then `aggregate` waits for all three to finish.

A useful mental model is:

- no `depends_on` = entry step
- one dependency = run after that step
- shared dependency = parallel candidates
- multiple dependencies = wait-for-all fan-in

---

### `output` — Result Formatting

Controls what gets printed to stdout when the workflow finishes.

```yaml
output:
  steps: [summary, aggregate]      # Which step outputs to include
  format: markdown                  # Output format: markdown, json, plain
  truncate:                         # Truncation for template injection
    strategy: "chars"               # Strategy: chars, lines, tokens
    limit: 2000                     # Maximum units to keep
```

| Field | Type | Default | Description |
|---|---|---|---|
| `steps` | []string | all completed steps | Step IDs whose outputs are included in final output |
| `format` | string | `markdown` | Output format: `markdown`, `json`, or `plain` |
| `truncate` | object | none | Truncation config for output injection (see [§9](#9-output-truncation)) |

**Output Formats:**

- **`markdown`** — Each step output under a `## Step: <id>` heading.
- **`json`** — A JSON object with `steps.<id>.status` and `steps.<id>.output` fields.
- **`plain`** — Delimited with `=== <id> ===` separators.

If `steps` is empty, all completed steps are included (sorted alphabetically).

---

## 6. Agent Files (`.agent.md`)

Agents define the persona, capabilities, and system prompt for an LLM session. Agent files use the VS Code custom agents format: YAML frontmatter + markdown body.

### Structure

```markdown
---
name: security-reviewer
description: Reviews code for OWASP Top 10 vulnerabilities
tools:
  - grep
  - glob
  - view
model: gpt-4o
---

# Security Reviewer

You are an expert security code reviewer. Focus on:

1. **Injection attacks** — SQL injection, XSS, command injection
2. **Authentication flaws** — weak password handling, missing MFA
3. **Access control** — broken authorization checks

Always cite specific file paths and line numbers.
Provide severity ratings: CRITICAL, HIGH, MEDIUM, LOW.
```

The **frontmatter** (between `---` delimiters) contains structured metadata. The **markdown body** below it becomes the agent's **system prompt** — the instructions sent to the LLM at the start of each session.

### Frontmatter Fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Agent name. Defaults to the filename stem if omitted. |
| `description` | string | Human-readable description |
| `tools` | []string | Copilot CLI tool names the agent can use (e.g., `grep`, `glob`, `view`, `bash`). Both YAML block and flow formats are valid: `tools: ['grep', 'view']` or as a list. |
| `model` | string or []string | LLM model name(s). First in list is preferred. |
| `agents` | []string | Subagents this agent can delegate to |
| `mcp-servers` | object | MCP server configurations per agent |
| `handoffs` | []object | Agent-to-agent transition metadata |
| `hooks` | object | Session lifecycle hooks (`onPreToolUse`, `onPostToolUse`) |
| `argument-hint` | string | Hint text for interactive use |
| `user-invocable` | bool | Whether users can invoke directly (interactive-only) |
| `disable-model-invocation` | bool | Disable direct model calls (interactive-only) |
| `target` | string | Target specification |

#### `model` — String or List

The model field accepts either a single string or a priority list:

```yaml
# Single model
model: gpt-4o

# Priority list: tries first, falls back to second
model:
  - gpt-4o
  - gpt-4o-mini
```

The first model in the list is used. If the model is unavailable, the executor falls back to running without the `--model` flag.

#### `mcp-servers` — MCP Server Configuration

```yaml
mcp-servers:
  security-tools:
    command: docker
    args: ["run", "--rm", "security-scanner:latest"]
    env:
      SCAN_DEPTH: "3"
```

| Field | Type | Description |
|---|---|---|
| `command` | string | Command to launch the MCP server |
| `args` | []string | Command-line arguments |
| `env` | map | Environment variables for the server process |

#### `handoffs` — Agent-to-Agent Transitions

```yaml
handoffs:
  - label: "Send to Aggregator"
    agent: aggregator
    prompt: "Aggregate these findings..."
    send: true
    model: "gpt-4o"
```

Handoffs are parsed and stored as metadata. They are not currently used for DAG construction — use `depends_on` in steps for that.

### Agent Discovery

goflow searches for agent files in multiple locations, with a defined priority. Higher-priority locations overwrite lower ones when agents share the same name.

**Priority order (highest → lowest):**

| Priority | Location | Notes |
|---|---|---|
| 1 (highest) | Explicit `agents.*.file` in workflow YAML | Always wins |
| 2 | `.github/agents/*.agent.md` | Workspace-level GitHub agents |
| 3 | `.claude/agents/*.md` | Claude format — auto-normalized |
| 4 | `~/.copilot/agents/*.agent.md` | User-level Copilot agents |
| 5 (lowest) | Paths in `config.agent_search_paths` | Custom scan directories |

Files must have the `.agent.md` or `.md` extension. Directories that don't exist are silently skipped.

### Tool Naming Conventions

Tool names differ across platforms that use the `.agent.md` format:

| Platform | Naming Style | Example |
|---|---|---|
| **Copilot CLI** (used by goflow) | lowercase | `grep`, `view`, `bash` |
| **VS Code** | `category/toolName` or set name | `search/textSearch`, `read/readFile`, or `search` |
| **Claude Code** | PascalCase | `Read`, `Grep`, `Bash` |

When authoring agent files for goflow, use **Copilot CLI tool names** — these are the names passed to the CLI's `--available-tools` flag.

### Claude-Format Compatibility

Agent files found under `.claude/agents/` are automatically normalized:

- Comma-separated tool strings (e.g., `"Read, Grep, Bash"`) are split into arrays.
- Tool names are mapped to Copilot CLI equivalents:

| Claude Name | Copilot CLI Name |
|---|---|
| `Read` | `view` |
| `Grep` | `grep` |
| `Glob` | `glob` |
| `Bash` | `bash` |
| `Write` | `create_file` |
| `Edit` | `replace_string_in_file` |
| `MultiEdit` | `multi_replace_string_in_file` |

Unknown tool names (e.g., `WebFetch`, `Agent`, `WebSearch`) are kept as-is and passed through to the CLI without transformation.

---

## 7. Template Variables

Prompts support two types of template variables that are resolved at runtime before sending to the LLM.

### Step Output References

```
{{steps.<step-id>.output}}
```

Replaced with the full text output of a previously completed step. The referenced step must be an upstream dependency (directly or transitively via `depends_on`).

```yaml
- id: summarize
  agent: aggregator
  prompt: |
    Summarize these findings:
    Security: {{steps.review-security.output}}
    Performance: {{steps.review-perf.output}}
  depends_on: [review-security, review-perf]
```

**Rules:**
- The referenced step must have completed before this step runs (enforced by DAG ordering).
- If a referenced step was **skipped** (condition not met), it has no output — the template resolution will fail.
- Template references are validated at parse time: unknown step IDs cause a validation error.

### Input References

```
{{inputs.<input-name>}}
```

Replaced with the value of a runtime input (from `--inputs` CLI flag or the YAML default).

```yaml
- id: analyze
  agent: security-reviewer
  prompt: "Review files matching: {{inputs.files}}"
```

**Resolution order for inputs:**
1. CLI `--inputs key=value` (highest priority)
2. Workflow YAML `inputs.<name>.default`
3. Error if neither exists and the template references it

---

## 8. Conditions — Conditional Step Execution

Steps can have a `condition` that gates execution based on a prior step's output. If the condition is not met, the step is **skipped** (status = `skipped`) and execution continues.

```yaml
- id: approve-action
  agent: aggregator
  prompt: "Generate an approval summary."
  depends_on: [decide]
  condition:
    step: decide
    contains: "APPROVE"
```

### Condition Fields

| Field | Type | Description |
|---|---|---|
| `step` | string | **Required.** The step ID whose output to check. |
| `contains` | string | True if the step's output contains this substring. |
| `not_contains` | string | True if the step's output does NOT contain this substring. |
| `equals` | string | True if the step's output (trimmed) exactly equals this string. |

Only one operator (`contains`, `not_contains`, or `equals`) should be specified per condition.

### Condition Rules

- The `step` referenced in a condition must be a **transitive upstream dependency**. The engine validates that you can reach the condition step by following the `depends_on` chain. This ensures the output is available when the condition is evaluated.
- If a condition struct is present but no operator is specified, the step always executes.
- `equals` comparison trims whitespace from both the output and the expected value before comparing.

### Example: Branching

```yaml
steps:
  - id: decide
    agent: decision-maker
    prompt: "Should we approve? Output APPROVE or REQUEST_CHANGES."

  - id: on-approve
    agent: aggregator
    prompt: "Generate approval message."
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"

  - id: on-changes
    agent: aggregator
    prompt: "List required changes."
    depends_on: [decide]
    condition:
      step: decide
      contains: "REQUEST_CHANGES"
```

Both `on-approve` and `on-changes` depend on `decide` and are in the same DAG level. Only the one whose condition matches will execute; the other is skipped.

---

## 9. Output Truncation

When step outputs are injected into subsequent prompts via `{{steps.X.output}}`, large outputs can overflow the LLM's context window or waste tokens. The `truncate` configuration limits injected output size.

```yaml
output:
  truncate:
    strategy: "chars"
    limit: 2000
```

### Strategies

| Strategy | Behavior |
|---|---|
| `chars` | Keep the first `limit` characters. |
| `lines` | Keep the first `limit` lines. |
| `tokens` | Approximate 1 token ≈ 4 characters; keep the first `limit × 4` characters. |

When truncation occurs, a suffix is appended:

```
... [truncated: 15000 chars total, showing first 2000]
```

If no `truncate` config is provided, outputs are injected in full.

---

## 10. Shared Memory

Shared memory provides a communication channel for agents running in parallel. It is a single memory file that any agent can read from and write to.

### Configuration

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true
    initial_content: "Review session started. Coordinate findings here."
    initial_file: ""               # Or load from a file path
```

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Enable shared memory |
| `inject_into_prompt` | bool | `false` | Prepend memory content to every prompt |
| `initial_content` | string | `""` | Seed content for the memory file |
| `initial_file` | string | `""` | Load initial content from a file |

### How It Works

- The shared memory file (`memory.md`) is created in the run's audit directory.
- **Writes** are serialized via a mutex — safe for concurrent goroutines.
- Each entry is timestamped and attributed to the writing agent:
  ```
  [2026-03-20T14:32:15Z] [security-reviewer] Found SQL injection in db/query.go:42
  ```
- When `inject_into_prompt` is `true`, the current memory content is prepended to every step's prompt in a clearly delimited block:
  ```
  --- Shared Memory (read-only context from other agents) ---
  [2026-03-20T14:32:15Z] [security-reviewer] Found SQL injection in db/query.go:42
  --- End Shared Memory ---

  <actual step prompt follows>
  ```
- When `inject_into_prompt` is `false`, agents can use the `read_memory` and `write_memory` tools (if registered) to interact with shared memory.

### Tool-Based Access

Two tools can be registered with SDK sessions:

| Tool | Description |
|---|---|
| `read_memory` | Returns the full shared memory content |
| `write_memory` | Appends a timestamped entry to shared memory |

> **Recommendation:** Use `inject_into_prompt: true`. LLMs may ignore optional tools, but they always see injected prompt content.

---

## 11. Audit Trail

Every workflow run creates a full audit trail for transparency, debugging, and reproducibility.

### Directory Structure

```
.workflow-runs/
└── 2026-03-20T14-32-05_code-review-pipeline/
    ├── workflow.meta.json       # Run metadata
    ├── workflow.yaml            # Snapshot of the workflow YAML
    ├── final_output.md          # Formatted output from specified steps
    ├── memory.md                # Shared memory final state (if enabled)
    └── steps/
        ├── 00_analyze/
        │   ├── step.meta.json   # Step metadata (agent, model, timing, status)
        │   ├── prompt.md        # The resolved prompt sent to the LLM
        │   └── output.md        # The LLM's response
        ├── 01_review-security/
        │   ├── step.meta.json
        │   ├── prompt.md
        │   └── output.md
        └── 01_review-performance/
            ├── step.meta.json
            ├── prompt.md
            └── output.md
```

#### `workflow.meta.json`

```json
{
  "workflow_name": "code-review-pipeline",
  "started_at": "2026-03-20T14:32:05Z",
  "completed_at": "2026-03-20T14:33:12Z",
  "status": "completed",
  "inputs": { "files": "src/**/*.go" },
  "config_hash": "a1b2c3d4e5f6a7b8"
}
```

#### `step.meta.json`

```json
{
  "step_id": "review-security",
  "agent": "security-reviewer",
  "agent_file": "/path/to/security-reviewer.agent.md",
  "model": "gpt-4o",
  "status": "completed",
  "started_at": "2026-03-20T14:32:10Z",
  "completed_at": "2026-03-20T14:32:45Z",
  "duration_seconds": 35.2,
  "output_file": "output.md",
  "depends_on": ["analyze"],
  "condition": null,
  "condition_result": true,
  "interactive": false,
  "session_id": "copilot-cli-2"
}
```

**Step directory naming:** Steps are prefixed with their DAG depth (zero-padded): `00_`, `01_`, `02_`. Steps at the same depth share the same prefix — this makes it visually obvious which steps ran in parallel.

### Retention Policy

The `config.audit_retention` field controls how many run directories are kept. When a new run starts, older directories beyond the limit are deleted (oldest first, sorted by directory name which includes the timestamp).

```yaml
config:
  audit_retention: 10    # Keep last 10 runs
```

Set to `0` to keep all runs (no automatic cleanup).

---

## 12. Interactive Mode — Clarification Questions

By default, goflow runs every step in **non-interactive mode**: the Copilot CLI flag `--no-ask-user` is passed, and agents cannot pause to ask the user for input. Interactive mode lifts this restriction, allowing agents to ask clarification questions mid-execution and wait for your response in the terminal.

### How It Works

When a step runs in interactive mode:

1. The `--no-ask-user` flag is **omitted** from the underlying Copilot CLI invocation.
2. The CLI's standard input is connected to your terminal, so the LLM can prompt you with a question.
3. Your answer is read from stdin and sent back to the agent.
4. The agent continues execution with your clarification.

Questions and choices are printed to **stderr** (so they don't mix with captured output), and your typed answer is read from **stdin**.

### Enabling Interactive Mode

There are three levels of control, evaluated in priority order:

| Level | Where | Effect |
|---|---|---|
| **Step override** (highest) | `steps[].interactive: true/false` | Forces this specific step interactive or non-interactive. |
| **Workflow config** | `config.interactive: true` | Enables interactive mode for all steps that don't have a step-level override. |
| **CLI flag** (lowest) | `--interactive` | Enables interactive mode for all steps that don't have a step or config override. |

A step is interactive if **any** of the three levels enables it and no higher-priority level disables it.

**CLI flag:**

```bash
goflow run --workflow pipeline.yaml --interactive
```

**Workflow config:**

```yaml
config:
  interactive: true    # All steps can ask questions
```

**Per-step override:**

```yaml
steps:
  - id: gather-requirements
    agent: requirements-agent
    prompt: "What features should this module support?"
    interactive: true    # This step can ask questions

  - id: generate-code
    agent: code-generator
    prompt: "Generate code based on: {{steps.gather-requirements.output}}"
    interactive: false   # This step runs silently
```

### Per-Step Control

The step-level `interactive` field is a pointer (`*bool` in Go). This three-state design means:

- **`interactive: true`** — always interactive, even if config and CLI say otherwise.
- **`interactive: false`** — never interactive, even if config or CLI enable it.
- **omitted** — inherits from `config.interactive` OR the `--interactive` CLI flag.

This lets you build workflows where most steps run silently but one specific step pauses for human input.

### Parallelism Behavior

When interactive steps appear in a parallel group (same DAG level), the orchestrator separates them:

1. **Non-interactive steps** run concurrently as usual.
2. **Interactive steps** run sequentially **after** all non-interactive steps in the group complete.

This prevents overlapping terminal prompts from multiple agents asking questions at the same time.

```
DAG Level 2:  [step-A (non-interactive), step-B (interactive), step-C (non-interactive)]

Execution:    step-A ──┐
              step-C ──┘──► step-B (waits, then runs alone)
```

### Audit Trail

The `step.meta.json` file includes an `interactive` field (`true`/`false`) that records whether the step ran in interactive mode. This is useful for understanding post-hoc which steps had human input.

---

## 13. Mock Mode — Testing Without LLMs

Mock mode lets you validate workflow structure, DAG ordering, template resolution, and conditions without calling the Copilot CLI or any LLM.

```bash
goflow run --workflow pipeline.yaml --mock --verbose
```

In mock mode:
- Every step returns the string `"mock output"` as its output.
- The mock executor supports substring-based response matching for targeted testing.
- The full audit trail is still produced.
- Template resolution and condition evaluation work normally.

Mock mode is useful for:
- Verifying that your DAG dependencies are correct.
- Testing conditional branches (though all outputs will be `"mock output"`).
- Checking that template variables resolve without errors.
- Validating the audit trail structure.

---

## 14. Practical Guide: Building Workflows

### Step 1: Define Your Agents

Start by identifying the specialized roles you need. Each agent should have a clear, focused responsibility.

**Create agent files** in the `agents/` directory (or `.github/agents/`):

```markdown
---
name: my-reviewer
description: Reviews code for X
tools:
  - grep
  - view
model: gpt-4o
---

# My Reviewer

You are an expert in X. Your job is to analyze code and report findings.

Always cite file paths and line numbers.
Rate issues by severity.
```

**Tips for good agent prompts:**
- Be specific about what the agent should focus on.
- Define the expected output format (bullet points, severity ratings, etc.).
- Tell the agent to cite file paths and line numbers when reviewing code.
- Keep prompts focused — a single agent shouldn't do everything.

### Step 2: Map Out Steps and Dependencies

Before writing YAML, sketch the dependency graph:

```
1. What steps do I need?
2. Which steps depend on which?
3. Which steps can run in parallel?
4. Are there any conditional branches?
```

Example sketch:
```
gather-context (no deps)
    ├── review-security (depends on gather-context)
    ├── review-perf (depends on gather-context)
    └── review-style (depends on gather-context)
                 ↓ (all three feed into)
              summarize (depends on all reviews)
                 ↓
              decide (depends on summarize)
              ├── approve (condition: APPROVE)
              └── request-changes (condition: REQUEST_CHANGES)
```

### Step 3: Write the Workflow YAML

Translate your sketch into YAML. Start with the agents, then write the steps in dependency order:

```yaml
name: "my-pipeline"
description: "Multi-phase code review"

inputs:
  files:
    description: "Files to review"
    default: "**/*.go"

config:
  audit_dir: ".workflow-runs"
  audit_retention: 5

agents:
  security:
    file: "./agents/security-reviewer.agent.md"
  perf:
    file: "./agents/performance-reviewer.agent.md"
  summarizer:
    inline:
      description: "Summarizes findings"
      prompt: "You are a technical lead who creates actionable summaries."
      tools: ["view"]

steps:
  - id: gather
    agent: security
    prompt: "List all files matching: {{inputs.files}}"

  - id: review-sec
    agent: security
    prompt: "Security review based on: {{steps.gather.output}}"
    depends_on: [gather]

  - id: review-perf
    agent: perf
    prompt: "Performance review based on: {{steps.gather.output}}"
    depends_on: [gather]

  - id: summarize
    agent: summarizer
    prompt: |
      Create a summary from:
      Security: {{steps.review-sec.output}}
      Performance: {{steps.review-perf.output}}
    depends_on: [review-sec, review-perf]

output:
  steps: [summarize]
  format: markdown
```

### Step 4: Test with Mock Mode

Validate the workflow structure before using real LLM calls:

```bash
goflow run --workflow my-pipeline.yaml --mock --verbose
```

Check:
- Does the workflow parse without errors?
- Are all agents found?
- Do the steps execute in the expected order?
- Does the audit trail look correct?

### Step 5: Run for Real

Once validated, run with the real executor:

```bash
goflow run --workflow my-pipeline.yaml \
  --inputs files='src/**/*.go' \
  --verbose
```

Review the audit trail in `.workflow-runs/` to see the exact prompts sent and outputs received.

---

## 15. Examples

### Minimal Sequential Workflow

Three steps running one after another, each depending on the previous:

```yaml
name: "simple-review"
description: "Sequential three-step code review"

inputs:
  files:
    description: "Files to review"
    default: "*.go"

config:
  audit_dir: ".workflow-runs"
  audit_retention: 5

agents:
  security-reviewer:
    file: "../agents/security-reviewer.agent.md"
  performance-reviewer:
    file: "../agents/performance-reviewer.agent.md"
  aggregator:
    file: "../agents/aggregator.agent.md"

steps:
  - id: security-review
    agent: security-reviewer
    prompt: |
      Review the following files for security vulnerabilities: {{inputs.files}}
      Focus on OWASP Top 10 issues.

  - id: perf-review
    agent: performance-reviewer
    prompt: |
      Review code for performance issues.
      Prior security findings: {{steps.security-review.output}}
    depends_on: [security-review]

  - id: summary
    agent: aggregator
    prompt: |
      Summarize all review findings:
      Security: {{steps.security-review.output}}
      Performance: {{steps.perf-review.output}}
    depends_on: [perf-review]

output:
  steps: [summary]
  format: markdown
```

**DAG:**
```
security-review → perf-review → summary
     (L0)            (L1)         (L2)
```

### Parallel Fan-Out / Fan-In Pipeline

One analysis step fans out to three parallel reviews, which fan back in to an aggregation step:

```yaml
name: "parallel-review"
description: "Fan-out to parallel reviewers, fan-in to aggregator"

inputs:
  files:
    description: "Files to review"
    default: "src/**/*.go"

config:
  audit_dir: ".workflow-runs"
  audit_retention: 10
  max_concurrency: 3
  shared_memory:
    enabled: true
    inject_into_prompt: true

agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"
  performance-reviewer:
    file: "./agents/performance-reviewer.agent.md"
  style-reviewer:
    inline:
      description: "Reviews code style"
      prompt: "You review code style, naming, and Go idioms."
      tools: ["grep", "view"]
  aggregator:
    file: "./agents/aggregator.agent.md"

steps:
  - id: analyze
    agent: security-reviewer
    prompt: "Analyze the codebase for: {{inputs.files}}"

  - id: review-security
    agent: security-reviewer
    prompt: "Security review. Context: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-performance
    agent: performance-reviewer
    prompt: "Performance review. Context: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-style
    agent: style-reviewer
    prompt: "Style review. Context: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: aggregate
    agent: aggregator
    prompt: |
      Combine reviews:
      Security: {{steps.review-security.output}}
      Performance: {{steps.review-performance.output}}
      Style: {{steps.review-style.output}}
    depends_on: [review-security, review-performance, review-style]

output:
  steps: [aggregate]
  format: markdown
```

**DAG:**
```
              analyze (L0)
             /   |    \
  review-sec  review-perf  review-style  (L1 — parallel)
             \   |    /
            aggregate (L2)
```

### Conditional Branching

A decision step triggers different follow-up actions based on its output:

```yaml
name: "conditional-review"
description: "Review with approval gate"

agents:
  reviewer:
    file: "./agents/security-reviewer.agent.md"
  decision-maker:
    inline:
      description: "Makes go/no-go decisions"
      prompt: "Output exactly APPROVE or REQUEST_CHANGES with explanation."
  action-agent:
    file: "./agents/aggregator.agent.md"

steps:
  - id: review
    agent: reviewer
    prompt: "Review the code for security issues."

  - id: decide
    agent: decision-maker
    prompt: "Based on this review, approve? {{steps.review.output}}"
    depends_on: [review]

  - id: on-approve
    agent: action-agent
    prompt: "Generate approval summary for PR."
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"

  - id: on-changes-needed
    agent: action-agent
    prompt: "List required changes: {{steps.review.output}}"
    depends_on: [decide]
    condition:
      step: decide
      contains: "REQUEST_CHANGES"

output:
  steps: [on-approve, on-changes-needed]
  format: markdown
```

**DAG:**
```
review (L0) → decide (L1) → on-approve (L2, conditional)
                           → on-changes-needed (L2, conditional)
```

Only one of the two conditional steps will produce output.

---

## 16. Troubleshooting

### "copilot CLI not found"

The real executor requires the `copilot` binary on `$PATH`. Verify:

```bash
which copilot
```

If not installed, use `--mock` for testing or install the Copilot CLI.

### "agent X not found"

- Check that the agent name in `steps.*.agent` exactly matches a key in the `agents` map.
- If using `file:`, verify the path is correct relative to the workflow file's location.
- Run with `--verbose` to see how many agents were resolved.

### "cycle detected among steps"

Your `depends_on` edges form a circular dependency. Check the listed step IDs and remove the cycle.

### "template references unknown step"

A `{{steps.X.output}}` reference uses a step ID that doesn't exist. Check for typos in the step ID.

### "condition step must be an upstream dependency"

The step referenced in a `condition.step` field is not reachable via the `depends_on` chain. The condition step must be a direct or transitive dependency so its output is guaranteed to exist.

### Steps execute in unexpected order

The DAG builder determines execution order, not the YAML ordering. Use `--verbose` to see which steps run at which level. Add `depends_on` edges to enforce ordering.

### Large outputs causing slow responses

Use the `truncate` configuration to limit output size:

```yaml
output:
  truncate:
    strategy: chars
    limit: 2000
```

### Audit directory growing too large

Set `config.audit_retention` to automatically clean up old runs:

```yaml
config:
  audit_retention: 10
```

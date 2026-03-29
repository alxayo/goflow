# goflow — User Guide & Tutorial

A step-by-step guide to building and running multi-agent AI workflows, from your first single-step workflow to advanced parallel pipelines with conditional branching, shared memory, and model selection.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Prerequisites & Installation](#2-prerequisites--installation)
3. [Quick Start — Your First Workflow](#3-quick-start--your-first-workflow)
   - [3.1 A Single-Step Workflow with an Inline Agent](#31-a-single-step-workflow-with-an-inline-agent)
   - [3.2 Running the Workflow](#32-running-the-workflow)
   - [3.3 Using Mock Mode](#33-using-mock-mode)
   - [3.4 Inspecting the Audit Trail](#34-inspecting-the-audit-trail)
4. [Adding Inputs — Parameterizing Your Workflow](#4-adding-inputs--parameterizing-your-workflow)
5. [Multi-Step Sequential Pipelines](#5-multi-step-sequential-pipelines)
   - [5.1 Chaining Steps with `depends_on`](#51-chaining-steps-with-depends_on)
   - [5.2 Passing Data Between Steps with Templates](#52-passing-data-between-steps-with-templates)
6. [Extracting Agents to Files](#6-extracting-agents-to-files)
   - [6.1 The `.agent.md` Format](#61-the-agentmd-format)
   - [6.2 Referencing Agent Files from a Workflow](#62-referencing-agent-files-from-a-workflow)
   - [6.3 Agent Discovery — Automatic File-Based Loading](#63-agent-discovery--automatic-file-based-loading)
   - [6.4 Claude-Format Agent Compatibility](#64-claude-format-agent-compatibility)
7. [Parallel Execution — Fan-Out & Fan-In](#7-parallel-execution--fan-out--fan-in)
   - [7.1 How the DAG Works](#71-how-the-dag-works)
   - [7.2 Fan-Out: One Step to Many](#72-fan-out-one-step-to-many)
   - [7.3 Fan-In: Many Steps to One](#73-fan-in-many-steps-to-one)
   - [7.4 Controlling Concurrency](#74-controlling-concurrency)
8. [Conditional Steps — Branching Logic](#8-conditional-steps--branching-logic)
   - [8.1 The `condition` Field](#81-the-condition-field)
   - [8.2 Decision Gates with `contains`](#82-decision-gates-with-contains)
   - [8.3 Using `not_contains` and `equals`](#83-using-not_contains-and-equals)
9. [Model Selection & Fallback](#9-model-selection--fallback)
   - [9.1 Workflow-Level Default Model](#91-workflow-level-default-model)
   - [9.2 Agent-Level Model Preferences](#92-agent-level-model-preferences)
   - [9.3 Step-Level Model Overrides](#93-step-level-model-overrides)
   - [9.4 Fallback Priority Chain](#94-fallback-priority-chain)
   - [9.5 BYOK — Bring Your Own Key Providers](#95-byok--bring-your-own-key-providers)
10. [Shared Memory — Cross-Agent Communication](#10-shared-memory--cross-agent-communication)
11. [Output Control](#11-output-control)
    - [11.1 Selecting Which Steps to Output](#111-selecting-which-steps-to-output)
    - [11.2 Output Formats](#112-output-formats)
    - [11.3 Output Truncation](#113-output-truncation)
12. [Tool Restrictions — Principle of Least Privilege](#12-tool-restrictions--principle-of-least-privilege)
13. [Skills — Reusable Knowledge Modules](#13-skills--reusable-knowledge-modules)
14. [MCP Servers — External Tool Integrations](#14-mcp-servers--external-tool-integrations)
15. [Audit Trail & Debugging](#15-audit-trail--debugging)
    - [15.1 Audit Directory Structure](#151-audit-directory-structure)
    - [15.2 Retention Policy](#152-retention-policy)
16. [Putting It All Together — Full Pipeline Example](#16-putting-it-all-together--full-pipeline-example)
17. [CLI Reference](#17-cli-reference)
18. [Troubleshooting](#18-troubleshooting)
19. [YAML Quick Reference](#19-yaml-quick-reference)

---

## 1. Introduction

goflow is a command-line tool that orchestrates multi-step AI agent pipelines. Instead of manually running agents one at a time and copy-pasting results between them, you describe the entire pipeline in a YAML file:

- **Which agents** to use (security reviewer, performance auditor, aggregator, …)
- **What prompts** to send to each agent
- **How steps depend on each other** — sequential, parallel, or conditional
- **How outputs flow** between steps via template variables

The engine parses your workflow, builds a directed acyclic graph (DAG) of dependencies, resolves agents, and executes every step in the correct order — automatically running independent steps in parallel when possible. Every run produces a complete audit trail.

### What You Will Learn

This guide walks you through the system progressively:

| Section | You Will Build | Concepts Introduced |
|---|---|---|
| [Quick Start](#3-quick-start--your-first-workflow) | A single-step workflow with an inline agent | Workflow YAML basics, inline agents, `run` command |
| [Inputs](#4-adding-inputs--parameterizing-your-workflow) | A parameterized workflow | `inputs`, `--inputs` CLI flag, `{{inputs.X}}` templates |
| [Multi-Step](#5-multi-step-sequential-pipelines) | A 3-step sequential pipeline | `depends_on`, `{{steps.X.output}}` templates |
| [Agent Files](#6-extracting-agents-to-files) | Reusable `.agent.md` files | Agent file format, discovery paths, Claude compatibility |
| [Parallelism](#7-parallel-execution--fan-out--fan-in) | A fan-out / fan-in pipeline | DAG levels, parallel execution, `max_concurrency` |
| [Conditions](#8-conditional-steps--branching-logic) | A decision gate with branches | `condition`, `contains`, `not_contains`, `equals` |
| [Model Selection](#9-model-selection--fallback) | Workflows with model overrides | Priority chain, fallback, BYOK providers |
| [Shared Memory](#10-shared-memory--cross-agent-communication) | Parallel agents that share context | `shared_memory`, prompt injection |
| [Full Pipeline](#16-putting-it-all-together--full-pipeline-example) | A production-grade review pipeline | Combines every concept |

---

## 2. Prerequisites & Installation

### Requirements

| Requirement | Notes |
|---|---|
| **Go 1.21+** | [Install Go](https://go.dev/doc/install) |
| **Copilot CLI** | Must be on `$PATH` or at `~/.copilot/copilot`. Not needed for `--mock` mode. |
| **macOS, Linux, or WSL** | Copilot CLI availability may vary |

### Build from Source

```bash
cd ~/Code/workflow-runner
go build -o goflow ./cmd/workflow-runner/main.go
```

This produces a `goflow` binary in the current directory. Verify it works:

```bash
./goflow
# Expected: usage output
```

### Verify Copilot CLI (Optional for Real Runs)

```bash
which copilot
# or
copilot --version
```

If Copilot CLI is not installed, you can still test all workflow structures using `--mock` mode.

---

## 3. Quick Start — Your First Workflow

### 3.1 A Single-Step Workflow with an Inline Agent

Create a file called `my-first-workflow.yaml`:

```yaml
name: "hello-workflow"
description: "My first workflow — a single step with an inline agent"

agents:
  greeter:
    inline:
      description: "A friendly assistant"
      prompt: "You are a friendly assistant. Be concise and helpful."

steps:
  - id: greet
    agent: greeter
    prompt: "Say hello and explain what goflow is in two sentences."

output:
  steps: [greet]
  format: markdown
```

**What is happening here:**

- **`name`** and **`description`** identify the workflow.
- **`agents`** defines one agent called `greeter` using an **inline** definition — no external file needed. The `prompt` field is the agent's system prompt (its persona/instructions).
- **`steps`** has a single step called `greet` that uses the `greeter` agent and sends it a user prompt.
- **`output`** specifies that we want the output from the `greet` step, formatted as markdown.

### 3.2 Running the Workflow

Run it with the Copilot CLI (real LLM):

```bash
./goflow run --workflow my-first-workflow.yaml --verbose
```

Or run it with mock mode (deterministic, no LLM required):

```bash
./goflow run --workflow my-first-workflow.yaml --mock --verbose
```

The `--verbose` flag prints step progress and timing to stderr. The final output goes to stdout.

### 3.3 Using Mock Mode

Mock mode is invaluable during workflow development. It:

- Returns `"mock output"` for every step
- Runs the full workflow pipeline (parsing, DAG, templates, conditions, audit)
- Produces a complete audit trail
- Requires no Copilot CLI or API key

Use it to validate your YAML structure, dependency graph, and template wiring before spending tokens on real LLM calls.

```bash
./goflow run --workflow my-first-workflow.yaml --mock --verbose
```

### 3.4 Inspecting the Audit Trail

Every run creates a timestamped directory under `.workflow-runs/`:

```bash
ls .workflow-runs/
# Example: 2026-03-26T10-15-30_hello-workflow/
```

Inside the run directory:

```
.workflow-runs/2026-03-26T10-15-30_hello-workflow/
├── workflow.meta.json       # Run metadata (timing, status, inputs)
├── workflow.yaml            # Snapshot of the workflow file
├── final_output.md          # The formatted final output
└── steps/
    └── 00_greet/
        ├── step.meta.json   # Step metadata (agent, model, timing)
        ├── prompt.md        # The resolved prompt sent to the LLM
        └── output.md        # The LLM's response
```

Read the output:

```bash
cat .workflow-runs/*/steps/00_greet/output.md
```

---

## 4. Adding Inputs — Parameterizing Your Workflow

Hard-coding values in prompts makes workflows rigid. Use **inputs** to make them configurable at runtime.

Create `parameterized-workflow.yaml`:

```yaml
name: "parameterized-review"
description: "A review workflow that accepts target files as input"

inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "*.go"
  focus_area:
    description: "What to focus on"
    default: "general code quality"

agents:
  reviewer:
    inline:
      description: "Reviews code"
      prompt: "You are a code reviewer. Be thorough and cite line numbers."
      tools: ["grep", "view"]

steps:
  - id: review
    agent: reviewer
    prompt: |
      Review the files matching: {{inputs.files}}
      Focus on: {{inputs.focus_area}}

output:
  steps: [review]
  format: markdown
```

**Key points:**

- **`inputs`** declares named variables with optional defaults.
- **`{{inputs.files}}`** and **`{{inputs.focus_area}}`** are template variables that get replaced at runtime.
- CLI `--inputs` values override the YAML defaults.

Run with defaults:

```bash
./goflow run --workflow parameterized-workflow.yaml --verbose
```

Override inputs from the CLI:

```bash
./goflow run --workflow parameterized-workflow.yaml \
  --inputs files='src/**/*.go' \
  --inputs focus_area='security vulnerabilities' \
  --verbose
```

---

## 5. Multi-Step Sequential Pipelines

Real workflows have multiple steps where each step builds on the output of the previous one.

### 5.1 Chaining Steps with `depends_on`

Create `sequential-pipeline.yaml`:

```yaml
name: "sequential-review"
description: "Three-step sequential analysis pipeline"

inputs:
  files:
    description: "Files to review"
    default: "*.go"

agents:
  analyzer:
    inline:
      description: "Analyzes code structure"
      prompt: "You analyze code structure and identify key areas of concern."
      tools: ["grep", "view"]
  reviewer:
    inline:
      description: "Reviews code in detail"
      prompt: "You are a detailed code reviewer. Be thorough."
      tools: ["grep", "view"]
  summarizer:
    inline:
      description: "Summarizes findings"
      prompt: "You create concise, actionable summaries of code review findings."

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze the structure of files: {{inputs.files}}"

  - id: review
    agent: reviewer
    prompt: |
      Perform a detailed review based on this analysis:
      {{steps.analyze.output}}
    depends_on: [analyze]

  - id: summarize
    agent: summarizer
    prompt: |
      Create an executive summary of these review findings:
      {{steps.review.output}}
    depends_on: [review]

output:
  steps: [summarize]
  format: markdown
```

**How it works:**

```
analyze (Level 0) → review (Level 1) → summarize (Level 2)
```

- `analyze` runs first — it has no `depends_on`.
- `review` runs after `analyze` completes, and receives `analyze`'s output via `{{steps.analyze.output}}`.
- `summarize` runs after `review`, receiving `review`'s output.

### 5.2 Passing Data Between Steps with Templates

The template syntax `{{steps.<step-id>.output}}` injects the full text output of a completed step into another step's prompt.

**Rules:**

- The referenced step must be an upstream dependency (directly or transitively via `depends_on`).
- Template references are validated at parse time — unknown step IDs cause an error.
- If a referenced step was **skipped** (condition not met), it has no output and template resolution will fail.

You can reference multiple step outputs in a single prompt:

```yaml
- id: combine
  agent: summarizer
  prompt: |
    Combine these findings:

    ## Security
    {{steps.security-review.output}}

    ## Performance
    {{steps.perf-review.output}}
  depends_on: [security-review, perf-review]
```

---

## 6. Extracting Agents to Files

Inline agents are quick to define but hard to reuse. For agents you use across multiple workflows, extract them to `.agent.md` files.

### 6.1 The `.agent.md` Format

An agent file uses YAML frontmatter plus a markdown body. The body becomes the agent's **system prompt** — the instructions sent to the LLM.

Create `agents/code-reviewer.agent.md`:

```markdown
---
name: code-reviewer
description: Reviews code for quality and best practices
tools:
  - grep
  - view
  - semantic_search
model: gpt-4o
---

# Code Reviewer

You are an expert code reviewer. Analyze code for:

1. **Bugs** — logic errors, off-by-one, null dereferences
2. **Code smells** — duplication, overly complex methods, poor naming
3. **Best practices** — error handling, input validation, documentation

Always cite specific file paths and line numbers.
Rate each issue by severity: CRITICAL, HIGH, MEDIUM, LOW.
```

**Frontmatter fields:**

| Field | Type | Description |
|---|---|---|
| `name` | string | Agent name (defaults to filename if omitted) |
| `description` | string | What the agent does |
| `tools` | []string | Tools the agent can use (see [§12](#12-tool-restrictions--principle-of-least-privilege)) |
| `model` | string or []string | Preferred LLM model(s) (see [§9](#9-model-selection--fallback)) |
| `agents` | []string | Sub-agents this agent can delegate to |
| `mcp-servers` | object | MCP server configurations (see [§14](#14-mcp-servers--external-tool-integrations)) |
| `handoffs` | []object | Agent-to-agent transition metadata |
| `hooks` | object | Session lifecycle hooks |

The **markdown body** (everything below the frontmatter) is the system prompt. Write clear, specific instructions: what the agent should focus on, what format to use, what to avoid.

### 6.2 Referencing Agent Files from a Workflow

Reference agent files with the `file` key in the `agents` map:

```yaml
agents:
  my-reviewer:
    file: "./agents/code-reviewer.agent.md"
```

The path is relative to the workflow file's location, not the current working directory. This allows workflows to be portable and runnable from any directory. You can also mix file-based and inline agents:

```yaml
agents:
  security-reviewer:
    file: "../agents/security-reviewer.agent.md"
  quick-summarizer:
    inline:
      description: "Quick summary agent"
      prompt: "You create brief summaries. Keep it under 100 words."
```

### 6.3 Agent Discovery — Automatic File-Based Loading

goflow searches for agent files in multiple standard locations, similar to how VS Code discovers custom agents. You only need to use the `file` key for agents outside these locations.

**Discovery priority (highest → lowest):**

| Priority | Location | Notes |
|---|---|---|
| 1 (highest) | Explicit `agents.*.file` in workflow YAML | Always wins |
| 2 | `.github/agents/*.agent.md` | Workspace-level GitHub agents |
| 3 | `.claude/agents/*.md` | Claude format — auto-normalized |
| 4 | `~/.copilot/agents/*.agent.md` | User-level global agents |
| 5 (lowest) | Paths in `config.agent_search_paths` | Custom directories |

If two agents share the same name across locations, the higher-priority location wins. Directories that don't exist are silently skipped.

You can add extra scan paths in your workflow config:

```yaml
config:
  agent_search_paths:
    - "./my-custom-agents"
    - "/shared/team-agents"
```

### 6.4 Claude-Format Agent Compatibility

Agent files found under `.claude/agents/` are automatically normalized:

- Comma-separated tool strings (e.g., `"Read, Grep, Bash"`) are split into arrays.
- Tool names are mapped to their Copilot CLI equivalents:

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

> **Note — Tool naming conventions differ across platforms.** Copilot CLI uses lowercase names (`grep`, `view`, `bash`), VS Code uses `category/toolName` identifiers (`search/textSearch`, `read/readFile`), and Claude Code uses PascalCase (`Read`, `Grep`, `Bash`). When writing agent files for goflow, use Copilot CLI names.

---

## 7. Parallel Execution — Fan-Out & Fan-In

This is where goflow really shines. When multiple steps depend on the same upstream step but not on each other, they are placed in the same DAG level and can run in parallel.

### 7.1 How the DAG Works

The workflow engine does **not** use an explicit `parallel: true` flag. Instead, parallelism is inferred from the dependency graph:

- Steps with **no `depends_on`** are entry steps (Level 0).
- Steps whose **dependencies are all satisfied** at the same time form a level together.
- Steps within the same level can run concurrently.
- Levels are processed in order — Level 0 first, then Level 1, then Level 2, etc.

Mental model:

| Pattern | How to Express |
|---|---|
| Run A before B | `B.depends_on: [A]` |
| Run B and C in parallel | Both depend on the same upstream step, not on each other |
| Wait for B and C, then run D | `D.depends_on: [B, C]` |

### 7.2 Fan-Out: One Step to Many

In a fan-out, one step's output is distributed to multiple downstream steps that run in parallel:

```yaml
name: "fan-out-demo"
description: "One analysis step fans out to three parallel reviews"

agents:
  analyzer:
    inline:
      description: "Analyzes code structure"
      prompt: "You analyze code structure."
      tools: ["grep", "view"]
  security-reviewer:
    inline:
      description: "Security reviewer"
      prompt: "You review code for security vulnerabilities. Cite file paths."
      tools: ["grep", "view"]
  perf-reviewer:
    inline:
      description: "Performance reviewer"
      prompt: "You review code for performance issues. Cite file paths."
      tools: ["grep", "view"]
  style-reviewer:
    inline:
      description: "Style reviewer"
      prompt: "You review code style and naming conventions."
      tools: ["grep", "view"]

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze all Go files in this project."

  - id: review-security
    agent: security-reviewer
    prompt: "Security review based on: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-performance
    agent: perf-reviewer
    prompt: "Performance review based on: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-style
    agent: style-reviewer
    prompt: "Style review based on: {{steps.analyze.output}}"
    depends_on: [analyze]

output:
  steps: [review-security, review-performance, review-style]
  format: markdown
```

**DAG:**
```
           analyze (Level 0)
          /       |         \
review-security  review-performance  review-style  (Level 1 — parallel)
```

All three reviews receive the same analysis output and run at the same time.

### 7.3 Fan-In: Many Steps to One

Fan-in is the complement of fan-out. An aggregation step waits for multiple parallel steps to complete, then combines their outputs:

```yaml
  - id: aggregate
    agent: aggregator
    prompt: |
      Combine all review results into a unified report:

      ## Security
      {{steps.review-security.output}}

      ## Performance
      {{steps.review-performance.output}}

      ## Style
      {{steps.review-style.output}}
    depends_on: [review-security, review-performance, review-style]
```

**The complete fan-out / fan-in pattern:**

```
           analyze (Level 0)
          /       |         \
review-security  review-perf  review-style  (Level 1 — parallel)
          \       |         /
           aggregate (Level 2)
```

`aggregate` only runs once **all three** reviews have completed.

### 7.4 Controlling Concurrency

By default, all steps in a level run at once. Use `max_concurrency` to limit how many goroutines run simultaneously:

```yaml
config:
  max_concurrency: 3    # At most 3 steps run in parallel
```

Set to `0` (the default) for unlimited concurrency.

---

## 8. Conditional Steps — Branching Logic

Conditional steps only execute if a prior step's output meets a criteria. This enables decision gates, approval flows, and branching pipelines.

### 8.1 The `condition` Field

A step's `condition` specifies which step's output to check and what operator to apply:

```yaml
- id: my-step
  agent: some-agent
  prompt: "..."
  depends_on: [decision-step]
  condition:
    step: decision-step          # Which step's output to check
    contains: "KEYWORD"          # Operator: contains, not_contains, or equals
```

If the condition is **not met**, the step is **skipped** (status = `skipped`). Skipped steps still count as "completed" for dependency purposes — downstream steps won't be blocked.

### 8.2 Decision Gates with `contains`

A common pattern is a decision step that outputs a keyword, followed by conditional branches:

```yaml
name: "approval-gate"
description: "Review with approval decision"

agents:
  reviewer:
    inline:
      description: "Code reviewer"
      prompt: "You review code for issues."
      tools: ["grep", "view"]
  decision-maker:
    inline:
      description: "Makes approval decisions"
      prompt: |
        Analyze the review findings and make a decision.
        Output exactly one of: APPROVE, REQUEST_CHANGES, or NEEDS_DISCUSSION.
        Follow with a brief explanation.
  action-agent:
    inline:
      description: "Takes action based on decisions"
      prompt: "You generate actionable summaries and to-do lists."

steps:
  - id: review
    agent: reviewer
    prompt: "Review all Go files in this project."

  - id: decide
    agent: decision-maker
    prompt: |
      Based on this review, should the code be approved?
      {{steps.review.output}}
    depends_on: [review]

  - id: on-approve
    agent: action-agent
    prompt: "Generate a concise approval summary for the PR."
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"

  - id: on-changes-needed
    agent: action-agent
    prompt: |
      Create a detailed list of required changes:
      {{steps.review.output}}
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
review (L0) → decide (L1) → on-approve (L2, conditional: APPROVE)
                           → on-changes-needed (L2, conditional: REQUEST_CHANGES)
```

Only one branch produces output based on the decision.

### 8.3 Using `not_contains` and `equals`

Three condition operators are available:

| Operator | Behavior |
|---|---|
| `contains: "STRING"` | True if output contains the substring (case-sensitive) |
| `not_contains: "STRING"` | True if output does NOT contain the substring |
| `equals: "STRING"` | True if output (trimmed) exactly matches the string |

Only one operator should be specified per condition.

```yaml
# Skip if there are no critical issues
- id: escalation
  agent: escalator
  prompt: "Escalate critical findings."
  depends_on: [review]
  condition:
    step: review
    contains: "CRITICAL"

# Run only if no issues found
- id: all-clear
  agent: reporter
  prompt: "Generate an all-clear report."
  depends_on: [review]
  condition:
    step: review
    not_contains: "CRITICAL"
```

**Important rule:** The `condition.step` must be a transitive upstream dependency. The engine validates that you can reach the condition step by following the `depends_on` chain, ensuring the output is available when the condition is evaluated.

---

## 9. Model Selection & Fallback

goflow supports model selection at three levels, creating a priority-ordered fallback chain.

### 9.1 Workflow-Level Default Model

Set a baseline model for all steps via `config.model`:

```yaml
config:
  model: "gpt-4o"    # Default for all steps
```

This is the lowest priority — it applies only when neither the agent nor the step specifies a model.

### 9.2 Agent-Level Model Preferences

Define preferred model(s) in the agent's `.agent.md` frontmatter. A single model:

```yaml
---
name: security-reviewer
model: gpt-4o
---
```

Or a priority list with fallbacks:

```yaml
---
name: security-reviewer
model:
  - gpt-5         # Try first
  - gpt-4o        # Fallback if gpt-5 unavailable
  - gpt-4o-mini   # Last resort
---
```

The executor tries each model in order. If a model is unavailable, it moves to the next one.

### 9.3 Step-Level Model Overrides

Override the model for a specific step directly in the workflow YAML:

```yaml
steps:
  - id: complex-analysis
    agent: security-reviewer
    model: claude-sonnet-4.5       # Overrides agent's model for this step
    prompt: "Deep analysis of authentication logic."
    depends_on: [gather-context]
```

Step-level overrides have the **highest priority**. Use them when a specific step requires more capability than the agent's default.

### 9.4 Fallback Priority Chain

When a step executes, models are tried in this order:

```
1. Step-level model       (highest priority)
2. Agent-level model(s)   (in order, if a list)
3. Workflow config.model  (lowest explicit priority)
4. Copilot CLI default    (if all above are unavailable)
```

**Example resolution:**

```yaml
# Workflow config
config:
  model: gpt-4

# Agent file
---
model:
  - gpt-5
  - gpt-4o
---

# Step definition
steps:
  - id: scan
    agent: security-reviewer
    model: claude-sonnet-4.5
```

Resolved priority list: `["claude-sonnet-4.5", "gpt-5", "gpt-4o", "gpt-4"]`

Duplicates are automatically removed while preserving priority order. Only "model unavailable" errors trigger fallback; other errors (network, auth) cause immediate failure.

### 9.5 BYOK — Bring Your Own Key Providers

Use your own API key and model provider instead of the Copilot-hosted models:

```yaml
config:
  model: "my-custom-model"
  provider:
    type: "openai"
    base_url: "https://my-api.example.com/v1"
    api_key_env: "MY_API_KEY"     # References an environment variable
```

Or via environment variables:

```bash
export COPILOT_PROVIDER_BASE_URL=https://my-api.example.com/v1
export COPILOT_PROVIDER_API_KEY=sk-...
export COPILOT_MODEL=my-custom-model
```

---

## 10. Shared Memory — Cross-Agent Communication

When agents run in parallel, they are isolated by default — each has its own session. Shared memory provides a communication channel for agents to coordinate findings during a parallel execution round.

### Enabling Shared Memory

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true           # Recommended
    initial_content: |
      # Shared Review Context
      Review session started. Record findings here.
```

### How It Works

1. A `memory.md` file is created in the run's audit directory.
2. It is initialized with the `initial_content` (or loaded from `initial_file`).
3. Agents can write timestamped, attributed entries to the memory file.
4. Writes are serialized via a mutex — safe for concurrent goroutines.

Each entry looks like:
```
[2026-03-26T14:32:15Z] [security-reviewer] Found SQL injection in db/query.go:42
```

### Prompt Injection vs Tool Access

| Mode | How It Works | Recommendation |
|---|---|---|
| `inject_into_prompt: true` | Memory content is prepended to every step's prompt | **Recommended.** The LLM always sees it. |
| `inject_into_prompt: false` | Agents use `read_memory` / `write_memory` tools | LLMs may ignore optional tools. |

When `inject_into_prompt` is `true`, each step's prompt starts with:

```
--- Shared Memory (read-only context from other agents) ---
[2026-03-26T14:32:15Z] [security-reviewer] Found SQL injection in db/query.go:42
--- End Shared Memory ---

<actual step prompt follows>
```

### Full Example

```yaml
name: "coordinated-review"
description: "Parallel reviewers that share context"

config:
  max_concurrency: 3
  shared_memory:
    enabled: true
    inject_into_prompt: true
    initial_content: |
      # Code Review Coordination
      Log important findings here so other reviewers can see them.

agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"
  performance-reviewer:
    file: "./agents/performance-reviewer.agent.md"
  aggregator:
    file: "./agents/aggregator.agent.md"

steps:
  - id: review-security
    agent: security-reviewer
    prompt: "Review all Go files for security vulnerabilities."

  - id: review-performance
    agent: performance-reviewer
    prompt: "Review all Go files for performance issues."

  - id: aggregate
    agent: aggregator
    prompt: |
      Combine the reviews:
      Security: {{steps.review-security.output}}
      Performance: {{steps.review-performance.output}}
    depends_on: [review-security, review-performance]

output:
  steps: [aggregate]
  format: markdown
```

---

## 11. Output Control

### 11.1 Selecting Which Steps to Output

The `output.steps` field controls which step outputs are included in the final printed result:

```yaml
output:
  steps: [summary, aggregate]    # Only these steps appear in output
```

If `steps` is empty or omitted, all completed steps are included (sorted alphabetically).

### 11.2 Output Formats

Three formats are available:

| Format | Description |
|---|---|
| `markdown` | Each step output under a `## Step: <id>` heading |
| `json` | A JSON object with `steps.<id>.status` and `steps.<id>.output` |
| `plain` | Delimited with `=== <id> ===` separators |

```yaml
output:
  format: "json"    # or "markdown" or "plain"
```

### 11.3 Output Truncation

When step outputs are injected into downstream prompts via `{{steps.X.output}}`, large outputs can overflow the LLM's context window. Configure truncation to limit injected output size:

```yaml
output:
  truncate:
    strategy: "chars"     # Strategy: chars, lines, or tokens
    limit: 2000           # Maximum units to keep
```

| Strategy | Behavior |
|---|---|
| `chars` | Keep the first `limit` characters |
| `lines` | Keep the first `limit` lines |
| `tokens` | Approximate: 1 token ≈ 4 characters; keep `limit × 4` characters |

When truncation occurs, a suffix is appended:
```
... [truncated: 15000 chars total, showing first 2000]
```

---

## 12. Tool Restrictions — Principle of Least Privilege

Each agent can be restricted to a specific set of tools. This is defined in the agent's `tools` field.

### Available Built-In Tools

These are the Copilot CLI tool names accepted in agent `tools` fields:

| Tool | Purpose |
|---|---|
| `grep` | Text search in files (regex and literal) |
| `view` | Read file contents |
| `edit` | Edit existing files |
| `create_file` | Create new files |
| `glob` | Glob-based file discovery |
| `list_dir` | List directory contents |
| `semantic_search` | Semantic code search |
| `bash` | Execute shell commands |
| `fetch_webpage` | Fetch external web content |

> **Tip:** Both YAML formats are valid for the `tools` field: `tools: ['grep', 'view']` (flow) or as a block list.

### Restricting Tools

In an agent file:

```yaml
---
name: read-only-reviewer
tools:
  - grep
  - view
  - semantic_search
---
```

This agent can **only** read code. It cannot edit files, create files, or run terminal commands.

In an inline agent:

```yaml
agents:
  safe-scanner:
    inline:
      description: "Read-only scanner"
      prompt: "You analyze code. Never modify files."
      tools: ["grep", "view", "semantic_search"]
```

**If `tools` is omitted or `null`**, the agent inherits access to **all** session tools. Explicitly list tools to enforce least-privilege access.

### Common Tool Profiles

| Profile | Tools | Use Case |
|---|---|---|
| Read-only reviewer | `grep`, `view`, `semantic_search` | Code analysis, security scanning |
| Web-enabled scanner | `fetch_webpage`, `view` | Fetching external content |
| Full access | *(omit `tools`)* | Complex multi-step tasks that need file editing |
| File editor | `view`, `edit`, `create_file`, `bash` | Code generation, refactoring |

---

## 13. Skills — Reusable Knowledge Modules

Skills are markdown files (`SKILL.md`) that inject domain-specific knowledge into an agent's session context. They function like reusable prompt libraries.

### Directory Structure

```
skills/
├── code-review/
│   └── SKILL.md
└── security-best-practices/
    └── SKILL.md
```

### SKILL.md Format

```markdown
---
name: code-review
description: Specialized code review guidelines
---

# Code Review Guidelines

When reviewing code, always check for:

1. **Error Handling** — Are all errors handled? No swallowed errors?
2. **Input Validation** — Are inputs validated at system boundaries?
3. **Resource Management** — Are connections/files properly closed?
4. **Naming** — Are names descriptive and consistent?
```

### Attaching Skills

At the workflow level (applies to all steps):

```yaml
skills:
  - "./skills/code-review"
  - "./skills/security-best-practices"
```

At the step level (applies to only that step):

```yaml
steps:
  - id: security-scan
    agent: security-reviewer
    prompt: "Scan for vulnerabilities."
    skills:
      - "security-best-practices"
```

> **Note:** The parser recognizes skill references, but skill content injection into SDK sessions may not yet be fully wired in the current implementation. Skills defined as directories are passed via `SkillDirectories` in the session configuration.

---

## 14. MCP Servers — External Tool Integrations

MCP (Model Context Protocol) servers extend what tools an agent has access to — databases, APIs, external scanners, and more. They are defined per-agent in the `.agent.md` frontmatter.

### Defining MCP Servers

```yaml
---
name: db-analyst
description: Analyzes database queries
tools:
  - grep
  - view
  - db-tools/*        # All tools from the db-tools MCP server
mcp-servers:
  db-tools:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"]
    env:
      DB_TIMEOUT: "30"
---
```

The `db-tools/*` glob in `tools` exposes all tools provided by the `db-tools` MCP server to this agent.

### MCP in Docker

```yaml
mcp-servers:
  security-scanner:
    command: docker
    args: ["run", "--rm", "security-scanner:latest"]
    env:
      SCAN_DEPTH: "3"
```

### Per-Agent Isolation

MCP servers defined in an agent file are scoped to that agent — other agents in the same workflow won't see them unless they also define the same MCP server. This provides natural tool isolation.

---

## 15. Audit Trail & Debugging

### 15.1 Audit Directory Structure

Every workflow run creates a timestamped directory containing a complete record of what happened:

```
.workflow-runs/
└── 2026-03-26T14-32-05_my-pipeline/
    ├── workflow.meta.json       # Run metadata
    ├── workflow.yaml            # Snapshot of the workflow file
    ├── final_output.md          # Formatted output
    ├── memory.md                # Shared memory state (if enabled)
    └── steps/
        ├── 00_analyze/
        │   ├── step.meta.json   # Agent, model, timing, status
        │   ├── prompt.md        # The resolved prompt
        │   └── output.md        # The LLM's response
        ├── 01_review-security/
        │   └── ...
        └── 01_review-perf/      # Same prefix = ran in parallel
            └── ...
```

**Step directory naming:** Steps are prefixed with their DAG depth (zero-padded). Steps at the same depth share the same number, making it visually clear which steps ran in parallel.

**`workflow.meta.json` example:**

```json
{
  "workflow_name": "my-pipeline",
  "started_at": "2026-03-26T14:32:05Z",
  "completed_at": "2026-03-26T14:33:12Z",
  "status": "completed",
  "inputs": { "files": "src/**/*.go" }
}
```

**`step.meta.json` example:**

```json
{
  "step_id": "review-security",
  "agent": "security-reviewer",
  "agent_file": "./agents/security-reviewer.agent.md",
  "model": "gpt-4o",
  "status": "completed",
  "started_at": "2026-03-26T14:32:10Z",
  "completed_at": "2026-03-26T14:32:45Z",
  "duration_seconds": 35.2,
  "depends_on": ["analyze"],
  "condition": null,
  "condition_result": true,
  "session_id": "copilot-cli-2"
}
```

### 15.2 Retention Policy

Control how many run directories are kept with `audit_retention`. When a new run starts, older directories beyond the limit are deleted (oldest first).

```yaml
config:
  audit_retention: 10    # Keep last 10 runs (0 = keep all)
```

---

## 16. Putting It All Together — Full Pipeline Example

This example combines every concept: file-based agents, inline agents, parallel fan-out, fan-in aggregation, conditional branching, model selection, shared memory, and output truncation.

Create `full-pipeline.yaml`:

```yaml
name: "code-review-pipeline"
description: "Multi-agent code review with parallel analysis, approval gate, and shared memory"

inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "src/**/*.go"
  severity_filter:
    description: "Minimum severity level"
    default: "MEDIUM"

config:
  model: "gpt-4o"                         # Workflow-level default model
  audit_dir: ".workflow-runs"
  audit_retention: 10
  max_concurrency: 3
  shared_memory:
    enabled: true
    inject_into_prompt: true
    initial_content: |
      # Shared Review Context
      Review session started. Coordinate findings here.

agents:
  # File-based agents (reusable across workflows)
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"
  performance-reviewer:
    file: "./agents/performance-reviewer.agent.md"
  aggregator:
    file: "./agents/aggregator.agent.md"

  # Inline agents (specific to this workflow)
  style-reviewer:
    inline:
      description: "Reviews code style and naming conventions"
      prompt: |
        You are a code style reviewer. Check naming conventions,
        formatting, readability, and Go idioms. Cite file paths
        and line numbers. Rate issues: HIGH, MEDIUM, LOW.
      tools: ["grep", "glob", "view"]
  decision-maker:
    inline:
      description: "Makes approval decisions based on review results"
      prompt: |
        You analyze review results and make a go/no-go decision.
        Output exactly one of: APPROVE, REQUEST_CHANGES, or NEEDS_DISCUSSION.
        Follow your output with a brief explanation.

steps:
  # Level 0 — Entry point
  - id: analyze
    agent: security-reviewer
    prompt: "Analyze the codebase structure for files matching: {{inputs.files}}"

  # Level 1 — Fan-out: three parallel reviews
  - id: review-security
    agent: security-reviewer
    model: gpt-5                           # Step-level model override for deeper analysis
    prompt: |
      Review this code for security issues.
      Focus on severity >= {{inputs.severity_filter}}.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]

  - id: review-performance
    agent: performance-reviewer
    prompt: |
      Review this code for performance issues.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]

  - id: review-style
    agent: style-reviewer
    prompt: |
      Review this code for style issues.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]

  # Level 2 — Fan-in: aggregate all reviews
  - id: aggregate
    agent: aggregator
    prompt: |
      Combine these review results into a unified report:

      ## Security Review
      {{steps.review-security.output}}

      ## Performance Review
      {{steps.review-performance.output}}

      ## Style Review
      {{steps.review-style.output}}
    depends_on: [review-security, review-performance, review-style]

  # Level 3 — Decision gate
  - id: decide
    agent: decision-maker
    prompt: |
      Based on this review report, should this code be approved?
      {{steps.aggregate.output}}
    depends_on: [aggregate]

  # Level 4 — Conditional branches
  - id: approve-action
    agent: aggregator
    prompt: "Generate a concise approval summary suitable for a PR comment."
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"

  - id: changes-action
    agent: aggregator
    prompt: |
      Generate a detailed list of required changes based on:
      {{steps.aggregate.output}}
    depends_on: [decide]
    condition:
      step: decide
      contains: "REQUEST_CHANGES"

output:
  steps: [approve-action, changes-action, aggregate]
  format: markdown
  truncate:
    strategy: chars
    limit: 2000
```

**DAG visualization:**

```
                   analyze (L0)
                  /    |     \
    review-security  review-perf  review-style  (L1 — parallel)
                  \    |     /
                  aggregate (L2)
                      |
                   decide (L3)
                  /          \
   approve-action            changes-action  (L4 — conditional)
   (if APPROVE)              (if REQUEST_CHANGES)
```

**Run it:**

```bash
# Test with mock mode first
./goflow run --workflow full-pipeline.yaml --mock --verbose

# Then run for real
./goflow run --workflow full-pipeline.yaml \
  --inputs files='pkg/**/*.go' \
  --inputs severity_filter='HIGH' \
  --verbose
```

---

## 17. CLI Reference

```
goflow run [options]
```

| Flag | Required | Description |
|---|---|---|
| `--workflow` | Yes | Path to the workflow YAML file |
| `--inputs` | No | Key=value input pair (repeatable for multiple inputs) |
| `--audit-dir` | No | Override the audit directory (default: from `config.audit_dir`) |
| `--mock` | No | Use mock executor — no LLM calls, returns `"mock output"` |
| `--verbose` | No | Print step statuses, timing, and debug info to stderr |

### Exit Codes

| Code | Meaning |
|---|---|
| `0` | Workflow completed successfully |
| `1` | Error (parse failure, agent not found, step execution failure, etc.) |

### Examples

```bash
# Basic run
./goflow run --workflow pipeline.yaml

# With inputs and verbose output
./goflow run --workflow pipeline.yaml \
  --inputs files='src/**/*.go' \
  --inputs severity_filter='HIGH' \
  --verbose

# Mock mode for testing
./goflow run --workflow pipeline.yaml --mock --verbose

# Custom audit directory
./goflow run --workflow pipeline.yaml --audit-dir ./my-audit-logs

# Run example workflow from repository root (agent paths resolve relative to workflow file)
go run ./cmd/workflow-runner run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

---

## 18. Troubleshooting

### "copilot CLI not found"

The real executor requires the `copilot` binary on `$PATH`. Verify with `which copilot`. If not installed, use `--mock` mode for testing.

### "agent X not found"

- Check that the agent name in `steps.*.agent` exactly matches a key in the `agents` map.
- If using `file:`, verify the path is correct relative to the workflow file's location.
- Run with `--verbose` to see how many agents were resolved.

### "cycle detected among steps"

Your `depends_on` edges form a circular dependency. Review the listed step IDs and remove the cycle.

### "template references unknown step"

A `{{steps.X.output}}` reference uses a step ID that doesn't exist. Check for typos.

### "condition step must be an upstream dependency"

The `condition.step` field references a step not reachable via the `depends_on` chain. Add the missing dependency.

### Steps execute in unexpected order

The DAG builder determines order, not YAML position. Use `--verbose` to see which steps run at which level. Add `depends_on` edges to enforce ordering.

### Large outputs causing slow responses

Enable output truncation:

```yaml
output:
  truncate:
    strategy: chars
    limit: 2000
```

### Audit directory growing too large

Set `config.audit_retention` to automatically clean up old runs.

### "All models unavailable" error

Every model in the fallback chain was unavailable. Check network connectivity, API key validity, and model name spelling.

### Model not being used

- Step-level `model` has highest priority — check if it's set.
- Then agent's `model` frontmatter.
- Then workflow `config.model`.
- Duplicates are removed; first occurrence wins.

---

## 19. YAML Quick Reference

A complete reference card for every field in a workflow YAML file.

### Top Level

```yaml
name: "string"                    # Required. Workflow name.
description: "string"             # Optional. Description.
inputs: { ... }                   # Optional. Runtime variables.
config: { ... }                   # Optional. Global settings.
agents: { ... }                   # Required. Agent definitions.
skills: [...]                     # Optional. Skill directories.
steps: [...]                      # Required. Step definitions.
output: { ... }                   # Optional. Output formatting.
```

### Inputs

```yaml
inputs:
  <name>:
    description: "string"         # Human-readable description
    default: "string"             # Default value (overridden by --inputs)
```

### Config

```yaml
config:
  model: "string"                 # Default LLM model
  audit_dir: "string"             # Audit directory (default: .workflow-runs)
  audit_retention: int            # Max runs to keep (0 = unlimited)
  max_concurrency: int            # Max parallel steps (0 = unlimited)
  log_level: "string"             # debug, info, warn, error
  agent_search_paths: [...]       # Extra directories for agent discovery
  shared_memory:
    enabled: bool
    inject_into_prompt: bool
    initial_content: "string"
    initial_file: "string"
  provider:                       # BYOK settings
    type: "string"
    base_url: "string"
    api_key_env: "string"
```

### Agents

```yaml
agents:
  <name>:
    file: "path/to/agent.agent.md"    # File-based (mutually exclusive with inline)
  <name>:
    inline:                            # Inline definition
      description: "string"
      prompt: "string"                 # System prompt
      tools: ["tool1", "tool2"]        # Available tools
      model: "string"                  # Model override
```

### Steps

```yaml
steps:
  - id: "string"                  # Required. Unique identifier.
    agent: "string"               # Required. Agent name from agents map.
    prompt: "string"              # Required. Supports {{templates}}.
    model: "string"               # Optional. Step-level model override.
    depends_on: ["step-id"]       # Optional. Upstream dependencies.
    condition:                    # Optional. Conditional execution.
      step: "string"             #   Step ID to check.
      contains: "string"         #   Output contains substring.
      not_contains: "string"     #   Output does NOT contain substring.
      equals: "string"           #   Output exactly matches (trimmed).
    skills: ["skill-name"]       # Optional. Per-step skills.
```

### Output

```yaml
output:
  steps: ["step-id"]             # Steps to include in output (default: all)
  format: "markdown"             # markdown, json, or plain
  truncate:
    strategy: "chars"            # chars, lines, or tokens
    limit: 2000                  # Maximum units
```

### Template Variables

| Syntax | Resolves To |
|---|---|
| `{{inputs.<name>}}` | Runtime input value |
| `{{steps.<id>.output}}` | Completed step's output text |

# goflow — Feasibility Analysis & Plan

## Executive Summary

**Verdict: Feasible, with caveats.** The Copilot SDK provides all the core primitives needed to build a YAML-driven goflow with fan-out/fan-in, conditional branching, and agent orchestration. However, the SDK does not provide a built-in workflow engine — you must build the orchestration layer yourself on top of the session/tool/agent primitives. The good news: nothing in the SDK's architecture blocks this.

---

## Part 1: Copilot SDK Capabilities Analysis

### What the SDK Provides

| Capability | SDK Support | Notes |
|---|---|---|
| **Multiple concurrent sessions** | ✅ Native | One client can spawn N independent sessions, each with its own model, tools, and conversation history. Demonstrated in the cookbook's `multiple-sessions.go`. |
| **Custom Agents (sub-agents)** | ✅ Native | Define named agents with scoped system prompts, tool restrictions, and MCP servers. Runtime can auto-delegate based on intent. Events: `subagent.started`, `subagent.completed`, `subagent.failed`. |
| **Custom Skills** | ✅ Native | Load reusable `SKILL.md` prompt modules from directories via `skillDirectories`. |
| **Custom Tools** | ✅ Native | Define Go/TS/Python/C# functions as tools the LLM can call. Handlers run concurrently when the model invokes multiple tools. |
| **Session Hooks** | ✅ Native | `OnPreToolUse`, `OnPostToolUse`, `OnUserPromptSubmitted`, `OnSessionStart`, `OnSessionEnd`, `OnErrorOccurred` — with retry/skip/abort strategies. |
| **Streaming events** | ✅ Native | 40+ event types including `session.idle` (completion signal), `assistant.message`, `subagent.*` lifecycle events. |
| **BYOK / Custom Providers** | ✅ Native | OpenAI, Azure, Anthropic, Ollama, Foundry Local. No GitHub subscription required with BYOK. |
| **Session persistence & resume** | ✅ Native | `ResumeSession()` with session IDs. Infinite sessions with automatic compaction. |
| **MCP Server integration** | ✅ Native | Per-session and per-agent MCP server configuration. |
| **OpenTelemetry tracing** | ✅ Native | Distributed tracing with W3C Trace Context propagation between SDK ↔ CLI. |

### Patterns for Multi-Agent Execution

#### Sequential (Pipeline)
```
Agent A → Agent B → Agent C
```
Create sessions sequentially. Send a prompt to Session A, wait for `session.idle`, extract the result from `assistant.message`, then pass it as input to Session B.

**SDK mechanism:** `session.Send()` + event listener for `session.idle` → next session's `Send()`.

#### Parallel (Fan-Out)
```
         ┌→ Agent B
Agent A ─┤→ Agent C
         └→ Agent D
```
Create multiple sessions (B, C, D) and call `session.Send()` on all of them concurrently (goroutines in Go, Promise.all in TS, asyncio.gather in Python). Each session runs independently.

**SDK mechanism:** Multiple `CreateSession()` calls + concurrent `Send()` calls. The SDK explicitly supports this — sessions are independent and share one CLI server process.

#### Fan-In (Aggregation)
```
Agent B ─┐
Agent C ─┤→ Agent E (aggregator)
Agent D ─┘
```
Wait for all parallel sessions to reach `session.idle`, collect their outputs, then compose a prompt for the aggregator session.

**SDK mechanism:** Channel/event synchronization (`sync.WaitGroup` in Go, `Promise.all` in TS). No built-in fan-in primitive — you build it.

#### Conditional Branching
```
Agent A → if condition → Agent B
                       → Agent C
```
Inspect the output of Agent A (from `assistant.message` event data), evaluate a condition (string match, JSON field check, LLM-based classification), and route to the appropriate next session.

**SDK mechanism:** Your orchestrator code parses the result and makes the routing decision. The SDK gives you the event data; branching logic is yours.

### What the SDK Does NOT Provide

| Gap | Impact | Mitigation |
|---|---|---|
| **No built-in workflow engine** | You must build the DAG execution, step sequencing, fan-out/fan-in synchronization, and conditional routing yourself. | This is the core of what we're building. Standard patterns in Go (goroutines + channels + WaitGroups). |
| **No YAML workflow DSL** | The SDK has no concept of a workflow definition file. | We define our own YAML schema and parser. |
| **No native inter-session data passing** | Sessions are isolated. One session's output doesn't automatically flow to another. | The orchestrator extracts `assistant.message` content and injects it into the next session's prompt. |
| **No VS Code `.agent.md` file parsing** | The SDK's `customAgents` takes programmatic config, not `.agent.md` files. VS Code agent files have rich YAML frontmatter (name, description, tools, agents, model, handoffs, hooks, mcp-servers) + markdown body. | We parse `.agent.md` files ourselves with full frontmatter support. We also scan VS Code discovery paths (`.github/agents/`, `.claude/agents/`, `~/.copilot/agents/`). Claude-format agents are also supported with tool name mapping. |
| **No VS Code `SKILL.md` integration out-of-box** | The SDK does support `skillDirectories`, but we need to bridge from our YAML references to actual paths. | Map skill references in workflow YAML → absolute paths → pass to `SessionConfig.SkillDirectories`. |
| **Agent inference is runtime-controlled** | The Copilot runtime decides which sub-agent to invoke based on prompt intent. You can't force "run agent X now" deterministically via `customAgents`. | **Key workaround:** Instead of using `customAgents` delegation, create a dedicated session per agent step with that agent's system prompt and tools. This gives deterministic control. Alternatively, use `agent` field in SessionConfig to pre-select. |

---

## Part 2: Architecture Plan

### Session Resume Capability

**Clarification:** The Copilot SDK natively supports resuming sessions via `ResumeSession(session_id)`.
The "no mid-workflow resume" mentioned in Phase 6 refers to **workflow-level** checkpoints, not individual
session state.

**For MVP:** The runner will persist `session_id` in each step's `step.meta.json`. If a step fails mid-way
and the user re-runs the workflow with `--resume-step <step-id>`, the orchestrator will attempt to resume
that session from where it left off via the SDK's `ResumeSession()` primitive. This is straightforward to
implement in Phase 1.

### High-Level Architecture

```
┌──────────────────────────────────────────────────────┐
│                   workflow.yaml                       │
│  (defines steps, agents, fan-out, fan-in, conditions) │
└──────────────────┬───────────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────────┐
│                  goflow (Go binary)                   │
│                                                       │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐ │
│  │ YAML Parser  │  │ DAG Builder  │  │  Executor    │ │
│  │ + Validator  │→ │ (topo sort)  │→ │ (goroutines) │ │
│  └─────────────┘  └──────────────┘  └─────────────┘ │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │           Copilot SDK Client (shared)            │ │
│  │  Session A  │  Session B  │  Session C  │  ...   │ │
│  └─────────────────────────────────────────────────┘ │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │         Agent/Skill File Loader                  │ │
│  │  .agent.md → SessionConfig                       │ │
│  │  SKILL.md  → SkillDirectories                    │ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────────┐
│             Copilot CLI (server mode)                 │
│             JSON-RPC over stdio/TCP                   │
└──────────────────────────────────────────────────────┘
```

### Workflow YAML Schema (Draft)

```yaml
# workflow.yaml
name: "code-review-pipeline"
description: "Automated code review with parallel analysis and aggregation"

# Workflow inputs — variables passed at runtime via CLI flags
inputs:
  files:
    description: "Comma-separated list of files to review"
    default: "src/**/*.go"
  target_branch:
    description: "Target branch for comparison"
    default: "main"
  severity_filter:
    description: "Minimum severity level (LOW, MEDIUM, HIGH, CRITICAL)"
    default: "MEDIUM"

# Global settings
config:
  model: "gpt-5"                    # Default model for all steps
  audit_dir: ".workflow-runs"       # Audit trail output directory
  audit_retention: 10               # Keep last N runs (0 = infinite)
  shared_memory:
    enabled: false                  # Enable shared memory between parallel agents
    inject_into_prompt: true        # Force inject memory into prompts instead of relying on tool calls

  provider:                          # Optional BYOK config
    type: "openai"
    base_url: "https://api.openai.com/v1"
    api_key_env: "OPENAI_API_KEY"   # Reference env var, never hardcode
  streaming: true
  log_level: "info"
  audit_dir: "./.workflow-runs"     # Where audit trails are written
  agent_search_paths:                # Additional paths to discover .agent.md files
    - "./my-agents"
    - "/shared/team-agents"
  telemetry:
    otlp_endpoint: "http://localhost:4318"
  shared_memory:                     # Shared memory for parallel agents
    enabled: true
    initial_content: |
      # Shared Context
      Repository: my-app
      Branch: feature/auth-refactor
# Agent definitions — can reference .agent.md files or inline
agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"   # Load from VS Code-style agent file
  
  performance-reviewer:
    file: "./agents/performance-reviewer.agent.md"
  
  style-reviewer:
    inline:                                         # Or define inline
      description: "Reviews code style and naming conventions"
      prompt: "You are a code style reviewer. Check naming conventions, formatting, and readability."
      tools: ["grep", "glob", "view"]
  
  aggregator:
    file: "./agents/aggregator.agent.md"

  decision-maker:
    inline:
      description: "Decides if code is ready to merge based on review results"
      prompt: |
        You analyze review results and make a go/no-go decision.
        Output exactly one of: APPROVE, REQUEST_CHANGES, or NEEDS_DISCUSSION.
        Follow your output with a brief explanation.

# Skill directories to load
skills:
  - "./skills/code-review"
  - "./skills/security"

# Workflow steps — DAG definition
steps:
  # Step 1: Initial analysis (entry point)
  - id: analyze
    agent: security-reviewer         # Which agent runs this step
    prompt: "Analyze the codebase in the current directory for security vulnerabilities."
    skills:
      - "code-review"               # Additional skills for this step only

  # Step 2: Fan-out — parallel steps
  - id: review-security
    agent: security-reviewer
    prompt: |
      Review this code for security issues.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]            # Runs after 'analyze' completes

  - id: review-performance
    agent: performance-reviewer
    prompt: |
      Review this code for performance issues.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]            # Also depends on 'analyze' — runs in PARALLEL with review-security

  - id: review-style
    agent: style-reviewer
    prompt: |
      Review this code for style issues.
      Previous analysis: {{steps.analyze.output}}
    depends_on: [analyze]

  # Step 3: Fan-in — aggregation
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
    depends_on: [review-security, review-performance, review-style]  # Fan-in: waits for ALL

  # Step 4: Conditional branching
  - id: decide
    agent: decision-maker
    prompt: |
      Based on this review report, should this code be approved?
      {{steps.aggregate.output}}
    depends_on: [aggregate]

  # Step 5a: Conditional — runs only if decision is APPROVE
  - id: approve-action
    agent: aggregator
    prompt: "Generate a concise approval summary suitable for a PR comment."
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"            # Simple string match

  # Step 5b: Conditional — runs only if decision is REQUEST_CHANGES
  - id: changes-action
    agent: aggregator
    prompt: |
      Generate a detailed list of required changes based on:
      {{steps.aggregate.output}}
    depends_on: [decide]
    condition:
      step: decide
      contains: "REQUEST_CHANGES"

# Output configuration
output:
  steps: [approve-action, changes-action]  # Which step outputs to display
  format: "markdown"                        # Output format
  truncate:                                 # Prevent injected outputs from blowing context window
    strategy: "chars"                       # 'chars', 'lines', or 'tokens'
    limit: 2000                             # Truncate to this limit
```

### Agent File Format (`.agent.md`) — Full VS Code Compatibility

goflow must parse `.agent.md` files exactly as VS Code defines them
(see [VS Code Custom Agents docs](https://code.visualstudio.com/docs/copilot/customization/custom-agents)).
This ensures agents are reusable between interactive VS Code sessions and
automated workflow runs.

#### Supported Frontmatter Fields

| Field | Type | Required | goflow Mapping |
|---|---|---|---|
| `name` | string | No (defaults to filename) | Step display name, audit log labels |
| `description` | string | No | Logged in audit trail; used for agent selection context |
| `argument-hint` | string | No | Ignored (interactive-only) |
| `tools` | string[] | No | Mapped to SDK tool restrictions on the session. If omitted → all tools. Supports `<server>/*` MCP glob. |
| `agents` | string[] | No | Lists subagent names this agent can invoke. `*` = all, `[]` = none. Mapped to SDK `customAgents` on the session. |
| `model` | string or string[] | No | Overrides `config.model` for this step. If array, first available model is used. |
| `user-invocable` | bool | No | Ignored (interactive-only; all workflow agents are invoked programmatically) |
| `disable-model-invocation` | bool | No | Ignored (goflow controls invocation, not LLM inference) |
| `target` | string | No | Ignored (`vscode` or `github-copilot` — not relevant for SDK execution) |
| `mcp-servers` | object | No | Passed through to SDK `SessionConfig.McpServers` (per-agent MCP config) |
| `handoffs` | list | No | **Supported.** Defines workflow transitions — see "Handoffs as Workflow Edges" below. |
| `handoffs[].label` | string | — | Display label for the transition (logged in audit trail) |
| `handoffs[].agent` | string | — | Target agent name → resolved to a workflow step or triggers dynamic step creation |
| `handoffs[].prompt` | string | — | Prompt to send to the target agent |
| `handoffs[].send` | bool | — | If `true`, auto-execute the handoff (default in workflow mode). If `false`, log as suggestion only. |
| `handoffs[].model` | string | — | Model override for the handoff target |
| `hooks` | object | No | **Supported.** Mapped to SDK `SessionHooks` (OnPreToolUse, OnPostToolUse, etc.) |

The **body** (markdown below the frontmatter) becomes the agent's system prompt,
injected into `SessionConfig.SystemMessage.Content`.

#### Example Agent File

```markdown
---
name: security-reviewer
description: Reviews code for OWASP Top 10 vulnerabilities
tools:
  - grep
  - glob
  - view
model: gpt-5
agents:
  - researcher
handoffs:
  - label: Send to Aggregator
    agent: aggregator
    prompt: Aggregate the security findings above into a report.
    send: true
---

# Security Reviewer

You are an expert security code reviewer. Focus on:

1. **Injection attacks** — SQL, XSS, command injection
2. **Authentication flaws** — weak password handling, missing MFA
3. **Access control** — broken authorization checks
4. **Cryptographic failures** — hardcoded secrets, weak algorithms

Always cite specific file paths and line numbers.
Provide severity ratings: CRITICAL, HIGH, MEDIUM, LOW.
```

#### Agent Discovery Paths

goflow searches for `.agent.md` files in the same locations VS Code
uses, plus explicit paths from the workflow YAML:

| Source | Path | Priority |
|---|---|---|
| Workflow YAML `agents.*.file` | Explicit path | Highest |
| Workspace `.github/agents/` | `.github/agents/*.agent.md` and `.github/agents/*.md` | High (wins over `.claude/agents/` if both exist) |
| Workspace `.claude/agents/` | `.claude/agents/*.md` (Claude format) | High |
| User profile | `~/.copilot/agents/*.agent.md` | Low |
| Additional locations | Configurable via `config.agent_search_paths` in workflow YAML | Configurable |

**Tie-breaker rule:** If an agent with the same name exists in both `.github/agents/` and `.claude/agents/`, the `.github/agents/` version takes precedence.

Claude-format agents (`.claude/agents/*.md`) are also supported: `tools` as
comma-separated strings are split into arrays, and Claude tool names are mapped
to VS Code equivalents (e.g., `Read` → `view`, `Grep` → `grep`, `Bash` → `bash`).

#### Handoffs Metadata (Information Only)

**MVP Note:** Handoffs in `.agent.md` files are parsed for tool completeness and
audit logging, but they are **not** used to auto-generate the workflow DAG. The reason
is that VS Code handoffs are dynamic runtime decisions made by the LLM, not static
declarative edges. Converting them to a static DAG would require either:

- Inferring from handoff metadata (fragile, non-deterministic)
- Running the agent to completion and observing which handoff it chose (expensive, cannot plan parallelism)

**Recommendation for future phases:** In workflows where hand-to-hand agent conversations
are desired, use `depends_on` to chain agents explicitly in YAML. Or implement a
"conversation mode" (Phase 3+) where agents can dynamically invoke each other.
**For MVP, stick to explicit YAML step definitions.**

### Core Components

#### 1. YAML Parser & Validator (`pkg/workflow/parser.go`)
- Parse workflow YAML with strict validation
- Validate agent references exist (file or inline)
- Validate step dependencies form a valid DAG (no cycles)
- Validate condition references point to valid steps
- Template syntax validation (detect `{{steps.X.output}}` references)

#### 2. Agent File Loader (`pkg/agents/loader.go`)
- Parse `.agent.md` files (YAML frontmatter + markdown body)
- Full VS Code frontmatter support (all fields from the table above)
- Map to Copilot SDK `SessionConfig` fields:
  - `name` → tracking metadata, audit log labels
  - `description` → audit log context
  - `prompt` (markdown body) → `SystemMessage.Content`
  - `tools` → tool restrictions (supports `<server>/*` MCP globs)
  - `model` → `SessionConfig.Model` override (array = priority list)
  - `agents` → `SessionConfig.CustomAgents` for subagent availability
  - `mcp-servers` → `SessionConfig.McpServers`
  - `handoffs` → DAG edges (in handoff mode) or metadata (in static mode)
  - `hooks` → `SessionConfig.Hooks`
- Agent discovery: scan `.github/agents/`, `.claude/agents/`, `~/.copilot/agents/`, and explicit `file:` paths
- Claude format support: comma-separated tool strings → arrays, tool name mapping
- Fallback to inline agent definitions from workflow YAML

#### 3. DAG Builder (`pkg/workflow/dag.go`)
- Build dependency graph from `depends_on` fields
- Topological sort for execution order
- Identify parallelizable groups (steps with same resolved dependencies)
- Cycle detection with clear error messages

#### 4. Template Engine (`pkg/workflow/template.go`)
- `{{steps.<id>.output}}` variable substitution
- Resolve step outputs into prompt templates
- Error on unresolved references (step not yet completed)
- **Context window management:** Implement truncation/summarization strategies to prevent blown context windows when injecting large outputs from parallel agents:
  - `truncate: {strategy: 'chars', limit: 2000}` — truncate to N characters
  - `truncate: {strategy: 'lines', limit: 50}` — truncate to N lines
  - `truncate: {strategy: 'tokens', limit: 1000}` — truncate to N tokens (requires tokenizer)
  - Emit warnings when truncation occurs

#### 5. Step Executor (`pkg/executor/executor.go`)
- For each step:
  1. Check conditions (if any) — skip if condition not met, record skip in audit
      2. Resolve prompt template (substitute `{{steps.X.output}}`)
  3. Create a Copilot SDK session with the agent's config (system prompt, tools, model, MCP servers, hooks)
  4. Register audit logger as session event subscriber (`session.On()`)
  5. Register shared memory tools (`read_memory`, `write_memory`) if shared memory is enabled
  6. Create step audit directory, begin writing `transcript.jsonl`
  7. Send the resolved prompt
  8. Wait for `session.idle` event
  9. Extract the final `assistant.message` content as step output
  10. Write `output.md`, `prompt.md`, finalize `step.meta.json`
  11. Store output in a shared results map for template resolution
  12. Disconnect session

#### 6. Workflow Orchestrator (`pkg/orchestrator/orchestrator.go`)
- Execute the DAG:
  1. Start with steps that have no dependencies
  2. For each "level" of the DAG, launch steps concurrently (goroutines)
  3. Use `sync.WaitGroup` per level for fan-in synchronization
  4. After a level completes, evaluate conditions for the next level's steps
  5. Continue until all reachable steps are complete or a step fails
- Error handling: configurable per-step (retry N times, skip, abort workflow)
- Timeout: per-step and per-workflow timeouts

#### 7. Audit Logger (`pkg/audit/logger.go`)

Every workflow run produces a complete, browsable audit trail in a dedicated
folder. This is critical for transparency and debugging.

**Audit directory structure per run:**

```
.workflow-runs/
└── 2026-03-20T14-32-05_code-review-pipeline/
    ├── workflow.meta.json         # Run metadata (start time, end time, status, config hash)
    ├── workflow.yaml              # Snapshot of the workflow file used
    ├── dag.dot                    # DOT graph of the execution DAG
    ├── memory.md                  # Shared memory file (if configured)
    ├── steps/
    │   ├── 01_analyze/
    │   │   ├── step.meta.json     # Step metadata (agent, model, start/end, status, duration)
    │   │   ├── prompt.md          # The resolved prompt sent to the session
    │   │   ├── transcript.jsonl   # Full session event log (every event, streaming deltas)
    │   │   ├── output.md          # Final assistant.message content (the step's output)
    │   │   ├── tool_calls.jsonl   # All tool invocations with args and results
    │   │   └── errors.log         # Errors, if any
    │   ├── 02_review-security/
    │   │   └── ...                # Same structure
    │   ├── 03_review-performance/
    │   │   └── ...
    │   └── 03_review-style/       # Same sequence number = ran in parallel
    │       └── ...
    └── final_output.md            # Aggregated workflow output
```

**Key design points:**

- **Real-time writing:** Transcript and tool call files are appended in real-time
  as events arrive (JSONL format — one JSON object per line). Users can `tail -f`
  any transcript file to monitor a step live.
- **Sequence numbering:** Step folders are prefixed with a two-digit sequence
  number based on execution order. Parallel steps share the same number.
- **Interactive monitoring:** A companion `goflow watch` command tails
  all active step transcripts in a multiplexed terminal view (one pane per
  active parallel step).
- **Structured metadata:** `step.meta.json` records:
  ```json
  {
    "step_id": "review-security",
    "agent": "security-reviewer",
    "agent_file": "./agents/security-reviewer.agent.md",
    "model": "gpt-5",
    "status": "completed",
    "started_at": "2026-03-20T14:32:07Z",
    "completed_at": "2026-03-20T14:33:12Z",
    "duration_seconds": 65,
    "token_usage": { "prompt": 2340, "completion": 1890 },
    "output_file": "output.md",
    "depends_on": ["analyze"],
    "condition": null,
    "condition_result": null,
    "error": null
  }
  ```
- **Output as reusable context:** Each `output.md` file is a clean, standalone
  document that can be referenced by subsequent workflows or steps. The
  `{{steps.X.output}}` template engine reads from these files.
- **Configurable location:** The audit directory defaults to `.workflow-runs/`
  in the working directory but is configurable via `config.audit_dir` in the
  workflow YAML or `--audit-dir` CLI flag.

**SDK integration:** The audit logger subscribes to all session events via
`session.On()` and writes every event to `transcript.jsonl`. Tool calls are
extraced from `tool.call` / `tool.result` events into `tool_calls.jsonl`.
The `assistant.message` event's `Content` field is written to `output.md`.

#### 8. Shared Memory Manager (`pkg/memory/manager.go`)

When running agents in parallel, they sometimes need access to shared context.
The shared memory file provides a synchronization mechanism.

**How it works:**

- A `memory.md` file is created in the audit directory at workflow start.
- It is initialized with content from `config.shared_memory.initial_content` or
  an initial file path (`config.shared_memory.file`).
- Each parallel agent session gets a custom `read_memory` tool and a
  `write_memory` tool registered via the SDK's `Tools` API.
- `read_memory` returns the current contents of `memory.md`.
- `write_memory` appends a timestamped, agent-attributed entry to `memory.md`.
  Writes are serialized via a `sync.Mutex` to prevent corruption.
- The agent's system prompt is augmented with instructions about the shared
  memory: _"You have access to a shared memory file via the `read_memory` and
  `write_memory` tools. Use `read_memory` to check for context from other agents.
  Use `write_memory` to record findings that other agents should know about."_

**Workflow YAML configuration:**

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true    # Force inject memory into prompts (recommended)
    initial_content: |           # or use initial_file: "./initial-memory.md"
      # Project Context
      # Shared Context
      Project: my-app
      Branch: feature/auth-refactor
```

**Memory file format (after agents write to it):**

```markdown
# Shared Context
Project: my-app
Branch: feature/auth-refactor

---
[2026-03-20T14:32:15Z] [security-reviewer] Found SQL injection in auth/login.go:42
[2026-03-20T14:32:18Z] [performance-reviewer] N+1 query detected in models/user.go:88
[2026-03-20T14:32:22Z] [security-reviewer] Hardcoded API key in config/secrets.go:12
```

**For MVP:** Force memory injection via `inject_into_prompt: true` to eliminate reliance on tool calls.
Agents will see the memory in their prompt, not as an optional tool.

#### 9. Result Reporter (`pkg/reporter/reporter.go`)
- Collect outputs from specified output steps
- Format as markdown, JSON, or plain text
- Write to stdout, file, or both
- Write `final_output.md` to the audit directory

### Project Structure

```
workflow-runner/
├── cmd/
│   └── workflow-runner/
│       └── main.go                 # CLI entry point (run, watch, resume)
├── pkg/
│   ├── workflow/
│   │   ├── parser.go              # YAML parsing & validation (with input validation)
│   │   ├── parser_test.go
│   │   ├── dag.go                 # DAG building & topo sort (with cycle detection)
│   │   ├── dag_test.go
│   │   ├── template.go            # {{}} template resolution + truncation/summarization
│   │   ├── template_test.go
│   │   └── types.go               # Workflow, Step, Agent, Condition types
│   ├── agents/
│   │   ├── loader.go              # .agent.md file parser (VS Code + Claude format)
│   │   ├── loader_test.go
│   │   └── discovery.go           # Agent file discovery (with .github/.claude tie-breaker)
│   ├── executor/
│   │   ├── executor.go            # Single step execution via SDK
│   │   └── executor_test.go
│   ├── orchestrator/
│   │   ├── orchestrator.go        # DAG-based concurrent orchestration (with resume support)
│   │   └── orchestrator_test.go
│   ├── audit/
│   │   ├── logger.go              # Audit trail writer (JSONL transcripts, outputs)
│   │   ├── logger_test.go
│   │   ├── cleanup.go             # Audit retention policy (delete old runs)
│   │   └── watcher.go             # Live monitoring (tail -f multiplexed view)
│   ├── memory/
│   │   ├── manager.go             # Shared memory file (forced injection + optional tool writes)
│   │   └── manager_test.go
│   └── reporter/
│       ├── reporter.go            # Output formatting + final_output.md
│       └── reporter_test.go
├── agents/                         # Example .agent.md files
│   ├── security-reviewer.agent.md
│   ├── performance-reviewer.agent.md
│   └── aggregator.agent.md
├── skills/                         # Example SKILL.md directories
│   └── code-review/
│       └── SKILL.md
├── examples/
│   ├── code-review-pipeline.yaml  # Example workflow
│   ├── simple-sequential.yaml
├── go.mod
├── go.sum
└── README.md
```

### Execution Flow

```
1. CLI parses command:
   - `goflow run --workflow file.yaml [--inputs file=src/main.go --inputs branch=feature/x] [--audit-dir ./runs]`
   - `goflow watch --run <run-dir>`  (live monitoring)
   - `goflow resume --run <run-dir> --step <step-id>`  (resume from checkpoint)
   
2. Load & validate workflow YAML

3. Merge runtime inputs (CLI flags) with workflow inputs; validate against input schema

4. Discover & load agent files:
   a. Explicit paths from workflow YAML `agents.*.file`
   b. Search .github/agents/ (take precedence if tied with .claude/agents/)
   c. Search .claude/agents/ (if .github/ not found)
   d. Search ~/.copilot/agents/
   e. Validate all agent references in steps resolve to loaded agents

5. Build DAG from step dependencies with cycle detection
   - If cycles detected, fail with clear error
   - (Note: Do NOT use handoffs for DAG construction; they are metadata only)

6. Create audit directory:
   a. Create .workflow-runs/<timestamp>_<workflow-name>/
   b. Write workflow.meta.json (inputs, config hash, workflow name)
   c. Initialize shared memory file (if configured with inject_into_prompt: true)
   d. Apply audit retention policy (delete runs older than retention threshold)

7. Initialize Copilot SDK client
   - **Concurrency test flag:** For MVP, support --cli-concurrency-test to verify CLI handles concurrent requests
   - Exit with recommendations if CLI is blocking

8. Execute DAG:
   a. Find steps with no unmet dependencies → "ready" set
   b. For each ready step:
      - Create session with agent config (system prompt, tools, model)
      - Inject runtime inputs and current shared memory (if enabled) into prompt template
      - Apply output truncation strategy to any {{steps.X.output}} references
      - Submit prompt to session, write session_id to step.meta.json
      - Listen for session.idle event, extract assistant.message content
      - Append to step/output.md and transcript.jsonl
      - Mark step complete, add to "ready" set any steps that depended on it
   c. Repeat until all reachable steps are complete or a step fails
   d. If a step fails and config.on_failure: "skip" → continue; if "abort" → stop

9. Aggregate outputs from specified output steps

10. Write final_output.md to audit directory

11. Format and display final report to stdout

12. Stop SDK client
```

---

## Part 3: Key Design Decisions & Risk Mitigations

### CLI Concurrency Architecture

**Risk:** The Copilot CLI (server mode) is single-threaded on the JSON-RPC pipe. If one request blocks,
all "parallel" goroutines in the Go orchestrator are blocked waiting on IPC, negating parallelism benefits.

**Mitigation for MVP:**
1. **Immediate verification:** Before finalizing the architecture, run a concurrency test:
   - Spawn 3 parallel SDK sessions that each submit heavy inference tasks.
   - Measure actual wall-clock execution time vs. sequential baseline.
   - If wall-clock time ≈ sequential time → CLI is blocking.
2. **If CLI blocks:** Change architecture to spawn one CLI process per session (or per N sessions),
   rather than one shared client. This trades memory for true parallelism.
3. **If CLI handles concurrency well:** Proceed with shared client architecture.

**Decision point:** This must be resolved before Phase 1 completion.

### Why separate sessions per step (not `customAgents` sub-agent delegation)?

The SDK's `customAgents` feature lets the runtime auto-select agents based on intent. This is great for interactive chat but **problematic for deterministic workflows**:

- You can't guarantee the runtime will pick the right agent for each step
- You can't run agents in parallel (sub-agents run sequentially within one session)
- You can't easily extract structured intermediate outputs for fan-in

**Our approach:** Each workflow step gets its own SDK session with a tailored `SystemMessage` (from the agent file). This gives us:
- Deterministic execution (no inference guessing)
- True parallel execution (Go goroutines + independent sessions)
- Clean data passing between steps
- Per-step model selection (different models for different tasks)

### Why Go?

- The Copilot SDK's Go implementation is mature and well-documented
- Goroutines + channels are ideal for fan-out/fan-in patterns
- Strong typing catches workflow configuration errors at parse time
- Single binary distribution (no runtime dependencies)
- **CLI discovery:** The runner looks for the Copilot CLI on `$PATH`, in default install locations (`~/.copilot/`, `/usr/local/bin/copilot`), or via `--copilot-bin` flag. Users install the CLI separately; the runner does not bundle or redistribute it.

### Condition System

Start simple, expand later:

**Phase 1 (MVP):**
- `contains: "STRING"` — output contains string
- `not_contains: "STRING"` — output doesn't contain string
- `equals: "STRING"` — exact match (trimmed)

**Phase 2:**
- `regex: "PATTERN"` — regex match on output
- `json_path: "$.status"` + `equals: "success"` — JSON field extraction
- `llm_eval:` — use a lightweight LLM call to classify the output

### Error Handling Strategy

```yaml
steps:
  - id: risky-step
    agent: some-agent
    prompt: "..."
    on_error: skip          # Options: fail (default), skip, retry
    retry_count: 3          # Only used with on_error: retry
    timeout: 120s           # Per-step timeout
```

---

## Part 4: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| SDK is in "Technical Preview" — breaking changes | Medium | High | Pin SDK version, abstract SDK calls behind interfaces |
| Rate limiting (GitHub Copilot quota) with many parallel sessions | Medium | Medium | Configurable concurrency limit, BYOK avoids Copilot quotas |
| Agent output is non-deterministic — conditions may be unreliable | High | Medium | Use explicit output formats in prompts, LLM-based condition evaluation as fallback |
| CLI process stability under many concurrent sessions | Low | High | Stress test early, fall back to sequential execution |
| `.agent.md` format drift between VS Code and our parser | Low | Medium | Parse all known frontmatter fields, ignore unknown ones gracefully, test against real VS Code agent files |
| Audit directory disk usage under heavy workflow runs | Medium | Low | Configurable retention policy, option to compress completed runs |
| Shared memory contention with many parallel writers | Low | Medium | Mutex serialization, append-only format, agent-attributed entries |


---

## Part 5: Implementation Phases

### Phase 1 — MVP (Foundation)
- [ ] Go module init, Copilot SDK dependency
- [ ] YAML parser for workflow files (strict validation)
- [ ] `.agent.md` file loader with full VS Code frontmatter support
- [ ] Agent discovery (`.github/agents/`, `~/.copilot/agents/`, explicit paths)
- [ ] Claude-format agent file support (`.claude/agents/`)
- [ ] Sequential step execution (no parallelism)
- [ ] Simple `{{steps.X.output}}` template resolution
- [ ] Basic audit logging (audit directory, step folders, output.md, step.meta.json)
- [ ] Basic CLI: `goflow run --workflow file.yaml`
- [ ] Example workflow: 3-step sequential pipeline

### Phase 2 — Parallelism & Fan-Out/Fan-In
- [ ] DAG builder with topological sort
- [ ] Concurrent step execution (goroutines + WaitGroup)
- [ ] Fan-in synchronization (wait for all dependencies)
- [ ] Configurable max concurrency
- [ ] Shared memory file for parallel agents (read_memory / write_memory tools)
- [ ] Example workflow: fan-out/fan-in code review pipeline with shared memory

### Phase 3 — Audit Trail & Monitoring
- [ ] Full transcript logging (transcript.jsonl — every session event)
- [ ] Tool call logging (tool_calls.jsonl)
- [ ] Real-time monitoring: `goflow watch --run <dir>` (multiplexed tail)
- [ ] Errors log per step
- [ ] DAG visualization (dag.dot export)

### Phase 4 — Conditional Branching & Handoffs
- [ ] `contains` / `not_contains` / `equals` conditions
- [ ] Conditional step skipping
- [ ] Handoff mode: auto-generate DAG from agent `handoffs` frontmatter
- [ ] Example workflow: review → decide → branch

### Phase 5 — Production Hardening
- [ ] Per-step and per-workflow timeouts
- [ ] Error handling strategies (fail/skip/retry)
- [ ] Structured logging with step context
- [ ] OpenTelemetry integration (trace per workflow, span per step)
- [ ] BYOK provider configuration from YAML
- [ ] Skill directory integration
- [ ] Agent `hooks` frontmatter → SDK SessionHooks mapping

### Phase 6 — Advanced Features
- [ ] Regex and JSON path conditions
- [ ] LLM-based condition evaluation
- [ ] Loop/iteration steps (for-each over a list)
- [ ] Sub-workflow inclusion (`import: other-workflow.yaml`)
- [ ] Dry-run mode (validate workflow without executing)
- [ ] Output to file, webhook, or PR comment
- [ ] Watch mode (re-run on file changes)

---

## Part 6: What IS NOT Possible (Honest Gaps)

1. **No real-time inter-agent communication.** The SDK sessions are isolated. One agent can't "call" another agent mid-execution. Data only flows between steps, not during a step. If you need Agent A to consult Agent B mid-thought, that's not supported — you'd need to model it as separate sequential steps. **Partial mitigation:** The shared memory file allows a lightweight form of cross-agent signaling during parallel execution, but it's advisory (the LLM may or may not check it).

2. **No guaranteed structured output.** LLM outputs are natural language. Conditions that depend on exact string matching can be brittle. Mitigation: careful prompt engineering + LLM-based classification as fallback.

3. **No native VS Code UI integration.** This runner is a standalone CLI tool. It reads the same `.agent.md` and `SKILL.md` files as VS Code, but it doesn't integrate with VS Code's chat panel. It's complementary — you author agents in VS Code, run them at scale with goflow.

4. **Single CLI process bottleneck.** All sessions share one Copilot CLI server process. Under heavy parallel load, this could become a bottleneck. Mitigation: TCP mode with multiple CLI servers, or concurrency limits. **(Verify during Phase 1 via `--cli-concurrency-test` flag.)**

5. **SDK is Technical Preview.** APIs may change. The `customAgents` and `skillDirectories` features are relatively new.

6. **Shared memory is best-effort.** The `read_memory`/`write_memory` tool mechanism (if used) relies on the LLM choosing to use those tools. MVP uses `inject_into_prompt: true` which forces the memory into the prompt, but agents still may ignore it. For hard dependencies, use step-level `depends_on`.

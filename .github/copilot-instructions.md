# goflow — Copilot Instructions

**Project:** An AI workflow orchestration engine that coordinates multi-agent LLM workflows with parallelism, powered by the Copilot SDK.

**Status:** Design phase (no code yet). Implement Phase 1 first (see roadmap below).

**Tech Stack:** Go, Copilot SDK, YAML, DAG algorithms, Goroutines/WaitGroups.

---

## Quick Start for Developers

### Prerequisites
- Go 1.21+ installed
- Copilot CLI installed (`copilot` on PATH or `~/.copilot/copilot`)
- macOS, Linux, or WSL (Copilot CLI availability varies)

### Project Clone & Setup
```bash
cd ~/Code/workflow-runner

# Expected to find:
# - PLAN.md (comprehensive technical design)
# - .github/agents/adversary-reviewer.agent.md (example agent)
# - (Soon) go.mod, go.sum (when Phase 1 implementation starts)
```

### Core Build Commands (Will Be Added in Phase 1)
```bash
# Initialize Go module (will be done once)
go mod init github.com/alex-workflow-runner

# Add Copilot SDK dependency
go get github.com/github/copilot-sdk/go

# Build the CLI binary
go build -o goflow ./cmd/workflow-runner/main.go

# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Benchmark tests (if added)
go test -bench=. ./...

# Generate coverage report
go test -cover ./...
```

---

## Architecture at a Glance

See [PLAN.md](./../../PLAN.md) for the complete technical design. **Key concepts:**

### Execution Models
- **Sequential:** Agent A → B → C
- **Parallel (Fan-Out):** A spawns B, C, D concurrently via goroutines
- **Fan-In:** Wait for B, C, D; then Agent E aggregates outputs
- **Conditional:** If condition met, skip or branch step

### Component Structure
```
cmd/workflow-runner/
  └── main.go                         # CLI entry point
pkg/
  ├── workflow/
  │   ├── parser.go                  # YAML parsing + validation
  │   ├── dag.go                     # DAG building (topo sort)
  │   ├── template.go                # {{steps.X.output}} resolution
  │   └── types.go                   # Core types
  ├── agents/
  │   ├── loader.go                  # .agent.md parser
  │   └── discovery.go               # Agent file discovery
  ├── executor/
  │   └── executor.go                # Single step execution
  ├── orchestrator/
  │   └── orchestrator.go            # DAG execution + parallelism
  ├── audit/
  │   ├── logger.go                  # Audit trail recording
  │   ├── cleanup.go                 # Retention policy
  │   └── watcher.go                 # Live monitoring
  ├── memory/
  │   └── manager.go                 # Shared memory (parallel agents)
  └── reporter/
      └── reporter.go                # Output formatting
examples/
  ├── code-review-pipeline.yaml      # Example workflow
  └── simple-sequential.yaml
agents/
  ├── security-reviewer.agent.md     # Example agents
  ├── performance-reviewer.agent.md
  └── aggregator.agent.md
skills/
  └── code-review/SKILL.md           # Example skill module
```

### Key Design Patterns
1. **One Copilot SDK session per workflow step** (deterministic agent selection)
2. **Goroutines + sync.WaitGroup** for parallelism per DAG level
3. **Audit directory per run** (`.workflow-runs/<timestamp>_<name>/`) for full transparency
4. **Context truncation** for `{{steps.X.output}}` template injection (prevent context window overflow)
5. **Shared memory file** for lightweight cross-agent signaling during parallel execution

---

## Workflow YAML Format

### Basic Structure
```yaml
name: "code-review-pipeline"
description: "Multi-agent code review with parallelism"

# Runtime inputs passed via CLI: --inputs key=value
inputs:
  files:
    description: "Files to review (glob pattern)"
    default: "src/**/*.go"

config:
  model: "gpt-5"
  audit_dir: ".workflow-runs"
  audit_retention: 10           # Keep last 10 runs
  shared_memory:
    enabled: true
    inject_into_prompt: true    # Force inject into prompts (recommended)

agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"  # Load from .agent.md
  performance-reviewer:
    file: "./agents/performance-reviewer.agent.md"
  aggregator:
    inline:                       # Or define inline
      description: "Aggregates all reviews"
      prompt: "You are an aggregator..."
      tools: [grep, view]
      model: "gpt-5"

steps:
  - id: analyze
    agent: security-reviewer
    prompt: "Analyze {{inputs.files}}"

  - id: review-security
    agent: security-reviewer
    prompt: "Security review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-performance
    agent: performance-reviewer
    prompt: "Performance review: {{steps.analyze.output}}"
    depends_on: [analyze]          # Runs in PARALLEL with review-security

  - id: aggregate
    agent: aggregator
    prompt: |
      Combine reviews:
      {{steps.review-security.output}}
      {{steps.review-performance.output}}
    depends_on: [review-security, review-performance]  # Fan-in: wait for BOTH

  - id: decide
    agent: aggregator
    prompt: "Approve? {{steps.aggregate.output}}"
    depends_on: [aggregate]
    condition:
      step: decide
      contains: "APPROVE"

output:
  steps: [decide, aggregate]
  format: "markdown"
  truncate:
    strategy: "chars"
    limit: 2000
```

### Conditions (Phase 1 MVP)
- `contains: "STRING"` — output contains substring
- `not_contains: "STRING"` — output doesn't contain substring
- `equals: "STRING"` — exact match (trimmed)

**Future:** Regex, JSON paths, LLM-based classification (Phase 4+).

---

## Agent File Format (`.agent.md`)

Fully compatible with VS Code agents. Structure:

```markdown
---
name: security-reviewer
description: Reviews code for vulnerabilities
tools:
  - grep
  - semantic_search
  - view
model: gpt-5
agents: []                          # Subagents this agent can delegate to
mcp-servers:                        # MCP server configs per agent
  sec-tools:
    command: docker
    args: ["run", "security:latest"]
handoffs:                           # Metadata (not used for DAG in MVP)
  - label: Send to Aggregator
    agent: aggregator
    prompt: "Aggregate findings..."
hooks:                              # Session lifecycle (Phase 5)
  onPreToolUse: ""
  onPostToolUse: ""
---

# Security Reviewer

You are an expert security reviewer. Focus on:
1. **Injection attacks**
2. **Authentication flaws**
3. **Access control issues**

Always cite file paths and line numbers.
Provide severity: CRITICAL, HIGH, MEDIUM, LOW.
```

**Agent Discovery Paths (in order):**
1. Explicit `agents.*.file` in workflow YAML
2. `.github/agents/*.agent.md` (highest priority if tied)
3. `.claude/agents/*.md` (Claude format, auto-mapped)
4. `~/.copilot/agents/*.agent.md`
5. Paths in `config.agent_search_paths`

---

## Audit Trail Structure

Each workflow run creates a timestamped directory:

```
.workflow-runs/
└── 2026-03-20T14-32-05_code-review-pipeline/
    ├── workflow.meta.json           # Run metadata (start, end, status, config hash)
    ├── workflow.yaml                # Snapshot of workflow
    ├── dag.dot                      # Graphviz DAG visualization
    ├── memory.md                    # Shared memory final state (if enabled)
    └── steps/
        ├── 01_analyze/
        │   ├── step.meta.json       # Agent, model, timing, session_id, token usage
        │   ├── prompt.md            # The resolved prompt sent to LLM
        │   ├── transcript.jsonl     # Full session event stream (append-only, one JSON obj/line)
        │   ├── output.md            # Final assistant message (the step result)
        │   ├── tool_calls.jsonl     # Tool invocations with args/results
        │   └── errors.log           # Errors if any
        ├── 02_review-security/      # Parallel → same sequence number
        └── 03_review-performance/   # Can tail these in real-time
```

**CLI Commands (Planned):**
```bash
# Run a workflow
goflow run --workflow code-review.yaml \
  --inputs files=src/main.go \
  --inputs branch=feature/x

# Monitor live
goflow watch --run .workflow-runs/2026-03-20T14-32-05_code-review-pipeline

# Resume from checkpoint
goflow resume --run .workflow-runs/2026-03-20T14-32-05_code-review-pipeline \
  --step review-security
```

---

## Implementation Roadmap

| Phase | Features | Priority | Est. Scope |
|-------|----------|----------|-----------|
| **1 (MVP)** | YAML parser, agent loader, sequential execution, basic audit logs | 🔴 Must | 2-3 weeks |
| **2** | Parallelism, fan-in/fan-out, shared memory, DAG optimization | 🔴 Must | 1-2 weeks |
| **3** | Audit UI (watch mode), live transcript tailing | 🟡 Should | 1 week |
| **4** | Conditions, branching, handoff metadata parsing | 🟡 Should | 1 week |
| **5** | Timeouts, retries, OTel tracing, provider config | 🟡 Should | 1-2 weeks |
| **6** | Advanced (loops, sub-workflows, dry-run, regex conditions) | 🟢 Nice | 2+ weeks |

---

## Dev Practices & Conventions

### Implementation Style
- Prefer maintainable, readable, boring code over clever abstractions.
- Prefer the simplest correct solution; only add abstractions when they reduce complexity.
- Keep control flow straightforward and avoid deep nesting when early returns or small helpers make intent clearer.
- Match the surrounding package style and naming before introducing new patterns.
- Separate feature changes from refactors and formatting-only edits.

### Code Organization
- **Keep logic modular:** Each package has a single responsibility (parser, executor, etc.)
- **Export via interfaces:** Hide SDK details behind clean internal APIs (e.g., `type SessionExecutor interface`)
- **Tests alongside code:** `executor.go` → `executor_test.go` in the same directory
- **Avoid init() functions:** Use explicit setup/factory functions instead

### Documentation Expectations
- Add package/file header comments for new source files: 2-5 lines describing the file's purpose and how it fits in the module.
- Document public functions, methods, types, and classes with purpose, parameters, return values, and important edge cases.
- Add comments for intent and rationale, not line-by-line narration of obvious code.
- Place short rationale comments near non-obvious decisions, especially around DAG ordering, prompt truncation, retries, concurrency, and audit behavior.
- Keep comments current; if code changes invalidate a comment, update or remove the comment in the same change.

### Error Handling
```go
// ✅ Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("parsing workflow step %q: %w", stepID, err)
}

// ❌ Avoid: Silent errors
if err != nil {
    log.Println(err)  // User won't know what failed in a workflow
}
```

### Naming Conventions
- **Interfaces:** `SessionExecutor`, `WorkflowParser`, `AuditLogger` (descriptive, -er suffix for "doer" interfaces)
- **Functions:** `RunStep()`, `BuildDAG()`, `ResolveTemplate()` (verb + noun)
- **Packages:** Lowercase, one word if possible (`workflow`, `executor`, `agents`)
- **Constants:** `ContextWindowLimit`, `DefaultTimeout`, `AuditDirName` (PascalCase)

### Testing
```go
// ✅ Good: Table-driven tests for multiple scenarios
func TestParseWorkflow(t *testing.T) {
    tests := []struct {
        name      string
        yaml      string
        wantError bool
    }{
        {"valid sequential", "...", false},
        {"circular dependency", "...", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ParseWorkflow(tt.yaml)
            if (err != nil) != tt.wantError {
                t.Errorf("got error=%v, want=%v", err, tt.wantError)
            }
        })
    }
}
```

- Add or update tests with each behavior change.
- Prefer focused unit tests first; add integration-style coverage only where orchestration boundaries matter.
- Cover edge cases called out in the implementation docs and public API comments.

### Concurrency
- **Use sync.WaitGroup** per DAG level (not complex sync.Cond patterns)
- **Marshal outputs** from goroutines into a thread-safe results map (sync.Map or mutex)
- **Document goroutine lifetimes:** "Spawned for each ready step, waits for session.idle event"

### Logging & Audit
- **Audit events over logs:** Write to `transcript.jsonl` for all important state changes
- **Include context:** Step ID, agent name, model used, timing, errors
- **Struct logging:** Use structs with JSON tags rather than format strings

### Quality Gates
- Before finalizing code changes, run formatting and tests relevant to the changed area.
- For Go code, the default checks are `gofmt -w ./...` and `go test ./...`; run narrower commands during iteration when faster, then broaden before handoff.
- Run linting when configured for the repo; if linting is unavailable, state that explicitly in the final summary.
- Do not leave the branch in a failing state. If something cannot be verified locally, note what is missing and why.

### Commit Hygiene
- Keep commits small and atomic: one logical change per commit.
- Do not mix feature work, refactors, and formatting-only changes in the same commit.
- Local checkpoint commits are acceptable while iterating, but do not propose a half-done or failing commit as ready for review.
- When preparing commit suggestions for a PR, provide:
  - a title in imperative mood, 50 characters or fewer
  - a body covering what changed, why it changed, and any behavioral impact
  - a 3-6 bullet diff summary that matches the actual change set
- If multiple checkpoint commits exist, propose an optional squash plan before PR creation.

---

## Common Patterns & Recipes

### Pattern: DAG Topological Sort
```go
// pkg/workflow/dag.go
type DAGLevel []Step  // Steps that can run in parallel

func (wf *Workflow) GetExecutionOrder() ([]DAGLevel, error) {
    levels := []DAGLevel{}
    remaining := make(map[string]Step)
    for _, step := range wf.Steps {
        remaining[step.ID] = step
    }
    
    for len(remaining) > 0 {
        readySteps := []Step{}
        for id, step := range remaining {
            if allDependenciesSatisfied(step, wf.Steps) {
                readySteps = append(readySteps, step)
            }
        }
        if len(readySteps) == 0 {
            return nil, fmt.Errorf("circular dependency detected")
        }
        levels = append(levels, readySteps)
        for _, step := range readySteps {
            delete(remaining, step.ID)
        }
    }
    return levels, nil
}
```

### Pattern: Goroutine + WaitGroup Synchronization
```go
// pkg/orchestrator/orchestrator.go
func (orch *Orchestrator) ExecuteLevel(ctx context.Context, level DAGLevel) error {
    var wg sync.WaitGroup
    resultsCh := make(chan StepResult, len(level))
    
    for _, step := range level {
        wg.Add(1)
        go func(s Step) {
            defer wg.Done()
            result, err := orch.executor.Execute(ctx, s)
            resultsCh <- StepResult{Step: s, Result: result, Error: err}
        }(step)
    }
    
    wg.Wait()
    close(resultsCh)
    
    for result := range resultsCh {
        if result.Error != nil {
            return fmt.Errorf("step %s failed: %w", result.Step.ID, result.Error)
        }
        orch.results[result.Step.ID] = result.Result
    }
    return nil
}
```

### Pattern: Template Variable Resolution
```go
// pkg/workflow/template.go
func (wf *Workflow) ResolveTemplate(prompt string, results map[string]string) (string, error) {
    // Find {{steps.X.output}} references
    re := regexp.MustCompile(`\{\{steps\.(\w+)\.output\}\}`)
    resolved := re.ReplaceAllStringFunc(prompt, func(match string) string {
        stepID := extractStepID(match)
        if result, ok := results[stepID]; ok {
            // Apply truncation if configured
            return wf.TruncateOutput(result)
        }
        return fmt.Sprintf("ERROR: step %s not found", stepID)
    })
    return resolved, nil
}
```

---

## Known Pitfalls

| Pitfall | Why It Matters | How to Avoid |
|---------|---|---|
| **Using Copilot SDK's `customAgents` for step routing** | Runtime agent selection is non-deterministic; can't guarantee which agent runs. | Create one session per step with explicit agent config. |
| **Relying on shared memory without checking in prompt** | LLMs ignore optional tools. | Use `config.shared_memory.inject_into_prompt: true` to force it into the prompt body. |
| **Injecting huge step outputs without truncation** | Blows context window, wasted tokens, timeouts. | Implement `truncate` strategy (chars, lines, tokens). |
| **No cycle detection in DAG builder** | Infinite loops; unclear error messages. | Check for cycles in `BuildDAG()` before execution. |
| **Blocking operations in goroutines** | If SDK CLI is single-threaded, all "parallel" steps block. | Test with `--cli-concurrency-test` flag in Phase 1 to verify CLI behavior. |
| **Forgetting to persist session_id in audit** | Can't resume workflows on failure. | Write `session_id` to `step.meta.json` immediately after session creation. |

---

## IDE Setup & Tips

### Go Extensions (VS Code)
- **Go** (golang.go): Language support, debugging, testing
- **Error Lens**: Inline error messages
- **Go Test Explorer**: Run tests from sidebar

### Commands
```bash
# Format code
gofmt -w ./...

# Lint (if you have golangci-lint installed)
golangci-lint run ./...

# View test coverage in browser
go test -cover ./... && go tool cover -html=coverage.out

# Debug a test
go test -run TestParseWorkflow -v ./pkg/workflow
```

### Debugging Workflow Execution
```go
// Add debug logging
import "fmt"

// In executor
fmt.Fprintf(os.Stderr, "DEBUG: executing step %s with prompt: %q\n", s.ID, prompt)
```

---

## Relationship to PLAN.md

This file is a **quick reference for developers**. For comprehensive technical details, see [PLAN.md](./../../PLAN.md):
- **Part 1:** SDK capabilities & patterns
- **Part 2:** Full architecture (components, YAML schema, execution flow)
- **Part 3:** Design decisions & risk mitigations
- **Part 4:** Risk assessment matrix
- **Part 5:** Phase breakdown
- **Part 6:** Known limitations

---

## Next Steps: Create Initial Scaffold

When ready to start Phase 1 implementation, suggest:
1. **`/create-structure go goflow`** — Generate initial Go module, project structure
2. **`/create-agent security-reviewer`** — Create a working `.agent.md` agent file
3. **`/create-prompt code-review-workflow`** — Example workflow YAML prompt
4. **`/create-test dag-builder`** — Scaffold initial test for DAG builder

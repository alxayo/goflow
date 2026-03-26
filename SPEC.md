# Workflow Runner — Technical Specification & Implementation Plan

> **Scope:** Phase 1 (MVP) + Phase 2 (Parallelism). Phases 3–6 are outlined but not
> broken down into commit-level tasks.
>
> **Generated from:** [PLAN.md](PLAN.md) feasibility analysis.

---

## Table of Contents

1. [Task Inventory](#1-task-inventory)
2. [Dependency Graph](#2-dependency-graph)
3. [Parallelization Strategy](#3-parallelization-strategy)
4. [Task Detail Sheets](#4-task-detail-sheets)
5. [Phase 2 Tasks](#5-phase-2-tasks)
6. [Phase 3–6 Outline](#6-phase-3-6-outline)

---

## 1. Task Inventory

Each task is sized for a single atomic git commit. IDs use the format `P<phase>T<seq>`.

### Phase 1 — MVP Foundation

| ID | Task | Package | Est. Files | Depends On |
|----|------|---------|------------|------------|
| **P1T01** | Go module init + project scaffold | root | 5 | — |
| **P1T02** | Core domain types (`types.go`) | `pkg/workflow` | 1 | P1T01 |
| **P1T03** | YAML parser — happy path | `pkg/workflow` | 2 | P1T02 |
| **P1T04** | YAML parser — validation & error paths | `pkg/workflow` | 2 | P1T03 |
| **P1T05** | Agent types & frontmatter structs | `pkg/agents` | 1 | P1T02 |
| **P1T06** | Agent `.agent.md` loader — parser | `pkg/agents` | 2 | P1T05 |
| **P1T07** | Agent discovery (multi-path scan) | `pkg/agents` | 2 | P1T06 |
| **P1T08** | Claude-format agent support | `pkg/agents` | 2 | P1T06 |
| **P1T09** | DAG builder + topological sort | `pkg/workflow` | 2 | P1T02 |
| **P1T10** | Template engine (`{{steps.X.output}}` + `{{inputs.Y}}`) | `pkg/workflow` | 2 | P1T02 |
| **P1T11** | Output truncation strategies | `pkg/workflow` | 2 | P1T10 |
| **P1T12** | Condition evaluator (contains/not_contains/equals) | `pkg/workflow` | 2 | P1T02 |
| **P1T13** | Audit logger — directory creation + step.meta.json | `pkg/audit` | 2 | P1T02 |
| **P1T14** | Audit logger — output.md + prompt.md writing | `pkg/audit` | 2 | P1T13 |
| **P1T15** | Audit cleanup (retention policy) | `pkg/audit` | 2 | P1T13 |
| **P1T16** | SDK adapter interface + mock implementation | `pkg/executor` | 2 | P1T02 |
| **P1T17** | Step executor — core execution loop | `pkg/executor` | 2 | P1T16, P1T10, P1T12, P1T13 |
| **P1T18** | Sequential orchestrator | `pkg/orchestrator` | 2 | P1T09, P1T17 |
| **P1T19** | Result reporter (markdown/JSON/plain) | `pkg/reporter` | 2 | P1T02 |
| **P1T20** | CLI entry point (`run` command) | `cmd/workflow-runner` | 1 | P1T03, P1T07, P1T18, P1T19 |
| **P1T21** | Example: 3-step sequential workflow + agents | `examples/`, `agents/` | 4 | P1T20 |
| **P1T22** | Integration test — end-to-end with mock SDK | root | 2 | P1T20 |

### Phase 2 — Parallelism & Fan-Out/Fan-In

| ID | Task | Package | Est. Files | Depends On |
|----|------|---------|------------|------------|
| **P2T01** | Parallel orchestrator (goroutines + WaitGroup) | `pkg/orchestrator` | 2 | P1T18 |
| **P2T02** | Thread-safe results store | `pkg/orchestrator` | 2 | P2T01 |
| **P2T03** | Configurable max concurrency (semaphore) | `pkg/orchestrator` | 2 | P2T01 |
| **P2T04** | Shared memory manager | `pkg/memory` | 2 | P1T02 |
| **P2T05** | Shared memory tools (read_memory/write_memory) | `pkg/memory` | 2 | P2T04, P2T01 |
| **P2T06** | Prompt injection for shared memory | `pkg/memory` | 2 | P2T04, P1T10 |
| **P2T07** | Example: fan-out/fan-in pipeline | `examples/` | 2 | P2T01 |
| **P2T08** | Integration test — parallel execution | root | 2 | P2T01 |

---

## 2. Dependency Graph

```
Legend:  ──▶  "must complete before"
        ┄┄▶  "should complete before, but can develop in parallel with stubs"

                              ┌─────────┐
                              │ P1T01   │  Go module init + scaffold
                              │ (root)  │
                              └────┬────┘
                                   │
                              ┌────▼────┐
                       ┌──────┤ P1T02   ├──────────────────┬──────────────┐
                       │      │ types   │                  │              │
                       │      └────┬────┘                  │              │
                       │           │                       │              │
                 ┌─────▼──┐  ┌────▼────┐  ┌─────────┐ ┌───▼─────┐ ┌─────▼──┐
                 │ P1T05  │  │ P1T03   │  │ P1T09   │ │ P1T10   │ │ P1T12  │
                 │ agent  │  │ parser  │  │  DAG    │ │template │ │condition│
                 │ types  │  │ happy   │  │ builder │ │ engine  │ │  eval  │
                 └───┬────┘  └────┬────┘  └────┬────┘ └───┬─────┘ └───┬────┘
                     │            │            │           │            │
                ┌────▼────┐ ┌────▼────┐        │      ┌───▼─────┐     │
                │ P1T06   │ │ P1T04   │        │      │ P1T11   │     │
                │ .agent  │ │ parser  │        │      │truncate │     │
                │ loader  │ │ valid.  │        │      └─────────┘     │
                └──┬───┬──┘ └────┬────┘        │                      │
                   │   │         │             │                      │
             ┌─────▼┐ ┌▼──────┐  │             │    ┌──────────┐     │
             │P1T07 │ │P1T08  │  │             │    │ P1T13    │     │
             │disco-│ │claude │  │             │    │ audit    │     │
             │very  │ │agen  │  │             │    │ dirs+meta│     │
             └──┬───┘ └──────┘  │             │    └──┬───┬───┘     │
                │                │             │       │   │         │
                │                │             │  ┌────▼┐ ┌▼──────┐  │
                │                │             │  │P1T14│ │P1T15  │  │
                │                │             │  │audit│ │cleanup │  │
                │                │             │  │write│ └───────┘  │
                │                │             │  └──┬──┘            │
                │                │             │     │               │
                │           ┌────▼─────────────▼─────▼───────────────▼──┐
                │           │                P1T16                      │
                │           │  SDK adapter interface + mock             │
                │           └──────────────────┬────────────────────────┘
                │                              │
                │           ┌──────────────────▼────────────────────────┐
                │           │                P1T17                      │
                │           │  Step executor — core loop                │
                │           └──────────────────┬────────────────────────┘
                │                              │
                │           ┌──────────────────▼────────────────────────┐
                │           │                P1T18                      │
                │           │  Sequential orchestrator                  │
                │           └──────────────────┬────────────────────────┘
                │                              │
                │     ┌──────────┐             │    ┌──────────┐
                │     │ P1T19    │             │    │          │
                │     │ reporter │             │    │          │
                │     └────┬─────┘             │    │          │
                │          │                   │    │          │
                └──────────┼───────────────────┼────┘          │
                           │                   │               │
                      ┌────▼───────────────────▼───┐           │
                      │          P1T20             │           │
                      │  CLI entry point (run)     │           │
                      └────────────┬───────────────┘           │
                                   │                           │
                      ┌────────────▼──┐   ┌────────────────────▼──┐
                      │    P1T21      │   │       P1T22           │
                      │  example wf   │   │  integration test     │
                      └───────────────┘   └───────────────────────┘
                                   │
                    ═══════════════╪══════════════ Phase 2 ═══
                                   │
                      ┌────────────▼──────────────┐
                      │         P2T01             │
                      │  Parallel orchestrator    │
                      └───┬────────┬──────────┬───┘
                          │        │          │
                    ┌─────▼──┐ ┌───▼────┐ ┌───▼────┐  ┌─────────┐
                    │ P2T02  │ │ P2T03  │ │ P2T04  │  │         │
                    │ safe   │ │ max    │ │ shared │  │         │
                    │ store  │ │ concur │ │ memory │  │         │
                    └────────┘ └────────┘ └──┬──┬──┘  │         │
                                             │  │     │         │
                                       ┌─────▼┐ ┌▼────▼┐        │
                                       │P2T05 │ │P2T06 │        │
                                       │mem   │ │mem   │        │
                                       │tools │ │inject│        │
                                       └──────┘ └──────┘        │
                                                                │
                      ┌─────────────────────────────────────────┘
                      │
                 ┌────▼─────┐  ┌──────────┐
                 │  P2T07   │  │  P2T08   │
                 │ example  │  │ integ.   │
                 │ parallel │  │ test     │
                 └──────────┘  └──────────┘
```

### Textual Adjacency List

```
P1T01 → P1T02
P1T02 → P1T03, P1T05, P1T09, P1T10, P1T12, P1T13, P1T16, P1T19
P1T03 → P1T04, P1T20
P1T05 → P1T06
P1T06 → P1T07, P1T08
P1T07 → P1T20
P1T09 → P1T18
P1T10 → P1T11, P1T17
P1T12 → P1T17
P1T13 → P1T14, P1T15, P1T17
P1T16 → P1T17
P1T17 → P1T18
P1T18 → P1T20
P1T19 → P1T20
P1T20 → P1T21, P1T22
P1T18 → P2T01
P2T01 → P2T02, P2T03, P2T07, P2T08
P2T04 → P2T05, P2T06
P1T10 → P2T06
```

---

## 3. Parallelization Strategy

Tasks that share no dependency edges can be developed concurrently by different
contributors (or in parallel coding sessions). Below are the maximum-parallelism
work streams after each synchronization point.

### Wave 1 — After P1T02 (types) is merged

These 7 tasks are **fully independent** and can all proceed in parallel:

| Stream | Tasks | Description |
|--------|-------|-------------|
| A — Parsing | P1T03 → P1T04 | YAML parser + validation |
| B — Agents | P1T05 → P1T06 → P1T07 + P1T08 | Agent types, loader, discovery, Claude |
| C — DAG | P1T09 | DAG builder & topo sort |
| D — Templates | P1T10 → P1T11 | Template engine + truncation |
| E — Conditions | P1T12 | Condition evaluator |
| F — Audit | P1T13 → P1T14 + P1T15 | Audit logging + cleanup |
| G — Reporter | P1T19 | Result reporter |

### Wave 2 — After streams A–F converge

| Stream | Tasks | Description |
|--------|-------|-------------|
| H — Executor | P1T16 → P1T17 | SDK adapter + step executor |
| (G continues) | P1T19 | Can ship independently |

### Wave 3 — After executor + DAG + reporter are done

| Stream | Tasks | Description |
|--------|-------|-------------|
| I — Orchestrator | P1T18 | Sequential orchestrator |

### Wave 4 — After orchestrator + CLI

| Stream | Tasks | Description |
|--------|-------|-------------|
| J — CLI | P1T20 | CLI entry point |
| K — Examples | P1T21 | Example workflows |
| L — Integration | P1T22 | End-to-end test |

### Gantt-Style Visual (→ = serial, ‖ = parallel)

```
Time ──────────────────────────────────────────────────────────────────────▶

P1T01 ─▶ P1T02 ─┬─▶ P1T03 ─▶ P1T04 ─────────────────────────┐
                 ├─▶ P1T05 ─▶ P1T06 ─┬▶ P1T07 ───────────────┤
                 │                    └▶ P1T08                 │
                 ├─▶ P1T09 ───────────────────────────────┐    │
                 ├─▶ P1T10 ─▶ P1T11                       │    │
                 ├─▶ P1T12 ────────────────┐              │    │
                 ├─▶ P1T13 ─┬▶ P1T14      │              │    │
                 │          └▶ P1T15       │              │    │
                 ├─▶ P1T16 ───────────────┐│              │    │
                 └─▶ P1T19 ──────────────┐││              │    │
                                         ││▼              │    │
                                         │P1T17 ─────────┤    │
                                         │               ▼    │
                                         │             P1T18 ─┤
                                         │               │    │
                                         └───────────────┼────┘
                                                         ▼
                                                       P1T20
                                                      ┌──┴──┐
                                                   P1T21  P1T22
```

---

## 4. Task Detail Sheets

Each sheet specifies the exact types, functions, file paths, and test cases for
a commit-sized task. Sample code is included where it clarifies intent.

---

### P1T01 — Go Module Init + Project Scaffold

**Commit message:** `feat: init Go module and project directory scaffold`

**Actions:**
1. `go mod init github.com/alex-workflow-runner/workflow-runner`
2. Create empty directory structure
3. Add root `.gitignore`
4. Add placeholder `README.md`

**Files created:**

```
go.mod
.gitignore
README.md
cmd/workflow-runner/.gitkeep
pkg/workflow/.gitkeep
pkg/agents/.gitkeep
pkg/executor/.gitkeep
pkg/orchestrator/.gitkeep
pkg/audit/.gitkeep
pkg/memory/.gitkeep
pkg/reporter/.gitkeep
```

**`.gitignore`:**
```gitignore
# Binaries
workflow-runner
*.exe

# IDE
.idea/
*.swp

# Test/build artifacts
coverage.out
*.test

# Audit runs (local only)
.workflow-runs/

# OS
.DS_Store
```

**Verify:** `go build ./...` succeeds (nothing to build yet, but module resolves).

---

### P1T02 — Core Domain Types

**Commit message:** `feat(workflow): add core domain types for workflow, steps, agents, config`

**File:** `pkg/workflow/types.go`

```go
// Package workflow defines the core data model for workflow definitions,
// including steps, agent references, conditions, and configuration.
// It is the shared vocabulary used by the parser, DAG builder, template
// engine, executor, and orchestrator.
package workflow

// Workflow is the top-level representation of a parsed workflow YAML file.
type Workflow struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Inputs      map[string]Input  `yaml:"inputs"`
	Config      Config            `yaml:"config"`
	Agents      map[string]AgentRef `yaml:"agents"`
	Skills      []string          `yaml:"skills"`
	Steps       []Step            `yaml:"steps"`
	Output      OutputConfig      `yaml:"output"`
}

// Input defines a workflow-level input variable with optional default.
type Input struct {
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
}

// Config holds global workflow configuration.
type Config struct {
	Model            string           `yaml:"model"`
	AuditDir         string           `yaml:"audit_dir"`
	AuditRetention   int              `yaml:"audit_retention"`
	SharedMemory     SharedMemoryConfig `yaml:"shared_memory"`
	Provider         *ProviderConfig  `yaml:"provider,omitempty"`
	Streaming        bool             `yaml:"streaming"`
	LogLevel         string           `yaml:"log_level"`
	AgentSearchPaths []string         `yaml:"agent_search_paths"`
}

// SharedMemoryConfig controls shared memory between parallel agents.
type SharedMemoryConfig struct {
	Enabled         bool   `yaml:"enabled"`
	InjectIntoPrompt bool  `yaml:"inject_into_prompt"`
	InitialContent  string `yaml:"initial_content"`
	InitialFile     string `yaml:"initial_file"`
}

// ProviderConfig holds BYOK provider settings.
type ProviderConfig struct {
	Type      string `yaml:"type"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}

// AgentRef is a reference to an agent — either a file path or an inline definition.
type AgentRef struct {
	File   string       `yaml:"file,omitempty"`
	Inline *InlineAgent `yaml:"inline,omitempty"`
}

// InlineAgent defines an agent directly in the workflow YAML.
type InlineAgent struct {
	Description string   `yaml:"description"`
	Prompt      string   `yaml:"prompt"`
	Tools       []string `yaml:"tools"`
	Model       string   `yaml:"model"`
}

// Step represents a single execution unit in the workflow DAG.
type Step struct {
	ID        string    `yaml:"id"`
	Agent     string    `yaml:"agent"`
	Prompt    string    `yaml:"prompt"`
	DependsOn []string  `yaml:"depends_on"`
	Condition *Condition `yaml:"condition,omitempty"`
	Skills    []string  `yaml:"skills"`
	OnError   string    `yaml:"on_error"`   // "fail" (default), "skip", "retry"
	RetryCount int      `yaml:"retry_count"`
	Timeout   string    `yaml:"timeout"`
}

// Condition defines when a step should execute based on a prior step's output.
type Condition struct {
	Step        string `yaml:"step"`
	Contains    string `yaml:"contains,omitempty"`
	NotContains string `yaml:"not_contains,omitempty"`
	Equals      string `yaml:"equals,omitempty"`
}

// OutputConfig controls what gets reported after workflow completion.
type OutputConfig struct {
	Steps    []string        `yaml:"steps"`
	Format   string          `yaml:"format"` // "markdown", "json", "plain"
	Truncate *TruncateConfig `yaml:"truncate,omitempty"`
}

// TruncateConfig limits the size of injected step outputs.
type TruncateConfig struct {
	Strategy string `yaml:"strategy"` // "chars", "lines", "tokens"
	Limit    int    `yaml:"limit"`
}

// StepStatus represents the current execution state of a step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusFailed    StepStatus = "failed"
)

// StepResult holds the outcome of executing a single step.
type StepResult struct {
	StepID    string     `json:"step_id"`
	Status    StepStatus `json:"status"`
	Output    string     `json:"output"`
	Error     error      `json:"-"`
	ErrorMsg  string     `json:"error,omitempty"`
	StartedAt string     `json:"started_at"`
	EndedAt   string     `json:"ended_at"`
	SessionID string     `json:"session_id,omitempty"`
}
```

**Verify:** `go build ./pkg/workflow/`

---

### P1T03 — YAML Parser (Happy Path)

**Commit message:** `feat(workflow): YAML parser with happy-path parsing`

**Files:** `pkg/workflow/parser.go`, `pkg/workflow/parser_test.go`

**Dependencies:** `gopkg.in/yaml.v3`

```go
// Package workflow — parser.go
//
// ParseWorkflow reads a workflow YAML file and returns a validated Workflow
// struct. It performs structural parsing and basic type checking. Deep
// semantic validation (cycle detection, agent resolution) is handled by
// dedicated components.

package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseWorkflow reads a YAML file at the given path and returns a Workflow.
func ParseWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %q: %w", path, err)
	}
	return ParseWorkflowBytes(data)
}

// ParseWorkflowBytes parses YAML bytes into a Workflow struct.
func ParseWorkflowBytes(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	// Apply defaults
	if wf.Config.AuditDir == "" {
		wf.Config.AuditDir = ".workflow-runs"
	}
	if wf.Config.LogLevel == "" {
		wf.Config.LogLevel = "info"
	}
	if wf.Output.Format == "" {
		wf.Output.Format = "markdown"
	}

	return &wf, nil
}
```

**Test cases (table-driven):**

| Case | Input | Expected |
|------|-------|----------|
| Valid minimal workflow | name + 1 step | Parses OK |
| Valid full workflow | All fields populated | All fields deserialized |
| Default audit_dir | Config without audit_dir | `.workflow-runs` |
| Default format | No output.format | `markdown` |
| Invalid YAML | `[[[broken` | Error contains "parsing workflow YAML" |

**Verify:** `go test ./pkg/workflow/ -run TestParseWorkflow`

---

### P1T04 — YAML Parser Validation & Error Paths

**Commit message:** `feat(workflow): add semantic validation to YAML parser`

**File:** extends `pkg/workflow/parser.go`, `pkg/workflow/parser_test.go`

**New function:**

```go
// ValidateWorkflow performs semantic validation on a parsed workflow.
// It checks: required fields, unique step IDs, agent references exist,
// depends_on references exist, condition references exist, and template
// references are syntactically valid.
func ValidateWorkflow(wf *Workflow) error {
	// 1. Name is required
	// 2. At least one step
	// 3. Step IDs unique
	// 4. Each step.Agent references a key in wf.Agents
	// 5. Each depends_on references a valid step ID
	// 6. Each condition.Step references a valid step ID
	// 7. condition.Step must be in depends_on (transitive)
	// 8. Template references ({{steps.X.output}}) point to valid step IDs
	// 9. No step depends on itself
}
```

**Validation rules — detailed:**

| Rule | Error message template |
|------|----------------------|
| Empty name | `workflow name is required` |
| No steps | `workflow must have at least one step` |
| Duplicate step ID | `duplicate step ID %q` |
| Step missing agent | `step %q: agent is required` |
| Step missing prompt | `step %q: prompt is required` |
| Unknown agent ref | `step %q: agent %q not defined in workflow agents` |
| Unknown depends_on | `step %q: depends_on references unknown step %q` |
| Self-dependency | `step %q: cannot depend on itself` |
| Unknown condition step | `step %q: condition references unknown step %q` |
| Condition step not upstream | `step %q: condition step %q must be an upstream dependency` |
| Invalid template ref | `step %q: template references unknown step %q in {{steps.%s.output}}` |
| Agent has neither file nor inline | `agent %q: must have either 'file' or 'inline' defined` |

**Test cases:** One test per validation rule, plus a compound test with multiple errors.

---

### P1T05 — Agent Types & Frontmatter Structs

**Commit message:** `feat(agents): add agent domain types and frontmatter structs`

**File:** `pkg/agents/types.go`

```go
// Package agents handles loading, parsing, and discovery of .agent.md files
// compatible with the VS Code custom agents format. It also supports
// Claude-format agent files with automatic tool name mapping.
package agents

// Agent represents a fully resolved agent definition ready for session creation.
type Agent struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Tools       []string          `yaml:"tools"`
	Agents      []string          `yaml:"agents"`
	Model       ModelSpec         `yaml:"-"` // Parsed from string or []string
	Prompt      string            `yaml:"-"` // Markdown body (system prompt)
	MCPServers  map[string]MCPServerConfig `yaml:"mcp-servers"`
	Handoffs    []Handoff         `yaml:"handoffs"`
	Hooks       *HooksConfig      `yaml:"hooks,omitempty"`
	SourceFile  string            `yaml:"-"` // Path to the .agent.md file

	// Fields parsed but ignored at runtime (interactive-only).
	ArgumentHint          string `yaml:"argument-hint"`
	UserInvocable         *bool  `yaml:"user-invocable"`
	DisableModelInvocation *bool `yaml:"disable-model-invocation"`
	Target                string `yaml:"target"`
}

// ModelSpec holds either a single model string or a priority list.
type ModelSpec struct {
	Models []string // First available wins
}

// MCPServerConfig mirrors the VS Code MCP server definition.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// Handoff defines a transition from one agent to another.
type Handoff struct {
	Label  string `yaml:"label"`
	Agent  string `yaml:"agent"`
	Prompt string `yaml:"prompt"`
	Send   *bool  `yaml:"send,omitempty"`
	Model  string `yaml:"model,omitempty"`
}

// HooksConfig mirrors the VS Code agent hooks structure.
type HooksConfig struct {
	OnPreToolUse  string `yaml:"onPreToolUse"`
	OnPostToolUse string `yaml:"onPostToolUse"`
}
```

**Verify:** `go build ./pkg/agents/`

---

### P1T06 — Agent `.agent.md` Loader

**Commit message:** `feat(agents): parse .agent.md files with YAML frontmatter + markdown body`

**Files:** `pkg/agents/loader.go`, `pkg/agents/loader_test.go`

**Core function:**

```go
// LoadAgentFile parses a .agent.md file and returns an Agent.
// The file must contain YAML frontmatter delimited by "---" lines,
// followed by a markdown body that becomes the agent's system prompt.
func LoadAgentFile(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %q: %w", path, err)
	}
	return ParseAgentMarkdown(data, path)
}

// ParseAgentMarkdown splits a markdown file into YAML frontmatter and body,
// parses the frontmatter into Agent fields, and stores the body as Prompt.
func ParseAgentMarkdown(data []byte, sourcePath string) (*Agent, error) {
	// 1. Split on "---" delimiters
	// 2. Parse frontmatter as YAML into Agent struct
	// 3. Handle model field: string → []string{s}, []string → as-is
	// 4. Store markdown body as agent.Prompt
	// 5. Default agent.Name to filename (without extension) if empty
	// 6. Set agent.SourceFile = sourcePath
}
```

**Frontmatter splitting algorithm:**

```
Line 1: must be "---"
Scan until next "---" → frontmatter block
Everything after second "---" → markdown body (trim leading newline)
```

**Test cases:**

| Case | Input | Expected |
|------|-------|----------|
| Full frontmatter + body | All fields | All parsed |
| Minimal (name only) | `---\nname: x\n---\n# Prompt` | Name=x, Prompt="# Prompt" |
| No frontmatter | Just markdown | Error: missing frontmatter |
| Model as string | `model: gpt-5` | ModelSpec{["gpt-5"]} |
| Model as array | `model: [gpt-5, gpt-4]` | ModelSpec{["gpt-5","gpt-4"]} |
| Default name from filename | No name in frontmatter | Derived from path |
| Handoffs parsed | handoffs array | Handoff structs populated |
| Unknown fields | Extra YAML keys | Ignored gracefully |

---

### P1T07 — Agent Discovery (Multi-Path Scan)

**Commit message:** `feat(agents): multi-path agent discovery with priority resolution`

**Files:** `pkg/agents/discovery.go`, `pkg/agents/discovery_test.go`

```go
// DiscoverAgents scans all configured agent search paths and returns a map
// of agent-name → Agent. When agents with the same name exist in multiple
// locations, higher-priority sources win.
//
// Priority order (highest first):
//   1. Explicit paths from workflow YAML (agents.*.file)
//   2. .github/agents/*.agent.md and .github/agents/*.md
//   3. .claude/agents/*.md (Claude format)
//   4. ~/.copilot/agents/*.agent.md
//   5. config.agent_search_paths entries
func DiscoverAgents(workspaceDir string, extraPaths []string) (map[string]*Agent, error)

// ResolveAgents loads agents from explicit workflow YAML refs and merges them
// with discovered agents. Explicit refs always take highest priority.
//
// workspaceDir is used for agent discovery in standard locations.
// workflowDir is used for resolving relative file paths in explicit agent refs.
// This design allows workflows to reference agent files relative to their own
// location, making them portable across directories.
//
// Returns an error if any workflow step references an agent that cannot be found.
func ResolveAgents(wf *Workflow, workspaceDir, workflowDir string) (map[string]*Agent, error)
```

**Key behavior:**
- Glob patterns: `*.agent.md` and `*.md` in `.github/agents/`
- Tie-breaker: `.github/agents/foo.agent.md` wins over `.claude/agents/foo.md`
- Agent name derived from `name` frontmatter field, falling back to filename stem
- Inline agents from workflow YAML are also resolved (not from files)
- **Relative agent file paths** in workflow YAML resolve against the workflow file's directory, not the current working directory

**Test cases:**
- Discovery from `.github/agents/` directory
- Discovery from `.claude/agents/` directory
- Tie-breaker: `.github` wins over `.claude`
- Explicit file path overrides discovered
- Inline agent resolved without file
- Missing agent referenced by step → error
- Empty directories → no agents found (not an error)
- Relative file path resolves against workflow directory

---

### P1T08 — Claude-Format Agent Support

**Commit message:** `feat(agents): Claude-format agent file support with tool name mapping`

**Files:** extends `pkg/agents/loader.go`, `pkg/agents/loader_test.go`

```go
// claudeToolMap maps Claude tool names to VS Code equivalents.
var claudeToolMap = map[string]string{
	"Read":     "view",
	"Grep":     "grep",
	"Glob":     "glob",
	"Bash":     "bash",
	"Write":    "create_file",
	"Edit":     "replace_string_in_file",
	"MultiEdit":"multi_replace_string_in_file",
}

// NormalizeClaudeAgent detects Claude-format conventions and maps them
// to VS Code-compatible fields. Heuristics:
//   - tools as comma-separated string → split into array
//   - Tool names mapped via claudeToolMap
//   - File located under .claude/agents/ directory
func NormalizeClaudeAgent(agent *Agent) {
	// Only runs when SourceFile is under .claude/agents/
}
```

**Test cases:**
- `tools: "Read, Grep, Bash"` → `["view", "grep", "bash"]`
- Mixed known/unknown tool names → known mapped, unknown kept as-is
- Already-array tools → no-op
- Non-Claude agent file → no-op

---

### P1T09 — DAG Builder + Topological Sort

**Commit message:** `feat(workflow): DAG builder with topological sort and cycle detection`

**Files:** `pkg/workflow/dag.go`, `pkg/workflow/dag_test.go`

```go
// DAGLevel is a set of steps that can execute concurrently — they share
// the same topological depth (all dependencies already satisfied).
type DAGLevel struct {
	Steps []Step
	Depth int // 0-indexed topological depth
}

// BuildDAG constructs an execution plan from the workflow steps.
// Steps are grouped into levels using Kahn's algorithm (BFS topo sort).
// Returns an error if a cycle is detected.
func BuildDAG(steps []Step) ([]DAGLevel, error) {
	// 1. Build adjacency list and in-degree map
	// 2. Seed queue with steps that have in-degree 0
	// 3. BFS: dequeue, add to current level, decrement dependents' in-degree
	// 4. When queue is empty and remaining steps still have in-degree > 0 → cycle
	// 5. Return levels in execution order
}

// ValidateDAG checks for structural issues beyond cycles:
//   - Orphan steps (depend on non-existent steps — caught by parser, but belt-and-suspenders)
//   - Disconnected subgraphs (warn, don't error)
func ValidateDAG(steps []Step) error
```

**Algorithm — Kahn's BFS:**

```
1. For each step, compute in-degree (count of depends_on entries)
2. Queue all steps with in-degree == 0 (no dependencies)
3. While queue is not empty:
   a. Drain entire queue into current level (these run in parallel)
   b. For each step in level, for each dependent step:
      - Decrement dependent's in-degree
      - If in-degree reaches 0, add to next queue
   c. Append level to result
4. If processed count < total steps → cycle exists
   - Walk remaining steps to identify the cycle for the error message
```

**Test cases:**

| Case | Steps | Expected Levels |
|------|-------|-----------------|
| Single step, no deps | A | [[A]] |
| Linear chain | A→B→C | [[A],[B],[C]] |
| Fan-out | A→{B,C,D} | [[A],[B,C,D]] |
| Fan-in | {B,C,D}→E | [[B,C,D],[E]] |
| Diamond | A→{B,C}→D | [[A],[B,C],[D]] |
| Complex DAG | A→{B,C}, B→D, C→D, D→E | [[A],[B,C],[D],[E]] |
| Multiple roots | {A,B}→C | [[A,B],[C]] |
| Cycle: A→B→A | A↔B | Error: cycle detected |
| Self-loop | A→A | Error: cycle detected |
| 3-node cycle | A→B→C→A | Error: cycle detected |

---

### P1T10 — Template Engine

**Commit message:** `feat(workflow): template engine for step output and input variable resolution`

**Files:** `pkg/workflow/template.go`, `pkg/workflow/template_test.go`

```go
// ResolveTemplate substitutes {{steps.X.output}} and {{inputs.Y}} placeholders
// in a prompt string. Returns an error if a referenced step has no result yet
// or an input variable is not defined.
//
// The results map is keyed by step ID → output string.
// The inputs map is keyed by input name → resolved value (CLI override or default).
func ResolveTemplate(prompt string, results map[string]string, inputs map[string]string) (string, error)

// extractTemplateRefs returns all step IDs referenced via {{steps.X.output}}
// in the given prompt string. Used by the validator to check references exist.
func ExtractTemplateRefs(prompt string) []string

// extractInputRefs returns all input names referenced via {{inputs.Y}}.
func ExtractInputRefs(prompt string) []string
```

**Regex patterns:**
- Step output: `\{\{steps\.([a-zA-Z0-9_-]+)\.output\}\}`
- Inputs: `\{\{inputs\.([a-zA-Z0-9_-]+)\}\}`

**Test cases:**
- No templates → passthrough
- Single step ref → substituted
- Multiple step refs → all substituted
- Input variable → substituted
- Mixed steps + inputs → all substituted
- Unknown step ref → error with step name
- Unknown input ref → error with input name
- Empty result for known step → empty string (not error)

---

### P1T11 — Output Truncation Strategies

**Commit message:** `feat(workflow): output truncation strategies for context window management`

**Files:** extends `pkg/workflow/template.go`, `pkg/workflow/template_test.go`

```go
// TruncateOutput applies the configured truncation strategy to a step's output
// before injecting it into a template. Returns the truncated string and a bool
// indicating whether truncation occurred.
func TruncateOutput(output string, cfg *TruncateConfig) (string, bool) {
	if cfg == nil {
		return output, false
	}
	switch cfg.Strategy {
	case "chars":
		return truncateChars(output, cfg.Limit)
	case "lines":
		return truncateLines(output, cfg.Limit)
	case "tokens":
		// Approximate: 1 token ≈ 4 chars (conservative)
		return truncateChars(output, cfg.Limit*4)
	default:
		return output, false
	}
}
```

**Truncation format when truncated:**
```
<original content up to limit>

... [truncated: 15234 chars total, showing first 2000]
```

**Test cases:**
- Under limit → no-op
- Exactly at limit → no-op
- Over limit (chars) → truncated + suffix
- Over limit (lines) → truncated + suffix
- Token strategy → approximation
- Nil config → passthrough
- Unknown strategy → passthrough

---

### P1T12 — Condition Evaluator

**Commit message:** `feat(workflow): condition evaluator for contains/not_contains/equals`

**Files:** `pkg/workflow/condition.go`, `pkg/workflow/condition_test.go`

```go
// EvaluateCondition checks whether a step's condition is met based on the
// referenced step's output. Returns true if the step should execute.
// If the step has no condition, always returns true.
func EvaluateCondition(cond *Condition, results map[string]string) (bool, error) {
	if cond == nil {
		return true, nil
	}

	output, ok := results[cond.Step]
	if !ok {
		return false, fmt.Errorf("condition references step %q which has no result", cond.Step)
	}

	switch {
	case cond.Contains != "":
		return strings.Contains(output, cond.Contains), nil
	case cond.NotContains != "":
		return !strings.Contains(output, cond.NotContains), nil
	case cond.Equals != "":
		return strings.TrimSpace(output) == strings.TrimSpace(cond.Equals), nil
	default:
		// Condition with no operator → treat as always true (log warning)
		return true, nil
	}
}
```

**Test cases:**
- contains: match → true
- contains: no match → false
- not_contains: present → false
- not_contains: absent → true
- equals: exact match → true
- equals: with whitespace → trimmed match
- equals: mismatch → false
- nil condition → true
- Missing step output → error
- Empty operators → true (with warning)

---

### P1T13 — Audit Logger (Directory + Metadata)

**Commit message:** `feat(audit): audit directory creation and step metadata writer`

**Files:** `pkg/audit/logger.go`, `pkg/audit/logger_test.go`

```go
// RunLogger manages the audit trail for a single workflow run.
// It creates the run directory, writes metadata files, and provides
// per-step loggers.
type RunLogger struct {
	RunDir       string
	WorkflowName string
	startedAt    time.Time
}

// NewRunLogger creates a timestamped audit directory for a workflow run.
// Directory format: <audit_dir>/<timestamp>_<workflow-name>/
func NewRunLogger(auditDir, workflowName string) (*RunLogger, error)

// WriteWorkflowMeta writes the initial workflow.meta.json with run metadata.
func (rl *RunLogger) WriteWorkflowMeta(wf *Workflow, inputs map[string]string) error

// SnapshotWorkflow copies the workflow YAML into the audit directory.
func (rl *RunLogger) SnapshotWorkflow(yamlPath string) error

// StepLogger creates a per-step audit logger. The step directory is created
// with a sequence-number prefix (e.g., "01_analyze/").
type StepLogger struct {
	StepDir  string
	StepID   string
	SeqNum   int
}

// NewStepLogger creates the step subdirectory and returns a StepLogger.
func (rl *RunLogger) NewStepLogger(stepID string, seqNum int) (*StepLogger, error)

// WriteStepMeta writes step.meta.json for a completed or failed step.
func (sl *StepLogger) WriteStepMeta(meta StepMeta) error

// StepMeta is the structured metadata for a single step execution.
type StepMeta struct {
	StepID       string            `json:"step_id"`
	Agent        string            `json:"agent"`
	AgentFile    string            `json:"agent_file"`
	Model        string            `json:"model"`
	Status       string            `json:"status"`
	StartedAt    string            `json:"started_at"`
	CompletedAt  string            `json:"completed_at"`
	DurationSecs float64           `json:"duration_seconds"`
	OutputFile   string            `json:"output_file"`
	DependsOn    []string          `json:"depends_on"`
	Condition    *Condition        `json:"condition,omitempty"`
	ConditionMet *bool             `json:"condition_result,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Error        string            `json:"error,omitempty"`
}
```

**Directory naming:** `2026-03-20T14-32-05_code-review-pipeline`  
Timestamp uses `-` instead of `:` for filesystem compatibility.

**Test cases:**
- Directory created with correct timestamp prefix
- workflow.meta.json is valid JSON
- Step directory created with sequence number
- Step metadata serialized correctly
- Concurrent step logger creation (same seq number for parallel)

---

### P1T14 — Audit Logger (Output + Prompt Writing)

**Commit message:** `feat(audit): step prompt and output file writing`

**Files:** extends `pkg/audit/logger.go`, `pkg/audit/logger_test.go`

```go
// WritePrompt writes the resolved prompt to prompt.md in the step directory.
func (sl *StepLogger) WritePrompt(prompt string) error

// WriteOutput writes the step's final output to output.md.
func (sl *StepLogger) WriteOutput(output string) error

// FinalizeRun writes final_output.md and updates workflow.meta.json status.
func (rl *RunLogger) FinalizeRun(status string, outputs map[string]string, outputSteps []string) error
```

**Test cases:**
- prompt.md contains exact resolved prompt
- output.md contains exact assistant output
- final_output.md aggregates specified output steps
- workflow.meta.json updated with end time and status

---

### P1T15 — Audit Cleanup (Retention Policy)

**Commit message:** `feat(audit): retention policy for old workflow runs`

**Files:** `pkg/audit/cleanup.go`, `pkg/audit/cleanup_test.go`

```go
// ApplyRetention deletes old run directories to keep only the most recent N.
// If retention is 0, keeps all runs. Directories are sorted by name (which
// embeds the timestamp, so lexicographic order = chronological order).
func ApplyRetention(auditDir string, retention int) error
```

**Behavior:**
- List all directories in `auditDir`
- Sort lexicographically (timestamp prefix ensures chronological)
- If count > retention, delete oldest `(count - retention)` directories
- retention=0 → keep all
- Missing audit dir → no-op

**Test cases:**
- 5 runs, retention=3 → 2 oldest deleted
- 3 runs, retention=5 → nothing deleted
- retention=0 → nothing deleted
- Empty directory → no-op

---

### P1T16 — SDK Adapter Interface + Mock

**Commit message:** `feat(executor): SDK adapter interface with mock implementation`

**Files:** `pkg/executor/sdk.go`, `pkg/executor/mock_sdk.go`

```go
// SessionExecutor abstracts the Copilot SDK session lifecycle.
// This interface allows testing without a real SDK/CLI connection.
type SessionExecutor interface {
	// CreateSession starts a new SDK session with the given configuration.
	CreateSession(ctx context.Context, cfg SessionConfig) (Session, error)
}

// Session represents an active SDK session.
type Session interface {
	// Send submits a prompt and blocks until the session reaches idle.
	// Returns the final assistant message content.
	Send(ctx context.Context, prompt string) (string, error)

	// SessionID returns the unique session identifier (for resume support).
	SessionID() string

	// Close terminates the session and releases resources.
	Close() error
}

// SessionConfig holds the configuration for creating a new SDK session.
type SessionConfig struct {
	SystemPrompt string
	Model        string
	Tools        []string
	MCPServers   map[string]interface{}
}

// --- Mock implementation for testing ---

// MockSessionExecutor returns pre-configured responses for testing.
type MockSessionExecutor struct {
	// Responses maps step prompts (or substrings) to mock outputs.
	Responses map[string]string
	// Error to return from CreateSession (if set).
	CreateErr error
}

// MockSession holds a single mock session's state.
type MockSession struct {
	id        string
	responses map[string]string
	sendErr   error
}
```

**The mock allows full unit testing of the executor and orchestrator without needing
the Copilot CLI installed.**

---

### P1T17 — Step Executor (Core Loop)

**Commit message:** `feat(executor): step executor with template resolution, conditions, and audit`

**Files:** `pkg/executor/executor.go`, `pkg/executor/executor_test.go`

```go
// StepExecutor executes a single workflow step: resolves its prompt template,
// evaluates conditions, creates an SDK session, sends the prompt, captures
// the output, and writes audit files.
type StepExecutor struct {
	SDK         SessionExecutor
	AuditLogger *audit.RunLogger
	Truncate    *workflow.TruncateConfig
}

// Execute runs a single step and returns its result.
// It is the caller's responsibility to ensure dependencies are satisfied.
func (se *StepExecutor) Execute(
	ctx context.Context,
	step workflow.Step,
	agent *agents.Agent,
	results map[string]string,
	inputs map[string]string,
	seqNum int,
) (*workflow.StepResult, error) {
	// 1. Evaluate condition → skip if not met
	// 2. Resolve prompt template ({{steps.X.output}}, {{inputs.Y}})
	// 3. Apply truncation to injected outputs
	// 4. Create step audit logger
	// 5. Write prompt.md
	// 6. Create SDK session with agent config
	// 7. Send resolved prompt
	// 8. Capture output
	// 9. Write output.md + step.meta.json
	// 10. Return StepResult
}
```

**Test cases (using MockSessionExecutor):**
- Happy path: step executes and returns output
- Condition not met → step skipped, status=skipped
- Template resolution error → StepResult with error
- SDK session creation failure → propagated error
- SDK send failure → propagated error
- Audit files written correctly

---

### P1T18 — Sequential Orchestrator

**Commit message:** `feat(orchestrator): sequential DAG execution engine`

**Files:** `pkg/orchestrator/orchestrator.go`, `pkg/orchestrator/orchestrator_test.go`

```go
// Orchestrator executes a workflow's DAG by processing levels sequentially.
// In Phase 1 (MVP), steps within a level are also executed sequentially.
// Phase 2 upgrades this to concurrent execution within levels.
type Orchestrator struct {
	Executor    *executor.StepExecutor
	Agents      map[string]*agents.Agent
	Inputs      map[string]string
}

// Run executes the workflow, processing DAG levels in order.
// Returns a map of step ID → output for all completed steps.
func (o *Orchestrator) Run(ctx context.Context, wf *workflow.Workflow) (map[string]*workflow.StepResult, error) {
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	results := make(map[string]*workflow.StepResult)
	outputMap := make(map[string]string) // step ID → output text

	for _, level := range levels {
		for _, step := range level.Steps {
			agent := o.Agents[step.Agent]
			result, err := o.Executor.Execute(ctx, step, agent, outputMap, o.Inputs, level.Depth)
			if err != nil {
				return results, fmt.Errorf("step %q failed: %w", step.ID, err)
			}
			results[step.ID] = result
			if result.Status == workflow.StepStatusCompleted {
				outputMap[step.ID] = result.Output
			}
		}
	}
	return results, nil
}
```

**Test cases:**
- 3-step linear pipeline → all execute in order
- Step with condition: met → executes
- Step with condition: not met → skipped
- Step failure → error propagated
- Empty workflow → no-op

---

### P1T19 — Result Reporter

**Commit message:** `feat(reporter): output formatting with markdown, JSON, and plain text`

**Files:** `pkg/reporter/reporter.go`, `pkg/reporter/reporter_test.go`

```go
// FormatOutput collects results from the specified output steps and formats
// them according to the output configuration.
func FormatOutput(
	results map[string]*workflow.StepResult,
	outputCfg workflow.OutputConfig,
) (string, error) {
	// 1. Collect outputs from outputCfg.Steps
	// 2. Skip steps that were skipped or have no output
	// 3. Format according to outputCfg.Format
}
```

**Format templates:**

Markdown:
```markdown
# Workflow Results

## Step: approve-action
<output>

## Step: changes-action
<output>
```

JSON:
```json
{
  "steps": {
    "approve-action": { "status": "completed", "output": "..." },
    "changes-action": { "status": "skipped" }
  }
}
```

Plain:
```
=== approve-action ===
<output>

=== changes-action ===
<output>
```

---

### P1T20 — CLI Entry Point

**Commit message:** `feat(cli): add 'run' command with workflow loading and execution`

**File:** `cmd/workflow-runner/main.go`

```go
func main() {
	// Parse CLI flags:
	//   workflow-runner run --workflow <path> [--inputs key=value ...] [--audit-dir <dir>]

	// 1. Parse --workflow flag (required)
	// 2. Parse --inputs flags (repeated key=value pairs)
	// 3. Parse --audit-dir flag (optional override)
	// 4. Load and validate workflow YAML
	// 5. Merge CLI inputs with workflow input defaults
	// 6. Discover and resolve agents
	// 7. Decide SDK executor (real or error if CLI not found)
	// 8. Create audit logger
	// 9. Apply retention policy
	// 10. Build orchestrator
	// 11. Run workflow
	// 12. Format and print output
	// 13. Finalize audit trail
}
```

**CLI interface (using standard `flag` package for MVP):**

```
Usage: workflow-runner run [options]

Options:
  --workflow    Path to workflow YAML file (required)
  --inputs      Key=value input pairs (repeatable)
  --audit-dir   Override audit directory
  --dry-run     Validate without executing (Phase 6, flag reserved)
  --verbose     Enable verbose logging
```

**Error behavior:**
- Missing --workflow → print usage, exit 1
- Invalid YAML → print validation errors, exit 1
- Missing agents → print which agents not found, exit 1
- Step failure → print error + audit path, exit 1

---

### P1T21 — Example: Sequential Workflow + Agents

**Commit message:** `docs: add 3-step sequential example workflow and agent files`

**Files:**

1. `examples/simple-sequential.yaml`
2. `agents/security-reviewer.agent.md`
3. `agents/performance-reviewer.agent.md`
4. `agents/aggregator.agent.md`

**`examples/simple-sequential.yaml`:**

```yaml
name: "simple-sequential-review"
description: "Three-step sequential code review pipeline"

inputs:
  files:
    description: "Files to review"
    default: "*.go"

config:
  model: "gpt-4o"
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

---

### P1T22 — Integration Test (End-to-End with Mock)

**Commit message:** `test: end-to-end integration test with mock SDK`

**Files:** `integration_test.go` (root), or `pkg/orchestrator/integration_test.go`

**Test scenario:**
1. Load `examples/simple-sequential.yaml`
2. Resolve agents from `agents/` directory
3. Use `MockSessionExecutor` with canned responses
4. Run orchestrator
5. Assert: all 3 steps completed
6. Assert: template substitution occurred (step 2 prompt contains step 1 output)
7. Assert: audit directory created with expected structure
8. Assert: reporter output matches expected format

---

## 5. Phase 2 Tasks

### P2T01 — Parallel Orchestrator

**Commit message:** `feat(orchestrator): parallel step execution within DAG levels`

**Files:** extends `pkg/orchestrator/orchestrator.go`, `pkg/orchestrator/orchestrator_test.go`

```go
// RunParallel executes the workflow DAG with concurrent step execution
// within each level. It uses goroutines and sync.WaitGroup for fan-out,
// and a results channel for fan-in.
func (o *Orchestrator) RunParallel(ctx context.Context, wf *workflow.Workflow) (map[string]*workflow.StepResult, error) {
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	results := NewResultsStore()

	for _, level := range levels {
		var wg sync.WaitGroup
		errCh := make(chan error, len(level.Steps))

		for _, step := range level.Steps {
			wg.Add(1)
			go func(s workflow.Step) {
				defer wg.Done()
				agent := o.Agents[s.Agent]
				result, err := o.Executor.Execute(
					ctx, s, agent, results.OutputMap(), o.Inputs, level.Depth,
				)
				if err != nil {
					errCh <- fmt.Errorf("step %q: %w", s.ID, err)
					return
				}
				results.Store(s.ID, result)
			}(step)
		}

		wg.Wait()
		close(errCh)

		// Collect errors
		for err := range errCh {
			return results.All(), err // Fail fast on first error
		}
	}
	return results.All(), nil
}
```

---

### P2T02 — Thread-Safe Results Store

**Commit message:** `feat(orchestrator): thread-safe results store for concurrent step execution`

**Files:** `pkg/orchestrator/results.go`, `pkg/orchestrator/results_test.go`

```go
// ResultsStore is a concurrent-safe map for storing step execution results.
// It is used during parallel DAG execution where multiple goroutines write
// results simultaneously.
type ResultsStore struct {
	mu      sync.RWMutex
	results map[string]*workflow.StepResult
}

func NewResultsStore() *ResultsStore
func (rs *ResultsStore) Store(stepID string, result *workflow.StepResult)
func (rs *ResultsStore) Get(stepID string) (*workflow.StepResult, bool)
func (rs *ResultsStore) OutputMap() map[string]string  // snapshot for template resolution
func (rs *ResultsStore) All() map[string]*workflow.StepResult
```

**Important:** `OutputMap()` returns a snapshot (copy) to avoid races during template resolution.

---

### P2T03 — Configurable Max Concurrency

**Commit message:** `feat(orchestrator): semaphore-based max concurrency limiter`

**Files:** extends `pkg/orchestrator/orchestrator.go`

```go
// Semaphore limits concurrent step execution. If a level has 10 steps
// but maxConcurrency=3, only 3 goroutines run at a time.
type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(max int) *Semaphore {
	return &Semaphore{ch: make(chan struct{}, max)}
}

func (s *Semaphore) Acquire() { s.ch <- struct{}{} }
func (s *Semaphore) Release() { <-s.ch }
```

**Config:**
```yaml
config:
  max_concurrency: 4  # 0 = unlimited
```

---

### P2T04 — Shared Memory Manager

**Commit message:** `feat(memory): shared memory manager with mutex-protected read/write`

**Files:** `pkg/memory/manager.go`, `pkg/memory/manager_test.go`

```go
// Manager provides a shared memory file that parallel agents can read from
// and write to. Writes are serialized via a mutex. The memory content can
// be injected into prompts or exposed as tools.
type Manager struct {
	mu       sync.RWMutex
	content  strings.Builder
	filePath string // Path within audit directory
}

// NewManager creates a shared memory manager, optionally initialized with content.
func NewManager(auditDir string, initialContent string) (*Manager, error)

// Read returns the current memory content. Thread-safe.
func (m *Manager) Read() string

// Write appends a timestamped, agent-attributed entry. Thread-safe.
func (m *Manager) Write(agentName string, entry string) error

// Flush persists the current memory content to disk.
func (m *Manager) Flush() error
```

**Write format:**
```
[2026-03-20T14:32:15Z] [security-reviewer] Found SQL injection in auth/login.go:42
```

---

### P2T05 — Shared Memory Tools

**Commit message:** `feat(memory): read_memory and write_memory SDK tool definitions`

**Files:** `pkg/memory/tools.go`, `pkg/memory/tools_test.go`

Registers `read_memory` and `write_memory` as custom tools on the SDK session
so the LLM can interact with shared memory during execution.

---

### P2T06 — Prompt Injection for Shared Memory

**Commit message:** `feat(memory): inject shared memory content into step prompts`

**Files:** extends `pkg/memory/manager.go`

When `inject_into_prompt: true`, prepend the current shared memory content
to the prompt before sending it to the SDK session. This ensures the agent
sees cross-agent context without needing to call `read_memory`.

---

### P2T07 — Example: Fan-Out/Fan-In Pipeline

**Commit message:** `docs: add fan-out/fan-in example workflow with shared memory`

**File:** `examples/code-review-pipeline.yaml`

A workflow with:
- 1 analysis step
- 3 parallel review steps (security, performance, style)
- 1 aggregation step (fan-in)

---

### P2T08 — Integration Test: Parallel Execution

**Commit message:** `test: integration test verifying parallel step execution`

Verify that:
- Steps in the same DAG level execute concurrently (measure wall-clock time)
- Fan-in waits for all parallel steps before proceeding
- Results from parallel steps are correctly merged
- Shared memory is accessible across parallel steps

---

## 6. Phase 3–6 Outline

These phases are not broken into commit-level tasks yet. They will be
specified after Phase 1 and 2 are implemented and learnings are captured.

### Phase 3 — Audit Trail & Monitoring

- Full transcript logging (`transcript.jsonl` — every session event via `session.On()`)
- Tool call logging (`tool_calls.jsonl`)
- `workflow-runner watch --run <dir>` — multiplexed live monitoring
- Per-step `errors.log`
- DAG visualization export (`dag.dot` for Graphviz)

### Phase 4 — Conditional Branching & Handoffs

- `contains` / `not_contains` / `equals` conditions (note: MVP already implements evaluation
  logic in P1T12, but the orchestrator skip logic may need refinement)
- Handoff mode: auto-generate DAG edges from agent `handoffs` frontmatter
- Example: review → decide → branch (approve or request changes)

### Phase 5 — Production Hardening

- Per-step and per-workflow `context.WithTimeout`
- Error handling strategies: `on_error: fail | skip | retry` + `retry_count`
- Structured logging (`slog` with step context)
- OpenTelemetry integration (trace per workflow, span per step)
- BYOK provider configuration from YAML
- Skill directory integration (`SessionConfig.SkillDirectories`)
- Agent `hooks` frontmatter → SDK `SessionHooks` mapping

### Phase 6 — Advanced Features

- Regex and JSON path conditions
- LLM-based condition evaluation (lightweight classifier call)
- Loop/iteration steps (`for_each: "{{steps.X.output | split_lines}}"`)
- Sub-workflow inclusion (`import: other-workflow.yaml`)
- Dry-run mode (`--dry-run` validates without executing)
- Output to file, webhook, or PR comment
- Watch mode (re-run on file changes via `fsnotify`)

---

## Appendix A: Go Module Dependencies

| Dependency | Purpose | Task |
|------------|---------|------|
| `gopkg.in/yaml.v3` | YAML parsing | P1T03 |
| `github.com/github/copilot-sdk/go` | Copilot SDK | P1T16 (real adapter) |

No other external dependencies for Phase 1. Standard library covers:
- `regexp` — template parsing
- `encoding/json` — audit files
- `os`, `path/filepath` — file I/O
- `sync` — WaitGroup, Mutex, RWMutex
- `time` — timestamps
- `strings` — condition evaluation
- `context` — cancellation
- `flag` — CLI parsing
- `fmt`, `log` — error wrapping, logging

---

## Appendix B: Critical Path

The **critical path** (longest chain of serial dependencies) determines the
minimum calendar time to complete Phase 1:

```
P1T01 → P1T02 → P1T16 → P1T17 → P1T18 → P1T20 → P1T22
  1        2       16      17      18      20      22
```

That's **7 serial tasks** on the critical path. All other tasks can be
developed in parallel on feature branches and merged as their dependencies
are met. The critical path determines the minimum sequential work; maximizing
parallelism on the non-critical streams (agents, DAG, templates, audit, reporter)
is key to fast delivery.

---

## Appendix C: File-to-Task Mapping

| File | Created in | Modified in |
|------|-----------|-------------|
| `go.mod` | P1T01 | P1T03 (add yaml.v3) |
| `pkg/workflow/types.go` | P1T02 | — |
| `pkg/workflow/parser.go` | P1T03 | P1T04 |
| `pkg/workflow/parser_test.go` | P1T03 | P1T04 |
| `pkg/workflow/dag.go` | P1T09 | — |
| `pkg/workflow/dag_test.go` | P1T09 | — |
| `pkg/workflow/template.go` | P1T10 | P1T11 |
| `pkg/workflow/template_test.go` | P1T10 | P1T11 |
| `pkg/workflow/condition.go` | P1T12 | — |
| `pkg/workflow/condition_test.go` | P1T12 | — |
| `pkg/agents/types.go` | P1T05 | — |
| `pkg/agents/loader.go` | P1T06 | P1T08 |
| `pkg/agents/loader_test.go` | P1T06 | P1T08 |
| `pkg/agents/discovery.go` | P1T07 | — |
| `pkg/agents/discovery_test.go` | P1T07 | — |
| `pkg/executor/sdk.go` | P1T16 | — |
| `pkg/executor/mock_sdk.go` | P1T16 | — |
| `pkg/executor/executor.go` | P1T17 | — |
| `pkg/executor/executor_test.go` | P1T17 | — |
| `pkg/orchestrator/orchestrator.go` | P1T18 | P2T01, P2T03 |
| `pkg/orchestrator/orchestrator_test.go` | P1T18 | P2T01, P2T08 |
| `pkg/orchestrator/results.go` | P2T02 | — |
| `pkg/orchestrator/results_test.go` | P2T02 | — |
| `pkg/audit/logger.go` | P1T13 | P1T14 |
| `pkg/audit/logger_test.go` | P1T13 | P1T14 |
| `pkg/audit/cleanup.go` | P1T15 | — |
| `pkg/audit/cleanup_test.go` | P1T15 | — |
| `pkg/memory/manager.go` | P2T04 | P2T06 |
| `pkg/memory/manager_test.go` | P2T04 | — |
| `pkg/memory/tools.go` | P2T05 | — |
| `pkg/memory/tools_test.go` | P2T05 | — |
| `pkg/reporter/reporter.go` | P1T19 | — |
| `pkg/reporter/reporter_test.go` | P1T19 | — |
| `cmd/workflow-runner/main.go` | P1T20 | — |
| `examples/simple-sequential.yaml` | P1T21 | — |
| `examples/code-review-pipeline.yaml` | P2T07 | — |
| `agents/security-reviewer.agent.md` | P1T21 | — |
| `agents/performance-reviewer.agent.md` | P1T21 | — |
| `agents/aggregator.agent.md` | P1T21 | — |

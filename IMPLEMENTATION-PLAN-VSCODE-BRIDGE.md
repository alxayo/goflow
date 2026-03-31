# Implementation Plan: VS Code Extension Bridge Mode

## Executive Summary

Add a `--bridge` mode to goflow that enables it to serve as a backend for VS Code extensions (replacing sec-check's Python agent). This unlocks:
- Using goflow as the backend for the sec-check VS Code extension
- Running any goflow workflow from VS Code with live progress UI
- Extensibility beyond security scanning (code review, decision helpers, etc.)

## Feasibility Research Summary

### Architecture Alignment

Both sec-check and goflow use the **Copilot SDK** natively:
- **sec-check:** `from copilot import CopilotClient` (Python SDK)
- **goflow:** `github.com/github/copilot-sdk/go` (Go SDK)

Both implementations:
1. Create a single SDK client (one connection to Copilot CLI)
2. Spawn multiple concurrent sessions for parallel agent execution
3. Use language-specific concurrency (Python: `asyncio.gather` + semaphore; Go: goroutines + WaitGroup)
4. Handle session events, tool invocations, and streaming output

The goflow `security-scan.yaml` workflow already mirrors sec-check's architecture:
```
discover → [bandit, guarddog, shellcheck, graudit, trivy] → aggregate → remediation
```

### Key Findings

| Aspect | Status |
|--------|--------|
| SDK capability parity | ✅ Both use Copilot SDK for session management |
| Parallel execution architecture | ✅ Identical semantics (concurrent sessions, semaphore limits) |
| Workflow language maturity | ✅ goflow YAML DAG is production-ready (Phase 1-2 complete) |
| Extension protocol requirements | ✅ JSON Lines protocol is language-agnostic |
| Skill/Agent reusability | ✅ `.agent.md` and `SKILL.md` work in both CLI and SDK |

### Current Extension Architecture (sec-check)

The sec-check VS Code extension communicates with a Python backend via:
- **Transport:** stdin/stdout JSON Lines protocol
- **Commands:** scan, cancel, discover, list_workflows
- **Events:** ready, progress, result, tool_status, log, error
- **UI Components:** Dashboard webview, tree views, chat participant, status bar

## Implementation Plan

### Phase 1: Bridge Protocol & Core Infrastructure

#### 1.1 Define Bridge Message Types
**File:** `pkg/bridge/types.go`

Define JSON-serializable message types for commands, events, and data structures:

**Commands (extension → goflow):**
```go
type BridgeCommand struct {
    Type   string      `json:"type"` // "scan", "cancel", "discover", "list_workflows"
    Folder string      `json:"folder,omitempty"`
    Mode   string      `json:"mode,omitempty"` // "parallel", "serial"
    Config ScanConfig  `json:"config,omitempty"`
}

type ScanConfig struct {
    Workflow       string            `json:"workflow,omitempty"`
    Inputs         map[string]string `json:"inputs,omitempty"`
    MaxConcurrency int               `json:"max_concurrency,omitempty"`
}
```

**Events (goflow → extension):**
```go
type ProgressMessage struct {
    Type       string        `json:"type"` // "progress"
    Phase      string        `json:"phase"` // "discovery", "parallel_scan", "synthesis", "complete", "error"
    Step       string        `json:"step,omitempty"`
    Scanners   []ScannerInfo `json:"scanners,omitempty"`
    IssuesFound int          `json:"issues_found,omitempty"`
    Message    string        `json:"message,omitempty"`
}

type ResultMessage struct {
    Type          string    `json:"type"` // "result"
    Status        string    `json:"status"` // "success", "error", "timeout"
    Findings      []Finding `json:"findings,omitempty"`
    ReportContent string    `json:"report_content,omitempty"`
    ReportPath    string    `json:"report_path,omitempty"`
}

type ScannerInfo struct {
    Name          string `json:"name"`
    State         string `json:"state"` // "pending", "running", "completed", "failed", "skipped"
    ToolAvailable bool   `json:"tool_available"`
    ToolName      string `json:"tool_name,omitempty"`
    IssuesFound   int    `json:"issues_found,omitempty"`
    Output        string `json:"output,omitempty"`
}

type Finding struct {
    Severity    string `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
    Title       string `json:"title"`
    Description string `json:"description"`
    FilePath    string `json:"file_path,omitempty"`
    LineNumber  int    `json:"line_number,omitempty"`
    Scanner     string `json:"scanner,omitempty"`
    RuleID      string `json:"rule_id,omitempty"`
}
```

#### 1.2 Implement Bridge Runner
**File:** `pkg/bridge/runner.go`

Core bridge execution engine:

```go
type BridgeRunner struct {
    Stdin       io.Reader
    Stdout      io.Writer
    Stderr      io.Writer
    WorkflowDir string
}

func (br *BridgeRunner) Run(ctx context.Context) error {
    // 1. Initialize and emit {"type": "ready"}
    // 2. Read JSON Lines commands from stdin
    // 3. Route commands (scan, cancel, discover, list_workflows)
    // 4. Execute workflows with progress callbacks
    // 5. Emit progress/result events to stdout (JSON Lines)
}

func (br *BridgeRunner) handleScan(cmd *BridgeCommand) error {
    // 1. Load workflow from path
    // 2. Merge workflow inputs with command config
    // 3. Create orchestrator with progress callbacks
    // 4. Execute workflow (Run or RunParallel based on mode)
    // 5. Parse findings from output
    // 6. Emit result message
}

func (br *BridgeRunner) handleCancel() error {
    // Abort running workflow via context cancellation
}

func (br *BridgeRunner) handleDiscover(folder string) error {
    // Query workflow discovery and emit tool_status
}

func (br *BridgeRunner) handleListWorkflows(folder string) error {
    // List available workflows in workspace
}
```

#### 1.3 Wire Progress Callbacks
**Files:** `pkg/orchestrator/orchestrator.go` (modify), `pkg/bridge/progress.go` (new)

Add progress callback support to orchestrator:

```go
type ProgressCallbacks struct {
    OnPhaseChange      func(phase string)
    OnStepStart        func(stepID string, agent string)
    OnStepComplete     func(stepID, output string)
    OnStepError        func(stepID string, err error)
    OnSessionEvent     func(event SessionEventInfo)
    OnScannerStateChange func(scannerName, state string, issuesFound int)
}

type Orchestrator struct {
    // ... existing fields ...
    ProgressCallbacks *ProgressCallbacks
}
```

Bridge uses callbacks to emit events:
- Emit `{"type": "progress", "phase": "discovery"}` when discovery starts
- Emit `{"type": "progress", "scanners": [...]}` when scanner states change
- Emit `{"type": "progress", "step": "scan-python", "message": "..."}` during execution
- Emit `{"type": "result", ...}` on completion

#### 1.4 Add --bridge CLI Command
**File:** `cmd/workflow-runner/main.go` (modify)

Add bridge command to CLI:

```go
const usage = `Usage: goflow run [options]
       goflow bridge [options]
       goflow version
...
`

func main() {
    os.Exit(run())
}

func run() int {
    return runArgs(os.Args[1:], os.Stdout, os.Stderr)
}

func runArgs(args []string, stdout, stderr io.Writer) int {
    if len(args) == 0 {
        fmt.Fprint(stderr, usage)
        return 1
    }

    switch args[0] {
    case "run":
        return runCommand(args[1:], stdout, stderr)
    case "bridge":
        return bridgeCommand(args[1:], stdout, stderr)
    case "version", "--version", "-version":
        fmt.Fprintln(stdout, buildInfo())
        return 0
    // ...
    }
}

func bridgeCommand(args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("bridge", flag.ContinueOnError)
    fs.SetOutput(stderr)
    
    workflowDir := fs.String("workflow-dir", ".", "Base directory for workflow discovery")
    verbose := fs.Bool("verbose", false, "Enable verbose logging")
    
    if err := fs.Parse(args); err != nil {
        return 1
    }
    
    runner := bridge.NewBridgeRunner(os.Stdin, stdout, stderr, *workflowDir)
    if *verbose {
        runner.SetVerbose(true)
    }
    
    if err := runner.Run(context.Background()); err != nil {
        fmt.Fprintf(stderr, "error: %v\n", err)
        return 1
    }
    return 0
}
```

---

### Phase 2: Workflow Discovery & Listing

#### 2.1 Implement Workflow Discovery
**File:** `pkg/bridge/discovery.go`

```go
type WorkflowInfo struct {
    Name        string                  `json:"name"`
    Path        string                  `json:"path"`
    Description string                  `json:"description"`
    Inputs      map[string]InputSchema  `json:"inputs,omitempty"`
}

type InputSchema struct {
    Description string `json:"description,omitempty"`
    Default     string `json:"default,omitempty"`
    Type        string `json:"type,omitempty"` // "string", "number", "boolean"
}

func DiscoverWorkflows(rootDir string) ([]WorkflowInfo, error) {
    // Search patterns (in order of priority):
    // 1. ./*.yaml (root workflows)
    // 2. ./workflows/*.yaml
    // 3. ./examples/*/*.yaml
    // 4. ./.github/workflows/*.yaml (filter for goflow workflows via name or schema)
    //
    // For each YAML file:
    // - Parse workflow.name, workflow.description, workflow.inputs
    // - Build WorkflowInfo
    // - Return sorted list
}
```

#### 2.2 Handle Discovery & List Commands
**File:** `pkg/bridge/runner.go` (add methods)

```go
func (br *BridgeRunner) handleDiscover(folder string) error {
    // Call DiscoverWorkflows(folder)
    // Emit {"type": "tool_status", "workflows": [WorkflowInfo, ...]}
}

func (br *BridgeRunner) handleListWorkflows(folder string) error {
    // Same as handleDiscover — list available workflows
}
```

---

### Phase 3: Findings Extraction & Parsing

#### 3.1 Create Findings Parser
**File:** `pkg/bridge/findings.go`

Parse structured findings from workflow report output:

```go
func ParseFindings(markdownOutput string) ([]Finding, error) {
    // Strategy: Use regex patterns to extract findings from Markdown tables, bullet lists, and code blocks
    //
    // Expected formats:
    // 1. Markdown table:
    //    | Severity | File | Line | Issue |
    //    | CRITICAL | src/main.go | 42 | hardcoded password |
    //
    // 2. Structured JSON in fence:
    //    ```json
    //    {"findings": [...]}
    //    ```
    //
    // 3. Markdown list:
    //    - **CRITICAL**: src/main.go:42 — hardcoded password
    //
    // Return []Finding with normalized severity levels
}

func NormalizeSeverity(level string) string {
    // Map various severity formats to: CRITICAL, HIGH, MEDIUM, LOW
    // Handles: critical/CRITICAL/Critical, error/Error, ERROR, high/HIGH, warning/WARNING, etc.
}
```

#### 3.2 Extract Findings from Step Results
Modify `StepExecutor` to optionally return structured findings hint:

```go
type StepResult struct {
    // ... existing fields ...
    FindingsHint string // JSON-encoded or markdown that contains findings data
}
```

Bridge parser will use this along with `output` text to extract findings.

---

### Phase 4: Testing & Validation

#### 4.1 Unit Tests for Bridge Components

**File:** `pkg/bridge/types_test.go`
- Test JSON marshalling/unmarshalling of all message types
- Validate command and event schema

**File:** `pkg/bridge/discovery_test.go`
- Test workflow discovery with mock file systems
- Verify sorting and filtering

**File:** `pkg/bridge/findings_test.go`
- Test finding extraction from various markdown formats
- Test severity normalization

#### 4.2 Integration Tests

**File:** `pkg/bridge/runner_test.go`
- Mock stdin/stdout for protocol round-trip testing
- Test scan command → progress events → result event flow
- Test cancel command during execution
- Test error handling and graceful shutdown

#### 4.3 End-to-End Bridge Test

**File:** `integration_bridge_test.go`
- Spawn goflow bridge subprocess
- Send scan command (e.g., security-scan workflow)
- Verify event sequence: ready → progress (discovery) → progress (scanners) → result
- Verify findings are parsed correctly

---

### Phase 5: Documentation

#### 5.1 Bridge Protocol Specification
**File:** `docs/reference/bridge-protocol.md`

Document JSON Lines protocol:
- Message types (commands and events)
- State machine (ready → scan → progress* → result → ready)
- Error handling and recovery
- Example conversations

#### 5.2 Bridge Mode Usage Guide
**File:** `docs/guide/bridge-mode.md`

- How to run goflow as a bridge
- Command-line options
- Workflow discovery and selection
- Configuration options

#### 5.3 VS Code Extension Integration Guide
**File:** `docs/guide/vscode-extension-integration.md`

Guidance for adapting sec-check extension:
- Replace Python bridge spawn with goflow spawn
- Map existing protocol commands (protocol is compatible)
- Workflow selection UI (new capability)
- Settings configuration for goflow binary path

---

### Phase 6: VS Code Extension Adaptation (Informational)

This phase is outside goflow scope but documented for clarity:

**Changes in sec-check `vscode-extension/src/backend/bridge.ts`:**

```typescript
// Current (Python):
this.process = spawn(pythonPath, ["-m", "agentsec.vscode_bridge"]);

// New (goflow):
const goflowPath = vscode.workspace
    .getConfiguration("goflow")
    .get("binaryPath", "goflow");

const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || ".";

this.process = spawn(goflowPath, ["bridge", "--workflow-dir", workspaceRoot], {
    cwd: workspaceRoot,
    stdio: ["pipe", "pipe", "pipe"],
    env: { ...process.env },
});
```

The JSON Lines protocol remains compatible — no parsing changes needed.

**Workflow Selection Enhancement (optional):**

Add UI for users to select which workflow to run:
```typescript
const workflows = await discoverWorkflows(workspaceRoot);
const selected = await vscode.window.showQuickPick(
    workflows.map(w => ({ label: w.name, description: w.description, workflow: w }))
);
```

---

## Technical Specifications

### Command Flow: Scan Execution

```
Extension sends: {"type": "scan", "folder": "/path/to/code", "config": {"workflow": "security-scan.yaml", "mode": "parallel"}}
            ↓
BridgeRunner.handleScan()
            ↓
Load security-scan.yaml from examples/security-scan/
Merge inputs with config
            ↓
Create Orchestrator with progress callbacks
Call orchestrator.RunParallel(ctx, workflow)
            ↓
During execution:
  - onPhaseChange("discovery") → emit progress
  - onStepStart("discover") → emit progress
  - onStepComplete("discover") → store output
  - onPhaseChange("parallel_scan") → emit progress
  - Loop: onScannerStateChange → emit scanner status updates
  - onStepComplete("aggregate") → emit progress
  - onPhaseChange("complete") → emit result with findings
            ↓
ParseFindings(aggregator output)
            ↓
Emit: {"type": "result", "status": "success", "findings": [...], "report_path": "..."}
```

### Event Sequence Example (Security Scan)

```json
{"type": "ready"}
{"type": "progress", "phase": "discovery", "step": "discover", "message": "Scanning for files and checking tool availability..."}
{"type": "progress", "phase": "parallel_scan", "scanners": [{"name": "bandit", "state": "running"}, {"name": "guarddog", "state": "pending"}, ...]}
{"type": "progress", "phase": "parallel_scan", "scanners": [{"name": "bandit", "state": "completed", "issues_found": 3}, {"name": "guarddog", "state": "running"}, ...]}
{"type": "progress", "phase": "parallel_scan", "scanners": [{"name": "guarddog", "state": "completed", "issues_found": 1}, ...]}
{"type": "progress", "phase": "synthesis", "step": "aggregate", "message": "Aggregating findings from all scanners..."}
{"type": "result", "status": "success", "findings": [{"severity": "CRITICAL", "title": "eval() detected", "file_path": "app.py", "line_number": 42}, ...], "report_path": ".workflow-runs/2026-03-31T14-30-00_security-scan/steps/08_aggregate/output.md"}
```

---

## Interface Changes

### Orchestrator Enhancement

```go
type Orchestrator struct {
    Executor           *executor.StepExecutor
    Agents             map[string]*agents.Agent
    Inputs             map[string]string
    MaxConcurrency     int
    CLIInteractive     bool
    OnUserInput        executor.UserInputHandler
    ProgressCallbacks  *ProgressCallbacks  // NEW
}

type ProgressCallbacks struct {
    OnPhaseChange       func(phase string)
    OnStepStart         func(stepID, agent string)
    OnStepComplete      func(stepID, output string)
    OnStepError         func(stepID string, err error)
    OnSessionEvent      func(event SessionEventInfo)
    OnScannerStateChange func(name, state string, issuesFound int)
}
```

All callbacks are optional (nil-safe). Existing code unaffected.

---

## Directory Structure

```
pkg/bridge/
├── types.go                 # Message types (BridgeCommand, ProgressMessage, etc.)
├── runner.go                # BridgeRunner main loop
├── discovery.go             # DiscoverWorkflows implementation
├── findings.go              # ParseFindings implementation
├── progress.go              # Progress callback helpers
├── types_test.go
├── discovery_test.go
├── findings_test.go
└── runner_test.go

cmd/workflow-runner/
└── main.go                  # (modify) Add bridge command and bridgeCommand func

docs/
├── reference/
│   └── bridge-protocol.md   # Protocol specification
└── guide/
    ├── bridge-mode.md       # Usage guide
    └── vscode-extension-integration.md  # Extension adaptation guide
```

---

## Acceptance Criteria

- [ ] `goflow bridge` starts and emits `{"type": "ready"}` to stdout
- [ ] Bridge reads JSON Lines commands from stdin
- [ ] `scan` command loads workflow, executes with progress events
- [ ] Progress events follow the defined schema and complete successfully
- [ ] `cancel` command aborts running workflow cleanly
- [ ] `discover` command lists available workflows in workspace
- [ ] `list_workflows` command returns WorkflowInfo with inputs metadata
- [ ] Findings are extracted from workflow output as structured JSON
- [ ] Unit tests cover message types, discovery, findings parsing
- [ ] Integration test covers full scan command flow
- [ ] End-to-end bridge test verifies event sequence for security-scan workflow
- [ ] Protocol documentation is complete and examples are working
- [ ] Bridge mode works with existing security-scan workflow and custom workflows

---

## Integration Points

### Workflow Ecosystem
- Existing workflows (security-scan, code-review, etc.) work unchanged
- New workflows can be created with standard YAML dialect
- No breaking changes to workflow syntax

### SDK Integration
- Goflow SDK executor continues to manage single client + multiple sessions
- Executor callbacks hook into SDK session event stream
- Progress events derived from SessionEventInfo (streaming mode)

### CLI
- Bridge mode accessible via `goflow bridge` subcommand
- Works alongside `goflow run` command
- No changes to existing CLI interface

---

## Migration Path for sec-check

1. **Phase 1-5:** Implement goflow bridge
2. **Testing:** Verify bridge with goflow workflows (security-scan, custom)
3. **Extension adaptation:** Fork/patch sec-check extension to spawn goflow instead of Python
4. **Deployment:** Release goflow binary via package managers (brew, etc.)
5. **User migration:** Users update sec-check extension which now uses goflow backend
6. **Benefits unlocked:**
   - Single-binary distribution (no Python venv setup)
   - Access to all goflow workflows from VS Code
   - Extensibility without rebuilding extension

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Protocol mismatch with extension | Comprehensive protocol tests, example conversations in docs |
| Performance regression vs Python | Benchmark bridge event emission, optimize JSON marshalling if needed |
| Discovery discovers non-goflow workflows | Add heuristics (check for `steps:` key, validate schema) or allow manual selection |
| Finding parsing fragile | Support multiple formats (markdown table, JSON, YAML), add parser tests for each |
| Breaking changes to goflow API | Only add fields to messages, don't remove/rename; version protocol if needed |

---

## Related Work

- **sec-check:** https://github.com/alxayo/sec-check (Python CLI + VS Code extension)
- **goflow workflows:** `examples/security-scan/`, `examples/code-review-pipeline/`, etc.
- **Copilot SDK (Go):** `github.com/github/copilot-sdk/go`
- **Copilot SDK (Python):** `from copilot import CopilotClient`

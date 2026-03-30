# Architecture

Technical overview of goflow's internal architecture and execution model.

!!! note "Current CLI status"
  The codebase contains both sequential and parallel orchestrator implementations, shared-memory building blocks, and truncation helpers. The current `goflow run` command uses the parallel orchestrator path (`RunParallel`), but does not automatically wire shared memory and does not automatically apply truncation during normal execution.

---

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          goflow CLI                              │
│  ┌───────────┐  ┌───────────┐  ┌────────────┐  ┌────────────┐   │
│  │  Parser   │→ │ DAG       │→ │Orchestrator│→ │  Reporter  │   │
│  │           │  │ Builder   │  │            │  │            │   │
│  └───────────┘  └───────────┘  └────────────┘  └────────────┘   │
│        ↓              ↓              ↓                          │
│  ┌───────────┐  ┌───────────┐  ┌────────────┐  ┌────────────┐   │
│  │  Agent    │  │ Template  │  │  Executor  │  │   Audit    │   │
│  │  Loader   │  │  Engine   │  │            │  │   Logger   │   │
│  └───────────┘  └───────────┘  └────────────┘  └────────────┘   │
│                                      │                          │
│                              ┌───────┴───────┐                  │
│                              │    Session    │                  │
│                              │    Monitor    │                  │
│                              └───────┬───────┘                  │
│                                      │                          │
│                                      ↓                          │
│                              ┌────────────┐                     │
│                              │ Copilot   │                     │
│                              │   SDK     │                     │
│                              └────────────┘                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### Parser (`pkg/workflow/parser.go`)

- Reads and validates workflow YAML files
- Resolves agent file references
- Validates schema compliance
- Reports configuration errors

### Agent Loader (`pkg/agents/loader.go`)

- Discovers agent files in standard paths
- Parses `.agent.md` frontmatter
- Extracts system prompts from markdown body
- Validates agent configurations

### DAG Builder (`pkg/workflow/dag.go`)

- Constructs dependency graph from steps
- Performs topological sort
- Detects circular dependencies
- Groups steps into parallel execution levels

### Orchestrator (`pkg/orchestrator/orchestrator.go`)

- Coordinates step execution order
- Contains both sequential and parallel execution implementations
- Handles condition evaluation
- Tracks step results

### Executor (`pkg/executor/`)

- Executes individual steps via the Copilot SDK (default) or CLI subprocess (`--cli` fallback)
- Creates isolated sessions per step
- Uses **event-based session monitoring** for completion detection
- Applies templates
- Captures outputs
- Routes BYOK provider configuration to the SDK

### Session Monitor (`pkg/executor/monitor.go`)

- Tracks session state and progress via SDK events
- Handles 67+ event types (`session.idle`, `tool.execution_start`, `assistant.message_delta`, etc.)
- Provides real-time progress callbacks for verbose output
- Eliminates timeout requirements for long-running sessions

### Template Engine (`pkg/workflow/template.go`)

- Resolves `{{inputs.X}}` templates
- Resolves `{{steps.Y.output}}` templates
- Contains truncation helpers, though the normal CLI path does not currently invoke them automatically

### Reporter (`pkg/reporter/reporter.go`)

- Formats final output
- Selects output steps and formats markdown/json/plain text
- Supports markdown/json/plain formats

### Audit Logger (`pkg/audit/logger.go`)

- Creates run directories
- Records step prompts and outputs
- Saves workflow metadata
- Manages retention policy

---

## Execution Flow

### Phase 1: Initialization

```
1. Parse workflow YAML
2. Validate schema
3. Load referenced agent files
4. Resolve input values (CLI + defaults)
5. Create audit directory
```

### Phase 2: Planning

```
1. Build dependency graph
2. Detect circular dependencies → error if found
3. Topological sort into execution levels
4. Level 0: steps with no dependencies
5. Level N: steps whose dependencies are all in levels 0..N-1
```

### Phase 3: Execution

```
For each level L in [0, 1, 2, ...]:
  For each step S in level L (parallel):
    1. Evaluate condition → skip if not met
    2. Resolve templates in prompt
    3. Create SDK session (or CLI subprocess with --cli)
    4. Send prompt to AI
    5. Capture output
    6. Write to audit trail
  Wait for all level L steps to complete
```

### Phase 4: Output

```
1. Collect step outputs per output.steps
2. Apply truncation
3. Format as markdown/json/plain
4. Write to stdout and audit trail
```

---

## Parallel Execution Model

### DAG Levels

Steps are grouped by dependency depth:

```yaml
steps:
  - id: A          # Level 0 (no deps)
  - id: B          # Level 0 (no deps)
  - id: C          # Level 1 (deps: [A])
    depends_on: [A]
  - id: D          # Level 1 (deps: [B])
    depends_on: [B]
  - id: E          # Level 2 (deps: [C, D])
    depends_on: [C, D]
```

**Execution:**
```
Level 0: A, B (parallel)
Level 1: C, D (parallel, after Level 0)
Level 2: E (after Level 1)
```

### Goroutine Model

```go
func ExecuteLevel(steps []Step) {
    var wg sync.WaitGroup
    results := make(chan StepResult, len(steps))
    
    for _, step := range steps {
        wg.Add(1)
        go func(s Step) {
            defer wg.Done()
            result := execute(s)
            results <- result
        }(step)
    }
    
    wg.Wait()
    close(results)
}
```

### Synchronization

- **sync.WaitGroup** — Wait for all parallel steps
- **Thread-safe results map** — Store step outputs
- **No blocking across levels** — Next level starts only after current completes

---

## SDK & CLI Integration

goflow ships two executor backends. The **Copilot SDK executor** is the default;
the legacy **CLI subprocess executor** is available via `--cli`.

| Backend | Flag | Module | How it talks to Copilot |
|---|---|---|---|
| SDK (default) | _(none)_ | `pkg/executor/copilot_sdk.go` | JSON-RPC over stdio to a single managed CLI process |
| CLI fallback | `--cli` | `pkg/executor/copilot_cli.go` | Spawns a new `copilot` subprocess per `Send()` call |

Both backends require the Copilot CLI binary on `$PATH` (or at `~/.copilot/copilot`).
The SDK is a Go library (`github.com/github/copilot-sdk/go`) compiled into the `goflow`
binary — users do not install it separately.

### Session Per Step

Each step creates an isolated SDK session:

```go
config := &copilot.SessionConfig{
    Model:          step.Model,
    SystemMessage:  &copilot.SystemMessageConfig{Content: step.SystemPrompt},
    AvailableTools: step.Tools,
    Provider:       providerConfig,  // nil = GitHub Models, non-nil = BYOK
}
session, _ := client.CreateSession(ctx, config)
result, _ := session.SendAndWait(ctx, copilot.MessageOptions{Content: prompt})
```

### Why Session Per Step?

1. **Deterministic agent selection** — Explicit agent assignment per step
2. **Clean context** — No cross-contamination between steps
3. **Parallel execution** — Independent sessions can run concurrently
4. **Audit clarity** — Each step has isolated transcript
5. **BYOK support** — Each session can route to a custom provider

### Tool Exposure

Tools are configured on the SDK session:

```go
config.AvailableTools = step.Tools  // e.g., ["grep", "read_file"]
```

The underlying Copilot CLI runtime handles tool execution regardless of which executor
backend is used.

---

## Shared Memory Implementation

### Storage

Shared memory is a simple in-memory string with mutex protection:

```go
type SharedMemory struct {
    mu      sync.RWMutex
    content string
}

func (m *SharedMemory) Read() string {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.content
}

func (m *SharedMemory) Write(content string, mode string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if mode == "append" {
        m.content += "\n" + content
    } else {
        m.content = content
    }
}
```

### Tool Integration

Shared memory tools are registered with the SDK:

```go
session.RegisterTool("shared_memory_read", func() string {
    return memory.Read()
})

session.RegisterTool("shared_memory_write", func(content, mode string) {
    memory.Write(content, mode)
})
```

### Prompt Injection

When `inject_into_prompt: true`:

```go
func buildPrompt(originalPrompt string, memory *SharedMemory) string {
    return fmt.Sprintf(`## Shared Memory (Current State)
%s

---

%s`, memory.Read(), originalPrompt)
}
```

---

## Audit Trail Structure

### Directory Layout

```
.workflow-runs/
└── YYYY-MM-DDTHH-MM-SS_workflow-name/
    ├── workflow.meta.json   # Run metadata
    ├── workflow.yaml        # Workflow snapshot
    ├── final_output.md      # Formatted output
    ├── memory.md            # Shared memory state
    └── steps/
        ├── 00_step-a/
        │   ├── step.meta.json   # Step metadata
        │   ├── prompt.md        # Resolved prompt
        │   └── output.md        # AI response
        └── 01_step-b/
            └── ...
```

### Metadata Files

**workflow.meta.json:**
```json
{
  "name": "code-review",
  "started_at": "2026-03-26T10:00:00Z",
  "completed_at": "2026-03-26T10:02:30Z",
  "status": "completed",
  "duration_ms": 150000,
  "inputs": {
    "files": "src/*.go"
  }
}
```

**step.meta.json:**
```json
{
  "id": "analyze",
  "agent": "code-analyzer",
  "model": "gpt-4o",
  "started_at": "2026-03-26T10:00:05Z",
  "completed_at": "2026-03-26T10:00:45Z",
  "duration_ms": 40000,
  "status": "completed",
  "condition_met": true
}
```

---

## MCP Server Integration

### Configuration

MCP servers are defined in agent files:

```yaml
mcp-servers:
  db-tools:
    command: docker
    args: ["run", "--rm", "db-mcp:latest"]
    env:
      DB_HOST: localhost
```

### Lifecycle

1. **Start** — MCP server process started when agent session begins
2. **Communication** — SDK communicates via stdio
3. **Stop** — Process terminated when step completes

### Tool Registration

MCP server tools are registered dynamically:

```go
for _, tool := range mcpServer.AvailableTools() {
    session.RegisterMCPTool(mcpServer.Name, tool)
}
```

---

## Error Handling

### Validation Errors

Caught during Phase 1 (Initialization):

- Invalid YAML syntax
- Missing required fields
- Unknown agent references
- Missing required inputs

### Execution Errors

Caught during Phase 3 (Execution):

- SDK/CLI failures
- Tool execution errors
- Timeout violations
- Model errors

### Error Propagation

```
Step error → Stop workflow → Report error → Exit(3)
```

Partial results are saved to audit trail before exit.

---

## Configuration Precedence

Settings are resolved in this order (later wins):

1. **Built-in defaults**
2. **Workflow config section**
3. **Agent file settings**
4. **Step-level overrides**
5. **CLI flags**

**Example (model selection):**
```
Default: provider default
← Workflow: config.model: "gpt-4o"
← Agent: model: "claude-3-opus"
← Step: model: "gpt-4-turbo"
← CLI: --model gpt-4o-mini
```

---

## See Also

- [Workflow Schema](workflow-schema.md) — Configuration reference
- [Agent Format](agent-format.md) — Agent file structure
- [CLI Reference](cli.md) — Command-line options

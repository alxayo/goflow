# Copilot SDK Executor — Plan & Specification

**Goal:** Replace `CopilotCLIExecutor` as the default backend with a new `CopilotSDKExecutor` that uses the Copilot SDK Go library, unlocking BYOK (Bring Your Own Key), streaming events, session resume, and efficient JSON-RPC communication — while preserving all CLI built-in tools.

**Scope:** New executor backend only. No changes to the orchestrator, DAG, parser, templates, conditions, audit, or reporter. The existing `CopilotCLIExecutor` is demoted to a `--cli` fallback flag.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Why Both Backends](#2-why-both-backends)
3. [Key Insight: SDK Wraps CLI](#3-key-insight-sdk-wraps-cli)
4. [Task Breakdown](#4-task-breakdown)
5. [Detailed Task Specifications](#5-detailed-task-specifications)
6. [File Inventory](#6-file-inventory)
7. [YAML Schema (No Changes)](#7-yaml-schema-no-changes)
8. [CLI Flag Changes](#8-cli-flag-changes)
9. [Testing Strategy](#9-testing-strategy)
10. [Risks & Mitigations](#10-risks--mitigations)
11. [Dependencies](#11-dependencies)
12. [Out of Scope](#12-out-of-scope)

---

## 1. Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                     main.go (CLI entry)                       │
│                                                              │
│  if --mock                 → MockSessionExecutor (unchanged)  │
│  elif --cli                → CopilotCLIExecutor (fallback)    │
│  else (default)            → CopilotSDKExecutor (new default) │
└────────────────┬────────────────────┬────────────────────────┘
                 │                    │
     ┌───────────▼──────┐  ┌─────────▼──────────┐
     │ CopilotSDKExecutor│  │ CopilotCLIExecutor │
     │ (default)         │  │ (--cli fallback)    │
     │                   │  │                     │
     │ Uses SDK Go lib   │  │ Spawns CLI process  │
     │ JSON-RPC to CLI   │  │ exec.Command(...)   │
     │ Streaming events  │  │ Stdout capture      │
     │ Session resume    │  │ One-shot per Send   │
     │ BYOK support      │  │ No BYOK             │
     └───────────────────┘  └─────────────────────┘
                 │                    │
     Both implement SessionExecutor interface
     Both produce Session with Send/SessionID/Close
     
     ┌───────────────────────────────────────────┐
     │         Copilot CLI Runtime                │
     │  (Always provides built-in tools:          │
     │   grep, view, semantic_search,             │
     │   replace_string_in_file, run_in_terminal, │
     │   memory, file_search, list_dir,           │
     │   create_file, manage_todo_list)           │
     └───────────────────────────────────────────┘
```

Both executors ultimately use the Copilot CLI runtime for tool execution. The difference:
- **CopilotCLIExecutor**: spawns `copilot` as a subprocess per `Send()` call
- **CopilotSDKExecutor**: uses the SDK Go library which manages the CLI process via JSON-RPC, adds BYOK provider routing, streaming, and session lifecycle management

---

## 2. Why Both Backends

| Dimension | SDK Executor (new default) | CLI Executor (--cli fallback) |
|---|---|---|
| **Setup** | Need `copilot` + SDK dependency | Just need `copilot` on PATH |
| **BYOK** | Native — OpenAI, Azure, Anthropic, Ollama | Not available — GitHub Models only |
| **GitHub subscription** | Not required (with BYOK) | Required |
| **Rate limits** | Use your own API quota (with BYOK) | Subject to GitHub Copilot quota |
| **Streaming** | Real-time event stream | Blocked on process exit |
| **Session resume** | `ResumeSession(session_id)` | Not possible |
| **Built-in tools** | All available (CLI runtime underneath) | All available |
| **Process overhead** | Single managed process, JSON-RPC | One OS process per `Send()` |
| **Local models** | Ollama, Foundry Local via BYOK | Not supported |

**Decision:** SDK executor is the default — it provides BYOK, streaming, session resume, and better performance with no downsides (all CLI built-in tools remain available). The CLI executor is retained as a `--cli` fallback for environments where the SDK dependency is problematic.

---

## 3. Key Insight: SDK Wraps CLI

The Copilot SDK Go library does **not** implement its own tool runtime. It manages the Copilot CLI process and communicates via JSON-RPC/stdio. This means:

- **All 11+ built-in tools** (grep, view, semantic_search, etc.) are **always available** regardless of executor choice
- **Agent/SKILL/MCP auto-discovery** from the CLI filesystem scan still works
- **Tool restriction** via `--available-tools` is handled by the SDK when configuring sessions
- **BYOK only routes LLM inference** to your provider — tool execution stays local via CLI

SDK config fields are **additive** to CLI discovery:
- `customAgents` → added to CLI-discovered agents
- `skillDirectories` → added to CLI skill search paths
- `mcpServers` → merged with CLI-discovered MCP servers
- Built-in tools → **cannot be removed** via SDK config (only via CLI flags)

---

## 4. Task Breakdown

| Task | Title | Files | Deps | Est. |
|---|---|---|---|---|
| **S1** | Add Copilot SDK Go dependency | `go.mod`, `go.sum` | — | Small |
| **S2** | Implement `CopilotSDKExecutor` | `pkg/executor/copilot_sdk.go` | S1 | Medium |
| **S3** | Wire provider config into `SessionConfig` | `pkg/executor/sdk.go` | S2 | Small |
| **S4** | SDK default + `--cli` fallback in `main.go` | `cmd/workflow-runner/main.go` | S2, S3 | Small |
| **S5** | Unit tests for SDK executor | `pkg/executor/copilot_sdk_test.go` | S2 | Medium |
| **S6** | Integration test with BYOK provider | `integration_test.go` | S4 | Small |
| **S7** | Update docs for BYOK usage | `docs/reference/model-selection.md` | S4 | Small |

**Critical path:** S1 → S2 → S3 → S4

---

## 5. Detailed Task Specifications

### S1 — Add Copilot SDK Go Dependency

**Commit message:** `build: add copilot-sdk/go dependency`

```bash
go get github.com/github/copilot-sdk/go
```

**Acceptance criteria:**
- `go.mod` lists `github.com/github/copilot-sdk/go`
- `go build ./...` succeeds
- Existing tests still pass (`go test ./...`)

---

### S2 — Implement CopilotSDKExecutor

**Commit message:** `feat(executor): add CopilotSDKExecutor for BYOK provider support`

**File:** `pkg/executor/copilot_sdk.go`

This is the core implementation. It must implement the existing `SessionExecutor` interface exactly — no interface changes required.

#### Struct Design

```go
// CopilotSDKExecutor uses the Copilot SDK Go library for session management.
// It supports BYOK providers (OpenAI, Anthropic, Azure, Ollama) while
// preserving access to all CLI built-in tools via the underlying runtime.
type CopilotSDKExecutor struct {
    // client is the shared SDK client. Created once, reused across sessions.
    // The SDK manages the CLI process lifecycle internally.
    client *copilot.Client

    // provider holds the BYOK provider configuration from the workflow YAML.
    provider *ProviderConfig
}

// ProviderConfig mirrors workflow.ProviderConfig to avoid a circular import.
// Passed from main.go when constructing the executor.
type ProviderConfig struct {
    Type      string // "openai", "anthropic", "azure", "ollama"
    BaseURL   string // e.g., "https://api.openai.com/v1"
    APIKeyEnv string // env var name holding the API key (never the key itself)
}
```

Note: We define a local `ProviderConfig` in the executor package rather than importing `workflow.ProviderConfig` directly to avoid a circular dependency (`executor` → `workflow` → types used by executor). The `main.go` maps between them.

#### Constructor

```go
// NewCopilotSDKExecutor creates an SDK-backed executor with optional BYOK provider.
// If provider is nil, the SDK uses GitHub Models (default Copilot provider) —
// you still get streaming, session resume, and JSON-RPC efficiency.
//
// When provider is non-nil, the API key is resolved from the environment variable
// named in provider.APIKeyEnv. It is never stored in the struct — only passed to
// the SDK client at construction time.
func NewCopilotSDKExecutor(provider *ProviderConfig) (*CopilotSDKExecutor, error) {
    var opts []copilot.ClientOption

    if provider != nil {
        // BYOK mode: resolve API key from environment.
        apiKey := os.Getenv(provider.APIKeyEnv)
        if apiKey == "" {
            return nil, fmt.Errorf("BYOK provider %q: env var %q is empty or not set",
                provider.Type, provider.APIKeyEnv)
        }
        opts = append(opts, copilot.WithProvider(provider.Type, copilot.ProviderConfig{
            BaseURL: provider.BaseURL,
            APIKey:  apiKey,
        }))
    }
    // If provider is nil, no WithProvider option → SDK uses GitHub Models.

    // Create SDK client (starts CLI process via JSON-RPC internally).
    client, err := copilot.NewClient(opts...)
    if err != nil {
        return nil, fmt.Errorf("creating Copilot SDK client: %w", err)
    }

    return &CopilotSDKExecutor{
        client:   client,
        provider: provider,
    }, nil
}
```

#### CreateSession

```go
func (e *CopilotSDKExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
    // Map our SessionConfig to SDK's session config.
    sdkCfg := &copilot.SessionConfig{
        SystemPrompt: cfg.SystemPrompt,
    }

    // Model: use first from priority list (fallback handled at our layer).
    if len(cfg.Models) > 0 {
        sdkCfg.Model = cfg.Models[0]
    }

    // Tool restriction: if agent specifies tools, restrict the SDK session.
    // If empty, all CLI built-in tools remain available (default).
    if len(cfg.Tools) > 0 {
        sdkCfg.AvailableTools = cfg.Tools
    }

    // MCP servers from agent config.
    if len(cfg.MCPServers) > 0 {
        sdkCfg.MCPServers = cfg.MCPServers
    }

    // Extra directories for per-step resource discovery.
    if len(cfg.ExtraDirs) > 0 {
        sdkCfg.AdditionalDirs = cfg.ExtraDirs
    }

    // Create the SDK session.
    sdkSession, err := e.client.CreateSession(ctx, sdkCfg)
    if err != nil {
        return nil, fmt.Errorf("creating SDK session: %w", err)
    }

    return &CopilotSDKSession{
        session:     sdkSession,
        cfg:         cfg,
        models:      cfg.Models,
    }, nil
}
```

#### Session Implementation

```go
// CopilotSDKSession wraps an SDK session to satisfy our Session interface.
type CopilotSDKSession struct {
    session     *copilot.Session
    cfg         SessionConfig
    models      []string
}

func (s *CopilotSDKSession) Send(ctx context.Context, prompt string) (string, error) {
    finalPrompt := composePrompt(s.cfg.SystemPrompt, prompt)

    // Handle interactive mode the same way as CLI executor: extract question,
    // ask user, append answer to prompt.
    if s.cfg.Interactive {
        if s.cfg.OnUserInput == nil {
            return "", errors.New("interactive mode enabled but no user input handler configured")
        }
        question := interactiveQuestionFromPrompt(prompt)
        answer, err := s.cfg.OnUserInput(question, nil)
        if err != nil {
            return "", fmt.Errorf("getting user input: %w", err)
        }
        finalPrompt = fmt.Sprintf("%s\n\nUser clarification:\n%s",
            finalPrompt, strings.TrimSpace(answer))
    }

    // Send prompt via SDK. The SDK handles the JSON-RPC communication
    // with the CLI process, waits for session.idle, and returns the
    // final assistant message content.
    resp, err := s.session.Send(ctx, finalPrompt)
    if err != nil {
        // If model unavailable, try fallback models.
        if isSDKModelUnavailable(err) && len(s.models) > 1 {
            return s.tryFallbackModels(ctx, finalPrompt)
        }
        return "", fmt.Errorf("SDK session send: %w", err)
    }

    // Extract the final assistant message text from the response.
    output := extractAssistantMessage(resp)
    if output == "" {
        return "", errors.New("SDK session returned empty output")
    }
    return output, nil
}

// tryFallbackModels attempts remaining models in the priority list.
// Called when the primary model is unavailable.
func (s *CopilotSDKSession) tryFallbackModels(ctx context.Context, prompt string) (string, error) {
    for _, model := range s.models[1:] {
        // Reconfigure session model (SDK may support this) or create
        // a new session with the fallback model. Implementation depends
        // on SDK API — may need to call session.SetModel() or create
        // a sibling session from the same client.
        //
        // If SDK doesn't support mid-session model change, this falls
        // back to returning the original error and letting the executor
        // retry with a new session (same pattern as CLI executor).
        _ = model
    }
    return "", fmt.Errorf("all models unavailable")
}

func (s *CopilotSDKSession) SessionID() string {
    return s.session.ID()
}

func (s *CopilotSDKSession) Close() error {
    return s.session.Close()
}
```

#### Model Fallback Strategy

The CLI executor handles model fallback by spawning a new process per model attempt. The SDK executor should implement fallback at the `CreateSession` level instead:

```go
// Alternative: model fallback at CreateSession level.
// If the SDK doesn't support per-Send model switching, the CopilotSDKExecutor
// handles fallback by trying each model as a separate session.
func (e *CopilotSDKExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
    models := append([]string{}, cfg.Models...)
    if len(models) == 0 {
        models = []string{""} // empty = SDK/CLI default
    }

    var lastErr error
    for _, model := range models {
        sdkCfg := buildSDKConfig(cfg, model)
        sdkSession, err := e.client.CreateSession(ctx, sdkCfg)
        if err != nil {
            if isSDKModelUnavailable(err) {
                lastErr = err
                continue
            }
            return nil, fmt.Errorf("creating SDK session: %w", err)
        }
        return &CopilotSDKSession{session: sdkSession, cfg: cfg}, nil
    }
    return nil, fmt.Errorf("all models unavailable: %w", lastErr)
}
```

Which pattern to use depends on the SDK API — determined during S1 when exploring the SDK package. **Both patterns satisfy the `SessionExecutor` interface contract.**

#### Helper Functions (Reused)

The existing helper functions in `copilot_cli.go` are reused:
- `composePrompt(systemPrompt, userPrompt)` — already in `copilot_cli.go`
- `interactiveQuestionFromPrompt(prompt)` — already in `copilot_cli.go`

These should be kept as unexported package-level functions (which they already are), accessible from both `copilot_cli.go` and `copilot_sdk.go`.

#### Close/Cleanup

```go
// Close releases the SDK client resources. Should be called when the
// workflow run completes (via defer in main.go).
func (e *CopilotSDKExecutor) Close() error {
    if e.client != nil {
        return e.client.Close()
    }
    return nil
}
```

This is a new method not on the `SessionExecutor` interface. The caller in `main.go` type-asserts or uses a separate `io.Closer` check.

---

### S3 — Wire Provider Config into SessionConfig

**Commit message:** `feat(executor): pass provider config through session configuration`

**File:** `pkg/executor/sdk.go` (modify existing)

Add provider config to `SessionConfig` so the executor has access:

```go
// Addition to existing SessionConfig:
type SessionConfig struct {
    SystemPrompt string
    Models       []string
    Tools        []string
    MCPServers   map[string]interface{}
    ExtraDirs    []string
    Interactive  bool
    OnUserInput  UserInputHandler

    // Provider holds BYOK configuration when using the SDK executor.
    // Nil means "use GitHub Models" (default). The CopilotCLIExecutor
    // ignores this field.
}
```

This field is set by `main.go` when constructing the `StepExecutor` and is only read by `CopilotSDKExecutor`. The `CopilotCLIExecutor` ignores it.

**Impact:** The `SessionConfig` struct is used by tests via `MockSessionExecutor`. Adding a field is backward-compatible — existing tests don't set it, so it defaults to `nil`.

---

### S4 — SDK Default + `--cli` Fallback in main.go

**Commit message:** `feat(cli): make SDK executor the default, add --cli fallback flag`

**File:** `cmd/workflow-runner/main.go` (modify existing)

#### New CLI Flag

Add a `--cli` flag to the `run` subcommand:

```go
useCLI := fs.Bool("cli", false, "Use CLI subprocess executor instead of SDK (fallback)")
```

#### Updated Usage String

```go
const usage = `Usage: goflow run [options]

       goflow version

Options:
  --workflow      Path to workflow YAML file (required)
  --inputs        Key=value input pairs (repeatable)
  --audit-dir     Override audit directory (default from workflow config)
  --mock          Use mock executor instead of real backend
  --cli           Use CLI subprocess executor instead of SDK (fallback)
  --interactive   Allow agents to ask for user input during execution
  --verbose       Enable verbose logging
`
```

#### Executor Selection Block

Change the executor selection block (currently lines ~221-227):

```go
// BEFORE (current):
var sessionExecutor executor.SessionExecutor
if *useMock {
    fmt.Fprintln(stderr, "NOTE: Using mock executor.")
    sessionExecutor = &executor.MockSessionExecutor{DefaultResponse: "mock output"}
} else {
    sessionExecutor = &executor.CopilotCLIExecutor{}
}

// AFTER (new):
var sessionExecutor executor.SessionExecutor
var sdkExec *executor.CopilotSDKExecutor // tracked for Close()
if *useMock {
    fmt.Fprintln(stderr, "NOTE: Using mock executor.")
    sessionExecutor = &executor.MockSessionExecutor{DefaultResponse: "mock output"}
} else if *useCLI {
    // Explicit --cli flag: use legacy subprocess executor.
    sessionExecutor = &executor.CopilotCLIExecutor{}
    if *verbose {
        fmt.Fprintln(stderr, "Using CLI subprocess executor (--cli flag)")
    }
} else {
    // Default: SDK executor with optional BYOK provider.
    var provider *executor.ProviderConfig
    if wf.Config.Provider != nil {
        provider = &executor.ProviderConfig{
            Type:      wf.Config.Provider.Type,
            BaseURL:   wf.Config.Provider.BaseURL,
            APIKeyEnv: wf.Config.Provider.APIKeyEnv,
        }
    }
    var err error
    sdkExec, err = executor.NewCopilotSDKExecutor(provider)
    if err != nil {
        fmt.Fprintf(stderr, "error: %v\n", err)
        return 1
    }
    defer sdkExec.Close()
    sessionExecutor = sdkExec
    if *verbose {
        if provider != nil {
            fmt.Fprintf(stderr, "Using SDK executor with %s provider (BYOK)\n", provider.Type)
        } else {
            fmt.Fprintln(stderr, "Using SDK executor with GitHub Models (default)")
        }
    }
}
```

**Behavior:**
- `--mock` → MockSessionExecutor (unchanged, highest priority)
- `--cli` → CopilotCLIExecutor (legacy fallback, no BYOK)
- Default → CopilotSDKExecutor (new — all SDK benefits, BYOK if `config.provider` set)

**Priority:** `--mock` > `--cli` > SDK default.

---

### S5 — Unit Tests for SDK Executor

**Commit message:** `test(executor): unit tests for CopilotSDKExecutor`

**File:** `pkg/executor/copilot_sdk_test.go`

Tests should cover:

1. **Constructor validation:**
   - Missing API key env var (with BYOK provider) → error with clear message
   - Empty provider type (with BYOK provider) → error
   - Nil provider → executor created (GitHub Models mode)
   - Valid BYOK config → executor created

2. **CreateSession mapping:**
   - SessionConfig fields correctly mapped to SDK config
   - Tool restriction passed through
   - MCP servers passed through
   - ExtraDirs passed through
   - Empty models list → SDK default

3. **Interactive mode:**
   - Interactive flag triggers user input flow
   - Missing OnUserInput handler → error

4. **Model fallback:**
   - Primary model unavailable → tries next
   - All models unavailable → structured error

5. **Close/cleanup:**
   - Close() on executor releases client
   - Multiple Close() calls are safe

**Strategy:** The SDK client itself can be stubbed similar to MockSessionExecutor. Create a test helper that wraps the SDK client creation so tests can inject a mock SDK client without network calls.

```go
// For testability, allow injecting a pre-built client:
func NewCopilotSDKExecutorWithClient(client *copilot.Client) *CopilotSDKExecutor {
    return &CopilotSDKExecutor{client: client}
}
```

---

### S6 — Integration Test

**Commit message:** `test: integration test for BYOK provider selection`

**File:** `integration_test.go` (append to existing)

Test the full path: workflow YAML with `config.provider` → SDK executor selected → session created.

```go
func TestBYOKProviderSelection(t *testing.T) {
    // This test verifies the executor selection logic, not actual LLM calls.
    // Uses --mock to prevent real API calls while confirming the selection
    // path parses provider config correctly.

    yaml := `
name: byok-test
config:
  model: gpt-4o
  provider:
    type: openai
    base_url: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
agents:
  test-agent:
    inline:
      description: test
      prompt: you are a test agent
steps:
  - id: step1
    agent: test-agent
    prompt: hello
output:
  steps: [step1]
  format: markdown
`
    // Parse and verify provider config is preserved through the pipeline.
    wf, err := workflow.ParseWorkflowBytes([]byte(yaml))
    require.NoError(t, err)
    require.NotNil(t, wf.Config.Provider)
    assert.Equal(t, "openai", wf.Config.Provider.Type)
    assert.Equal(t, "OPENAI_API_KEY", wf.Config.Provider.APIKeyEnv)
}
```

---

### S7 — Documentation Update

**Commit message:** `docs: add BYOK provider configuration guide`

**File:** `docs/reference/model-selection.md` (update existing)

Update the existing note at line 193 ("ProviderConfig is parsed but not yet consumed") to document that it's now active.

Add a section covering:
- SDK executor is now the default backend
- BYOK is optional — without `config.provider`, SDK uses GitHub Models
- Supported provider types (`openai`, `anthropic`, `azure`, `ollama`)
- Environment variable setup for API keys
- Example YAML configurations for BYOK
- `--cli` flag for fallback to subprocess executor
- Note that all built-in tools remain available regardless of executor choice

---

## 6. File Inventory

| File | Action | Task |
|---|---|---|
| `go.mod` | Modify — add SDK dependency | S1 |
| `go.sum` | Auto-generated | S1 |
| `pkg/executor/copilot_sdk.go` | **Create** — SDK executor implementation | S2 |
| `pkg/executor/sdk.go` | Modify — add `Provider` field to SessionConfig | S3 |
| `cmd/workflow-runner/main.go` | Modify — executor selection logic | S4 |
| `pkg/executor/copilot_sdk_test.go` | **Create** — unit tests | S5 |
| `integration_test.go` | Modify — add BYOK test | S6 |
| `docs/reference/model-selection.md` | Modify — BYOK docs | S7 |

**Files NOT modified:**
- `pkg/executor/executor.go` — StepExecutor unchanged, it only uses `SessionExecutor` interface
- `pkg/executor/copilot_cli.go` — CLI executor stays exactly as-is
- `pkg/executor/mock_sdk.go` — mock unchanged
- `pkg/orchestrator/orchestrator.go` — orchestrator unchanged
- `pkg/workflow/types.go` — `ProviderConfig` already defined and parsed
- `pkg/workflow/parser.go` — already parses `config.provider` from YAML

---

## 7. YAML Schema (No Changes)

The workflow YAML schema already supports provider configuration. No parser changes needed.

```yaml
config:
  model: "gpt-4o"
  provider:                          # Already defined in types.go
    type: "openai"                   # "openai" | "anthropic" | "azure" | "ollama"
    base_url: "https://api.openai.com/v1"
    api_key_env: "OPENAI_API_KEY"    # Env var name (never the key itself)
```

The `ProviderConfig` struct in `pkg/workflow/types.go` (lines 51-55) is already parsed — it's just not consumed by any executor yet.

---

## 8. CLI Flag Changes

**One new flag:** `--cli` to opt into the legacy subprocess executor.

| Flag | Behavior |
|---|---|
| `--mock` | Uses mock executor (highest priority, overrides everything) |
| `--cli` | **New.** Uses legacy `CopilotCLIExecutor` subprocess executor. No BYOK, no streaming, no session resume. Useful as a fallback if the SDK has issues. |
| `--workflow` | Same — loads YAML, `config.provider` consumed by SDK executor if present |
| `--interactive` | Same — gates the user-input handler |
| `--verbose` | Same — now also prints which executor was selected |
| `--audit-dir` | Same — audit works identically |

**Without `--mock` or `--cli`**, the SDK executor is used by default. If `config.provider` is set in the workflow YAML, BYOK provider routing is enabled. If not, the SDK uses GitHub Models (same as the CLI executor did before, but with streaming/resume/efficiency benefits).

---

## 9. Testing Strategy

### Unit Tests (S5)

| Test | What It Verifies |
|---|---|
| `TestNewCopilotSDKExecutor_MissingAPIKey` | Constructor fails with clear error when BYOK env var not set |
| `TestNewCopilotSDKExecutor_EmptyProvider` | Constructor validates provider type when BYOK |
| `TestNewCopilotSDKExecutor_NilProvider` | Constructor succeeds with nil provider (GitHub Models) |
| `TestSDKSessionConfig_Mapping` | SessionConfig → SDK config field mapping |
| `TestSDKSession_ToolRestriction` | Agent tools list passed to SDK session |
| `TestSDKSession_Interactive` | Interactive prompt handling matches CLI behavior |
| `TestSDKSession_ModelFallback` | Falls through models on unavailability |
| `TestSDKExecutor_Close` | Client cleanup, double-close safety |

### Integration Tests (S6)

| Test | What It Verifies |
|---|---|
| `TestBYOKProviderSelection` | Provider config parsed and reaches SDK executor |
| `TestSDKExecutorDefault` | No provider, no `--cli` → SDK executor selected |
| `TestCLIFlagFallback` | `--cli` flag → CLI executor selected |
| `TestMockOverridesSDK` | `--mock` takes precedence over SDK default |

### Existing Test Compatibility

All existing tests use `MockSessionExecutor` and are unaffected. The new `Provider` field in `SessionConfig` defaults to `nil`, which is the existing behavior.

---

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| **SDK Go package API doesn't match assumptions** | S2 implementation may need adjustment | S1 explores SDK package API first; adapt struct/method mapping |
| **SDK requires specific CLI version** | Version mismatch breaks runtime | Document minimum CLI version; add version check in constructor |
| **Model fallback API differs from assumptions** | Fallback strategy may need redesign | Two fallback patterns documented (per-Send vs per-CreateSession); pick based on SDK API |
| **API key leaked in logs/audit** | Security vulnerability | Key resolved from env var at construction only; never stored in SessionConfig, audit, or logs |
| **SDK client not goroutine-safe** | Parallel step execution breaks | Verify in S1; if not safe, create one client per session (slightly less efficient) |
| **Interactive mode differs in SDK** | User input flow breaks | SDK executor reuses same interactive pattern as CLI (pre-ask, append to prompt) |

### Security Considerations

- **API keys:** Only the env var *name* is stored in YAML and structs. The actual key is read from `os.Getenv()` once during `NewCopilotSDKExecutor()` and passed directly to the SDK client. It is never logged, audited, or written to disk.
- **Provider URL validation:** The `base_url` is passed to the SDK as-is. The SDK handles TLS and connection validation. No URL construction or manipulation in goflow.
- **No credential persistence:** The `ProviderConfig` in `SessionConfig` contains only the env var name, not the resolved key.

---

## 11. Dependencies

### New External Dependency

```
github.com/github/copilot-sdk/go    — Copilot SDK Go library
```

This was already listed as a planned dependency in [SPEC.md](SPEC.md) (line 1680, P1T16) and [go.mod discussion in copilot-instructions.md](copilot-instructions.md) (line 34), but was deferred in favor of the CLI subprocess approach during Phase 1.

### Existing Dependencies (Unchanged)

```
gopkg.in/yaml.v3                    — YAML parsing (already in go.mod)
```

---

## 12. Out of Scope

The following are explicitly **not** part of this specification:

| Feature | Why Deferred |
|---|---|
| **Per-step provider override** | Step-level `provider:` config adds significant complexity; start with workflow-global provider |
| **Per-agent provider override** | Same — requires agent-to-provider routing logic |
| **Streaming output display** | SDK supports streaming events, but displaying them requires reporter changes |
| **Session resume (`goflow resume`)** | SDK supports `ResumeSession()`, but resume requires checkpoint persistence |
| **OTel tracing** | SDK supports it, but tracing requires telemetry infrastructure (Phase 5) |
| **Hooks (OnPreToolUse, etc.)** | SDK supports hooks, but wiring them requires audit/guardrail logic |
| **LangChain backend** | Alternative multi-provider approach; separate effort (Phase 5+) |
| **Local model support (Ollama)** | Technically works via BYOK with `type: ollama`, but untested — defer validation |
| **`copilot.Client` pooling/reuse** | Optimization for parallel execution; correctness first |

These features build naturally on top of the SDK executor once it's working. Each can be a follow-up spec.

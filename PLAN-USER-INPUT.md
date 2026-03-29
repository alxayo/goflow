# Plan: User Input & Clarification Support

**Goal:** Allow workflow steps to pause execution and ask the user for clarification, while preserving the default autonomous (headless) mode.

---

## Current State

The workflow runner operates in **fully autonomous mode**:

- `copilot_cli.go` passes `--no-ask-user` to every CLI invocation, suppressing the `ask_user` tool entirely.
- The `Session` interface has a simple `Send(ctx, prompt) (string, error)` — no mechanism for mid-execution interaction.
- The orchestrator treats each step as an atomic prompt→response operation.
- There is no YAML-level or CLI-level option to enable interactivity.

---

## Design

### Principle: Opt-In Interactivity

By default, workflows run autonomously (current behavior). Users opt in to clarification support at three levels, from broadest to most specific:

| Level | Controls | Default |
|-------|----------|---------|
| **CLI flag** `--interactive` | Global override: enables user input for the entire run | `false` |
| **Workflow config** `config.interactive` | Enables user input for all steps in this workflow | `false` |
| **Step field** `steps[].interactive` | Enables user input for a single step | `false` |

**Resolution order:** Step-level overrides workflow-level, which overrides CLI flag. If *any* of them is `true` for a given step, that step can ask the user for input.

### YAML Configuration

```yaml
config:
  interactive: true          # enable user input for all steps

steps:
  - id: analyze
    agent: security-reviewer
    prompt: "Review {{inputs.files}}"
    interactive: true         # enable for this step only
```

### CLI Flag

```bash
# Autonomous (default, current behavior)
workflow-runner run --workflow review.yaml

# Interactive — all steps can ask for clarification
workflow-runner run --workflow review.yaml --interactive
```

---

## Implementation Plan

### Layer 1: Data Model (`pkg/workflow/types.go`)

Add the interactive flag to both `Config` and `Step`:

```go
type Config struct {
    // ... existing fields ...
    Interactive bool `yaml:"interactive"`
}

type Step struct {
    // ... existing fields ...
    Interactive *bool `yaml:"interactive,omitempty"`
}
```

Using `*bool` for the step allows three states: unset (inherit from config/CLI), explicitly true, explicitly false. The config-level field uses plain `bool` (defaults to `false`).

Add a helper to resolve the effective interactive flag for a step:

```go
func IsInteractive(step Step, wfInteractive, cliInteractive bool) bool {
    if step.Interactive != nil {
        return *step.Interactive
    }
    return wfInteractive || cliInteractive
}
```

### Layer 2: Executor Interface (`pkg/executor/sdk.go`)

Add `Interactive` to `SessionConfig` so the executor knows whether to enable user input:

```go
type SessionConfig struct {
    // ... existing fields ...
    Interactive bool
}
```

Extend the `Session` interface to support a callback-driven user input mechanism. The key insight is that user input is inherently asynchronous from the SDK's perspective — the LLM invokes `ask_user`, we need to present the question to the user, wait for their answer, and return it to the SDK.

Define a callback type and add it to `SessionConfig`:

```go
// UserInputHandler is called when the LLM requests clarification.
// The handler should present the question to the user and return
// their response. Blocking until the user provides input is expected.
type UserInputHandler func(question string, choices []string) (answer string, err error)

type SessionConfig struct {
    // ... existing fields ...
    Interactive      bool
    OnUserInput      UserInputHandler
}
```

### Layer 3: Copilot CLI Executor (`pkg/executor/copilot_cli.go`)

This is the **critical layer** where the implementation diverges based on interactivity.

**Current approach** (non-interactive): Runs `copilot --no-ask-user -p <prompt>` as a one-shot command. This stays unchanged for non-interactive steps.

**New approach** (interactive): Must switch from one-shot CLI execution to **SDK session mode** so the `OnUserInputRequest` handler can be wired in. This means:

1. When `cfg.Interactive` is `true`, use the Copilot Go SDK (`github.com/github/copilot-sdk/go`) instead of shelling out to the CLI.
2. Set `OnUserInputRequest` on the `SessionConfig` to call through to the `UserInputHandler` provided by the orchestrator.
3. The SDK manages the CLI process lifecycle and JSON-RPC communication automatically.

```go
func (e *CopilotCLIExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
    if cfg.Interactive && cfg.OnUserInput != nil {
        return e.createSDKSession(ctx, cfg)
    }
    return e.createCLISession(ctx, cfg)   // existing path
}

func (e *CopilotCLIExecutor) createSDKSession(ctx context.Context, cfg SessionConfig) (Session, error) {
    client := copilot.NewClient(&copilot.ClientOptions{
        LogLevel: "error",
    })
    if err := client.Start(ctx); err != nil {
        return nil, fmt.Errorf("starting copilot SDK client: %w", err)
    }

    session, err := client.CreateSession(ctx, &copilot.SessionConfig{
        Model:               cfg.Models[0],
        OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
        OnUserInputRequest: func(req copilot.UserInputRequest, inv copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
            answer, err := cfg.OnUserInput(req.Question, req.Choices)
            if err != nil {
                return copilot.UserInputResponse{}, err
            }
            return copilot.UserInputResponse{
                Answer:      answer,
                WasFreeform: true,
            }, nil
        },
    })
    if err != nil {
        client.Stop()
        return nil, err
    }

    return &SDKSession{
        client:  client,
        session: session,
    }, nil
}
```

### Layer 4: Mock Executor (`pkg/executor/mock_sdk.go`)

Extend `MockSessionExecutor` and `MockSession` to support interactive testing:

```go
type MockSessionExecutor struct {
    // ... existing fields ...

    // SimulatedQuestions simulates the LLM asking for user input.
    // Maps prompt substrings to the question the "LLM" will ask.
    SimulatedQuestions map[string]string
}

type MockSession struct {
    // ... existing fields ...
    onUserInput      UserInputHandler
    simulatedQuestion string
}

func (ms *MockSession) Send(ctx context.Context, prompt string) (string, error) {
    // If this session has a simulated question, call the user input handler
    if ms.simulatedQuestion != "" && ms.onUserInput != nil {
        answer, err := ms.onUserInput(ms.simulatedQuestion, nil)
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("User answered: %s", answer), nil
    }
    // ... existing mock logic ...
}
```

### Layer 5: Step Executor (`pkg/executor/executor.go`)

Pass the interactive flag and the user-input handler into `SessionConfig`:

```go
func (se *StepExecutor) Execute(
    ctx context.Context,
    step workflow.Step,
    agent *agents.Agent,
    results map[string]string,
    inputs map[string]string,
    seqNum int,
) (*workflow.StepResult, error) {
    // ... existing logic ...

    sessionCfg := SessionConfig{
        SystemPrompt: agent.Prompt,
        Tools:        agent.Tools,
        ExtraDirs:    step.ExtraDirs,
        Models:       se.resolveModels(step, agent),
        Interactive:  se.Interactive,   // NEW
        OnUserInput:  se.OnUserInput,   // NEW
    }
    // ... rest unchanged ...
}
```

Add the new fields to `StepExecutor`:

```go
type StepExecutor struct {
    // ... existing fields ...
    Interactive bool
    OnUserInput UserInputHandler
}
```

### Layer 6: Orchestrator (`pkg/orchestrator/orchestrator.go`)

Thread the interactive flag through. The orchestrator itself doesn't need to change much — it just passes the resolved interactive state to the executor. The user-input handler is set once at orchestrator construction.

```go
type Orchestrator struct {
    // ... existing fields ...
    Interactive bool
    OnUserInput executor.UserInputHandler
}
```

In `Run()` and `RunParallel()`, resolve per-step interactivity:

```go
// Before executing a step, resolve its interactive flag.
stepInteractive := workflow.IsInteractive(step, wf.Config.Interactive, o.Interactive)
// Temporarily set on the executor for this step.
o.Executor.Interactive = stepInteractive
```

**Note on parallel execution:** When multiple steps run concurrently and more than one is interactive, user input prompts would interleave. Two options:

1. **Serialize interactive steps** — If a step is interactive, run it outside the parallel group. This is simpler and avoids confusing the user.
2. **Label prompts** — Prefix clarification questions with the step ID so the user knows which agent is asking. Allow concurrent prompts.

**Recommendation:** Option 1 for the initial implementation. Emit a warning when interactive steps appear in a parallel level.

### Layer 7: CLI (`cmd/workflow-runner/main.go`)

Add the `--interactive` flag and wire the terminal-based user input handler:

```go
interactive := fs.Bool("interactive", false, "Allow agents to ask for user input during execution")

// Build the terminal-based input handler.
var userInputHandler executor.UserInputHandler
if *interactive || wf.Config.Interactive {
    userInputHandler = terminalInputHandler
}

stepExec := &executor.StepExecutor{
    SDK:          sessionExecutor,
    AuditLogger:  auditLogger,
    Truncate:     wf.Output.Truncate,
    DefaultModel: wf.Config.Model,
    Interactive:  *interactive,
    OnUserInput:  userInputHandler,
}

orch := &orchestrator.Orchestrator{
    Executor:       stepExec,
    Agents:         resolvedAgents,
    Inputs:         mergedInputs,
    MaxConcurrency: wf.Config.MaxConcurrency,
    Interactive:    *interactive,
    OnUserInput:    userInputHandler,
}
```

Terminal input handler (in `main.go` or a new `pkg/input/terminal.go`):

```go
func terminalInputHandler(question string, choices []string) (string, error) {
    fmt.Fprintf(os.Stderr, "\n--- Agent needs clarification ---\n")
    fmt.Fprintf(os.Stderr, "%s\n", question)
    if len(choices) > 0 {
        for i, c := range choices {
            fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, c)
        }
        fmt.Fprintf(os.Stderr, "Enter choice number or type your answer: ")
    } else {
        fmt.Fprintf(os.Stderr, "> ")
    }

    reader := bufio.NewReader(os.Stdin)
    answer, err := reader.ReadString('\n')
    if err != nil {
        return "", fmt.Errorf("reading user input: %w", err)
    }
    return strings.TrimSpace(answer), nil
}
```

### Layer 8: Audit Trail

Log user input events in the step audit:

```go
// In audit/logger.go — add a new method
func (sl *StepLogger) WriteUserInput(question, answer string) error {
    entry := map[string]string{
        "type":     "user_input",
        "question": question,
        "answer":   answer,
        "time":     time.Now().UTC().Format(time.RFC3339),
    }
    return sl.appendTranscript(entry)
}
```

Update `step.meta.json` to include an `interactive` field and count of user inputs:

```json
{
  "step_id": "analyze",
  "interactive": true,
  "user_inputs": 2
}
```

---

## Execution Flow (Interactive Step)

```
User runs: workflow-runner run --workflow review.yaml --interactive

  CLI parses --interactive flag
    ↓
  Orchestrator resolves step.interactive = true for this step
    ↓
  StepExecutor.Execute() creates SessionConfig{Interactive: true, OnUserInput: handler}
    ↓
  CopilotCLIExecutor sees Interactive=true → uses SDK session mode (not one-shot CLI)
    ↓
  SDK session configured with OnUserInputRequest handler
    ↓
  session.Send(prompt) → LLM processes prompt
    ↓
  LLM invokes ask_user tool → SDK calls OnUserInputRequest
    ↓
  Handler calls UserInputHandler → prints question to stderr, reads from stdin
    ↓
  User types answer → returned to SDK → LLM continues with answer
    ↓
  LLM reaches session.idle → output returned to executor
    ↓
  Output stored in results map, DAG continues
```

---

## Dependency

This feature requires adding the Copilot Go SDK as a dependency for SDK-session-mode execution:

```bash
go get github.com/github/copilot-sdk/go
```

The existing one-shot CLI execution path (`--no-ask-user`) remains unchanged for non-interactive steps, so the SDK dependency is only exercised when interactive mode is active.

---

## Edge Cases & Considerations

| Case | Handling |
|------|----------|
| **Interactive step in parallel level** | Warn the user; serialize interactive steps within the level (run them after non-interactive ones complete). |
| **User sends EOF / Ctrl+C during input** | Return error from handler → step fails with "user aborted input" → workflow fails per `on_error` policy. |
| **LLM doesn't ask questions** | No-op; the handler is registered but never called. Step completes normally. |
| **Multiple questions in one step** | Handler is called each time the LLM invokes `ask_user`. Each interaction is logged in the audit trail. |
| **Mock executor in tests** | `MockSession` simulates questions via `SimulatedQuestions` map. Tests can verify the handler is called. |
| **`--mock` + `--interactive`** | Works: mock executor can simulate user input requests for end-to-end testing of the interactive flow. |
| **Agent prompt doesn't mention ask_user** | The LLM may still decide to use it if the tool is available. The `interactive` flag controls *availability*, not *forcing* the LLM to ask. |
| **Step timeout during user input** | Context cancellation propagates through the handler. The `bufio.Reader` will return an error when the context is cancelled. |

---

## Implementation Order

1. **`pkg/workflow/types.go`** — Add `Interactive` to `Config` and `Step`; add `IsInteractive()` helper.
2. **`pkg/executor/sdk.go`** — Add `UserInputHandler`, `Interactive`, `OnUserInput` to `SessionConfig`.
3. **`pkg/executor/copilot_cli.go`** — Add SDK-session-mode path for interactive steps.
4. **`pkg/executor/mock_sdk.go`** — Add `SimulatedQuestions` support.
5. **`pkg/executor/executor.go`** — Thread `Interactive` and `OnUserInput` through `StepExecutor`.
6. **`pkg/orchestrator/orchestrator.go`** — Thread interactive state, handle parallel+interactive warning.
7. **`pkg/audit/logger.go`** — Add `WriteUserInput()` method, extend `StepMeta`.
8. **`cmd/workflow-runner/main.go`** — Add `--interactive` flag, wire terminal handler.
9. **Tests** — Update existing tests, add new tests for interactive paths.
10. **Docs** — Update DOCS.md, YAML examples.

---

## Open Questions

1. **SDK vs CLI for interactive mode:** The plan above uses the Go SDK for interactive steps. An alternative is to run the CLI *without* `--no-ask-user` and pipe stdin/stdout. This avoids the SDK dependency but makes parsing user-input prompts from CLI output fragile. **Recommendation: Use the SDK.**

2. **Per-agent interactive setting:** Should the `.agent.md` file also support an `interactive` field? This would allow an agent to declare itself as "always needs clarification" regardless of the step config. **Recommendation: Defer to a follow-up; step-level and workflow-level is sufficient for now.**

3. **Non-terminal environments:** If workflow-runner is invoked from CI or a script, interactive mode should be auto-detected and disabled (check `os.Stdin` is a terminal via `isatty`). **Recommendation: Add `isatty` check and warn if `--interactive` is used without a terminal.**

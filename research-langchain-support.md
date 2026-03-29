# Research: LangChain Support for goflow

## 1. Problem Statement

The goflow currently depends on the Copilot CLI as its sole execution backend. This creates three constraints:

1. **No offline/on-prem support** — Copilot CLI requires GitHub authentication and internet access.
2. **Single provider** — All steps run through GitHub Models; no option to use Anthropic, local Ollama, vLLM, or other providers directly.
3. **No per-step endpoint routing** — While the model can vary per step, the provider/endpoint cannot.

Adding LangChain (`langchaingo`) as an alternative executor backend would remove all three constraints while preserving the existing architecture.

---

## 2. Copilot CLI vs LangChain Comparison

| Dimension | Copilot CLI (current) | LangChain (`langchaingo`) |
|---|---|---|
| **LLM providers** | GitHub Models only (GPT-4o, Claude, etc. via GitHub) | Any: OpenAI, Anthropic, Ollama, vLLM, llama.cpp, HuggingFace, Azure OpenAI, custom HTTP endpoints |
| **Offline / on-prem** | No — requires GitHub auth + internet | Yes — Ollama, llama.cpp, vLLM, any OpenAI-compatible local server |
| **Tool calling** | Built-in tool discovery (grep, semantic_search, etc. from VS Code/CLI) | You define tools explicitly; no IDE integration |
| **Agent files** | Native `.agent.md` support with frontmatter | No equivalent — agent config loading is custom |
| **Auth** | GitHub token (automatic with Copilot subscription) | Per-provider API keys, or none for local models |
| **Language** | CLI subprocess (any language can shell out) | Native Go library (`github.com/tmc/langchaingo`) |
| **Streaming** | Limited (CLI stdout) | Native streaming support |
| **Cost** | Included with Copilot subscription | Pay-per-token per provider, or free for local models |
| **MCP servers** | Native (per-agent `mcp-servers:` in `.agent.md`) | Not built-in — requires `mcp-go` + adapter glue code |
| **SKILL files** | Native (auto-discovered, `applyTo` pattern matching) | No concept — must implement as prompt injection |
| **Hooks** | `onPreToolUse` / `onPostToolUse` strings | `callbacks.Handler` interface — richer lifecycle events |

### What you gain

- **Ollama/llama.cpp** — fully offline, no API keys, runs on local hardware.
- **Multi-provider** — use Claude for reasoning steps, GPT-4o for coding, a local model for classification.
- **Per-step endpoint routing** — each step can target a different provider + base URL + model.
- **Cost control** — route cheap steps to local models, expensive steps to cloud.
- **No GitHub dependency** — works in air-gapped environments.

### What you lose

- **Built-in VS Code tools** — Copilot CLI provides `grep`, `semantic_search`, `read_file`, etc. out of the box. With LangChain, these must be implemented manually.
- **Agent file compatibility** — `.agent.md` files with `tools:` lists won't auto-wire to CLI tool discovery.
- **Zero-config auth** — Copilot "just works" with a subscription; LangChain needs explicit provider setup.
- **Native MCP server support** — Copilot CLI wires MCP servers declared in `.agent.md` automatically; LangChain requires manual bridging via `mcp-go`.
- **SKILL file discovery** — Copilot CLI auto-discovers and injects SKILL content; LangChain has no concept of SKILL files.

---

## 3. Per-Step Provider & Endpoint Routing

LangChain creates a **new LLM client per call** — there is no shared session. Each step can use a different provider, base URL, and model.

### 3.1 Resolution Priority

The existing three-level resolution pattern extends naturally:

```
Step.Provider  →  Agent.Provider  →  Config.Provider
Step.Model     →  Agent.Model     →  Config.Model
```

### 3.2 YAML Example

```yaml
config:
  # Workflow-wide defaults
  provider:
    type: "ollama"
    base_url: "http://localhost:11434"
  model: "llama3.3"

agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"
  fast-classifier:
    inline:
      description: "Quick triage classifier"
      prompt: "Classify the input..."
      model: "llama3.2:1b"
      provider:                          # Agent-level override
        type: "ollama"
        base_url: "http://gpu-box:11434"

steps:
  - id: classify
    agent: fast-classifier
    prompt: "Triage this: {{inputs.code}}"
    # Uses agent's provider (gpu-box ollama + llama3.2:1b)

  - id: deep-review
    agent: security-reviewer
    prompt: "Deep review: {{steps.classify.output}}"
    model: "claude-sonnet-4-20250514"
    provider:                            # Step-level override
      type: "anthropic"
      api_key_env: "ANTHROPIC_API_KEY"

  - id: summarize
    agent: aggregator
    prompt: "Summarize: {{steps.deep-review.output}}"
    # Falls back to workflow config (local ollama + llama3.3)
```

### 3.3 Supported Providers in `langchaingo`

| Provider | Package | Offline? | Per-step endpoint? |
|---|---|---|---|
| **Ollama** | `llms/ollama` | Yes | Yes — `WithServerURL()` per instance |
| **OpenAI** | `llms/openai` | No | Yes — `WithBaseURL()` for any OpenAI-compatible API |
| **Anthropic** | `llms/anthropic` | No | Yes — separate client per call |
| **HuggingFace** | `llms/huggingface` | Partial | Yes |
| **Local (llama.cpp)** | `llms/openai` + local server | Yes | Yes — point `WithBaseURL()` at localhost |
| **vLLM** | `llms/openai` (compatible API) | Yes | Yes |
| **Azure OpenAI** | `llms/openai` | No | Yes — `WithBaseURL()` + Azure token |
| **Google AI (Gemini)** | `llms/googleai` | No | Yes |
| **AWS Bedrock** | `llms/bedrock` | No | Yes |
| **Cohere** | `llms/cohere` | No | Yes |
| **Mistral AI** | `llms/mistral` | No | Yes |

---

## 4. Concurrency with Local Models (Hardware Constraints)

### 4.1 Problem

Local models on a single GPU cannot serve multiple inference requests in true parallel. When multiple goroutines hit a local model server simultaneously, the server queues them internally.

### 4.2 Behavior by Scenario

| Scenario | App crash? | Server behavior | Actual parallelism? | Recommended setting |
|---|---|---|---|---|
| Parallel + Ollama (single GPU) | No | Ollama queues internally | No — serialized | `max_concurrency: 1` |
| Parallel + vLLM (single GPU) | No | vLLM queues/batches | Minimal | `max_concurrency: 1` |
| Parallel + Ollama (multi-GPU) | No | Distributes across GPUs | Yes | `max_concurrency: <GPU count>` |
| Sequential mode (`Run()`) | No | N/A | N/A | Works out of the box |

### 4.3 Existing Architecture Support

The orchestrator already handles this via `max_concurrency` in `Config` and the `Semaphore` in `pkg/orchestrator/results.go`:

```yaml
config:
  max_concurrency: 1    # Forces sequential execution within parallel levels
  provider:
    type: "ollama"
    base_url: "http://localhost:11434"
```

With `max_concurrency: 1`, goroutines are spawned but the semaphore serializes them — only one executes at a time. DAG dependency order is preserved. This applies equally to both backends.

### 4.4 Recommendation

- **Single GPU**: Set `max_concurrency: 1`. Avoids unnecessary goroutine overhead and server-side queueing.
- **Multi-GPU**: Set `max_concurrency` to GPU count.
- **Cloud providers**: Leave at `0` (unlimited) — cloud APIs handle concurrency natively.

---

## 5. Tool System Comparison

### 5.1 LangChain Tool Interface

```go
// github.com/tmc/langchaingo/tools
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}
```

The LLM sees `Name` + `Description`, decides to call a tool, and LangChain routes the `input` string to the `Call()` implementation.

### 5.2 Built-in Tools Comparison

| Copilot CLI built-in tool | LangChain equivalent | Notes |
|---|---|---|
| `semantic_search` | **None** | Must build: embed codebase + vector store |
| `grep` / `glob` | **None** | Must build: `os.ReadDir`, `filepath.Glob` |
| `view` / `read_file` | **None** | Must build: `os.ReadFile` |
| `replace_string_in_file` | **None** | Must build: file edit logic |
| `run_in_terminal` | **None** | Must build: `os/exec` |
| `fetch_webpage` | `scraper` | Exists — basic HTML scraping |
| Web search | `serpapi`, `duckduckgo`, `perplexity` | Exist — web search |
| Calculator | `calculator` | Exists — math evaluation |
| SQL queries | `sqldatabase` | Exists — query databases |
| Wikipedia | `wikipedia` | Exists — lookup articles |
| Zapier actions | `zapier` | Exists — trigger Zapier NLAs |

### 5.3 Gap Analysis

LangChain ships **zero filesystem/code tools**. The Copilot CLI's core value for code workflows — `semantic_search`, `grep`, `glob`, `view`, `replace_string_in_file`, `run_in_terminal` — has no LangChain equivalent. These are VS Code/Copilot-specific tools backed by IDE infrastructure.

### 5.4 Custom Tool Implementations Required

**Simple tools (~20-50 lines each):**

```go
// read_file tool
type ReadFileTool struct{}
func (t ReadFileTool) Name() string        { return "read_file" }
func (t ReadFileTool) Description() string { return "Read the contents of a file at the given path" }
func (t ReadFileTool) Call(ctx context.Context, input string) (string, error) {
    data, err := os.ReadFile(filepath.Clean(input))
    if err != nil { return "", err }
    return string(data), nil
}
```

| Tool | Complexity | Approach |
|---|---|---|
| `read_file` / `view` | ~20 lines | `os.ReadFile` |
| `grep` | ~30 lines | `exec.Command("grep", ...)` or `bufio.Scanner` |
| `glob` | ~15 lines | `filepath.Glob` |
| `replace_string_in_file` | ~40 lines | Read, `strings.Replace`, write |
| `run_in_terminal` | ~40 lines | `exec.CommandContext` with timeout |
| `list_dir` | ~15 lines | `os.ReadDir` |

**Complex tool:**

| Tool | Complexity | Approach |
|---|---|---|
| `semantic_search` | ~200 lines + indexing step | Embedding model (e.g., `nomic-embed-text` via Ollama) + vector store (Chroma or in-memory). Requires a pre-workflow indexing pass. `langchaingo` provides `embeddings` + `vectorstores/chroma` packages. |

### 5.5 LangChain Function Calling (Native)

For models with native function calling support (GPT-4o, Claude, Llama 3.3+):

```go
content, err := llm.GenerateContent(ctx, messages, llms.WithTools([]llms.Tool{
    {
        Type: "function",
        Function: &llms.FunctionDefinition{
            Name:        "read_file",
            Description: "Read file contents at the given path",
            Parameters:  map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "path": map[string]any{"type": "string", "description": "File path to read"},
                },
                "required": []string{"path"},
            },
        },
    },
}))
// Handle tool_calls in response → execute → return result → LLM continues
```

The LLM decides when to call tools. The executor runs a loop: send prompt → LLM requests tool call → execute tool → return result → LLM continues until done.

---

## 6. VS Code Feature Compatibility: MCP, SKILLs, and Hooks

### 6.1 MCP Servers

**LangChain Go does NOT have built-in MCP support.** The `langchaingo` issue tracker confirms this:

- [Issue #1209](https://github.com/tmc/langchaingo/issues/1209) ("Is there a plan to support the mcp tool?") — **open since April 2025**, 16 👍, no official timeline.
- [Issue #1281](https://github.com/tmc/langchaingo/issues/1281) ("How to pass MCP server schema's to LLM") — **open**, users working around it manually.

**Community workaround:** A third-party adapter ([langchaingo-mcp-adapter](https://github.com/i2y/langchaingo-mcp-adapter)) bridges `mcp-go` (a standalone MCP client library) to LangChain's `tools.Tool` interface. It inlines MCP tool JSON schemas into tool descriptions. Users in the issue thread describe it as functional but "a kludge".

**Impact for goflow:** If workflows rely on MCP servers (e.g., Playwright MCP for Yahoo News scraping), switching to LangChain means:
1. Adding `mcp-go` as a dependency to act as the MCP client.
2. Writing an adapter (~100 lines) that converts MCP tool schemas to `llms.Tool` / `tools.Tool` for LangChain's function calling.
3. Managing MCP server process lifecycle (start/stop) ourselves — Copilot CLI handles this automatically.

```go
// Conceptual MCP-to-LangChain bridge
func MCPToolsToLangChain(mcpClient *mcp.Client) ([]llms.Tool, map[string]MCPToolExecutor) {
    mcpTools, _ := mcpClient.ListTools(ctx)
    var lcTools []llms.Tool
    executors := make(map[string]MCPToolExecutor)
    for _, t := range mcpTools {
        lcTools = append(lcTools, llms.Tool{
            Type: "function",
            Function: &llms.FunctionDefinition{
                Name:        t.Name,
                Description: t.Description,
                Parameters:  t.InputSchema, // JSON schema passthrough
            },
        })
        executors[t.Name] = MCPToolExecutor{client: mcpClient, toolName: t.Name}
    }
    return lcTools, executors
}
```

### 6.2 SKILL Files

**LangChain has no concept of SKILL files.** SKILL files (e.g., `skills/multi-news-scanner/SKILL.md`) are a VS Code / Copilot-specific convention — structured markdown with YAML frontmatter (`applyTo` patterns) that injects domain knowledge into the agent's context window.

LangChain has no equivalent mechanism for:
- Discovering SKILL files from disk by glob pattern.
- Matching SKILL `applyTo` patterns to runtime context.
- Injecting SKILL content into prompts automatically.

**However, SKILL files are just structured prompt injection.** The content gets prepended/appended to the system prompt before sending to the LLM. This is trivially implementable:

```go
// In the LangChain executor, before sending the prompt:
func (s *LangChainSession) buildSystemPrompt(agentPrompt string, skills []string) string {
    var sb strings.Builder
    sb.WriteString(agentPrompt)
    for _, skillPath := range skills {
        content, err := os.ReadFile(skillPath)
        if err == nil {
            sb.WriteString("\n\n--- SKILL: " + filepath.Base(skillPath) + " ---\n")
            sb.Write(content)
        }
    }
    return sb.String()
}
```

**No LangChain-specific support is needed** — skill injection happens in the goflow executor layer, before the prompt reaches LangChain. The existing `pkg/agents/` loader already parses SKILL references; they just need to be concatenated into the system prompt.

### 6.3 Hooks (`onPreToolUse` / `onPostToolUse`)

**LangChain's `callbacks.Handler` interface is MORE capable than VS Code hooks.**

The current goflow `HooksConfig` (in `pkg/agents/types.go`) supports:

```go
type HooksConfig struct {
    OnPreToolUse  string `yaml:"onPreToolUse"`
    OnPostToolUse string `yaml:"onPostToolUse"`
}
```

LangChain's `callbacks.Handler` interface provides a full lifecycle event system:

| VS Code Hook | LangChain Callback | Match? |
|---|---|---|
| `onPreToolUse` | `HandleToolStart(ctx, input)` | **Direct equivalent** — fires before tool execution |
| `onPostToolUse` | `HandleToolEnd(ctx, output)` | **Direct equivalent** — fires after tool execution |
| _(none)_ | `HandleToolError(ctx, err)` | **Extra** — fires on tool failure |
| _(none)_ | `HandleLLMStart(ctx, prompts)` | **Extra** — fires before each LLM call |
| _(none)_ | `HandleLLMGenerateContentEnd(ctx, res)` | **Extra** — fires after LLM response |
| _(none)_ | `HandleAgentAction(ctx, action)` | **Extra** — fires when agent decides to act |
| _(none)_ | `HandleAgentFinish(ctx, finish)` | **Extra** — fires when agent completes |
| _(none)_ | `HandleStreamingFunc(ctx, chunk)` | **Extra** — fires per streaming chunk |
| _(none)_ | `HandleChainStart/End/Error(ctx, ...)` | **Extra** — chain lifecycle events |
| _(none)_ | `HandleRetrieverStart/End(ctx, ...)` | **Extra** — retrieval lifecycle events |

Key LangChain callback features:
- **`CombiningHandler`** — stack multiple handlers (e.g., audit logging + metrics + custom hooks).
- **`SimpleHandler`** — embed and override only the methods you need (no-op defaults for the rest).
- **`LogHandler`** — built-in handler that prints all events to stdout.
- **Per-component attachment** — attach handlers to specific tools, chains, or agents rather than globally.

**Mapping `.agent.md` hooks to LangChain callbacks (~50 lines):**

```go
// Bridge VS Code hook config to LangChain callbacks
type AgentHooksHandler struct {
    callbacks.SimpleHandler  // No-op defaults for events we don't handle
    preToolScript  string
    postToolScript string
}

func (h AgentHooksHandler) HandleToolStart(ctx context.Context, input string) {
    if h.preToolScript != "" {
        exec.CommandContext(ctx, "sh", "-c", h.preToolScript).Run()
    }
}

func (h AgentHooksHandler) HandleToolEnd(ctx context.Context, output string) {
    if h.postToolScript != "" {
        exec.CommandContext(ctx, "sh", "-c", h.postToolScript).Run()
    }
}
```

### 6.4 Feature Compatibility Summary

| Feature | Copilot CLI | LangChain Go | Gap | Effort |
|---|---|---|---|---|
| **MCP servers** | Native (per-agent config) | Not built-in | `mcp-go` + adapter bridge | ~150 lines + `mcp-go` dep |
| **SKILL files** | Native (auto-discovery) | No concept | Prompt concatenation in executor | ~30 lines |
| **Hooks (pre/post tool)** | `onPreToolUse` / `onPostToolUse` | `callbacks.Handler` (richer) | Map hook strings to callback methods | ~50 lines |
| **Agent descriptions** (`.agent.md`) | Native format | No concept | Already handled by `pkg/agents/loader.go` | 0 lines |
| **Handoffs** | Parsed in frontmatter | No concept | Not used in DAG execution (metadata only) | 0 lines |

---

## 7. Architecture: How LangChain Fits

### 7.1 Key Insight: `SessionExecutor` Interface

The existing `SessionExecutor` interface in `pkg/executor/sdk.go` is the integration point:

```go
type SessionExecutor interface {
    CreateSession(ctx context.Context, cfg SessionConfig) (Session, error)
}

type Session interface {
    Send(ctx context.Context, prompt string) (string, error)
    SessionID() string
    Close() error
}
```

`CopilotCLIExecutor` is one implementation. A `LangChainExecutor` would be another — same interface, different backend. The orchestrator, DAG, templates, conditions, audit — none of it changes.

### 7.2 What Changes

```
pkg/executor/
  ├── sdk.go              # SessionExecutor interface (unchanged)
  ├── copilot_cli.go      # Existing Copilot CLI backend (unchanged)
  ├── langchain.go         # NEW: LangChain executor implementation
  ├── langchain_tools.go   # NEW: Custom tool implementations
  ├── langchain_mcp.go     # NEW: MCP-to-LangChain tool bridge
  └── langchain_hooks.go   # NEW: VS Code hooks → LangChain callbacks mapper
```

### 7.3 What Does NOT Change

- `pkg/orchestrator/` — no knowledge of backend
- `pkg/workflow/` — DAG, templates, conditions, parser
- `pkg/audit/` — records whichever model/provider was used
- `pkg/agents/` — agent loading and discovery
- `pkg/memory/` — shared memory between steps
- `pkg/reporter/` — output formatting

---

## 8. Implementation Plan

### Phase L1: Core LangChain Executor (Foundation)

**Goal:** Replace Copilot CLI subprocess calls with native LangChain LLM calls.

| Task | Description | Scope |
|---|---|---|
| L1.1 | Add `langchaingo` dependency to `go.mod` | Trivial |
| L1.2 | Add `Provider` field to `SessionConfig` | ~5 lines in `sdk.go` |
| L1.3 | Add `provider` field to `Step` type (step-level override) | ~5 lines in `types.go` |
| L1.4 | Implement `resolveProvider()` in executor (step → agent → config fallback) | ~30 lines, mirrors `resolveModels()` |
| L1.5 | Implement `LangChainExecutor` struct + `CreateSession()` | ~100 lines in `langchain.go` |
| L1.6 | Implement `LangChainSession.Send()` — single prompt, no tools | ~60 lines |
| L1.7 | Provider factory: create LLM client by type (ollama, openai, anthropic) | ~80 lines, switch on `provider.Type` |
| L1.8 | Add `executor` field to `Config` YAML (`"copilot"` or `"langchain"`) | ~5 lines in `types.go`, ~10 in `parser.go` |
| L1.9 | Wire executor selection in `cmd/workflow-runner/main.go` | ~20 lines |
| L1.10 | Unit tests with mock LLM (no real API calls) | ~100 lines |
| L1.11 | Integration test with Ollama (skipped if Ollama not running) | ~80 lines |

**Estimated new code:** ~500 lines

### Phase L2: Tool Support

**Goal:** Give LangChain-backed agents access to filesystem and code tools.

| Task | Description | Scope |
|---|---|---|
| L2.1 | Define `ToolRegistry` — maps tool names to `tools.Tool` implementations | ~40 lines |
| L2.2 | Implement `read_file` tool | ~25 lines |
| L2.3 | Implement `grep` tool | ~35 lines |
| L2.4 | Implement `glob` tool | ~20 lines |
| L2.5 | Implement `list_dir` tool | ~20 lines |
| L2.6 | Implement `replace_string_in_file` tool | ~45 lines |
| L2.7 | Implement `run_in_terminal` tool (with timeout + sandboxing) | ~50 lines |
| L2.8 | Implement `fetch_webpage` tool (or wrap langchain `scraper`) | ~30 lines |
| L2.9 | Add tool-call loop to `LangChainSession.Send()` | ~80 lines |
| L2.10 | Map agent `.agent.md` `tools:` list to `ToolRegistry` lookups | ~30 lines |
| L2.11 | Audit tool calls to `tool_calls.jsonl` | ~40 lines |
| L2.12 | Tests for each tool + tool-call loop | ~200 lines |

**Estimated new code:** ~615 lines

### Phase L3: Semantic Search (Advanced)

**Goal:** Replicate Copilot CLI's `semantic_search` for codebase-aware queries.

| Task | Description | Scope |
|---|---|---|
| L3.1 | Codebase indexer: walk files, chunk, embed via Ollama `nomic-embed-text` | ~150 lines |
| L3.2 | In-memory vector store (or Chroma integration) | ~80 lines |
| L3.3 | `semantic_search` tool implementation (query → top-K results) | ~50 lines |
| L3.4 | CLI command: `goflow index` to pre-build embeddings | ~40 lines |
| L3.5 | Cache index to disk, rebuild on file change (hash-based) | ~80 lines |
| L3.6 | Tests | ~100 lines |

**Estimated new code:** ~500 lines

### Phase L4: Per-Step Provider Routing

**Goal:** Each step can target a different LLM endpoint.

| Task | Description | Scope |
|---|---|---|
| L4.1 | Add `provider` to `InlineAgent` type | ~3 lines |
| L4.2 | Add `provider` to `.agent.md` frontmatter parsing | ~15 lines |
| L4.3 | Implement `resolveProvider()` with step → agent → config fallback | ~30 lines |
| L4.4 | Create LLM client per-session (not per-executor) | ~20 lines refactor |
| L4.5 | Tests for provider resolution across all levels | ~60 lines |

**Estimated new code:** ~130 lines

### Phase L5: MCP, SKILLs, and Hooks Bridge

**Goal:** Preserve compatibility with VS Code agent features when running on the LangChain backend.

| Task | Description | Scope |
|---|---|---|
| L5.1 | Add `mcp-go` dependency to `go.mod` | Trivial |
| L5.2 | MCP server process manager: start/stop MCP server subprocesses per agent config | ~80 lines |
| L5.3 | MCP tool discovery: call `ListTools()` on connected MCP server | ~30 lines |
| L5.4 | MCP-to-LangChain adapter: convert MCP tool schemas to `llms.Tool` definitions | ~60 lines |
| L5.5 | MCP tool executor: route LLM tool calls back to MCP server via `CallTool()` | ~50 lines |
| L5.6 | MCP lifecycle integration: wire into `LangChainSession` create/close | ~30 lines |
| L5.7 | SKILL file injection: read SKILL markdown, concatenate into system prompt before LLM call | ~30 lines |
| L5.8 | SKILL discovery: resolve skill paths from step/agent config and `skills/` directories | ~40 lines |
| L5.9 | Hooks-to-callbacks mapper: convert `.agent.md` `onPreToolUse`/`onPostToolUse` to `callbacks.Handler` | ~50 lines |
| L5.10 | Audit integration: log MCP tool calls to `tool_calls.jsonl` alongside native tools | ~30 lines |
| L5.11 | Tests for MCP bridge, SKILL injection, and hooks mapping | ~150 lines |

**Estimated new code:** ~550 lines

### Phase Summary

| Phase | New code | Dependencies | Priority |
|---|---|---|---|
| **L1: Core executor** | ~500 lines | `langchaingo` | Must — foundation |
| **L2: Tool support** | ~615 lines | L1 | Must — agents need tools |
| **L3: Semantic search** | ~500 lines | L2, embedding model | Should — not all workflows need it |
| **L4: Per-step routing** | ~130 lines | L1 | Should — high value, low effort |
| **L5: MCP, SKILLs, Hooks** | ~550 lines | L1, L2, `mcp-go` | Should — needed for full `.agent.md` compat |

**Total estimated new code:** ~2,295 lines (excluding tests: ~1,745 lines)

---

## 9. YAML Configuration Reference

### Workflow-level (Config)

```yaml
config:
  executor: "langchain"              # "copilot" (default) or "langchain"
  model: "llama3.3"                  # Default model for all steps
  provider:                          # Default provider for all steps
    type: "ollama"                   # ollama | openai | anthropic | azure_openai
    base_url: "http://localhost:11434"
    api_key_env: ""                  # Env var name (empty for local models)
  max_concurrency: 1                 # Recommended for single-GPU local models
```

### Agent-level (inline or `.agent.md`)

```yaml
# In workflow YAML (inline agent)
agents:
  fast-classifier:
    inline:
      model: "llama3.2:1b"
      provider:
        type: "ollama"
        base_url: "http://gpu-box:11434"
```

```markdown
<!-- In .agent.md frontmatter (future) -->
---
name: security-reviewer
model: claude-sonnet-4-20250514
provider:
  type: anthropic
  api_key_env: ANTHROPIC_API_KEY
tools:
  - read_file
  - grep
  - glob
---
```

### Step-level

```yaml
steps:
  - id: deep-review
    agent: security-reviewer
    model: "gpt-4o"                  # Override agent's model
    provider:                        # Override agent's provider
      type: "openai"
      api_key_env: "OPENAI_API_KEY"
```

---

## 10. Risk Assessment

| Risk | Impact | Mitigation |
|---|---|---|
| `langchaingo` API instability | Medium | Pin version in `go.mod`; wrap behind `SessionExecutor` interface |
| Tool-call loop infinite loops | High | Max iterations cap (e.g., 20 tool calls per step) |
| Local model quality too low | Medium | Allow per-step model override to cloud for critical steps |
| Semantic search index stale | Low | Hash-based cache invalidation; `goflow index` CLI command |
| `run_in_terminal` tool security | High | Allowlist commands, sandbox directories, timeout enforcement |
| Context window overflow with tool results | Medium | Reuse existing `TruncateConfig` for tool output truncation |
| Provider API key leakage in audit logs | High | Never log API keys; only reference env var names |
| MCP support missing in `langchaingo` | Medium | Use `mcp-go` directly + custom adapter; monitor [issue #1209](https://github.com/tmc/langchaingo/issues/1209) for native support |
| MCP server process lifecycle | Medium | Explicit start/stop in session create/close; kill on timeout |
| MCP adapter schema mismatch | Low | Use `mcp-go`'s `MarshalJSON()` (not manual `json.Marshal`) per [issue #1281](https://github.com/tmc/langchaingo/issues/1281) guidance |
| SKILL content inflating context window | Medium | Apply `TruncateConfig` to combined system prompt + skills; warn if total exceeds model's context limit |
| Hook scripts as shell commands | High | Validate hook strings; run in sandboxed subprocess with timeout; never pass untrusted input |

---

## 11. Alternative Agentic SDK Comparison

Beyond LangChain (`langchaingo`), several other agentic SDKs were evaluated for potential use as the goflow's execution backend. The key requirements are:

1. **Go support** — the goflow is written in Go
2. **Local model support** — Ollama, vLLM, llama.cpp for offline/on-prem
3. **MCP server support** — native Model Context Protocol integration
4. **SKILL/domain-knowledge injection** — equivalent to VS Code SKILL files
5. **Agent definitions** — comparable to `.agent.md` files
6. **Workflow/DAG orchestration** — multi-step, parallel, fan-in/fan-out
7. **Tool calling** — extensible tool system
8. **Hooks/callbacks** — lifecycle events for observability

### 11.1 SDK Overview

| SDK | Stars | Language | Go Support | License |
|---|---|---|---|---|
| **LangChain (langchaingo)** | 6.0k | Go | ✅ Native | MIT |
| **CrewAI** | 47.5k | Python | ❌ None | MIT |
| **LangGraph** | 27.8k | Python (+ JS/TS) | ❌ None | MIT |
| **OpenAI Agents SDK** | 20.4k | Python (+ JS/TS) | ❌ None | MIT |
| **Microsoft AutoGen** | 56.4k | Python / C# | ❌ None | MIT |
| **Microsoft Semantic Kernel** | 27.6k | Python / C# / Java | ❌ None | MIT |
| **Mastra** | 22.4k | TypeScript | ❌ None | Apache 2.0 |

### 11.2 Feature Comparison Matrix

| Capability | langchaingo | CrewAI | LangGraph | OpenAI Agents SDK | AutoGen | Semantic Kernel | Mastra |
|---|---|---|---|---|---|---|---|
| **MCP Servers** | ❌ (via `mcp-go` bridge) | ❌ Docs not found | ❌ Not built-in | ✅ Native (4 transports: Hosted, Streamable HTTP, SSE, stdio) | ✅ Native (`McpWorkbench`) | ✅ Native (plugin) | ✅ Native |
| **Local Models (Ollama)** | ✅ Native | ✅ Native | ✅ via LangChain | ✅ via LiteLLM/Any-LLM adapters (beta) | ✅ Supported | ✅ Native (Ollama, LMStudio, ONNX) | ✅ via AI SDK |
| **Per-Step Model Routing** | ✅ New client per call | ✅ Per-agent model config | ✅ Per-node | ✅ Per-agent `model` + `ModelProvider` + `MultiProvider` | ✅ Per-agent | ✅ Per-function | ✅ Per-agent |
| **Workflow/DAG** | ❌ No orchestration | ✅ Crews (sequential/hierarchical) + Flows (event-driven) | ✅ Graph-based state machine | ⚠️ Handoffs only (no explicit DAG) | ✅ Multi-agent orchestration | ✅ Process Framework | ✅ Graph-based (`.then()`, `.branch()`, `.parallel()`) |
| **Tool Calling** | ✅ Custom tools | ✅ Rich tool ecosystem (`crewai[tools]`) | ✅ LangChain tools | ✅ Function tools + MCP tools | ✅ `AgentTool` | ✅ Plugin system (native code, OpenAPI, MCP) | ✅ Custom tools |
| **Agent Definitions** | ❌ No agent file format | ✅ YAML `agents.yaml` + `tasks.yaml` | ❌ Code-only | ✅ `Agent()` with instructions, tools, handoffs, guardrails | ✅ `AgentChat` API | ✅ Kernel functions + plugins | ✅ Code-defined agents |
| **SKILL/Knowledge Injection** | ❌ None | ✅ "Knowledge" sources (files, text, URLs) | ❌ None | ⚠️ MCP Prompts (server-provided prompt templates) | ❌ None | ✅ Plugin-based | ❌ None |
| **Hooks/Callbacks** | ✅ `callbacks.Handler` | ✅ Step callbacks, verbose mode | ✅ State checkpoint callbacks | ✅ `Lifecycle` hooks (on_tool_start, etc.) + Guardrails (input/output validation) | ✅ Event-driven handlers | ✅ Filters (function/prompt) | ✅ Observability built-in |
| **Handoffs** | ❌ None | ⚠️ Agent delegation | ❌ Graph edges | ✅ Native (`handoffs` parameter, `Handoff` objects) | ✅ Agent routing | ❌ Not explicit | ❌ Not explicit |
| **Guardrails** | ❌ None | ❌ None | ❌ None | ✅ Native (input + output guardrails, tripwires) | ❌ None | ❌ None | ✅ Evals |
| **Streaming** | ✅ Native | ✅ Native | ✅ Native | ✅ Native (+ WebSocket) | ✅ Async streaming | ✅ Native | ✅ Native |
| **Human-in-the-Loop** | ❌ None | ✅ Built-in | ✅ Interrupts | ✅ Built-in | ✅ Conversation-based | ❌ Not built-in | ✅ Suspend/Resume |
| **Tracing/Observability** | ❌ Manual | ✅ AMP Control Plane | ✅ LangSmith | ✅ Built-in tracing (OpenAI or custom processor) | ✅ Built-in | ❌ Manual | ✅ Built-in OTel |

### 11.3 Detailed SDK Analysis

#### CrewAI (Python, 47.5k ⭐)

**Architecture:** Two-layer model — "Crews" for autonomous multi-agent collaboration and "Flows" for event-driven workflow control. Combines both for complex scenarios.

**Strengths:**
- Rich agent definition via YAML (`agents.yaml`, `tasks.yaml`) — closest to `.agent.md` concept
- "Knowledge" system for injecting domain-specific content into agents (similar to SKILLs)
- Built-in Ollama and LM Studio support
- Event-driven Flows support `or_` and `and_` logical operators, `@router` decorator for conditional branching
- YAML-based project structure (`crewai create crew <name>`)
- Large community (100k+ certified developers)

**Weaknesses:**
- **Python-only** — no Go SDK
- MCP support not documented (docs page returned empty/redirected)
- Standalone framework (rewrote everything from scratch, independent of LangChain)
- Requires subprocess interop for Go integration (significant friction)

**Verdict:** Strong concepts (Crews + Flows, Knowledge injection, YAML agents) but Python-only makes it impractical as a direct Go backend.

#### LangGraph (Python/JS, 27.8k ⭐)

**Architecture:** Low-level graph-based orchestration framework for stateful agents, inspired by Pregel and Apache Beam.

**Strengths:**
- Durable execution — persists through failures with checkpoint/resume
- Comprehensive memory (short-term working memory + long-term persistence)
- Human-in-the-loop via state inspection/modification at any point
- Graph-based workflow is conceptually similar to our DAG
- Can be used standalone (no LangChain dependency required)

**Weaknesses:**
- **Python-only** (JS/TS version exists but no Go)
- No native MCP support
- No agent file format — everything is code-defined
- Requires significant boilerplate for state management
- No SKILL/knowledge injection concept

**Verdict:** Most architecturally similar to goflow's DAG approach, but Python-only with no MCP support.

#### OpenAI Agents SDK (Python/JS, 20.4k ⭐)

**Architecture:** Lightweight, provider-agnostic framework for multi-agent workflows with agents, handoffs, tools, guardrails, and sessions.

**Strengths:**
- **Best-in-class MCP support** — 4 transport types (Hosted, Streamable HTTP, SSE, stdio), `MCPServerManager` for multi-server, tool filtering, approval flows, MCP prompts
- Provider-agnostic via `MultiProvider`, `AnyLLMModel`, `LiteLLM` adapters (100+ LLMs)
- Native handoffs (closest to `.agent.md` `handoffs:` field)
- Guardrails for input/output validation (unique feature)
- Built-in tracing with span data
- Rich lifecycle hooks
- Per-agent model routing via `ModelProvider` or direct `Model` injection
- `MCP Prompts` — server-provided prompt templates that could map to SKILL-like injection
- Sessions with auto conversation history

**Weaknesses:**
- **Python-only** (JS/TS port exists but no Go)
- No explicit workflow/DAG orchestration — uses handoffs for agent routing (not level-based parallel execution)
- LiteLLM/AnyLLM adapters are "beta, best-effort" for non-OpenAI providers
- No YAML agent definition format — all code
- Tracing uploads to OpenAI servers by default (must disable or redirect)

**Verdict:** Most feature-complete for MCP + agents + handoffs + guardrails. But Python-only and lacks explicit DAG orchestration.

#### Microsoft AutoGen (Python/C#, 56.4k ⭐)

**Architecture:** Multi-agent orchestration with native MCP via `McpWorkbench`. Being superseded by Microsoft Agent Framework.

**Strengths:**
- Native MCP support via `McpWorkbench` + `StdioServerParams`
- `AgentTool` for agent-as-tool patterns
- Layered API: Core, AgentChat, Extensions
- Large community

**Weaknesses:**
- **Python/C# only** — no Go SDK
- Being superseded — README directs new users to "Microsoft Agent Framework"
- Heavy abstraction layers
- No SKILL file concept

**Verdict:** Strong MCP support but being deprecated in favor of Microsoft Agent Framework. Python/C# only.

#### Microsoft Semantic Kernel (Python/C#/Java, 27.6k ⭐)

**Architecture:** Enterprise-grade SDK with plugin ecosystem, Process Framework for workflows, and native local model support.

**Strengths:**
- **Process Framework** — explicit workflow orchestration for complex business processes (closest to DAG orchestration)
- Plugin ecosystem: native code, prompt templates, OpenAPI specs, or MCP
- **Native local model support** — Ollama, LMStudio, ONNX runtime
- Native MCP as plugin type
- Enterprise backing (Microsoft)
- Multi-language: Python, .NET, Java

**Weaknesses:**
- **No Go SDK** — .NET-first, Python and Java secondary
- Heavy enterprise design — may be overengineered for this use case
- No agent file format

**Verdict:** Best enterprise option with Process Framework + native local models + MCP. But no Go support.

#### Mastra (TypeScript, 22.4k ⭐)

**Architecture:** TypeScript framework with agents, graph-based workflows, and MCP server authoring.

**Strengths:**
- Native MCP server authoring — can expose agents as MCP servers
- Graph-based workflow engine with `.then()`, `.branch()`, `.parallel()` — intuitive syntax
- Human-in-the-loop suspend/resume with persistent state
- Built-in evals and observability (OTel)
- 40+ provider integrations via AI SDK

**Weaknesses:**
- **TypeScript-only** — no Go SDK
- From Gatsby team — relatively new (YC W25)
- Smaller ecosystem compared to others

**Verdict:** Clean architecture with good workflow support, but TypeScript-only.

### 11.4 Critical Finding: No Go-Native SDK with Full Feature Set

**None of the major agentic SDKs besides `langchaingo` offer a Go implementation.** The landscape is Python-dominant (CrewAI, LangGraph, OpenAI Agents SDK, AutoGen) with some TypeScript options (Mastra, LangGraph.js, OpenAI Agents JS). This means:

1. **`langchaingo` remains the only viable Go-native option** — despite its gaps in MCP, SKILLs, and agent definitions.
2. **Subprocess interop** (shelling out to a Python SDK) would add significant complexity, defeat the purpose of a Go binary, and require Python runtime on the host.
3. **The bridge approach** (`mcp-go` for MCP, custom loader for SKILLs, `callbacks.Handler` for hooks) is the pragmatic path.

### 11.5 What We Can Learn from Other SDKs

Even though we can't directly use them, other SDKs provide design inspiration:

| Feature | Best Implementation | How to Apply in goflow |
|---|---|---|
| **MCP integration** | OpenAI Agents SDK (4 transports, MCPServerManager, tool filtering, approval flows) | Model our `mcp-go` bridge after OpenAI's multi-transport architecture; add tool filtering and approval hooks |
| **Agent definitions** | CrewAI (`agents.yaml` + `tasks.yaml`) | Keep our `.agent.md` format but ensure LangChain backend can fully parse it |
| **Knowledge/SKILLs** | CrewAI Knowledge sources + OpenAI Agents SDK MCP Prompts | Combine: load SKILL `.md` files as prompt injection (CrewAI approach) + support MCP prompt templates (OpenAI approach) |
| **Workflow orchestration** | Semantic Kernel Process Framework + Mastra graph engine | Our existing DAG approach is already strong; add Mastra-style `.then()/.branch()/.parallel()` as syntactic sugar in future phases |
| **Guardrails** | OpenAI Agents SDK (input/output validation, tripwires) | Add optional `guardrails:` section to workflow YAML for key steps (Phase 4+) |
| **Handoffs** | OpenAI Agents SDK (native with prompt context) | Map our `handoffs:` metadata in `.agent.md` to actual agent routing in the LangChain backend |
| **Human-in-the-loop** | Mastra (suspend/resume with persistent state) | Our existing `interactive` mode + audit persistence provides similar capability |
| **Tracing** | OpenAI Agents SDK + Mastra (OTel) | Our audit trail already captures this; add OTel export in Phase 5 |

### 11.6 Recommendation

**Stick with `langchaingo` + targeted bridges**, informed by the best patterns from other SDKs:

1. **MCP:** Use `mcp-go` library with a multi-transport adapter inspired by OpenAI Agents SDK's architecture (stdio + HTTP + SSE). ~200 lines of bridge code.
2. **SKILLs:** Implement as prompt injection (load `.md` files, concatenate into system prompt) following CrewAI's Knowledge pattern. ~30 lines.
3. **Agent definitions:** Keep `.agent.md` as-is. The LangChain backend reads the same frontmatter and maps it to `langchaingo` clients. ~50 lines (already designed in Section 6).
4. **Hooks:** Map `callbacks.Handler` to our audit system. ~50 lines.
5. **Guardrails:** Defer to Phase 4+ — add optional input/output validation inspired by OpenAI Agents SDK.
6. **Handoffs:** Defer to Phase 4+ — map `.agent.md` `handoffs:` metadata to actual agent routing.

**Total bridge code: ~330 lines** — significantly less than adopting a Python subprocess approach or rewriting in Python.

If the Go ecosystem eventually gets a more feature-complete agentic SDK (unlikely in the near term given Python's dominance), the `SessionExecutor` interface makes it easy to swap backends without changing the orchestrator, DAG, or audit layers.

---

## 12. Migration Path

The design is **additive, not a replacement**:

1. **Default remains Copilot CLI** — no breaking changes for existing users.
2. **Opt-in via `config.executor: "langchain"`** — explicit switch per workflow.
3. **Mixed workflows are possible** — though a single workflow uses one executor, different workflow files can use different executors.
4. **Agent files stay compatible** — `.agent.md` `tools:` lists map to either CLI built-ins or custom `ToolRegistry` implementations depending on the executor.
5. **Audit format unchanged** — `step.meta.json`, `output.md`, `prompt.md`, `tool_calls.jsonl` structure identical regardless of backend.
6. **MCP servers work on both backends** — Copilot CLI handles MCP natively; LangChain backend uses `mcp-go` bridge. Same `mcp-servers:` config in `.agent.md` drives both.
7. **SKILL files work on both backends** — Copilot CLI injects them via its own discovery; LangChain backend reads and concatenates them into the system prompt. Same `skills:` config.
8. **Hooks degrade gracefully** — `onPreToolUse`/`onPostToolUse` map to Copilot CLI hooks natively or LangChain `callbacks.Handler` methods. LangChain additionally exposes richer lifecycle events (LLM start/end, agent action/finish) that the Copilot CLI backend doesn't support.

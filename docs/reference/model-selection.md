# Model Selection & BYOK

goflow supports flexible model selection at multiple levels. Since goflow executes steps via Copilot CLI, the available models are those provided by your Copilot plan and any future BYOK (Bring Your Own Key) provider integrations.

---

## Model Priority Resolution

When a step executes, goflow builds a priority-ordered list of models. The Copilot CLI tries each in order, falling back to the next if unavailable:

```
┌──────────────────────────────────────────────┐
│          Model Resolution Order              │
├──────────────────────────────────────────────┤
│  1. Step-level override (step.model)         │
│  2. Agent model list (.agent.md model field) │
│  3. Workflow default (config.model)          │
│  4. Copilot CLI default                      │
└──────────────────────────────────────────────┘
```

### Example

```yaml title="workflow.yaml"
config:
  model: gpt-4                    # Level 3: workflow default

steps:
  - id: security-scan
    agent: security-reviewer
    model: claude-sonnet-4.5       # Level 1: step override
    prompt: "Scan for vulnerabilities"
```

```yaml title="security-reviewer.agent.md frontmatter"
model:
  - gpt-5       # Level 2: agent primary
  - gpt-4o      # Level 2: agent fallback
```

**Resolved priority list:** `claude-sonnet-4.5` → `gpt-5` → `gpt-4o` → `gpt-4` → CLI default

Duplicates are automatically removed while preserving priority order.

---

## Configuring Models

### Per Step

Override the model for a single step:

```yaml
steps:
  - id: complex-analysis
    agent: analyzer
    model: claude-sonnet-4.5
    prompt: "Deep analysis of the architecture"
```

### Per Agent

Set model preference in the `.agent.md` frontmatter:

```yaml
---
name: security-reviewer
model: gpt-5
---
```

Or with a fallback chain:

```yaml
---
name: security-reviewer
model:
  - gpt-5
  - gpt-4o
  - gpt-4
---
```

### Per Workflow

Set a default model for all steps:

```yaml
config:
  model: gpt-4.1
```

---

## Available Models

The models available to goflow are determined by your GitHub Copilot plan. Use the `--model` flag with Copilot CLI to see the available models:

```bash
copilot --model
```

Common models include:

| Model | Typical Use |
|-------|-------------|
| `claude-sonnet-4.5` | Default Copilot CLI model, good all-around |
| `gpt-4.1` | Strong reasoning, good for complex analysis |
| `gpt-4o` | Fast, good for simpler tasks |
| `o4-mini` | Reasoning model for complex problem solving |
| `claude-sonnet-4` | Anthropic's balanced model |

Model availability depends on your Copilot plan and region. Each model consumes premium requests at different multiplier rates.

---

## BYOK (Bring Your Own Key)

goflow supports Bring Your Own Key (BYOK) — use your own API keys for OpenAI, Anthropic, Azure, or Ollama instead of relying on GitHub Copilot's built-in model routing. When BYOK is configured, LLM inference requests are routed to your provider while all CLI built-in tools (grep, view, semantic_search, etc.) remain fully available.

To enable BYOK, add a `config.provider` section to your workflow YAML:

```yaml
config:
  provider:
    type: "openai"               # Provider type
    base_url: "https://api.openai.com/v1"  # API endpoint
    api_key_env: "OPENAI_API_KEY"          # Environment variable containing the API key
```

### Provider Config Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Provider identifier (e.g., `"openai"`, `"azure"`, `"ollama"`) |
| `base_url` | string | API endpoint URL |
| `api_key_env` | string | Name of the environment variable holding the API key |

### Ollama / Local LLMs

Local models via Ollama work without an API key:

```yaml
config:
  model: "llama3"
  provider:
    type: "ollama"
    base_url: "http://localhost:11434/v1"
    api_key_env: ""              # Ollama doesn't require an API key
```

### External Providers

```yaml
config:
  model: "gpt-4-turbo"
  provider:
    type: "openai"
    base_url: "https://api.openai.com/v1"
    api_key_env: "OPENAI_API_KEY"
```

```yaml
config:
  model: "claude-3-opus"
  provider:
    type: "anthropic"
    base_url: "https://api.anthropic.com/v1"
    api_key_env: "ANTHROPIC_API_KEY"
```

### Current Workaround

Even without BYOK, you can configure model selection through Copilot CLI itself:

1. **Use `--model` flag** — goflow passes the model name from the workflow/agent/step to Copilot CLI via `--model`
2. **Model fallback chains** — Define multiple models in the agent file; goflow tries each in order
3. **Copilot CLI configuration** — Configure default model preferences in `~/.copilot/config.json`
4. **`--cli` flag** — Use `goflow run --cli` to force the legacy CLI subprocess executor (no BYOK)

---

## Executor Backends

goflow uses two executor backends, both powered by the Copilot CLI runtime:

| Backend | When Used | BYOK | Streaming | Session Resume |
|---|---|---|---|---|
| **SDK Executor** (default) | No flags or `config.provider` set | Yes | Yes | Yes |
| **CLI Executor** (`--cli`) | `--cli` flag | No | No | No |
| **Mock Executor** (`--mock`) | `--mock` flag | N/A | N/A | N/A |

The SDK executor wraps the CLI via JSON-RPC. All built-in tools remain available regardless of backend choice. Without `config.provider`, the SDK executor uses GitHub Models (same as the CLI executor did before).

---

## Implementation Details

The model selection logic lives in `pkg/executor/executor.go`. During step execution:

1. The executor collects models from step, agent, and workflow config
2. Duplicates are removed (preserving priority order)
3. With the SDK executor: each model is tried via the SDK's session creation; if unavailable, the next is attempted
4. With the CLI executor (`--cli`): each model is tried via `--model MODEL_NAME` on the Copilot CLI invocation
5. If all specified models fail, the default model is used

The `ProviderConfig` struct in `pkg/workflow/types.go` is parsed from YAML and consumed by the SDK executor in `pkg/executor/copilot_sdk.go`. When `config.provider` is set, the provider is passed to the Copilot SDK's session configuration for BYOK routing.

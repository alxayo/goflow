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

!!! warning "Planned Feature — Not Yet Active"
    The BYOK provider configuration is **parsed from workflow YAML** but is **not yet wired into the runtime**. The fields below are reserved for future implementation. Currently, all model execution goes through Copilot CLI's built-in model routing.

goflow's workflow schema includes a `config.provider` section for future BYOK support:

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

### Planned: Ollama / Local LLMs

When BYOK is implemented, the intended configuration for local models would be:

```yaml
config:
  model: "llama3"
  provider:
    type: "ollama"
    base_url: "http://localhost:11434/v1"
    api_key_env: ""              # Ollama doesn't require an API key
```

### Planned: External Providers

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

Until BYOK is active in goflow, you can configure model selection through Copilot CLI itself:

1. **Use `--model` flag** — goflow passes the model name from the workflow/agent/step to Copilot CLI via `--model`
2. **Model fallback chains** — Define multiple models in the agent file; goflow tries each in order
3. **Copilot CLI configuration** — Configure default model preferences in `~/.copilot/config.json`

---

## Implementation Details

The model selection logic lives in `pkg/executor/executor.go`. During step execution:

1. The executor collects models from step, agent, and workflow config
2. Duplicates are removed (preserving priority order)
3. Each model is tried via `--model MODEL_NAME` on the Copilot CLI invocation
4. If a model returns an unavailability error, the next model is tried
5. If all specified models fail, the CLI runs without `--model` (using its default)

The `ProviderConfig` struct in `pkg/workflow/types.go` is parsed from YAML but not yet consumed by the executor.

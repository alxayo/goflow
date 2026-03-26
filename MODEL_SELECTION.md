# Model Selection in Workflow Runner

This document explains how model selection works in the workflow runner, including the priority order, fallback behavior, and integration with the Copilot CLI.

## Overview

The workflow runner supports model selection at three levels:
1. **Step-level** - Highest priority, specified in the workflow YAML
2. **Agent-level** - Specified in the `.agent.md` file frontmatter
3. **Workflow-level** - Default model in the workflow's `config.model`

When a step executes, the runner builds a **priority-ordered list** of models to try. The Copilot CLI attempts each model in order; if unavailable, it tries the next. If all specified models fail, the CLI picks its default model.

## Priority Resolution

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Model Resolution Order                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Step Model Override                                                  │
│     └── step.model in workflow YAML                                     │
│                                                                          │
│  2. Agent Model List                                                     │
│     └── model: [...] in .agent.md frontmatter                           │
│         (all models in order, forming a fallback chain)                 │
│                                                                          │
│  3. Workflow Default Model                                              │
│     └── config.model in workflow YAML                                   │
│                                                                          │
│  4. Copilot CLI Default                                                 │
│     └── If no models specified or all unavailable                       │
│         (typically GPT-4.1 as of March 2026)                            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Example Resolution

Given this configuration:

**Workflow YAML:**
```yaml
name: code-review
config:
  model: gpt-4  # Workflow default

steps:
  - id: security-scan
    agent: security-reviewer
    model: claude-sonnet-4.5  # Step override
    prompt: "Scan for vulnerabilities"
```

**Agent file (security-reviewer.agent.md):**
```yaml
---
name: security-reviewer
model:
  - gpt-5
  - gpt-4o
---
```

**Resolved model priority list:**
```
["claude-sonnet-4.5", "gpt-5", "gpt-4o", "gpt-4"]
   ^                    ^        ^         ^
   Step override        Agent models       Workflow default
```

The executor will try `claude-sonnet-4.5` first, then `gpt-5`, then `gpt-4o`, then `gpt-4`, and finally the CLI default if all are unavailable.

## Deduplication

Duplicate models are automatically removed while preserving priority order:

```yaml
# If step.model = "gpt-4", agent.model = ["gpt-5", "gpt-4"], config.model = "gpt-5"
# Input:  ["gpt-4", "gpt-5", "gpt-4", "gpt-5"]
# Output: ["gpt-4", "gpt-5"]  # Duplicates removed, order preserved
```

## Configuration Reference

### Step-Level Model

Override the model for a specific step in the workflow YAML:

```yaml
steps:
  - id: analyze
    agent: analyzer
    model: claude-sonnet-4.5  # Uses this model, ignoring agent's model
    prompt: "Analyze the code"
```

**Use case:** When a specific step requires a different model than the agent typically uses (e.g., a more capable model for complex analysis, or a faster model for simple tasks).

### Agent-Level Model

Define the preferred model(s) in the agent's `.agent.md` frontmatter:

```yaml
---
name: security-reviewer
model: gpt-5  # Single model
---
```

Or with a priority fallback list:

```yaml
---
name: security-reviewer
model:
  - gpt-5      # Try first
  - gpt-4o     # Fallback if gpt-5 unavailable
  - gpt-4      # Last resort
---
```

**Use case:** When an agent works best with specific models. The fallback list ensures graceful degradation if the preferred model is unavailable.

### Workflow-Level Default

Set a default model for all steps in the workflow config:

```yaml
name: code-review-pipeline
config:
  model: gpt-4o  # Default for all steps without explicit model

steps:
  - id: step1
    agent: agent-without-model  # Uses gpt-4o
    prompt: "..."
  - id: step2
    agent: agent-with-model     # Uses agent's model, gpt-4o as fallback
    prompt: "..."
```

**Use case:** When you want a consistent model across the workflow, while still allowing individual agents/steps to override.

## CLI Fallback Behavior

When the Copilot CLI receives a `--model` flag with an unavailable model, it returns:

```
Error: Model "model-name" from --model flag is not available.
```

The workflow runner handles this by:
1. Detecting the "not available" error
2. Trying the next model in the priority list
3. Eventually falling back to CLI default (no `--model` flag)

### Error Handling

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Model Fallback Flow                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Try model[0] ──► Success ──► Return output                             │
│       │                                                                  │
│       ▼ "not available"                                                  │
│  Try model[1] ──► Success ──► Return output                             │
│       │                                                                  │
│       ▼ "not available"                                                  │
│      ...                                                                 │
│       │                                                                  │
│       ▼ "not available"                                                  │
│  Try CLI default ──► Success ──► Return output                          │
│       │                                                                  │
│       ▼ Other error                                                      │
│  Return error ──► Step fails                                            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

**Important:** Only "model unavailable" errors trigger fallback. Other errors (network issues, authentication failures, etc.) cause immediate step failure.

## BYOK (Bring Your Own Key) Providers

When using a custom model provider, model selection works the same way but requires provider configuration:

```yaml
config:
  model: gpt-4
  provider:
    type: openai
    base_url: https://my-api.example.com/v1
    api_key_env: MY_API_KEY
```

Or via environment variables:

```bash
export COPILOT_PROVIDER_BASE_URL=https://my-api.example.com/v1
export COPILOT_PROVIDER_API_KEY=sk-...
export COPILOT_MODEL=my-custom-model
```

**Note:** The `COPILOT_MODEL` environment variable serves as a global default but is overridden by the `--model` CLI flag, which the workflow runner uses for specified models.

## Audit Logging

The resolved model (highest priority that will be attempted first) is recorded in each step's audit metadata:

```json
{
  "step_id": "analyze",
  "agent": "security-reviewer",
  "model": "claude-sonnet-4.5",
  "status": "completed",
  ...
}
```

**Note:** This records the *intended* model (first in the priority list), not necessarily the model that was actually used if fallback occurred. The CLI does not currently expose which model was ultimately selected after fallback.

## Implementation Details

### SessionConfig.Models

The executor passes a `Models []string` slice to the session config:

```go
// pkg/executor/sdk.go
type SessionConfig struct {
    SystemPrompt string
    Models       []string  // Priority-ordered list of models to try
    Tools        []string
    MCPServers   map[string]interface{}
    ExtraDirs    []string
}
```

### Model Resolution Logic

```go
// pkg/executor/executor.go
func (se *StepExecutor) resolveModels(step workflow.Step, agent *agents.Agent) []string {
    var models []string

    // Highest priority: step-level model override.
    if step.Model != "" {
        models = append(models, step.Model)
    }

    // Second: agent's model list (may have multiple fallbacks).
    models = append(models, agent.Model.Models...)

    // Third: workflow-level default model.
    if se.DefaultModel != "" {
        models = append(models, se.DefaultModel)
    }

    // Deduplicate while preserving order.
    return dedupeStrings(models)
}
```

### CLI Executor Fallback

```go
// pkg/executor/copilot_cli.go
func (s *CopilotCLISession) Send(ctx context.Context, prompt string) (string, error) {
    // Try each model in the priority list, then fall back to CLI default.
    modelsToTry := append([]string{}, s.cfg.Models...)
    modelsToTry = append(modelsToTry, "")  // Empty = CLI default

    for _, model := range modelsToTry {
        output, err := s.tryWithModel(ctx, prompt, model)
        if err == nil {
            return output, nil
        }
        if isModelUnavailableError(err) {
            continue  // Try next model
        }
        return "", err  // Other errors fail immediately
    }
    return "", fmt.Errorf("all models unavailable")
}
```

## Best Practices

1. **Use agent-level models for typical behavior**: Define the preferred model in the agent file so it works consistently across workflows.

2. **Use step-level overrides sparingly**: Only override when a specific step needs different model characteristics.

3. **Provide fallback models for availability**: If using premium models (GPT-5, Claude Opus), include cheaper alternatives as fallbacks.

4. **Set a workflow default for consistency**: When multiple agents in a workflow should share a model preference.

5. **Test with unavailable models**: Verify fallback behavior by specifying a non-existent model first in the list.

## Troubleshooting

### "All models unavailable" Error

This occurs when every model in the priority list is unavailable and the CLI default also fails. Check:
- Network connectivity
- API key validity
- Model name spelling
- Model availability in your region/subscription

### Model Not Being Used

If a step isn't using the expected model:
1. Check step-level `model` field (highest priority)
2. Check agent's `model` frontmatter
3. Check workflow's `config.model`
4. Remember: deduplication removes later duplicates

### Audit Shows Different Model

The audit log shows the *intended* first model, not the actual model used after fallback. The CLI doesn't expose which model ultimately processed the request.

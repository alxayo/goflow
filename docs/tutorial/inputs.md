# Adding Inputs

Make your workflows configurable by accepting runtime parameters.

---

## The Problem

Hardcoded values make workflows inflexible:

```yaml
steps:
  - id: review
    prompt: "Review the file src/main.go"  # Hardcoded! Can't reuse for other files
```

**Solution:** Use inputs — variables you provide when running the workflow.

---

## Defining Inputs

Add an `inputs` section to your workflow:

```yaml title="code-review.yaml" hl_lines="4-7"
name: "code-review"
description: "Review any file you specify"

inputs:
  files:
    description: "The files to review"
    default: "*.go"

agents:
  reviewer:
    inline:
      description: "Code reviewer"
      prompt: "You are an expert code reviewer."

steps:
  - id: review
    agent: reviewer
    prompt: "Review these files: {{inputs.files}}"

output:
  steps: [review]
  format: markdown
```

### Input Fields

| Field | Required | Description |
|-------|----------|-------------|
| `description` | No | Explains what the input is for |
| `default` | No | Default value if not provided at runtime |

!!! tip "Always Add Descriptions"
    Descriptions appear in error messages and help documentation, making workflows self-documenting.

---

## Running With Inputs

Provide inputs using the `--inputs` flag:

```bash
goflow run --workflow code-review.yaml --inputs files='src/**/*.go'
```

### Multiple Inputs

```bash
goflow run --workflow example.yaml \
  --inputs files='src/*.go' \
  --inputs mode=detailed
```

### Using Defaults

If an input has a default value, you can omit it:

```bash
# Uses default: "*.go"
goflow run --workflow code-review.yaml
```

---

## Input Template Syntax

Reference inputs using `{{inputs.NAME}}`:

```yaml
prompt: "Review {{inputs.files}} in {{inputs.mode}} mode"
```

### Where You Can Use Input Templates

| Location | Example |
|----------|---------|
| Step prompts | `prompt: "Analyze {{inputs.files}}"` |
| Agent prompts | `prompt: "You specialize in {{inputs.language}}"` |
| Conditions | `contains: "{{inputs.keyword}}"` |

---

## Complete Example

Let's build a configurable greeting workflow:

```yaml title="greeting-workflow.yaml"
name: "greeting-workflow"
description: "Generates a personalized greeting"

inputs:
  name:
    description: "Name of the person to greet"
    default: "World"
  style:
    description: "Greeting style: formal, casual, or pirate"
    default: "casual"

agents:
  greeter:
    inline:
      description: "Creates greetings in various styles"
      prompt: "You create greetings. Use {{inputs.style}} style."

steps:
  - id: greet
    agent: greeter
    prompt: "Create a greeting for {{inputs.name}}. Keep it to one sentence."

output:
  steps: [greet]
  format: markdown
```

### Try It

```bash
# Default greeting
goflow run --workflow greeting-workflow.yaml --mock

# Custom name
goflow run --workflow greeting-workflow.yaml --inputs name='Alice' --mock

# Custom name and style
goflow run --workflow greeting-workflow.yaml \
  --inputs name='Captain Jack' \
  --inputs style='pirate' \
  --verbose
```

---

## Input Validation

goflow validates inputs when the workflow starts:

### Missing Required Input

If an input has no default and you don't provide it:

```yaml
inputs:
  files:
    description: "Files to review"
    # No default!
```

```bash
goflow run --workflow example.yaml
# Error: missing required input: files
```

### Unknown Input

If you pass an input that isn't defined:

```bash
goflow run --workflow example.yaml --inputs typo='value'
# Warning: unknown input 'typo' (will be ignored)
```

---

## Best Practices

### 1. Use Descriptive Names

```yaml
# ✓ Good
inputs:
  target_files:
    description: "Files to analyze (glob pattern)"

# ✗ Bad
inputs:
  f:
    description: "files"
```

### 2. Provide Sensible Defaults

```yaml
inputs:
  depth:
    description: "Analysis depth (brief, normal, detailed)"
    default: "normal"  # Most users want this
```

### 3. Document Valid Values

```yaml
inputs:
  format:
    description: "Output format: 'json', 'yaml', or 'markdown'"
    default: "markdown"
```

---

## What You Learned

:white_check_mark: How to define inputs with `inputs:`  
:white_check_mark: How to provide inputs with `--inputs key='value'`  
:white_check_mark: How to use `{{inputs.name}}` in prompts  
:white_check_mark: How default values work  

---

## Next Steps

Now that you can make workflows configurable, let's learn to chain multiple steps together:

**[Multi-Step Pipelines →](multi-step.md)**

# Your First Workflow

Let's build a complete workflow from scratch, explaining every piece along the way.

---

## What We're Building

A simple workflow that:

1. Has one AI agent (a "greeter")
2. Runs one step (says hello)
3. Outputs the result

This is the simplest possible workflow — perfect for understanding the basics.

---

## Step 1: Create the Workflow File

Create a new file called `hello-workflow.yaml`:

```yaml title="hello-workflow.yaml"
name: "hello-workflow"
description: "My first goflow workflow — says hello"

agents:
  greeter:
    inline:
      description: "A friendly assistant"
      prompt: "You are a friendly assistant. Be concise and helpful."

steps:
  - id: say-hello
    agent: greeter
    prompt: "Say hello and explain what goflow does in two sentences."

output:
  steps: [say-hello]
  format: markdown
```

---

## Step 2: Understand Each Part

Let's break down what each section does:

### The `name` and `description`

```yaml
name: "hello-workflow"
description: "My first goflow workflow — says hello"
```

- **`name`** — A unique identifier for this workflow (required)
- **`description`** — Human-readable explanation (optional but helpful)

The name is used in the audit trail folder names.

---

### The `agents` Section

```yaml
agents:
  greeter:
    inline:
      description: "A friendly assistant"
      prompt: "You are a friendly assistant. Be concise and helpful."
```

Agents are the AI personas that perform tasks. Each agent has:

- **A name** (here: `greeter`) — you'll reference this in steps
- **A definition** — either `inline` (defined right here) or `file` (loaded from a `.agent.md` file)

For inline agents:

- **`description`** — What this agent does
- **`prompt`** — The **system prompt** sent to the AI. This is the agent's instructions/persona.

!!! info "System Prompt vs Step Prompt"
    - **System prompt** (in `agents.*.inline.prompt`) — The agent's personality and instructions, sent once at the start
    - **Step prompt** (in `steps.*.prompt`) — The actual task/question you want the agent to answer

---

### The `steps` Section

```yaml
steps:
  - id: say-hello
    agent: greeter
    prompt: "Say hello and explain what goflow does in two sentences."
```

Steps are the actual tasks that get executed. Each step has:

- **`id`** — Unique identifier (used for dependencies and template references)
- **`agent`** — Which agent from the `agents` section should run this step
- **`prompt`** — The question or task for the agent

Steps run in order based on their dependencies. We'll learn about `depends_on` in the [Multi-Step Pipelines](../tutorial/multi-step.md) tutorial.

---

### The `output` Section

```yaml
output:
  steps: [say-hello]
  format: markdown
```

Controls what gets printed when the workflow finishes:

- **`steps`** — Which step outputs to include (by `id`)
- **`format`** — Output format: `markdown`, `json`, or `plain`

---

## Step 3: Run It (Mock Mode)

Test the workflow without making real AI calls:

```bash
goflow run --workflow hello-workflow.yaml --mock --verbose
```

**Expected output:**

```
[INFO] Loading workflow: hello-workflow.yaml
[INFO] Starting workflow: hello-workflow
[INFO] Step 1/1: say-hello (greeter)
[INFO] Workflow completed in 0.02s

## Step: say-hello

mock output
```

The "mock output" confirms our workflow structure is correct. The `--verbose` flag shows progress.

---

## Step 4: Run It For Real

If you have Copilot CLI installed, run without `--mock`:

```bash
goflow run --workflow hello-workflow.yaml --verbose
```

Now you'll get real AI-generated content instead of "mock output".

---

## Step 5: Check the Audit Trail

Every run creates a detailed log:

```bash
ls .workflow-runs/
```

Open the latest folder:

```
.workflow-runs/2026-03-26T14-30-00_hello-workflow/
├── workflow.meta.json   # Timing, status, inputs
├── workflow.yaml        # Copy of your workflow file
├── final_output.md      # The output that was printed
└── steps/
    └── 00_say-hello/
        ├── step.meta.json  # Agent, model, duration
        ├── prompt.md       # Exact prompt sent to AI
        └── output.md       # AI's response
```

The audit trail is invaluable for:

- **Debugging** — See exactly what prompt was sent
- **Reproducibility** — Workflow file is saved with each run
- **Analysis** — Compare outputs across runs

---

## What You Learned

:white_check_mark: How to create a workflow YAML file  
:white_check_mark: The four main sections: `name`, `agents`, `steps`, `output`  
:white_check_mark: Difference between system prompt and step prompt  
:white_check_mark: How to run in mock mode vs real mode  
:white_check_mark: How to check the audit trail  

---

## Common Mistakes

!!! warning "Agent name must match"
    The `agent:` in a step must exactly match a name from the `agents:` section:
    ```yaml
    agents:
      greeter:  # <-- This name
        inline: ...
    
    steps:
      - id: hello
        agent: greeter  # <-- Must match exactly
    ```

!!! warning "Step IDs must be unique"
    Every step needs a unique `id`:
    ```yaml
    steps:
      - id: step-one   # ✓ Unique
        ...
      - id: step-two   # ✓ Unique
        ...
      - id: step-one   # ✗ ERROR: Duplicate!
    ```

---

## Next Steps

Now that you understand the basics:

- :material-variable: [Adding Inputs](../tutorial/inputs.md) — Make workflows configurable with `{{inputs.X}}`
- :material-stairs: [Multi-Step Pipelines](../tutorial/multi-step.md) — Chain steps with `depends_on`
- :material-school: [Full Tutorial](../tutorial/index.md) — Learn all features progressively

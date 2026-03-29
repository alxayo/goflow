# goflow

**Orchestrate multi-agent AI workflows with a single YAML file.**

goflow is an AI workflow engine that coordinates LLM agents in parallel pipelines — no manual copy-paste, no scripting chaos, just declarative power.

---

## :rocket: Why goflow?

| Feature | Description |
|---------|-------------|
| :gear: **Declarative DAG** | Define dependencies in YAML; the engine builds and executes the graph |
| :zap: **Parallel Execution** | Independent steps run concurrently via goroutines |
| :jigsaw: **Composable Agents** | Mix inline agents with reusable `.agent.md` files |
| :bookmark_tabs: **Full Audit Trail** | Every run writes prompts, outputs, metadata, and timing |
| :test_tube: **Safe Iteration** | `--mock` mode validates workflows without token spend |

---

!!! tip "Get started in 60 seconds"
    Clone the repo, build, and run your first workflow:
    ```bash
    git clone https://github.com/alxayo/goflow.git && cd goflow
    go build -o goflow ./cmd/goflow
    ./goflow run --workflow examples/simple-sequential.yaml --mock --verbose
    ```

---

## :bulb: Core Concepts

<div class="grid cards" markdown>

- :material-file-document-edit: **Workflow YAML** — The source of truth for execution
- :material-robot: **Agents** — Personas and tool permissions for each task
- :material-stairs: **Steps** — Units of work that can run sequentially or in parallel
- :material-code-braces: **Templates** — References like `{{steps.analyze.output}}`
- :material-folder-open: **Audit artifacts** — Structured run folders under `.workflow-runs/`

</div>

---

## :hammer_and_wrench: Build a Simple Workflow

Create `hello.yaml`:

```yaml title="hello.yaml"
name: "hello"
description: "My first goflow workflow"

agents:
  explainer:
    inline:
      description: "Explains things clearly"
      prompt: "You are a concise technical explainer."

steps:
  - id: intro
    agent: explainer
    prompt: "Explain what goflow does in three bullet points."

output:
  steps: [intro]
  format: markdown
```

Run it:

```bash
goflow run --workflow hello.yaml --mock --verbose
```

---

## :compass: What To Read Next

| Section | Description |
|---------|-------------|
| [:material-rocket-launch: **Quick Start**](quickstart.md) | Your first full workflow run |
| [:material-download: **Installation**](installation.md) | Build from source or download binaries |
| [:material-book-open-page-variant: **Guide**](guide/getting-started.md) | Step-by-step production workflow building |
| [:material-file-document: **Reference**](reference/workflow-schema.md) | Complete schema and advanced features |
| [:material-code-tags: **Examples**](examples/index.md) | Ready-to-use workflow patterns |

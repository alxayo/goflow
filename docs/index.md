<div class="hero" markdown>

# goflow

**Orchestrate multi-agent AI workflows with a single YAML file.**

goflow is a command-line tool that coordinates multi-step AI agent pipelines.
Define agents, wire them into a DAG, and let goflow handle parallelism, data passing, and audit trails — all from one YAML file.

[Get Started in 5 Minutes :material-rocket-launch:](getting-started/quickstart.md){ .md-button .md-button--primary }
[View on GitHub :material-github:](https://github.com/alxayo/goflow){ .md-button }

</div>

---

## :stopwatch: Quick Start — Up and Running in 5 Minutes

!!! tip "No Copilot CLI needed"
    The steps below use `--mock` mode, which simulates AI responses. You can complete this entire quick start without an API key.

### 1. Install

```bash
git clone https://github.com/alxayo/goflow.git
cd goflow
go build -o goflow ./cmd/workflow-runner/main.go
```

### 2. Run an Example Workflow

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --mock --verbose
```

### 3. See the Output

```
[INFO] Loading workflow: examples/simple-sequential.yaml
[INFO] Starting workflow: simple-sequential
[INFO] Step 1/3: security-review (security-reviewer)
[INFO] Step 2/3: perf-review (performance-reviewer)
[INFO] Step 3/3: summary (aggregator)
[INFO] Workflow completed in 0.05s

## Step: summary

mock output
```

### 4. Inspect the Audit Trail

```bash
ls .workflow-runs/*/
# workflow.meta.json  workflow.yaml  final_output.md  steps/
```

### 5. Run with Real AI (Optional)

```bash
./goflow run \
  --workflow examples/simple-sequential.yaml \
  --inputs files='pkg/workflow/*.go' \
  --verbose
```

Remove `--mock` and goflow calls a real LLM. Requires [Copilot CLI](getting-started/installation.md#setting-up-copilot-cli-for-real-ai-calls) on your PATH.

[:material-book-open-page-variant: Full Installation Guide](getting-started/installation.md){ .md-button }
[:material-file-document-edit: Build Your First Workflow](getting-started/first-workflow.md){ .md-button }

---

## :thinking: What Problem Does goflow Solve?

Imagine you want to review code with three specialized AI agents — a security reviewer, a performance reviewer, and an aggregator. Without goflow, you'd run each agent manually, copy outputs between them, and coordinate timing yourself.

**With goflow**, you describe the entire pipeline in YAML and run it with one command:

```yaml title="code-review.yaml"
name: "code-review"
agents:
  security-reviewer:
    inline:
      description: "Reviews code for security issues"
      prompt: "You are a security expert. Find vulnerabilities."
      tools: ["grep", "view"]
  performance-reviewer:
    inline:
      description: "Reviews code for performance issues"
      prompt: "You are a performance expert. Find bottlenecks."
      tools: ["grep", "view"]
  aggregator:
    inline:
      description: "Combines review findings"
      prompt: "You combine multiple reviews into a clear summary."

steps:
  - id: security-review
    agent: security-reviewer
    prompt: "Review all Go files for security vulnerabilities."

  - id: performance-review
    agent: performance-reviewer
    prompt: "Review all Go files for performance issues."

  - id: summary
    agent: aggregator
    prompt: |
      Combine these reviews into a final report:
      ## Security: {{steps.security-review.output}}
      ## Performance: {{steps.performance-review.output}}
    depends_on: [security-review, performance-review]

output:
  steps: [summary]
  format: markdown
```

```bash
goflow run --workflow code-review.yaml --verbose
```

goflow automatically:

- :zap: Runs `security-review` and `performance-review` **in parallel**
- :link: Injects their outputs into `summary` using `{{steps.X.output}}`
- :bookmark_tabs: Creates a complete **audit trail** of every prompt and response
- :test_tube: Supports `--mock` mode for testing without real AI tokens

---

## :rocket: Key Features

<div class="feature-grid" markdown>

<div class="feature-card" markdown>
### :material-file-document-edit: Declarative YAML
Define your entire pipeline — agents, steps, dependencies — in one file. No glue code required.
</div>

<div class="feature-card" markdown>
### :material-lightning-bolt: Automatic Parallelism
Steps that don't depend on each other run concurrently via goroutines. Fan-out and fan-in patterns are built in.
</div>

<div class="feature-card" markdown>
### :material-code-braces: Template Variables
Pass data between steps with `{{steps.X.output}}` and parameterize workflows with `{{inputs.Y}}`.
</div>

<div class="feature-card" markdown>
### :material-source-branch: Conditional Steps
Skip or run steps based on previous outputs using `contains`, `not_contains`, or `equals` conditions.
</div>

<div class="feature-card" markdown>
### :material-folder-search: Full Audit Trail
Every run saves prompts, outputs, timing, and metadata to a timestamped directory for full transparency.
</div>

<div class="feature-card" markdown>
### :material-test-tube: Mock Mode
Test your workflow structure end-to-end without making real API calls — instant results, zero cost.
</div>

<div class="feature-card" markdown>
### :material-robot: Reusable Agent Files
Define agents once in `.agent.md` files — compatible with VS Code custom agents — and use them across workflows.
</div>

<div class="feature-card" markdown>
### :material-account-question: Interactive Mode
Agents can pause mid-workflow to ask the user clarification questions, then continue with the answer.
</div>

</div>

---

## :gear: Powered by GitHub Copilot CLI

goflow is built on top of **GitHub Copilot CLI** — the standalone command-line agent from GitHub. Every workflow step is executed as a Copilot CLI session, which means goflow inherits the full Copilot ecosystem:

| Primitive | What It Is | How goflow Uses It |
|-----------|------------|-------------------|
| **Agent Files** (`.agent.md`) | Markdown files with YAML frontmatter defining persona, tools, model | Each workflow step references an agent |
| **Skills** (`SKILL.md`) | Folders of instructions and resources for specialized tasks | Attached at workflow or step level |
| **MCP Servers** | External tool servers using the Model Context Protocol | Declared per agent in `.agent.md` |
| **Hooks** (`.github/hooks/*.json`) | Shell commands at session lifecycle points | Loaded automatically by Copilot CLI |
| **Model Selection** | Choose from available models per step | Configurable per workflow, agent, or step |

!!! info "Copilot CLI Required for Real Execution"
    goflow requires [GitHub Copilot CLI](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/about-copilot-cli) installed locally (`copilot` on PATH) for real AI calls. Use `--mock` mode for testing without it.

### Supported Operating Systems

| OS | Support |
|----|---------|
| **macOS** | Intel and Apple Silicon |
| **Linux** | x64 and ARM64 |
| **Windows** | Via PowerShell and [WSL](https://learn.microsoft.com/en-us/windows/wsl/about) |

---

## :bulb: Core Concepts

<div class="grid cards" markdown>

- :material-file-document-edit: **Workflow YAML** — The file that defines your entire pipeline: agents, steps, dependencies, and output formatting
- :material-robot: **Agents** — AI personas with specific tools, instructions, and model preferences — defined inline or in `.agent.md` files
- :material-stairs: **Steps** — Individual tasks that agents perform, wired together via `depends_on` into a dependency graph (DAG)
- :material-code-braces: **Templates** — `{{steps.X.output}}` and `{{inputs.Y}}` placeholders that are resolved at runtime
- :material-folder-open: **Audit Trail** — Complete logs of every run stored under `.workflow-runs/` with prompts, outputs, and metadata

</div>

---

## :compass: Where to Start

| If You Want To... | Go Here |
|-------------------|---------|
| Install goflow and run your first command | [:material-download: Installation](getting-started/installation.md) |
| See goflow work in under 5 minutes | [:material-rocket-launch: Quick Start](getting-started/quickstart.md) |
| Build your first workflow step-by-step | [:material-book-open-page-variant: Your First Workflow](getting-started/first-workflow.md) |
| Learn features progressively | [:material-school: Tutorial](tutorial/index.md) |
| Look up specific YAML fields | [:material-file-document: Workflow Schema Reference](reference/workflow-schema.md) |
| See all CLI flags and options | [:material-console: CLI Reference](reference/cli.md) |
| Browse all configuration options | [:material-cog: Settings & Options](reference/settings-and-options.md) |
| Copy working workflow patterns | [:material-code-tags: Examples](examples/index.md) |
| Fix a problem | [:material-lifebuoy: Troubleshooting](troubleshooting.md) |

---

## :link: Links

- [:material-github: GitHub Repository](https://github.com/alxayo/goflow)
- [:material-folder-multiple: Example Workflows](https://github.com/alxayo/goflow/tree/main/examples)
- [:material-text-box-check: Changelog](changelog.md)

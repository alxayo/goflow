# goflow

**Orchestrate multi-agent AI workflows with a single YAML file.**

goflow is a command-line tool that coordinates multi-step AI agent pipelines. Instead of manually running agents one at a time and copy-pasting results between them, you describe the entire pipeline in a YAML file and let goflow handle the rest.

---

## :thinking: What Problem Does goflow Solve?

Imagine you want to review code with multiple specialized AI agents:

1. A **security reviewer** checks for vulnerabilities
2. A **performance reviewer** checks for bottlenecks  
3. An **aggregator** combines all findings into a report

Without goflow, you would need to:

- Run each agent manually
- Copy output from one agent to the next
- Coordinate which agents can run in parallel
- Keep track of all the results

**With goflow**, you define this once in YAML:

```yaml title="code-review.yaml"
name: "code-review"
description: "Multi-agent code review pipeline"

agents:
  security-reviewer:
    inline:
      description: "Reviews code for security issues"
      prompt: "You are a security expert. Find vulnerabilities and cite file paths."
      tools: ["grep", "view"]
  
  performance-reviewer:
    inline:
      description: "Reviews code for performance issues"  
      prompt: "You are a performance expert. Find bottlenecks and cite file paths."
      tools: ["grep", "view"]
  
  aggregator:
    inline:
      description: "Combines review findings"
      prompt: "You combine multiple reviews into a clear, actionable summary."

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
      
      ## Security Findings
      {{steps.security-review.output}}
      
      ## Performance Findings
      {{steps.performance-review.output}}
    depends_on: [security-review, performance-review]

output:
  steps: [summary]
  format: markdown
```

Then run it with one command:

```bash
goflow run --workflow code-review.yaml --verbose
```

goflow automatically:

- :zap: Runs `security-review` and `performance-review` **in parallel** (they don't depend on each other)
- :link: Injects their outputs into the `summary` step using `{{steps.X.output}}`
- :bookmark_tabs: Creates a complete audit trail of every prompt and response
- :test_tube: Supports `--mock` mode so you can test without using real AI tokens

---

## :rocket: Key Features

| Feature | What It Does |
|---------|--------------|
| **Declarative YAML** | Define your entire pipeline in one file — agents, steps, dependencies |
| **Automatic Parallelism** | Steps that don't depend on each other run simultaneously |
| **Template Variables** | Pass outputs between steps with `{{steps.X.output}}` |
| **Conditional Steps** | Skip or run steps based on previous outputs |
| **Full Audit Trail** | Every run saves prompts, outputs, and timing to disk |
| **Mock Mode** | Test your workflow structure without API calls |
| **Reusable Agents** | Define agents once in `.agent.md` files, use everywhere |

---

## :bulb: Core Concepts

Before diving in, here are the key ideas:

<div class="grid cards" markdown>

- :material-file-document-edit: **Workflow YAML** — The file that defines your entire pipeline
- :material-robot: **Agents** — AI personas with specific tools and instructions
- :material-stairs: **Steps** — Individual tasks that agents perform
- :material-code-braces: **Templates** — `{{steps.X.output}}` and `{{inputs.Y}}` placeholders
- :material-folder-open: **Audit Trail** — Complete logs under `.workflow-runs/`

</div>

---

## :compass: Where to Start

| If You Want To... | Go Here |
|-------------------|---------|
| Install goflow and run your first command | [:material-download: Installation](getting-started/installation.md) |
| See goflow work in 2 minutes | [:material-rocket-launch: Quick Start](getting-started/quickstart.md) |
| Build your first workflow step-by-step | [:material-book-open-page-variant: Your First Workflow](getting-started/first-workflow.md) |
| Learn features progressively | [:material-school: Tutorial](tutorial/index.md) |
| Look up specific YAML fields | [:material-file-document: Reference](reference/workflow-schema.md) |
| Copy working workflow patterns | [:material-code-tags: Examples](examples/index.md) |

---

## :link: Links

- [:material-github: GitHub Repository](https://github.com/alxayo/goflow)
- [:material-folder-multiple: Example Workflows](https://github.com/alxayo/goflow/tree/main/examples)

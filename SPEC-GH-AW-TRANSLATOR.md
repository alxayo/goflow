# gh-aw-translate — Specification

**Project:** A standalone CLI application that translates GitHub Agentic Workflow (gh-aw) `.md` files into goflow `.yaml` workflow definitions.

**Status:** Design phase.

**Repository:** Standalone Go application. Not part of the goflow binary.

---

## 1. Problem Statement

GitHub Agentic Workflows (gh-aw) define AI-powered repository automation using Markdown files with YAML frontmatter. These workflows run exclusively inside GitHub Actions with a single AI agent per workflow. Teams that want to:

- Run gh-aw-style workflows locally or outside GitHub Actions
- Combine multiple gh-aw workflows into a single multi-agent DAG
- Add explicit fan-out/fan-in parallelism and cross-step output piping

...need a way to convert gh-aw `.md` files into goflow `.yaml` files that the `goflow` CLI can execute.

### Design Constraints

These three constraints simplify the translation and are non-negotiable:

1. **Triggers are skipped.** gh-aw's `on:` event triggers (schedule, issue opened, slash commands, etc.) have no equivalent in goflow. The translated workflow is executed manually via `goflow run`. The trigger configuration is preserved as a comment in the output for documentation.

2. **Security harness is skipped.** gh-aw's AWF firewall, threat detection pipeline, permission isolation, content sanitization, rate limiting, and integrity filtering are not translated. The user reviews the workflow before executing it.

3. **Interactive mode replaces human gates.** Where gh-aw chains use event-driven human checkpoints (e.g., a human types `/plan` between research and planning phases), the translated goflow workflow uses `interactive: true` on the corresponding step so the user can review and confirm before the next agent runs.

---

## 2. Architecture Overview

### 2.1 Application Structure

```
gh-aw-translate/
├── cmd/
│   └── gh-aw-translate/
│       └── main.go                 # CLI entry point
├── pkg/
│   ├── parser/
│   │   ├── frontmatter.go          # Split .md into YAML frontmatter + Markdown body
│   │   ├── frontmatter_test.go
│   │   ├── ghaw.go                 # Parse gh-aw frontmatter into GHAWWorkflow struct
│   │   └── ghaw_test.go
│   ├── expression/
│   │   ├── scanner.go              # Find all ${{ ... }} expressions in text
│   │   ├── rewriter.go             # Replace ${{ ... }} with {{inputs.*}} / {{steps.*}}
│   │   └── scanner_test.go
│   ├── mapper/
│   │   ├── tools.go                # Map gh-aw tools → goflow agent tools
│   │   ├── tools_test.go
│   │   ├── safeoutputs.go          # Convert safe-outputs → prompt instructions
│   │   ├── safeoutputs_test.go
│   │   ├── engine.go               # Map engine config → goflow config
│   │   ├── engine_test.go
│   │   ├── mcp.go                  # Map mcp-servers → agent MCP config
│   │   └── mcp_test.go
│   ├── chain/
│   │   ├── detector.go             # Detect cross-workflow references
│   │   ├── detector_test.go
│   │   ├── merger.go               # Merge connected workflows into a DAG
│   │   └── merger_test.go
│   ├── emitter/
│   │   ├── workflow.go             # Emit goflow workflow YAML
│   │   ├── workflow_test.go
│   │   ├── agent.go                # Emit .agent.md files
│   │   └── agent_test.go
│   └── translator/
│       ├── translator.go           # Top-level orchestration: parse → map → emit
│       └── translator_test.go
├── testdata/
│   ├── input/                      # Sample gh-aw .md files
│   └── golden/                     # Expected goflow .yaml output
├── go.mod
├── go.sum
└── README.md
```

### 2.2 Data Flow

```
                    ┌──────────────┐
                    │  .md file(s) │  (gh-aw format)
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   parser/    │  Split frontmatter + body
                    │              │  Parse YAML into GHAWWorkflow
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  expression/ │  Scan ${{ ... }} expressions
                    │              │  Build inputs map
                    │              │  Rewrite to {{inputs.*}}
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   mapper/    │  Map tools, safe-outputs, engine, MCP
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   chain/     │  (multi-file mode only)
                    │              │  Detect dispatch-workflow / call-workflow
                    │              │  Detect workflow_run references
                    │              │  Merge into single DAG
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  emitter/    │  Generate goflow .yaml
                    │              │  Generate .agent.md files
                    │              │  Generate TRANSLATION_NOTES.md
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │   Output     │  .yaml + agents/ + notes
                    └──────────────┘
```

---

## 3. Input Format — gh-aw Workflow Structure

A gh-aw workflow is a Markdown file with YAML frontmatter:

```markdown
---
name: Issue Triage Bot
on:
  issues:
    types: [opened, reopened]
permissions:
  contents: read
  issues: read
engine:
  id: copilot
  model: gpt-5
  max-turns: 10
timeout-minutes: 10
tools:
  github:
    toolsets: [default, discussions]
  edit:
  web-fetch:
  bash: ["echo", "ls"]
safe-outputs:
  add-labels:
    allowed: [bug, enhancement, question, documentation]
  add-comment:
    max: 3
mcp-servers:
  custom-tool:
    command: node
    args: [./mcp-server.js]
---

# Issue Triage Bot

Analyze issue #${{ github.event.issue.number }} in repository ${{ github.repository }}.

## Your Tasks

1. Read the issue content using the GitHub tools
2. Categorize the issue type (bug, enhancement, question, documentation)
3. Add the appropriate label
4. Post a helpful triage comment with next steps

## Guidelines

- Be concise and helpful
- If unsure about categorization, default to "question"
- Always cite the specific part of the issue that informed your decision
```

### 3.1 GHAWWorkflow Internal Representation

```go
// GHAWWorkflow is the parsed representation of a gh-aw .md file.
type GHAWWorkflow struct {
    // Metadata
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Source      string `yaml:"source"`
    Labels      []string `yaml:"labels"`

    // Triggers (preserved for documentation, not translated)
    On interface{} `yaml:"on"`

    // Permissions (preserved for documentation, not translated)
    Permissions map[string]string `yaml:"permissions"`

    // Engine configuration
    Engine EngineConfig `yaml:"engine"`

    // Execution
    TimeoutMinutes int    `yaml:"timeout-minutes"`
    Concurrency    interface{} `yaml:"concurrency"`
    RunsOn         interface{} `yaml:"runs-on"`
    If             string `yaml:"if"`

    // Tools
    Tools ToolsConfig `yaml:"tools"`

    // MCP servers
    MCPServers map[string]MCPServerDef `yaml:"mcp-servers"`

    // MCP scripts (inline tools — best-effort)
    MCPScripts map[string]MCPScriptDef `yaml:"mcp-scripts"`

    // Safe outputs
    SafeOutputs map[string]interface{} `yaml:"safe-outputs"`

    // Imports
    Imports []string `yaml:"imports"`

    // Network (preserved for documentation)
    Network interface{} `yaml:"network"`

    // Secrets
    Secrets map[string]interface{} `yaml:"secrets"`

    // Custom steps (outside sandbox in gh-aw)
    Steps     interface{} `yaml:"steps"`
    PostSteps interface{} `yaml:"post-steps"`

    // Environment
    Env map[string]string `yaml:"env"`

    // Features
    Features map[string]interface{} `yaml:"features"`

    // Checkout
    Checkout interface{} `yaml:"checkout"`

    // Runtimes
    Runtimes map[string]interface{} `yaml:"runtimes"`

    // Cache
    Cache interface{} `yaml:"cache"`

    // Markdown body (the agent's instructions)
    Body string `yaml:"-"`

    // Source file path
    SourcePath string `yaml:"-"`
}

type EngineConfig struct {
    // String shorthand: "copilot", "claude", "codex", "gemini"
    // Or object with fields:
    ID             string            `yaml:"id"`
    Version        string            `yaml:"version"`
    Model          string            `yaml:"model"`
    Agent          string            `yaml:"agent"`
    MaxTurns       int               `yaml:"max-turns"`
    MaxConcurrency int               `yaml:"max-concurrency"`
    Env            map[string]string `yaml:"env"`
    Args           []string          `yaml:"args"`
}

type ToolsConfig struct {
    GitHub         *GitHubToolConfig `yaml:"github"`
    Edit           interface{}       `yaml:"edit"`
    WebFetch       interface{}       `yaml:"web-fetch"`
    WebSearch      interface{}       `yaml:"web-search"`
    Bash           interface{}       `yaml:"bash"`
    Playwright     interface{}       `yaml:"playwright"`
    CacheMemory    interface{}       `yaml:"cache-memory"`
    RepoMemory     interface{}       `yaml:"repo-memory"`
    AgenticWorkflows interface{}     `yaml:"agentic-workflows"`
}

type GitHubToolConfig struct {
    Toolsets    []string `yaml:"toolsets"`
    Allowed     []string `yaml:"allowed"`
    Mode        string   `yaml:"mode"`
    GitHubToken string   `yaml:"github-token"`
    Lockdown    bool     `yaml:"lockdown"`
}

type MCPServerDef struct {
    Command string            `yaml:"command"`
    Args    []string          `yaml:"args"`
    Env     map[string]string `yaml:"env"`
    Type    string            `yaml:"type"`    // "http" for HTTP MCP servers
    URL     string            `yaml:"url"`     // For HTTP MCP servers
    Headers map[string]string `yaml:"headers"` // For HTTP MCP servers
    Allowed []string          `yaml:"allowed"` // Tool allowlist
}

type MCPScriptDef struct {
    Description string                 `yaml:"description"`
    Inputs      map[string]interface{} `yaml:"inputs"`
    Script      string                 `yaml:"script"`  // JavaScript
    Run         string                 `yaml:"run"`     // Shell
    Py          string                 `yaml:"py"`      // Python
    Go          string                 `yaml:"go"`      // Go
    Env         map[string]string      `yaml:"env"`
    Timeout     int                    `yaml:"timeout"`
}
```

---

## 4. Output Format — goflow Workflow + Agent Files

### 4.1 goflow Workflow YAML

The emitter produces a goflow `.yaml` file conforming to the `workflow.Workflow` struct:

```yaml
# Translated from: .github/workflows/issue-triage.md
# Original triggers (not translated — run manually via goflow):
#   on:
#     issues:
#       types: [opened, reopened]

name: "issue-triage-bot"
description: "Issue Triage Bot"

inputs:
  issue_number:
    description: "Issue number (from ${{ github.event.issue.number }})"
  repository:
    description: "Repository in owner/repo format (from ${{ github.repository }})"
  trigger_content:
    description: "Sanitized event content (from ${{ steps.sanitized.outputs.text }})"

config:
  model: "gpt-5"
  interactive: true

agents:
  triage-agent:
    file: "./agents/triage-agent.agent.md"

steps:
  - id: triage
    agent: triage-agent
    prompt: |
      # Issue Triage Bot

      Analyze issue #{{inputs.issue_number}} in repository {{inputs.repository}}.

      ## Your Tasks

      1. Read the issue content using the GitHub tools
      2. Categorize the issue type (bug, enhancement, question, documentation)
      3. Add the appropriate label
      4. Post a helpful triage comment with next steps

      ## Guidelines

      - Be concise and helpful
      - If unsure about categorization, default to "question"
      - Always cite the specific part of the issue that informed your decision

      ## Output Instructions (from safe-outputs)

      When adding labels, use only these labels: bug, enhancement, question, documentation.
      When posting comments, create at most 3 comments.
    timeout: "10m"
    interactive: true

output:
  steps: [triage]
  format: "markdown"
```

### 4.2 Agent .agent.md Files

For each translated workflow, the emitter generates a `.agent.md` file containing only the agent's name, description, tools list, and system prompt. **MCP server configuration is NOT included in agent files** — goflow's executor does not read `mcp-servers` from agent files. MCP servers are configured via `.copilot/mcp.json` in the workspace (see §5.5.1 and §5.6).

```markdown
---
name: triage-agent
description: "Agent for issue-triage-bot workflow (translated from gh-aw)"
tools:
  - github
  - edit
  - web-fetch
  - bash
---

You are an AI agent translated from a GitHub Agentic Workflow.
Your primary tools include the GitHub MCP server for reading and writing
GitHub resources (issues, PRs, discussions, labels, comments).
```

### 4.3 Translation Notes

Every translation produces a `TRANSLATION_NOTES.md` documenting:

- Original file path
- Skipped features with rationale
- Warnings for unsupported elements
- Manual steps required (e.g., setting up MCP servers)

---

## 5. Translation Rules — Detailed Specification

### 5.1 Metadata Mapping

| gh-aw field | goflow field | Rule |
|---|---|---|
| `name:` | `name:` | Direct copy. If absent, derive from filename (e.g., `issue-triage.md` → `"issue-triage"`) |
| `description:` | `description:` | Direct copy. If absent, use the first `# Heading` from the Markdown body |
| `labels:` | — | Skipped. Documented in translation notes |
| `source:` | — | Preserved as YAML comment in output |
| `metadata:` | — | Skipped. Documented in translation notes |

### 5.2 Engine Mapping

| gh-aw engine | goflow config | Rule |
|---|---|---|
| `engine: copilot` (or absent) | No special config needed | goflow defaults to Copilot |
| `engine.model: "gpt-5"` | `config.model: "gpt-5"` | Direct mapping |
| `engine.id: copilot` | No special config | Default |
| `engine.id: claude` | `config.provider.type: "claude"` | Emit warning: BYOK provider setup required |
| `engine.id: codex` | — | Emit error: unsupported engine |
| `engine.id: gemini` | — | Emit error: unsupported engine |
| `engine.max-turns:` | — | Preserved as comment. goflow does not limit turns |
| `engine.agent: "my-agent"` | `agents.my-agent.file: ".github/agents/my-agent.agent.md"` | Reference the custom agent file |
| `engine.max-concurrency:` | `config.max_concurrency:` | Direct mapping |

### 5.3 Timeout Mapping

| gh-aw | goflow | Rule |
|---|---|---|
| `timeout-minutes: 10` | `steps[N].timeout: "10m"` | Convert integer minutes to Go duration string. Applied to each step |

### 5.4 Expression Rewriting

The expression scanner finds all `${{ ... }}` patterns in the Markdown body and frontmatter string values. Each unique expression is mapped to a goflow input variable.

**Deterministic Mapping Table:**

| gh-aw expression | goflow variable | Input description |
|---|---|---|
| `${{ github.event.issue.number }}` | `{{inputs.issue_number}}` | Issue number |
| `${{ github.event.issue.title }}` | `{{inputs.issue_title}}` | Issue title |
| `${{ github.event.issue.state }}` | `{{inputs.issue_state}}` | Issue state (open/closed) |
| `${{ github.event.pull_request.number }}` | `{{inputs.pr_number}}` | Pull request number |
| `${{ github.event.pull_request.title }}` | `{{inputs.pr_title}}` | Pull request title |
| `${{ github.event.pull_request.state }}` | `{{inputs.pr_state}}` | Pull request state |
| `${{ github.event.pull_request.head.sha }}` | `{{inputs.pr_head_sha}}` | PR head commit SHA |
| `${{ github.event.pull_request.base.sha }}` | `{{inputs.pr_base_sha}}` | PR base commit SHA |
| `${{ github.event.comment.id }}` | `{{inputs.comment_id}}` | Comment ID |
| `${{ github.event.discussion.number }}` | `{{inputs.discussion_number}}` | Discussion number |
| `${{ github.event.discussion.title }}` | `{{inputs.discussion_title}}` | Discussion title |
| `${{ github.event.release.tag_name }}` | `{{inputs.release_tag}}` | Release tag name |
| `${{ github.event.release.name }}` | `{{inputs.release_name}}` | Release name |
| `${{ github.repository }}` | `{{inputs.repository}}` | Repository (owner/repo) |
| `${{ github.repository_owner }}` | `{{inputs.repository_owner}}` | Repository owner |
| `${{ github.actor }}` | `{{inputs.actor}}` | Triggering user |
| `${{ github.event_name }}` | `{{inputs.event_name}}` | Event name |
| `${{ github.run_id }}` | `{{inputs.run_id}}` | Workflow run ID |
| `${{ github.run_number }}` | `{{inputs.run_number}}` | Workflow run number |
| `${{ github.server_url }}` | `{{inputs.server_url}}` | GitHub server URL |
| `${{ github.workspace }}` | `{{inputs.workspace}}` | Workspace path |
| `${{ github.workflow }}` | `{{inputs.workflow_name}}` | Workflow name |
| `${{ steps.sanitized.outputs.text }}` | `{{inputs.trigger_content}}` | Sanitized event content |
| `${{ github.event.inputs.X }}` | `{{inputs.X}}` | Workflow dispatch input (name preserved) |
| `${{ needs.*.outputs.* }}` | — | Cannot translate. Emit warning |
| `${{ steps.*.outputs.* }}` (except sanitized) | — | Cannot translate. Emit warning |

**Fallback rule:** Any `${{ ... }}` expression not in the mapping table is replaced with `{{inputs.unknown_expr_N}}` (with N being a sequential counter) and a warning is emitted in `TRANSLATION_NOTES.md`.

**Input generation:** For each unique expression found, the translator generates an entry in the output `inputs:` map:

```yaml
inputs:
  issue_number:
    description: "Issue number (originally ${{ github.event.issue.number }})"
  repository:
    description: "Repository in owner/repo format (originally ${{ github.repository }})"
```

If the gh-aw workflow has `workflow_dispatch.inputs`, those definitions are merged into the goflow `inputs:` map, preserving the original `description`, `default`, and `type` as a comment.

### 5.5 Tool Mapping

The translator builds the agent's `tools:` list from the gh-aw `tools:` configuration.

| gh-aw tool | Agent tool entry | Notes |
|---|---|---|
| `tools.github:` (any config) | Add `github` to tools list. Emit GitHub MCP server config if toolsets beyond default are needed | See §5.5.1 |
| `tools.edit:` | `edit` | Direct |
| `tools.web-fetch:` | `web-fetch` | Direct |
| `tools.web-search:` | `web-search` | Direct |
| `tools.bash:` (boolean) | `bash` | Direct |
| `tools.bash:` (string array) | `bash` | Allowlist noted in translation notes (not enforced by goflow) |
| `tools.playwright:` | `playwright` | Direct; emit warning about runtime availability |
| `tools.cache-memory:` | — | Skip. Emit warning: cross-run persistence not supported |
| `tools.repo-memory:` | — | Skip. Emit warning: cross-run persistence not supported |
| `tools.agentic-workflows:` | — | Skip. gh-aw introspection only |

#### 5.5.1 GitHub MCP Server Configuration

When `tools.github:` is present, the translator documents the GitHub MCP server requirement in `TRANSLATION_NOTES.md` and emits a `.copilot/mcp.json` workspace config file in the output directory:

```json
{
  "servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

**Important:** MCP servers are NOT placed in agent `.agent.md` files. goflow's executor does not read `mcp-servers` from agent files into the session config. MCP servers are discovered by the Copilot CLI via workspace configuration (`.copilot/mcp.json`) or via the step's `extra_dirs` field pointing to a directory containing the appropriate config.

If `tools.github.toolsets` specifies non-default toolsets, a comment is emitted documenting which toolsets were requested.

### 5.6 MCP Server Mapping

gh-aw `mcp-servers:` entries are emitted into the `.copilot/mcp.json` workspace config alongside the GitHub MCP server. They are NOT placed in agent files.

| gh-aw MCP type | Output location | Rule |
|---|---|---|
| Stdio server (`command:` + `args:`) | `.copilot/mcp.json` `servers` entry | Direct mapping |
| HTTP server (`type: http`, `url:`) | — | Emit warning: HTTP MCP servers require Copilot CLI support; document URL |
| Servers with `allowed:` filter | `.copilot/mcp.json` + note `allowed:` in translation notes | goflow does not enforce tool allowlists |
| Servers with `env:` using secrets | `.copilot/mcp.json`, replace `${{ secrets.X }}` with `${X}` env var reference | User must set env vars |

### 5.7 MCP Scripts Translation

`mcp-scripts:` define inline tools (JavaScript, shell, Python, Go). goflow has no inline script concept.

**Translation strategy:**

1. For each script, emit a warning in `TRANSLATION_NOTES.md`:
   ```
   WARNING: mcp-script "search-issues" uses inline JavaScript.
   This cannot be directly translated. Options:
   1. Create a standalone MCP server that implements this tool
   2. Instruct the agent to achieve the same result using existing tools
   ```

2. Extract the script's `description` and `inputs` schema and append to the prompt as guidance:
   ```
   ## Available Capabilities (from mcp-scripts, manual setup required)
   - search-issues: Search GitHub issues using API
     Inputs: query (string, required), limit (number, default: 10)
   ```

### 5.8 Safe-Outputs → Prompt Instructions

Each `safe-outputs` type is converted into structured prompt instructions appended to the step's prompt. The agent is expected to use the GitHub MCP server to perform these operations directly.

**Template per safe-output type:**

```go
var safeOutputTemplates = map[string]string{
    "create-issue": `When creating issues:
- Use the GitHub MCP server create_issue tool
{{- if .TitlePrefix }}
- Prefix all issue titles with "{{ .TitlePrefix }}"
{{- end }}
{{- if .Labels }}
- Add labels: {{ join .Labels ", " }}
{{- end }}
{{- if .Assignees }}
- Assign to: {{ join .Assignees ", " }}
{{- end }}
{{- if .Max }}
- Create at most {{ .Max }} issue(s)
{{- end }}
{{- if .CloseOlderIssues }}
- Close older issues from this workflow before creating new ones
{{- end }}`,

    "add-comment": `When posting comments:
- Use the GitHub MCP server add_comment tool
{{- if .Max }}
- Post at most {{ .Max }} comment(s)
{{- end }}
{{- if .HideOlderComments }}
- Minimize previous comments from this workflow before posting
{{- end }}`,

    "add-labels": `When adding labels:
- Use the GitHub MCP server add_labels tool
{{- if .Allowed }}
- Only use these labels: {{ join .Allowed ", " }}
{{- end }}
{{- if .Max }}
- Add at most {{ .Max }} label(s)
{{- end }}`,

    "create-pull-request": `When creating pull requests:
- Use the GitHub MCP server to create a PR
{{- if .TitlePrefix }}
- Prefix PR titles with "{{ .TitlePrefix }}"
{{- end }}
{{- if .Labels }}
- Add labels: {{ join .Labels ", " }}
{{- end }}
{{- if .Draft }}
- Create as draft PR
{{- end }}`,

    "create-discussion": `When creating discussions:
- Use the GitHub MCP server create_discussion tool
{{- if .TitlePrefix }}
- Prefix titles with "{{ .TitlePrefix }}"
{{- end }}
{{- if .Category }}
- Use category: {{ .Category }}
{{- end }}`,

    "close-issue": `When closing issues:
- Use the GitHub MCP server to close the issue with a comment explaining why`,

    "update-issue": `When updating issues:
- Use the GitHub MCP server to update issue fields`,

    "dispatch-workflow": `When dispatching workflows:
- NOTE: Workflow dispatch is handled via goflow DAG steps, not runtime dispatch`,

    "call-workflow": `When calling workflows:
- NOTE: Workflow calls are handled via goflow DAG steps, not runtime calls`,
}
```

Safe-output types not in the template map get a generic instruction:
```
When performing {{ .Type }} operations, use the GitHub MCP server.
Constraints from original workflow: {{ toYAML .Config }}
```

### 5.9 Import Resolution

gh-aw `imports:` load shared `.md` files that contribute tools, safe-outputs, and/or text content.

| Import type | Translation rule |
|---|---|
| Local `.md` file with frontmatter (tools/safe-outputs) | Parse the imported file, merge its tools into the agent's tools list, merge safe-output instructions into the prompt |
| Local `.md` file with text body only | Inline the text content into the prompt |
| `copilot-setup-steps.yml` | Skip. Emit note: custom setup steps are not supported |
| Remote imports (`owner/repo/path@ref`) | Emit warning: cannot resolve remote imports. Document the reference |

### 5.10 Environment Variables

| gh-aw field | goflow | Rule |
|---|---|---|
| `env:` (workflow-level) | — | Emit as comment. User sets env vars before running goflow |
| `secrets:` | — | Emit as comment. Replace `${{ secrets.X }}` with `${X}` and document required env vars |

### 5.11 Interactive Mode

The translator sets `interactive: true` at the workflow config level and optionally on individual steps.

**Rules:**
- If the gh-aw workflow uses a `slash_command:` trigger (implying human-initiated), set `config.interactive: false` (already human-initiated)
- If translating a multi-workflow chain where the original had human gates between phases, set `interactive: true` on the step(s) immediately following an inferred "human decision point"
- Default: `config.interactive: true` (conservative — let the user confirm before agent executes)

---

## 6. Multi-Workflow Chain Detection and Merging

### 6.1 Chain Detection

When the translator receives a directory of `.md` files, it scans for cross-workflow references:

**Reference types detected:**

| Mechanism | Detection rule |
|---|---|
| `safe-outputs.call-workflow.workflows: [A, B]` | Workflow names A, B become downstream steps of the current workflow |
| `safe-outputs.dispatch-workflow.workflows: [C, D]` | Workflow names C, D become downstream steps (with warning: semantics change from async to sync) |
| `on.workflow_run.workflows: ["X"]` | Current workflow becomes a downstream step of X |
| Event-driven chaining (A creates issue → B triggers on `issues: labeled`) | Heuristic: if A has `safe-outputs.create-issue` or `safe-outputs.add-labels` AND B triggers on `issues: types: [labeled]`, suggest merging. Emit as advisory, not automatic |

### 6.2 DAG Construction

Detected chains are merged into a single goflow workflow:

```
Input files:
  orchestrator.md    (has call-workflow: [worker-a, worker-b])
  worker-a.md        (has on: workflow_call)
  worker-b.md        (has on: workflow_call)

Output:
  orchestrator-pipeline.yaml
    steps:
      - id: orchestrator
        prompt: (orchestrator.md body)
        
      - id: worker-a
        prompt: (worker-a.md body)
        depends_on: [orchestrator]
        
      - id: worker-b
        prompt: (worker-b.md body)  
        depends_on: [orchestrator]
```

**DAG rules:**
- `call-workflow` targets → `depends_on` the calling step (synchronous)
- `dispatch-workflow` targets → `depends_on` the dispatching step (changed from async to sync; documented in notes)
- `workflow_run` references → `depends_on` the named workflow's step
- Circular references → error at translation time

**Agent assignment:** Each merged step gets a unique agent. If the original worker had `engine.agent`, use that agent file. Otherwise, generate a new inline agent from the worker's tools and body.

**Output piping:** When merging chains, the translator:
1. Does NOT automatically add `{{steps.X.output}}` references (there was no output piping in gh-aw)
2. Instead, adds a comment: `# This step can access {{steps.orchestrator.output}} if needed`
3. The user can manually wire up cross-step references after translation

### 6.3 Standalone vs Connected Workflows

When given a directory, the translator identifies connected components:
- Workflows that reference each other → merged into one goflow `.yaml`
- Standalone workflows (no cross-references) → each produces its own `.yaml`

---

## 7. CLI Interface

### 7.1 Commands

```bash
# Translate a single workflow
gh-aw-translate --input .github/workflows/my-workflow.md \
                --output ./translated/

# Translate a directory (auto-detect chains)
gh-aw-translate --input .github/workflows/ \
                --output ./translated/

# Translate with specific options
gh-aw-translate --input .github/workflows/my-workflow.md \
                --output ./translated/ \
                --interactive=false \
                --github-mcp-command "npx -y @modelcontextprotocol/server-github"

# Dry-run: show what would be generated without writing files
gh-aw-translate --input .github/workflows/ \
                --output ./translated/ \
                --dry-run

# Verbose: show all translation decisions and warnings
gh-aw-translate --input .github/workflows/ \
                --output ./translated/ \
                --verbose
```

### 7.2 Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--input`, `-i` | string | (required) | Path to a gh-aw `.md` file or directory of `.md` files |
| `--output`, `-o` | string | `./translated/` | Output directory for generated files |
| `--interactive` | bool | `true` | Set `interactive` on translated workflow steps |
| `--github-mcp-command` | string | `npx -y @modelcontextprotocol/server-github` | Command for the GitHub MCP server |
| `--dry-run` | bool | `false` | Print output without writing files |
| `--verbose` | bool | `false` | Show detailed translation decisions |
| `--merge-chains` | bool | `true` | Auto-detect and merge connected workflows into DAGs |
| `--model` | string | `""` | Override model for all translated workflows |
| `--skip-imports` | bool | `false` | Do not resolve `imports:` references |

### 7.3 Exit Codes

| Code | Meaning |
|---|---|
| 0 | Translation completed successfully (may include warnings) |
| 1 | Translation failed (parse error, unsupported engine, circular chain) |
| 2 | Invalid arguments |

---

## 8. Translation Notes Output

Every translation run produces a `TRANSLATION_NOTES.md` in the output directory:

```markdown
# Translation Notes

Generated by gh-aw-translate on 2026-03-31

## Files Translated

| Source | Output | Status |
|---|---|---|
| .github/workflows/issue-triage.md | translated/issue-triage.yaml | OK (3 warnings) |
| .github/workflows/plan.md | translated/plan.yaml | OK (1 warning) |

## Warnings

### issue-triage.md

1. **Trigger skipped**: `on: issues: types: [opened, reopened]` — run manually via `goflow run`
2. **Safe-output constraints are advisory**: `add-labels.allowed: [bug, enhancement, ...]` is included
   as a prompt instruction but NOT enforced at the platform level. The agent may deviate.
3. **Bash allowlist not enforced**: Original workflow restricted bash to `["echo", "ls"]`. 
   The translated agent has unrestricted bash access.

### plan.md

1. **Trigger skipped**: `on: slash_command: name: plan`

## Manual Steps Required

1. **Set GITHUB_TOKEN**: Export `GITHUB_TOKEN` with appropriate scopes before running goflow
2. **Install MCP server**: `npm install -g @modelcontextprotocol/server-github` (or let npx handle it)

## Unsupported Features (Skipped)

- `permissions:` — gh-aw permission isolation not applicable
- `network:` — gh-aw firewall not applicable
- `tools.cache-memory:` — Cross-run persistence not supported in goflow
- `safe-outputs.threat-detection:` — Not applicable outside gh-aw
- `rate-limit:` — Not applicable
- `features:` — gh-aw internal feature flags
```

---

## 9. Test Strategy

### 9.1 Unit Tests (per package)

| Package | Test focus |
|---|---|
| `parser/` | Frontmatter splitting edge cases: missing `---`, empty body, malformed YAML, BOM characters |
| `expression/` | All 30+ expression patterns from §5.4. Unknown expressions. Nested expressions. Expressions in code blocks (should not rewrite) |
| `mapper/tools` | All tool types from §5.5. Combined tools. Empty tools block |
| `mapper/safeoutputs` | All safe-output types from §5.8. Missing fields. Unknown types |
| `mapper/engine` | All engine variants from §5.2. String shorthand vs object |
| `mapper/mcp` | Stdio, HTTP, with secrets, with allowed list |
| `chain/` | Single workflow (no chain). Linear chain A→B→C. Fan-out A→{B,C}. Diamond A→{B,C}→D. Circular (error). Disconnected components |
| `emitter/` | Valid YAML output. Multi-line prompts. Special characters in prompts |

### 9.2 Golden File Tests

The `testdata/` directory contains pairs:
- `testdata/input/issue-triage.md` → `testdata/golden/issue-triage.yaml`
- `testdata/input/chain/orchestrator.md` + `testdata/input/chain/worker-a.md` → `testdata/golden/chain/orchestrator-pipeline.yaml`

Each test reads the input, runs the translator, and compares byte-for-byte against the golden file.

### 9.3 Integration Test

A single integration test runs the full translator against the sample gh-aw files from the gh-aw repository reference documentation and validates:
1. Output parses as valid goflow YAML (unmarshal into `workflow.Workflow`)
2. All agents parse as valid `.agent.md` (unmarshal into `agents.Agent`)
3. No errors (only warnings) for Copilot-engine workflows
4. `TRANSLATION_NOTES.md` is generated

---

## 10. Implementation Roadmap

### Phase 1 — Core Single-File Translation

| Task | ID | Description | Depends On |
|---|---|---|---|
| Project scaffold | T01 | `go mod init`, directory structure, CI setup | — |
| Frontmatter parser | T02 | Split `.md` into YAML + body | T01 |
| GHAWWorkflow types | T03 | Define all parsed types from §3.1 | T01 |
| Expression scanner | T04 | Find all `${{ ... }}` in text | T01 |
| Expression rewriter | T05 | Replace with `{{inputs.*}}` using mapping table from §5.4 | T04 |
| Input generator | T06 | Build `inputs:` map from discovered expressions | T05 |
| Tool mapper | T07 | Convert `tools:` to agent tools list per §5.5 | T03 |
| Engine mapper | T08 | Convert `engine:` to goflow config per §5.2 | T03 |
| MCP mapper | T09 | Convert `mcp-servers:` to `.copilot/mcp.json` per §5.6 | T03 |
| Safe-output mapper | T10 | Convert each safe-output type to prompt instructions per §5.8 | T03 |
| Workflow emitter | T11 | Generate goflow `.yaml` file | T05, T07, T08, T10 |
| Agent emitter | T12 | Generate `.agent.md` file | T07, T09 |
| Notes emitter | T13 | Generate `TRANSLATION_NOTES.md` | T11, T12 |
| CLI entry point | T14 | Parse flags, call translator, write output | T11, T12, T13 |
| Golden tests (single file) | T15 | 5+ test cases covering core patterns | T14 |

### Phase 2 — Multi-File Chain Merging

| Task | ID | Description | Depends On |
|---|---|---|---|
| Chain detector | T16 | Find cross-workflow references per §6.1 | T03 |
| DAG builder | T17 | Construct step dependency graph from detected chains per §6.2 | T16 |
| Chain merger | T18 | Produce unified goflow workflow from connected workflows | T17, T11, T12 |
| Directory mode CLI | T19 | Walk directory, detect chains, translate all files | T18, T14 |
| Golden tests (chains) | T20 | 3+ test cases: linear, fan-out, disconnected | T19 |

### Phase 3 — Polish

| Task | ID | Description | Depends On |
|---|---|---|---|
| Import resolution | T21 | Parse and merge imported `.md` files per §5.9 | T02, T07, T10 |
| MCP scripts best-effort | T22 | Extract descriptions and emit guidance per §5.7 | T03 |
| Dry-run mode | T23 | Print without writing | T14 |
| Verbose mode | T24 | Detailed translation trace | T14 |
| README documentation | T25 | Installation, usage, examples | T19 |
| Integration test | T26 | Full translator against reference gh-aw samples | T19 |

### Dependency Graph (Critical Path)

```
T01 → T02 → T03 ─┬─ T07 ─┬─ T11 ─┬─ T14 ─── T15
                  ├─ T08 ─┤       │
                  ├─ T09 ─┤       ├─ T12
                  ├─ T10 ─┘       │
                  │               └─ T13
 T01 → T04 → T05 → T06 ──────────┘

T03 → T16 → T17 → T18 → T19 → T20

T02 → T21
T03 → T22
T14 → T23, T24
T19 → T25, T26
```

**Phase 1 critical path:** T01 → T02 → T03 → T10 → T11 → T14 → T15 (7 serial tasks)

**Parallelizable after T03:** T07, T08, T09, T10 can all proceed independently.

**Parallelizable after T01:** T04 → T05 → T06 is independent of the T02 → T03 path.

---

## 11. External Dependencies

| Dependency | Purpose | Version |
|---|---|---|
| `gopkg.in/yaml.v3` | Parse YAML frontmatter and emit goflow YAML | Latest stable |
| Standard library only | Regex, file I/O, templates, testing | Go 1.21+ |

No other external dependencies. The translator is a pure file-in → file-out tool.

---

## 12. Non-Goals

These are explicitly out of scope:

1. **Runtime execution.** The translator only produces files. It does not run goflow.
2. **Bidirectional translation.** goflow → gh-aw is not supported.
3. **Security enforcement.** Safe-output constraints (max counts, label allowlists) are prompt instructions, not runtime enforcement.
4. **Trigger emulation.** No webhook server, no cron driver, no event listener.
5. **Remote import resolution.** Imports using `owner/repo/path@ref` format are documented but not fetched.
6. **Semantic validation of prompts.** The translator does not assess whether the translated prompt will produce correct agent behavior.
7. **goflow runtime changes.** The translator targets existing goflow YAML schema. No changes to goflow are proposed.

---

## 13. Example End-to-End Translation

### Input: `.github/workflows/daily-status.md`

```markdown
---
name: Daily Status Report
on:
  schedule: daily on weekdays
permissions:
  contents: read
  issues: read
  pull-requests: read
engine: copilot
timeout-minutes: 15
tools:
  github:
    toolsets: [default]
  web-search:
safe-outputs:
  create-issue:
    title-prefix: "[team-status] "
    labels: [report, daily-status]
    close-older-issues: true
    max: 1
---

# Daily Status Report

Create an upbeat daily status report for the team as a GitHub issue.

## What to include

- Recent repository activity for ${{ github.repository }}
  - Issues opened/closed in the last 24 hours
  - PRs merged
  - Notable commits to main branch
- Progress tracking and highlights
- Actionable next steps for maintainers

## Style

Keep the tone positive and motivating. Use emoji sparingly but effectively.
Focus on what was accomplished, not just what's pending.
```

### Output: `translated/daily-status.yaml`

```yaml
# Translated from: .github/workflows/daily-status.md
# Original triggers (not translated — run manually via goflow):
#   on:
#     schedule: daily on weekdays
# Original permissions: contents: read, issues: read, pull-requests: read

name: "daily-status-report"
description: "Daily Status Report"

inputs:
  repository:
    description: "Repository in owner/repo format (originally ${{ github.repository }})"

config:
  interactive: true

agents:
  daily-status-agent:
    file: "./agents/daily-status-agent.agent.md"

steps:
  - id: generate-report
    agent: daily-status-agent
    prompt: |
      # Daily Status Report

      Create an upbeat daily status report for the team as a GitHub issue.

      ## What to include

      - Recent repository activity for {{inputs.repository}}
        - Issues opened/closed in the last 24 hours
        - PRs merged
        - Notable commits to main branch
      - Progress tracking and highlights
      - Actionable next steps for maintainers

      ## Style

      Keep the tone positive and motivating. Use emoji sparingly but effectively.
      Focus on what was accomplished, not just what's pending.

      ## Output Instructions (from safe-outputs)

      When creating the status report issue:
      - Use the GitHub MCP server create_issue tool
      - Prefix the issue title with "[team-status] "
      - Add labels: report, daily-status
      - Close older issues from this workflow before creating new ones
      - Create at most 1 issue
    timeout: "15m"
    interactive: true

output:
  steps: [generate-report]
  format: "markdown"
```

### Output: `translated/agents/daily-status-agent.agent.md`

```markdown
---
name: daily-status-agent
description: "Agent for daily-status-report workflow (translated from gh-aw)"
tools:
  - github
  - web-search
---

You are an AI agent for generating daily status reports.
Use the GitHub MCP server to read repository data and create issues.
```

### Output: `translated/TRANSLATION_NOTES.md`

```markdown
# Translation Notes

Generated by gh-aw-translate on 2026-03-31

## Files Translated

| Source | Output | Status |
|---|---|---|
| .github/workflows/daily-status.md | translated/daily-status.yaml | OK (2 warnings) |

## Warnings

### daily-status.md

1. **Trigger skipped**: `on: schedule: daily on weekdays` — run manually via `goflow run`
2. **Safe-output constraints advisory**: `create-issue.close-older-issues` and `max: 1` are included
   as prompt instructions but not enforced at the platform level.

## Manual Steps Required

1. **Set GITHUB_TOKEN**: `export GITHUB_TOKEN=ghp_...` with `repo` and `issues` scopes
2. **Run**: `goflow run --workflow translated/daily-status.yaml --inputs repository=owner/repo`
```

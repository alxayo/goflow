# Agent, Skill & MCP Server Discovery and Per-Step Scoping

This document provides a comprehensive analysis of how agents, skills, and MCP servers are discovered and configured in the Copilot SDK and CLI, how the workflow runner currently handles these, and what would be required to implement **per-step scoping** — where each workflow step can have its own dedicated folder for agents, skills, and MCP servers.

> **IMPORTANT UPDATE**: Cross-referencing the Copilot SDK documentation with the Copilot CLI changelog reveals that the **CLI performs its own filesystem-based discovery**, and the SDK can either pass explicit configurations OR leverage the CLI's auto-discovery. This is a critical architectural detail.

---

## Table of Contents

1. [Current Discovery Mechanisms](#1-current-discovery-mechanisms)
   - [Copilot CLI: Native Discovery (The Runtime Engine)](#copilot-cli-native-discovery-the-runtime-engine)
   - [CLI Default Resources (Always Available)](#cli-default-resources-always-available)
   - [Tool Restriction via Agent Definition](#tool-restriction-via-agent-definition)
   - [Copilot SDK: Configuration Passing vs CLI Discovery](#copilot-sdk-configuration-passing-vs-cli-discovery)
   - [Workflow Runner: Current Implementation](#workflow-runner-current-implementation)
2. [Per-Step Scoping: Design & Implementation](#2-per-step-scoping-design--implementation)
   - [Concept Overview](#concept-overview)
   - [YAML Schema Changes](#yaml-schema-changes)
   - [Implementation Requirements](#implementation-requirements)
   - [Execution Flow with Per-Step Scoping](#execution-flow-with-per-step-scoping)
   - [Per-Step Parallel Execution Model](#per-step-parallel-execution-model)
3. [Model Usage and Enforcement](#3-model-usage-and-enforcement)
   - [Copilot SDK Model Selection](#copilot-sdk-model-selection)
   - [Current Workflow Runner Model Handling](#current-workflow-runner-model-handling)
   - [Per-Agent Model Enforcement](#per-agent-model-enforcement)
4. [Implementation Summary](#4-implementation-summary)
5. [Appendix: SDK Code Examples](#appendix-sdk-code-examples)

---

## 1. Current Discovery Mechanisms

### Copilot CLI: Native Discovery (The Runtime Engine)

The Copilot CLI is the **underlying runtime engine** that both the SDK and direct CLI usage rely upon. The CLI has **extensive built-in filesystem discovery** that automatically loads agents, skills, MCP servers, and instructions from well-known directory locations.

#### CLI Discovery Paths (from changelog v1.0.11)

> "Custom instructions, MCP servers, skills, and agents are now discovered at every directory level from the working directory up to the git root, enabling full monorepo support"

| Resource Type | Discovery Locations (in priority order) |
|---------------|----------------------------------------|
| **Custom Agents** | `.github/agents/*.agent.md` (repo)<br>`~/.copilot/agents/` (user)<br>Organization `.github` repository |
| **Skills** | `.agents/skills/` (repo, auto-loaded)<br>`~/.agents/skills/` (user, v1.0.11+)<br>`~/.copilot/skills/` (user)<br>Every directory up to git root |
| **MCP Servers** | `.mcp.json` (workspace, Claude-compatible)<br>`.vscode/mcp.json` (workspace)<br>`.devcontainer/devcontainer.json` (container)<br>`~/.copilot/mcp-config.json` (user) |
| **Instructions** | `.github/instructions/*.instructions.md` (repo)<br>`~/.copilot/instructions/*.instructions.md` (user)<br>Every directory up to git root |
| **Hooks** | `.github/hooks/` (repo)<br>`~/.copilot/hooks/` (user) |

#### Key CLI Discovery Features

1. **Hierarchical Discovery** (v1.0.11): Resources are discovered at every directory level from CWD up to git root
2. **Monorepo Support**: Each subdirectory can have its own agents/skills/MCP configs
3. **Plugin System**: Plugins can bundle agents, skills, and MCP servers
4. **Trust Model**: Workspace-level configs only load after folder trust is confirmed

#### Example: CLI Agent Discovery Flow

```
Working Directory: /project/packages/frontend/

CLI scans (in order):
1. /project/packages/frontend/.github/agents/
2. /project/packages/.github/agents/
3. /project/.github/agents/
4. ~/.copilot/agents/
5. Organization's .github repository (remote)
```

#### CLI Default Resources (Always Available)

These resources are provided by the Copilot CLI **regardless** of what's discovered in the filesystem. They form the baseline that every session has access to.

##### Built-in Tools

| Tool | Purpose |
|------|--------|
| `grep` / `grep_search` | Text search in files |
| `view` / `read_file` | Read file contents |
| `edit` / `replace_string_in_file` | Edit existing files |
| `create_file` | Create new files |
| `list_dir` | List directory contents |
| `file_search` | Glob-based file search |
| `semantic_search` | Semantic code search |
| `run_in_terminal` | Execute shell commands |
| `memory` | Persistent memory system |
| `manage_todo_list` | Task tracking |
| `task_complete` | Signal task completion |

##### Default MCP Server

| Server | Purpose | Notes |
|--------|---------|-------|
| `github` | GitHub API operations (issues, PRs, repos, etc.) | Enabled by default; can disable via `--disable-mcp-server=github` |

##### What Has NO Defaults

| Resource | Default | Source |
|----------|---------|--------|
| Custom Agents | None | Only from filesystem discovery or SDK config |
| Skills | None | Only from `.agents/skills/` or SDK config |
| Instructions | None | Only from `.github/instructions/` or user config |
| Hooks | None | Only from `.github/hooks/` or user config |
| Additional MCP Servers | None | Only from `.mcp.json` or SDK config |

> **Key Insight**: You cannot remove built-in tools or the default GitHub MCP server via SDK configuration — only via CLI flags (`--disable-mcp-server`, `--deny-tool`, `--available-tools`). However, you CAN restrict tools per-agent using the `tools` field (see below).

#### Tool Restriction via Agent Definition

While built-in tools are always *available* at the CLI level, **you CAN restrict which tools a specific agent can use** via the `tools` field in the agent definition (`.agent.md` frontmatter or SDK `customAgents` config).

##### How Tool Restriction Works

| `tools` Value | Behavior |
|---------------|----------|
| `tools: ["grep", "view"]` | Agent can **ONLY** use `grep` and `view` — no access to edit, bash, create_file, etc. |
| `tools: null` | Agent inherits **ALL** session tools (built-in + discovered) |
| `tools` omitted | Same as `null` — all tools available |

##### SDK Documentation Confirmation

From the [Copilot SDK Custom Agents documentation](https://github.com/github/copilot-sdk/blob/main/docs/features/custom-agents.md):

> | `tools` | `string[]` or `null` | | **Tool names the agent can use. `null` or omitted = all tools** |

> **Note:** When `tools` is `null` or omitted, the agent inherits access to all tools configured on the session. **Use explicit tool lists to enforce the principle of least privilege.**

##### Example: Agent File with Tool Restrictions

```markdown
---
name: security-reviewer
description: Reviews code for security vulnerabilities
tools:
  - grep
  - view
  - semantic_search
model: gpt-5
---

# Security Reviewer

You are a read-only security analyst. Analyze code for vulnerabilities.
Do not modify any files.
```

This agent can **only** use `grep`, `view`, and `semantic_search`. It **cannot** use:
- `edit` / `replace_string_in_file`
- `create_file`
- `run_in_terminal` / `bash`
- Any other built-in tools

##### Example: SDK Custom Agent with Tool Restrictions

```go
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",
    CustomAgents: []copilot.CustomAgent{
        {
            Name:        "reader",
            Description: "Read-only exploration of the codebase",
            Tools:       []string{"grep", "glob", "view"},  // ONLY these 3 tools
            Prompt:      "You explore and analyze code. Never modify files.",
        },
        {
            Name:        "writer",
            Description: "Makes code changes",
            Tools:       []string{"view", "edit", "bash"},  // Write access included
            Prompt:      "You make precise code changes as instructed.",
        },
        {
            Name:        "unrestricted",
            Description: "Full access for complex tasks",
            Tools:       nil,  // ALL tools available
            Prompt:      "Handle complex multi-step tasks using any tools.",
        },
    },
})
```

##### CLI Changelog Confirmation

From **v0.0.407** (Feb 11, 2026):
> "**Autopilot mode works with custom agents that specify explicit tools**"

This confirms the CLI respects the `tools` field and implements tool restrictions per agent.

##### Implications for Workflow Runner

| Method | Restricts Tools? | Notes |
|--------|------------------|-------|
| `tools` field in `.agent.md` | ✅ **YES** | Agent can only use listed tools |
| `tools` in SDK `customAgents` | ✅ **YES** | Same behavior via SDK config |
| CLI flags (`--deny-tool`) | ✅ **YES** | Session-wide restriction |
| SDK session config | ❌ **NO** | Cannot remove CLI built-in tools |

**Key takeaway:** For per-step tool isolation in the workflow runner, use the `tools` field in agent definitions rather than trying to remove tools at the session level.

### Copilot SDK: Configuration Passing vs CLI Discovery

The SDK provides **two modes** for configuring agents, skills, and MCP servers:

#### Mode 1: Explicit Configuration (Overrides CLI Discovery)

When you pass `customAgents`, `skillDirectories`, or `mcpServers` to `createSession()`, these are sent to the CLI and **supplement or override** the CLI's auto-discovered resources.

```go
// SDK passes explicit config to CLI - these ADD TO what CLI discovers
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",
    CustomAgents: []copilot.CustomAgent{
        {
            Name:   "my-agent",
            Prompt: "...",
        },
    },
    SkillDirectories: []string{"./my-skills"},
    MCPServers: map[string]copilot.MCPServerConfig{
        "my-mcp": {...},
    },
})
```

#### Mode 2: CLI Auto-Discovery (Let CLI Find Resources)

From changelog v1.0.7:
> "Add experimental SDK session APIs to list and manage skills, MCP servers, and plugins, **with optional config auto-discovery from the working directory**"

When you **don't** pass explicit configs, the CLI uses its native discovery to find resources in the filesystem. The SDK can also query what the CLI discovered:

```go
// Let CLI auto-discover from working directory
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",
    // No customAgents, skillDirectories, or mcpServers specified
    // CLI will use its native discovery paths
})

// SDK can query what CLI discovered
skills, _ := session.RPC.Skills.List()
mcpServers, _ := session.RPC.MCP.List()
```

#### How SDK Configs Interact with CLI Discovery

| SDK Config | CLI Behavior |
|------------|--------------|
| `customAgents` provided | SDK agents **added** to CLI-discovered agents |
| `skillDirectories` provided | SDK paths **added** to CLI skill search paths |
| `mcpServers` provided | SDK servers **merged** with CLI-discovered servers |
| Nothing provided | CLI uses **only** its native filesystem discovery |

**Critical Insight**: The CLI always performs some discovery (at minimum for built-in tools, GitHub MCP server, etc.). The SDK configs **extend**, not replace, the CLI's baseline capabilities.

### Custom Agents

Custom agents are defined inline when creating a session via the `customAgents` parameter:

```go
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",
    CustomAgents: []copilot.CustomAgent{
        {
            Name:        "security-reviewer",
            DisplayName: "Security Reviewer",
            Description: "Reviews code for vulnerabilities",
            Tools:       []string{"grep", "glob", "view"},
            Prompt:      "You are a security expert...",
            MCPServers:  map[string]copilot.MCPServerConfig{...},  // Per-agent MCP
            Infer:       true,  // Allow runtime auto-selection
        },
    },
    Agent: "security-reviewer",  // Pre-select this agent
})
```

**Key points:**
- Agents are **not discovered automatically** from filesystem
- Each agent can have its own `tools`, `mcpServers`, and `prompt`
- The `agent` field pre-selects which custom agent is active from session start
- The runtime can auto-delegate to sub-agents based on `description` matching

#### Skills (SDK)

Skills are loaded from **directories** specified at session creation time:

```go
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-4.1",
    SkillDirectories: []string{
        "./skills/code-review",
        "./skills/documentation",
    },
    DisabledSkills: []string{"experimental-feature"},
})
```

**Directory structure:**
```
skills/
├── code-review/
│   └── SKILL.md
└── documentation/
    └── SKILL.md
```

**SKILL.md format:**
```markdown
---
name: code-review
description: Specialized code review capabilities
---

# Code Review Guidelines

When reviewing code, always check for:
1. Security vulnerabilities
2. Performance issues
...
```

**Key points:**
- Skills are markdown files with optional YAML frontmatter
- The SDK scans `skillDirectories` for subdirectories containing `SKILL.md`
- Skill content is injected into the session context
- Skills can be selectively disabled via `disabledSkills`

#### MCP Servers (SDK)

MCP servers are configured at session creation and can be specified:
1. **At session level** — available to all agents in the session
2. **Per custom agent** — only available to that specific agent

**Session-level MCP:**
```go
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",
    MCPServers: map[string]copilot.MCPServerConfig{
        "filesystem": {
            "type":    "local",
            "command": "npx",
            "args":    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
            "tools":   []string{"*"},
        },
        "github": {
            "type":    "http",
            "url":     "https://api.githubcopilot.com/mcp/",
            "headers": map[string]string{"Authorization": "Bearer ${TOKEN}"},
            "tools":   []string{"*"},
        },
    },
})
```

**Per-agent MCP:**
```go
CustomAgents: []copilot.CustomAgent{
    {
        Name: "db-analyst",
        Prompt: "You are a database expert...",
        MCPServers: map[string]copilot.MCPServerConfig{
            "database": {
                "command": "npx",
                "args":    []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"},
            },
        },
    },
},
```

**Key points:**
- MCP servers can be local (stdio) or remote (HTTP/SSE)
- `tools` controls which MCP tools are exposed (`["*"]` = all, `[]` = none)
- Per-agent MCP servers provide isolation between agents

---

### Workflow Runner: Current Implementation

The workflow runner implements its own discovery layer on top of the SDK abstractions. **However**, this duplicates what the CLI already does natively.

#### Agent Discovery (Workflow Runner's Own Implementation)

Agents are discovered from multiple filesystem locations with priority ordering:

| Priority | Location | Description |
|---|---|---|
| 1 (highest) | Explicit `agents.*.file` in workflow YAML | Direct file reference |
| 2 | `.github/agents/*.agent.md` | Project-level agents |
| 3 | `.claude/agents/*.md` | Claude-format agents (auto-normalized) |
| 4 | `~/.copilot/agents/*.agent.md` | User-global agents |
| 5 (lowest) | `config.agent_search_paths` entries | Custom directories |

**Note**: This discovery logic **duplicates** what the Copilot CLI already does. The workflow runner could potentially delegate discovery to the CLI when using the SDK.

**Current code:** [pkg/agents/discovery.go](pkg/agents/discovery.go)

```go
func DiscoverAgents(workspaceDir string, extraPaths []string) (map[string]*Agent, error) {
    agents := make(map[string]*Agent)
    
    // Priority 4 (lowest): extra search paths from config
    for i := len(extraPaths) - 1; i >= 0; i-- {
        scanDir(extraPaths[i], false, agents)
    }
    
    // Priority 3: ~/.copilot/agents/
    copilotDir := filepath.Join(homeDir, ".copilot", "agents")
    scanDir(copilotDir, false, agents)
    
    // Priority 2: .claude/agents/ (with Claude normalization)
    claudeDir := filepath.Join(workspaceDir, ".claude", "agents")
    scanDir(claudeDir, true, agents)
    
    // Priority 1 (highest): .github/agents/
    githubDir := filepath.Join(workspaceDir, ".github", "agents")
    scanDir(githubDir, false, agents)
    
    return agents, nil
}
```

#### Architecture Decision: Own Discovery vs CLI Discovery

The workflow runner has two choices for implementation:

**Option A: Continue Own Discovery (Current)**
- Pros: Full control, works with CLI prompt mode (`-p`)
- Cons: Duplicates CLI logic, may diverge from CLI behavior

**Option B: Delegate to CLI/SDK Discovery**
- Pros: Stays in sync with CLI, leverages monorepo support
- Cons: Requires SDK executor (not CLI prompt mode), less control

#### Agent File Format (`.agent.md`)

```markdown
---
name: security-reviewer
description: Reviews code for security vulnerabilities
tools:
  - grep
  - semantic_search
  - view
model: gpt-5
mcp-servers:
  sec-tools:
    command: docker
    args: ["run", "security:latest"]
---

# Security Reviewer

You are an expert security reviewer. Focus on:
1. Injection attacks
2. Authentication flaws
...
```

**Current code:** [pkg/agents/loader.go](pkg/agents/loader.go)

#### Skills (Not Yet Implemented)

The workflow YAML includes a `skills` field at both workflow and step level:

```yaml
skills:              # Workflow-level skills
  - code-review
  
steps:
  - id: analyze
    agent: security-reviewer
    skills:          # Step-level skills (not yet implemented)
      - vulnerability-scanning
```

**Current status:** The parser recognizes `skills` fields but they are not passed to the SDK session.

#### MCP Servers (Partially Implemented)

MCP servers are defined in agent files and stored in the `Agent` struct:

```go
type Agent struct {
    // ...
    MCPServers  map[string]MCPServerConfig `yaml:"mcp-servers"`
}
```

**Current status:** The `SessionConfig` has an `MCPServers` field but the executor does not yet pass agent MCP servers to it.

---

### Key Finding: SDK ↔ CLI Relationship

After cross-referencing both repositories, here's the definitive architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                    YOUR APPLICATION                              │
│                  (Workflow Runner)                               │
└───────────────────────┬─────────────────────────────────────────┘
                        │
                        │ Go SDK / Python SDK / etc.
                        │ Passes: customAgents, skillDirectories, mcpServers
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│                    COPILOT SDK                                   │
│  - Manages CLI process lifecycle                                 │
│  - Passes session config to CLI via JSON-RPC                     │
│  - SDK configs EXTEND (not replace) CLI discovery                │
└───────────────────────┬─────────────────────────────────────────┘
                        │
                        │ JSON-RPC (stdio or TCP)
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│                    COPILOT CLI                                   │
│                 (The Runtime Engine)                             │
│                                                                  │
│  ALWAYS performs native discovery:                               │
│  • .github/agents/, ~/.copilot/agents/                          │
│  • .agents/skills/, ~/.copilot/skills/                          │
│  • .mcp.json, .vscode/mcp.json, ~/.copilot/mcp-config.json      │
│  • Built-in tools (grep, view, edit, etc.)                      │
│  • GitHub MCP server (default)                                   │
│                                                                  │
│  SDK-provided configs are MERGED with discovered configs         │
└─────────────────────────────────────────────────────────────────┘
```

#### Implications for Per-Step Scoping

1. **CLI CWD matters**: The CLI discovers resources relative to its working directory. Changing CWD per step would change what the CLI discovers.

2. **SDK configs add, don't isolate**: Passing `mcpServers` to the SDK **adds** to CLI-discovered MCP servers; it doesn't remove CLI's default servers.

3. **True isolation requires working directory control**: To truly isolate a step, you'd need to:
   - Set the CLI's working directory to the step's scope folder
   - OR explicitly disable CLI auto-discovery (if possible)
   - OR use `--disable-mcp-server` flags to remove unwanted servers

---

## 2. Per-Step Scoping: Design & Implementation

### Concept Overview

Per-step scoping allows each workflow step to operate in an isolated environment with:
- **Its own agents** — different from the global agent pool
- **Its own skills** — step-specific capabilities
- **Its own MCP servers** — step-specific external tools

This enables scenarios like:
1. A "security analysis" step that uses only security-focused tools
2. A "database migration" step with access to a PostgreSQL MCP server
3. Steps that should NOT have access to file editing tools

### YAML Schema Changes

#### Option A: Step-Level Directory Override

```yaml
steps:
  - id: security-scan
    agent: security-reviewer
    prompt: "Scan for vulnerabilities..."
    scope:                              # NEW: per-step scoping
      agent_dir: "./step-agents/security"
      skill_directories:
        - "./step-skills/security"
      mcp_config: "./step-mcp/security.yaml"
```

#### Option B: Inline MCP/Skill Definition per Step

```yaml
steps:
  - id: database-migration
    agent: db-migrator
    prompt: "Generate migration script..."
    mcp_servers:                        # NEW: inline MCP per step
      postgres:
        type: local
        command: npx
        args: ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"]
        tools: ["*"]
    skills:                             # Already supported in parser
      - sql-best-practices
    tool_restrictions:                  # NEW: limit available tools
      allow: [grep, view, run_sql]
      # OR
      deny: [edit, bash, create_file]
```

#### Option C: Named Scopes (Reusable)

```yaml
scopes:
  security:
    agent_dir: "./scopes/security/agents"
    skill_directories: ["./scopes/security/skills"]
    mcp_servers:
      vuln-scanner:
        command: docker
        args: ["run", "vulnerability-scanner:latest"]
    tool_restrictions:
      allow: [grep, view, semantic_search]
      
  database:
    agent_dir: "./scopes/database/agents"
    mcp_servers:
      postgres:
        type: local
        command: npx
        args: ["-y", "@modelcontextprotocol/server-postgres"]

steps:
  - id: security-scan
    agent: scanner
    scope: security              # Reference named scope
    prompt: "..."
    
  - id: db-migrate
    agent: migrator
    scope: database
    prompt: "..."
```

### Implementation Requirements

Given that the CLI performs its own discovery, per-step scoping can be achieved through **multiple strategies**:

#### Strategy 1: Working Directory Manipulation

The CLI discovers resources relative to its CWD. By launching each step's session from a different directory, you get different auto-discovered resources.

```go
// For step "security-scan", launch CLI from the security scope directory
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Cwd: "./scopes/security",  // CLI will discover from here
    Model: "gpt-5",
})
// CLI auto-discovers:
// ./scopes/security/.github/agents/
// ./scopes/security/.mcp.json
// ./scopes/security/.agents/skills/
```

**Pros**: Leverages CLI's native monorepo support
**Cons**: Agents can only access files within that subdirectory

#### Strategy 2: Explicit SDK Configuration

Pass explicit configs via SDK, which merge with CLI discovery.

```go
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    CustomAgents: []copilot.CustomAgent{...},
    SkillDirectories: []string{"./scopes/security/skills"},
    MCPServers: map[string]copilot.MCPServerConfig{...},
})
```

**Pros**: Full control over what's added
**Cons**: Cannot remove CLI-discovered resources

#### Strategy 3: Hybrid with CLI Flags

Use CLI flags to disable unwanted resources, then add scoped ones via SDK.

```go
// Via CLI args or SDK config
// --disable-mcp-server=github  // Remove default GitHub MCP
// --deny-tool='shell(rm)'       // Restrict dangerous tools

session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    MCPServers: map[string]copilot.MCPServerConfig{
        "scoped-mcp": {...},  // Only this MCP available
    },
})
```

#### 1. YAML Parser Changes

Add new fields to the `Step` struct in [pkg/workflow/types.go](pkg/workflow/types.go):

```go
type Step struct {
    ID        string     `yaml:"id"`
    Agent     string     `yaml:"agent"`
    Prompt    string     `yaml:"prompt"`
    DependsOn []string   `yaml:"depends_on"`
    Condition *Condition `yaml:"condition,omitempty"`
    
    // NEW: Per-step scoping
    Scope            *StepScope              `yaml:"scope,omitempty"`
    MCPServers       map[string]MCPConfig    `yaml:"mcp_servers,omitempty"`
    SkillDirectories []string                `yaml:"skill_directories,omitempty"`
    ToolRestrictions *ToolRestrictions       `yaml:"tool_restrictions,omitempty"`
}

type StepScope struct {
    AgentDir         string   `yaml:"agent_dir,omitempty"`
    SkillDirectories []string `yaml:"skill_directories,omitempty"`
    MCPConfigFile    string   `yaml:"mcp_config,omitempty"`
}

type ToolRestrictions struct {
    Allow []string `yaml:"allow,omitempty"`
    Deny  []string `yaml:"deny,omitempty"`
}
```

#### 2. SessionConfig Enhancement

Update [pkg/executor/sdk.go](pkg/executor/sdk.go) to support all SDK features:

```go
type SessionConfig struct {
    SystemPrompt     string
    Model            string
    Tools            []string                  // Tool names agent can use
    ToolRestrictions *ToolRestrictions         // Allow/deny lists
    MCPServers       map[string]MCPServerConfig
    SkillDirectories []string
    DisabledSkills   []string
    CustomAgents     []CustomAgentConfig
    Agent            string                    // Pre-selected agent name
}

type CustomAgentConfig struct {
    Name        string
    DisplayName string
    Description string
    Prompt      string
    Tools       []string
    MCPServers  map[string]MCPServerConfig
    Infer       bool
}
```

#### 3. Executor Changes

In [pkg/executor/executor.go](pkg/executor/executor.go), resolve per-step scope before session creation:

```go
func (se *StepExecutor) Execute(
    ctx context.Context,
    step workflow.Step,
    agent *agents.Agent,
    globalConfig workflow.Config,  // NEW: pass global config
    results map[string]string,
    inputs map[string]string,
    seqNum int,
) (*workflow.StepResult, error) {
    
    // 1. Resolve step-specific agent if scope.agent_dir is set
    effectiveAgent := agent
    if step.Scope != nil && step.Scope.AgentDir != "" {
        scopedAgents, err := agents.DiscoverAgentsFromDir(step.Scope.AgentDir)
        if err != nil {
            return nil, fmt.Errorf("loading scoped agents: %w", err)
        }
        if a, ok := scopedAgents[step.Agent]; ok {
            effectiveAgent = a
        }
    }
    
    // 2. Merge MCP servers: agent-level + step-level
    mcpServers := mergeOrOverrideMCP(effectiveAgent.MCPServers, step.MCPServers)
    
    // 3. Merge skill directories: global + step-level
    skillDirs := append(globalConfig.SkillDirectories, step.SkillDirectories...)
    if step.Scope != nil {
        skillDirs = append(skillDirs, step.Scope.SkillDirectories...)
    }
    
    // 4. Apply tool restrictions
    tools := effectiveAgent.Tools
    if step.ToolRestrictions != nil {
        tools = applyToolRestrictions(tools, step.ToolRestrictions)
    }
    
    // 5. Build session config
    sessionCfg := SessionConfig{
        SystemPrompt:     effectiveAgent.Prompt,
        Model:            resolveModel(effectiveAgent, globalConfig),
        Tools:            tools,
        MCPServers:       mcpServers,
        SkillDirectories: skillDirs,
    }
    
    // ... continue with session creation
}
```

#### 4. SDK Integration Layer

The real SDK session creation in a future Go SDK executor would look like:

```go
func (e *CopilotSDKExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
    client := copilot.NewClient(&copilot.ClientOptions{})
    
    session, err := client.CreateSession(ctx, &copilot.SessionConfig{
        Model: cfg.Model,
        SystemMessage: &copilot.SystemMessageConfig{
            Content: cfg.SystemPrompt,
        },
        MCPServers:       cfg.MCPServers,
        SkillDirectories: cfg.SkillDirectories,
        DisabledSkills:   cfg.DisabledSkills,
        CustomAgents:     cfg.CustomAgents,
        Agent:            cfg.Agent,
        OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
    })
    
    return &sdkSession{session: session}, nil
}
```

### Execution Flow with Per-Step Scoping

```
Step Execution with Scoping
────────────────────────────

1. Parse step definition
   └─► Extract scope config (agent_dir, skill_directories, mcp_servers, tool_restrictions)

2. Resolve effective agent
   ├─► If scope.agent_dir: discover agents ONLY from that directory
   └─► Else: use globally discovered agent

3. Merge MCP servers
   ├─► Start with agent-level MCP servers
   ├─► Merge/override with step-level mcp_servers
   └─► If scope.mcp_config: load and merge from that file

4. Resolve skill directories
   ├─► Start with global skill directories (from workflow config)
   ├─► Add step-level skill_directories
   └─► Add scope.skill_directories if scope is defined

5. Apply tool restrictions
   ├─► Start with agent's tools list
   ├─► If tool_restrictions.allow: intersect with allowed tools
   └─► If tool_restrictions.deny: subtract denied tools

6. Create isolated session
   └─► Pass all resolved config to SDK session

7. Execute step with full isolation
```

### Per-Step Parallel Execution Model

This diagram illustrates how CLI baseline resources combine with per-step additions during parallel execution:

```
Parallel Step Execution
═══════════════════════

┌─────────────────────────────────────────────────────────────┐
│                      CLI BASELINE                            │
│  (Same for ALL steps — discovered once from workspace)       │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ ALWAYS AVAILABLE (built-in):                            ││
│  │ • Built-in tools (grep, view, edit, create_file, etc.) ││
│  │ • GitHub MCP server (default)                           ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ DISCOVERED FROM FILESYSTEM:                             ││
│  │ • Agents from .github/agents/*.agent.md                 ││
│  │ • Skills from .agents/skills/                           ││
│  │ • MCP servers from .mcp.json                            ││
│  │ • Instructions from .github/instructions/               ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
          │                    │                    │
          │ MERGED             │ MERGED             │ MERGED
          ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Step A        │  │   Step B        │  │   Step C        │
│   Session       │  │   Session       │  │   Session       │
│                 │  │                 │  │                 │
│ CLI Baseline    │  │ CLI Baseline    │  │ CLI Baseline    │
│       +         │  │       +         │  │       +         │
│ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │
│ │ Step A adds:│ │  │ │ Step B adds:│ │  │ │ Step C adds:│ │
│ │ • Agent X   │ │  │ │ • postgres  │ │  │ │ • vuln-scan │ │
│ │ • skill-a   │ │  │ │   MCP       │ │  │ │   skill     │ │
│ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │
└─────────────────┘  └─────────────────┘  └─────────────────┘
     ISOLATED            ISOLATED            ISOLATED
     SESSION             SESSION             SESSION
```

**Key Properties:**

| Property | Behavior |
|----------|----------|
| **CLI Baseline** | Shared across all steps (discovered once at workflow start) |
| **Per-Step Additions** | Only available to that specific step's session |
| **Isolation** | Each step runs in its own session; additions don't leak between steps |
| **Parallel Safety** | Steps can run concurrently; each has independent session state |
| **Additive Only** | SDK configs can only ADD to CLI baseline, not remove from it |

**To Remove CLI Resources:**

Since SDK configs are additive, removing CLI-discovered resources requires CLI flags:

| Goal | CLI Flag |
|------|----------|
| Disable GitHub MCP | `--disable-mcp-server=github` |
| Restrict to specific tools | `--available-tools=grep,view,semantic_search` |
| Deny specific tools | `--deny-tool='shell(rm)'` |
| Change discovery root | Set CLI working directory to subdirectory |

---

## 3. Model Usage and Enforcement

### Copilot SDK Model Selection

The SDK supports model selection at multiple levels:

| Level | Configuration | Priority |
|---|---|---|
| Session | `SessionConfig.Model` | Base model for the session |
| Provider | `SessionConfig.Provider` | Override with BYOK provider |
| Custom Agent | Not supported directly | Agent inherits session model |

**Important:** The SDK does **not** support per-agent model selection within a single session. All custom agents in a session use the same model.

```go
// This gives ALL agents in the session the same model
session, err := client.CreateSession(ctx, &copilot.SessionConfig{
    Model: "gpt-5",  // All agents use this
    CustomAgents: []copilot.CustomAgent{
        {Name: "agent-a", Prompt: "..."},
        {Name: "agent-b", Prompt: "..."},
    },
})
```

### Current Workflow Runner Model Handling

The workflow runner resolves models in this order:

1. **Agent-level model** (from `.agent.md` frontmatter)
2. **Workflow-level model** (from `config.model`)
3. **Default** (Copilot CLI picks if nothing specified)

**Current code:** [pkg/executor/executor.go](pkg/executor/executor.go)

```go
sessionCfg := SessionConfig{
    SystemPrompt: agent.Prompt,
    Tools:        agent.Tools,
}
if len(agent.Model.Models) > 0 {
    sessionCfg.Model = agent.Model.Models[0]  // Use first model in list
}
```

**Current code:** [pkg/executor/copilot_cli.go](pkg/executor/copilot_cli.go)

```go
args := []string{
    "--allow-all-tools",
    "--no-ask-user",
    "-p", finalPrompt,
}
if s.cfg.Model != "" {
    args = append(args, "--model", s.cfg.Model)  // Pass to CLI
}
```

### Per-Agent Model Enforcement

Since the workflow runner creates **one session per step**, per-agent model selection already works:

```
Step 1: security-scan
├─► Agent: security-reviewer (model: gpt-5)
└─► Session created with model=gpt-5

Step 2: aggregation
├─► Agent: aggregator (model: claude-sonnet-4.5)
└─► Session created with model=claude-sonnet-4.5
```

**Current YAML support:**

```yaml
agents:
  security-reviewer:
    file: "./agents/security-reviewer.agent.md"  # Has model: gpt-5
  aggregator:
    inline:
      description: "Aggregates findings"
      model: "claude-sonnet-4.5"       # Per-agent model
      prompt: "..."
```

**.agent.md support:**

```markdown
---
name: security-reviewer
model: gpt-5
# OR priority list (first available is used):
model:
  - gpt-5
  - gpt-4o
---
```

**Enhancement needed:** Support `model` field on step definitions:

```yaml
steps:
  - id: analyze
    agent: generic-analyzer
    model: gpt-5-turbo              # Override agent's model for this step
    prompt: "..."
```

**Implementation:**

```go
// In step execution
model := ""
if step.Model != "" {
    model = step.Model  // Step-level override
} else if len(agent.Model.Models) > 0 {
    model = agent.Model.Models[0]  // Agent-level
} else if globalConfig.Model != "" {
    model = globalConfig.Model  // Workflow-level default
}
// else: let the SDK/CLI pick
```

---

## 4. Implementation Summary

### Critical Findings: SDK vs CLI Discovery

| Aspect | SDK Behavior | CLI Behavior | Combined Behavior |
|--------|-------------|--------------|-------------------|
| **Agent Discovery** | `customAgents` passed to CLI | Scans `.github/agents/`, `~/.copilot/agents/`, org `.github` repo | SDK agents **merged** with CLI-discovered agents |
| **Skill Discovery** | `skillDirectories` passed to CLI | Scans `.agents/skills/`, `~/.copilot/skills/`, every dir to git root | SDK paths **added** to CLI search paths |
| **MCP Discovery** | `mcpServers` passed to CLI | Scans `.mcp.json`, `.vscode/mcp.json`, `~/.copilot/mcp-config.json` | SDK servers **merged** with CLI-discovered servers |
| **Built-in Tools** | Cannot disable via SDK config | Always available (grep, view, edit, bash, etc.) | Always available |
| **GitHub MCP Server** | Cannot disable via SDK config | Enabled by default | Can disable via `--disable-mcp-server=github` flag |

### Key Implications for Workflow Runner

1. **The CLI is the runtime engine**: The SDK is a wrapper around the CLI. All agent execution happens inside the CLI process.

2. **CLI discovery always runs**: Even when using the SDK, the CLI scans its standard directories. You cannot fully isolate a step from CLI-discovered resources through SDK config alone.

3. **Per-step scoping options**:
   - **Change CWD**: Launch each step from its scope directory
   - **Use flags**: `--disable-mcp-server`, `--deny-tool`, `--available-tools`
   - **Extend only**: Add scoped resources via SDK, accept CLI baseline

4. **Workflow runner's own discovery is redundant**: The workflow runner reimplements agent discovery that the CLI already does. Consider delegating to CLI or using SDK's discovery APIs.

### What Currently Works

| Feature | Status | Details |
|---|---|---|
| Agent discovery from filesystem | ✅ Implemented | Workflow runner has own discovery (duplicates CLI) |
| Agent file parsing (`.agent.md`) | ✅ Implemented | YAML frontmatter + markdown body |
| Per-agent model from frontmatter | ✅ Implemented | Passed to CLI via `--model` flag |
| CLI-native agent discovery | ⚠️ Not leveraged | CLI discovers agents but workflow runner re-discovers |
| CLI-native skill discovery | ⚠️ Not leveraged | CLI discovers skills automatically |
| CLI-native MCP discovery | ⚠️ Not leveraged | CLI discovers MCP from `.mcp.json` etc. |
| Skills field in YAML | ⚠️ Parsed only | Not passed to SDK/CLI |
| MCP servers in agent files | ⚠️ Parsed only | Not passed to SDK/CLI |
| Per-step skill directories | ❌ Not implemented | Schema + execution needed |
| Per-step MCP servers | ❌ Not implemented | Schema + execution needed |
| Per-step agent directory | ❌ Not implemented | Schema + execution needed |
| Tool restrictions | ❌ Not implemented | Schema + execution needed (can use CLI flags) |

### Implementation Roadmap for Per-Step Scoping

| Phase | Work Item | Effort | Notes |
|---|---|---|---|
| **Phase 0** | Decide: own discovery vs delegate to CLI | Decision | CLI already does discovery; consider simplifying |
| **Phase 1** | Add `scope`, `mcp_servers`, `skill_directories`, `tool_restrictions` to `Step` struct | Small | YAML schema changes |
| **Phase 2** | Implement `StepScope` resolution in executor | Medium | Support CWD-based or explicit configs |
| **Phase 3** | Update `SessionConfig` with full SDK feature set | Small | Add skillDirectories, mcpServers |
| **Phase 4** | Implement MCP server merging logic | Medium | Remember: CLI servers are always present |
| **Phase 5** | Implement skill directory merging | Small | SDK adds to CLI paths |
| **Phase 6** | Implement tool restriction via CLI flags | Small | Use `--deny-tool`, `--available-tools` |
| **Phase 7** | Migrate from CLI executor to SDK executor | Large | Required for full SDK config support |
| **Phase 8** | Add step-level model override | Small | Already partially supported |
| **Phase 8** | Add step-level model override | Small |

### Critical Design Decisions

1. **One session per step** — Already implemented, enables per-step isolation naturally

2. **Merge vs. Override semantics for MCP servers:**
   - **Override:** Step MCP completely replaces agent MCP
   - **Merge:** Step MCP extends agent MCP (key conflicts: step wins)
   - **Recommendation:** Merge with step override on conflicts

3. **Skill directory precedence:**
   - Global > Step scope > Step inline
   - Or: More specific wins (Step inline > Step scope > Global)
   - **Recommendation:** Merge all, with step-level taking precedence on conflicts

4. **Tool restriction logic:**
   - If `allow` is set: only those tools are available
   - If `deny` is set: those tools are removed
   - If both: error or `allow ∩ ¬deny`?
   - **Recommendation:** Mutually exclusive (`allow` XOR `deny`)

---

## Appendix: SDK Code Examples

### Go SDK: Full Session Configuration

```go
import copilot "github.com/github/copilot-sdk/go"

func createScopedSession(
    agent *agents.Agent,
    step workflow.Step,
    globalConfig workflow.Config,
) (*copilot.Session, error) {
    client := copilot.NewClient(&copilot.ClientOptions{
        LogLevel: "info",
    })
    
    if err := client.Start(context.Background()); err != nil {
        return nil, err
    }
    
    // Resolve skill directories
    skillDirs := globalConfig.SkillDirectories
    if step.Scope != nil {
        skillDirs = append(skillDirs, step.Scope.SkillDirectories...)
    }
    skillDirs = append(skillDirs, step.SkillDirectories...)
    
    // Resolve MCP servers
    mcpServers := make(map[string]copilot.MCPServerConfig)
    for k, v := range agent.MCPServers {
        mcpServers[k] = toSDKMCPConfig(v)
    }
    for k, v := range step.MCPServers {
        mcpServers[k] = toSDKMCPConfig(v)  // Step overrides agent
    }
    
    // Apply tool restrictions
    tools := agent.Tools
    if step.ToolRestrictions != nil {
        if len(step.ToolRestrictions.Allow) > 0 {
            tools = intersect(tools, step.ToolRestrictions.Allow)
        }
        if len(step.ToolRestrictions.Deny) > 0 {
            tools = subtract(tools, step.ToolRestrictions.Deny)
        }
    }
    
    // Resolve model
    model := globalConfig.Model
    if len(agent.Model.Models) > 0 {
        model = agent.Model.Models[0]
    }
    if step.Model != "" {
        model = step.Model
    }
    
    session, err := client.CreateSession(context.Background(), &copilot.SessionConfig{
        Model: model,
        SystemMessage: &copilot.SystemMessageConfig{
            Content: agent.Prompt,
        },
        MCPServers:       mcpServers,
        SkillDirectories: skillDirs,
        OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
    })
    
    return session, err
}
```

### Example: Security-Scoped Step

```yaml
scopes:
  security:
    agent_dir: "./scopes/security/agents"
    skill_directories:
      - "./scopes/security/skills"
    mcp_servers:
      vuln-db:
        type: local
        command: docker
        args: ["run", "--rm", "vuln-scanner:latest"]
    tool_restrictions:
      allow: [grep, view, semantic_search, mcp_vuln-db_*]

steps:
  - id: security-analysis
    scope: security
    agent: vuln-scanner      # Resolved from ./scopes/security/agents/
    prompt: |
      Analyze the following code for security vulnerabilities:
      {{steps.load-code.output}}
    mcp_servers:             # Additional MCP just for this step
      sast-tool:
        type: http
        url: "http://localhost:8080/sast"
```

**Effect:**
1. Agent loaded from `./scopes/security/agents/vuln-scanner.agent.md`
2. Skills loaded from `./scopes/security/skills/`
3. MCP servers: `vuln-db` (from scope) + `sast-tool` (from step)
4. Tools restricted to: `grep`, `view`, `semantic_search`, and all `vuln-db` MCP tools

---

## Appendix A: CLI Discovery Paths Reference

These are the filesystem paths the Copilot CLI scans automatically (from changelog analysis):

### Agents
| Location | Scope | Notes |
|----------|-------|-------|
| `.github/agents/*.agent.md` | Repository | Primary location |
| `~/.copilot/agents/*.agent.md` | User | Personal agents |
| Organization `.github` repository | Organization | Remote agents |

### Skills
| Location | Scope | Notes |
|----------|-------|-------|
| `.agents/skills/` | Repository | Auto-loaded (v0.0.401+) |
| `~/.agents/skills/` | User | Personal skills (v1.0.11+) |
| `~/.copilot/skills/` | User | Personal skills |
| Every directory up to git root | Repository | Monorepo support (v1.0.11+) |

### MCP Servers
| Location | Scope | Notes |
|----------|-------|-------|
| `.mcp.json` | Workspace | Claude-compatible format |
| `.vscode/mcp.json` | Workspace | VS Code format (v0.0.407+) |
| `.devcontainer/devcontainer.json` | Container | Dev container config |
| `~/.copilot/mcp-config.json` | User | Personal MCP config |

### Instructions
| Location | Scope | Notes |
|----------|-------|-------|
| `.github/instructions/*.instructions.md` | Repository | Project instructions |
| `~/.copilot/instructions/*.instructions.md` | User | Personal instructions (v0.0.412+) |
| Every directory up to git root | Repository | Monorepo support (v1.0.11+) |

### Hooks
| Location | Scope | Notes |
|----------|-------|-------|
| `.github/hooks/` | Repository | Project hooks |
| `~/.copilot/hooks/` | User | Personal hooks (v0.0.422+) |

---

## References

- [Copilot SDK Documentation](https://github.com/github/copilot-sdk/blob/main/docs/index.md)
- [Copilot SDK Go README](https://github.com/github/copilot-sdk/blob/main/go/README.md)
- [Custom Agents Guide](https://github.com/github/copilot-sdk/blob/main/docs/features/custom-agents.md)
- [Skills Guide](https://github.com/github/copilot-sdk/blob/main/docs/features/skills.md)
- [MCP Servers Guide](https://github.com/github/copilot-sdk/blob/main/docs/features/mcp.md)
- [Copilot CLI Changelog](https://github.com/github/copilot-cli/blob/main/changelog.md) — **Critical source for discovery paths**
- [About Copilot CLI](https://docs.github.com/copilot/concepts/agents/about-copilot-cli)
- [Workflow Runner PLAN.md](./PLAN.md)
- [Workflow Runner DOCS.md](./DOCS.md)

# Comparison: goflow vs GitHub Agentic Workflows

This document compares our **goflow** project with **GitHub Agentic Workflows** (gh-aw), a product developed by GitHub Next and Microsoft Research.

---

## Executive Summary

While both projects orchestrate AI agents to automate tasks, they serve fundamentally different purposes:

| Aspect | goflow | GitHub Agentic Workflows |
|--------|-----------------|--------------------------|
| **Scope** | General-purpose agentic orchestration for *any* domain | Repository automation specifically for GitHub |
| **Platform** | Platform-agnostic (runs anywhere) | Locked to GitHub Actions |
| **Use Cases** | Software, operations, business processes, personal automation | Code reviews, issue triage, documentation, CI/CD |

**Key Insight:** GitHub Agentic Workflows is a specialized tool for GitHub repository automation. goflow is a universal orchestration engine that can automate *anything* — software development is just one of many possible applications.

---

## What is GitHub Agentic Workflows?

GitHub Agentic Workflows (gh-aw) is a GitHub-native feature that allows developers to define AI-powered automation tasks using Markdown files with YAML frontmatter. These workflows run inside GitHub Actions and are tightly integrated with GitHub's ecosystem (Issues, PRs, Discussions, Releases).

**Key Characteristics:**
- Markdown-based workflow definitions with natural language instructions
- Runs exclusively in GitHub Actions (containerized, sandboxed)
- Supports Copilot, Claude, and Codex as AI engines
- "Safe Outputs" system for controlled write operations (e.g., creating issues, adding labels)
- Designed for repository-centric tasks: triage, documentation, CI failure analysis

**Example Use Case:** 
> "Every morning, analyze open issues and PRs, then create a team status report as a GitHub Issue."

---

## What is goflow?

goflow is a **general-purpose, platform-agnostic AI workflow orchestration engine**. It uses YAML-defined Directed Acyclic Graphs (DAGs) to coordinate multiple AI agents through complex, parallel, and conditional workflows.

**Key Characteristics:**
- YAML-based DAG definitions with explicit step dependencies
- Runs anywhere: local machine, any CI/CD system, cloud VMs, on-premise servers
- Native parallelism with fan-out/fan-in patterns (goroutines + WaitGroups)
- MCP server integration for connecting to external systems
- VS Code `.agent.md` compatibility for agent definitions
- Full audit trail for transparency and debugging

**Example Use Cases:**
- Multi-agent code review pipelines
- Automated data extraction from APIs and databases
- Team communication analysis (Slack, Teams, Discord via MCP)
- Business process automation (invoice processing, report generation)
- IoT monitoring and alerting workflows
- Research pipelines (literature review, data synthesis)

---

## Key Differences

### 1. Scope: Specialized vs Universal

| GitHub Agentic Workflows | goflow |
|--------------------------|-----------------|
| Designed specifically for software repository tasks | Designed for **any** agentic workflow in any domain |
| Assumes GitHub as the primary interface | No assumptions about the domain or platform |
| "Safe Outputs" limited to GitHub API operations | Agents can interact with any system via MCP or custom tools |

**goflow is not a "software development tool" — it is a universal workflow engine.** The fact that the initial examples focus on code review is incidental. The same engine can orchestrate:
- A team of agents analyzing customer support tickets
- A pipeline that monitors news feeds and generates summaries
- An automation that checks inventory levels across multiple suppliers
- A personal assistant workflow that triages emails and schedules meetings

### 2. Platform Independence

| GitHub Agentic Workflows | goflow |
|--------------------------|-----------------|
| **Requires GitHub.** Workflows run in GitHub Actions containers. | **Runs anywhere.** CLI binary works on macOS, Linux, Windows, and inside any CI/CD system. |
| Cannot be used with GitLab, Bitbucket, Azure DevOps, or self-hosted forges | Works with any version control system or none at all |
| Requires code to be pushed to GitHub before workflows execute | Can run against local files, APIs, databases, or any data source |

**This is a fundamental architectural difference.** GitHub Agentic Workflows is a feature *of* GitHub. goflow is an independent tool that can be integrated *with* GitHub, GitLab, Jenkins, Azure Pipelines, Buildkite, or run standalone.

### 3. Workflow Complexity

| GitHub Agentic Workflows | goflow |
|--------------------------|-----------------|
| Single-agent, natural language Markdown files | Multi-agent DAGs with explicit dependencies |
| Implicit parallelism (determined by the agent) | Explicit parallelism (`depends_on: [step_a, step_b]`) |
| One workflow = one agent session | One workflow = coordinated execution of N agents |

**Example:** A complex code review pipeline in goflow:
```yaml
steps:
  - id: security-scan
    agent: security-reviewer
    prompt: "Scan for vulnerabilities..."
    
  - id: performance-scan
    agent: performance-reviewer
    prompt: "Analyze performance..."
    depends_on: []  # Runs in parallel with security-scan
    
  - id: style-check
    agent: style-reviewer
    prompt: "Check code style..."
    depends_on: []  # Also parallel
    
  - id: aggregate
    agent: aggregator
    prompt: |
      Combine findings:
      Security: {{steps.security-scan.output}}
      Performance: {{steps.performance-scan.output}}
      Style: {{steps.style-check.output}}
    depends_on: [security-scan, performance-scan, style-check]  # Fan-in
    
  - id: decision
    agent: decision-maker
    prompt: "Based on: {{steps.aggregate.output}}, approve or reject?"
    depends_on: [aggregate]
```

This level of explicit orchestration is not natively supported by GitHub Agentic Workflows.

### 4. External System Integration

| GitHub Agentic Workflows | goflow |
|--------------------------|-----------------|
| Outputs are "Safe Outputs" mapped to GitHub API calls | Agents can call any MCP server, API, or tool |
| Network-isolated sandbox | Full network access (configurable) |
| Cannot directly interact with Slack, Jira, Salesforce, etc. | MCP servers enable direct interaction with any system |

**goflow + MCP = Universal Automation:**
- Connect to Slack/Teams to analyze team communications
- Query databases to extract business intelligence
- Call REST/GraphQL APIs to fetch or push data
- Interact with local file systems, cloud storage, or IoT devices

---

## When to Use Each Tool

### Use GitHub Agentic Workflows When:
- Your automation is purely GitHub-centric (issues, PRs, releases)
- You want zero infrastructure setup (runs in GitHub Actions)
- You need GitHub's sandboxed security model
- You are already deep in the GitHub ecosystem

### Use goflow When:
- You need platform independence (not tied to GitHub)
- You are automating workflows **outside** software development
- You need explicit, complex multi-agent orchestration (DAG patterns)
- You need to integrate with external systems via MCP
- You want to run workflows locally during development
- You need full audit trails and transparency
- You use GitLab, Bitbucket, Azure DevOps, or no VCS at all

---

## Overlap and Coexistence

There is some overlap in the "repository automation" space. Both tools can:
- Automate code reviews
- Triage issues
- Generate documentation
- Analyze CI failures

However, **the overlap is narrow**. GitHub Agentic Workflows is a feature for GitHub users who want simple, natural-language automation of repository tasks. goflow is a general-purpose engine for anyone who needs to orchestrate complex, multi-agent workflows across any domain or platform.

**They can coexist:** A team might use GitHub Agentic Workflows for simple daily triage inside GitHub, while using goflow for complex cross-system pipelines that span GitHub, Jira, Slack, and internal databases.

---

## Conclusion

goflow is not competing with GitHub Agentic Workflows — it operates at a different level of abstraction and serves a broader audience.

**GitHub Agentic Workflows** = "Easy AI automation for GitHub repositories"

**goflow** = "Universal AI workflow orchestration for any domain, any platform, any system"

The platform independence, explicit DAG orchestration, and MCP integration make goflow suitable for use cases that GitHub Agentic Workflows cannot address, including non-software domains like business operations, data pipelines, team communication analysis, and personal automation.

# Agent Skills

Skills are folders of instructions, scripts, and resources that teach Copilot how to perform specialized tasks. goflow can attach skills at the workflow level or per step, so every agent in your pipeline has access to the domain knowledge it needs.

Skills are an [open standard](https://github.com/agentskills/agentskills) supported by Copilot CLI, Copilot coding agent, and VS Code agent mode.

---

## How Skills Work

When Copilot decides a skill is relevant (based on the skill's description and your prompt), the `SKILL.md` file is injected into the agent's context. The agent then follows the instructions and can use any scripts or resources in the skill's directory.

In goflow, skills are declared in the workflow YAML. Copilot CLI discovers and loads them automatically during step execution.

---

## Skill File Format

Each skill lives in its own directory and **must** contain a `SKILL.md` file.

```
skills/
└── webapp-testing/
    ├── SKILL.md              # Required — instructions for Copilot
    ├── run-tests.sh          # Optional — helper scripts
    └── examples/             # Optional — reference examples
        └── test-pattern.md
```

### `SKILL.md` Structure

A `SKILL.md` file is Markdown with YAML frontmatter:

```markdown
---
name: webapp-testing
description: >
  Guide for running and debugging webapp test suites.
  Use this when asked to run tests, fix failing tests,
  or add test coverage.
---

# Webapp Testing Skill

Follow these steps when running tests:

1. Use `npm test` to run the full suite
2. On failure, read the failing test file and the source it covers
3. Check for common issues: missing mocks, async timing, snapshot drift
4. Fix the root cause, not just the assertion
```

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier. Lowercase, hyphens for spaces. Should match the directory name |
| `description` | Yes | What the skill does and **when** Copilot should use it. This is how Copilot decides relevance |
| `license` | No | License that applies to this skill |
| `applyTo` | No | Glob pattern limiting which files trigger this skill |

---

## Using Skills in goflow

### Workflow-Level Skills

Attach skills to the entire workflow. All steps gain access:

```yaml
name: "security-scan"
description: "Multi-scanner security audit"

skills:
  - "./skills/bandit-security-scan/SKILL.md"
  - "./skills/trivy-security-scan/SKILL.md"

steps:
  - id: scan
    agent: scanner
    prompt: "Run a security scan on the project"
```

### Step-Level Skills

Attach skills to individual steps only:

```yaml
steps:
  - id: scan-python
    agent: bandit-scanner
    prompt: "Run Bandit on all Python files"
    skills:
      - "./skills/bandit-security-scan/SKILL.md"

  - id: scan-deps
    agent: trivy-scanner
    prompt: "Scan dependencies for CVEs"
    skills:
      - "./skills/trivy-security-scan/SKILL.md"
```

---

## Skill Discovery Paths

Skills are loaded from standard Copilot CLI discovery paths:

| Location | Scope |
|----------|-------|
| `.github/skills/` | Repository (project-level) |
| `.claude/skills/` | Repository (Claude compatibility) |
| `.agents/skills/` | Repository (agent skills standard) |
| `~/.copilot/skills/` | Personal (all projects) |
| `~/.claude/skills/` | Personal (Claude compatibility) |
| `~/.agents/skills/` | Personal (agent skills standard) |

When you reference skills explicitly in the workflow YAML (via `skills:` field), goflow passed the paths directly. For implicit discovery, Copilot CLI searches the standard locations automatically.

---

## Example: Security Scan Skills

The [security-scan](../examples/security-scan.md) example uses five skills, one per scanner tool:

```
examples/security-scan/
└── skills/
    ├── bandit-security-scan/
    │   └── SKILL.md
    ├── guarddog-security-scan/
    │   └── SKILL.md
    ├── shellcheck-security-scan/
    │   └── SKILL.md
    ├── graudit-security-scan/
    │   └── SKILL.md
    └── trivy-security-scan/
        └── SKILL.md
```

Each skill teaches the agent how to use a specific security tool:

```markdown title="skills/trivy-security-scan/SKILL.md"
---
name: trivy-security-scan
description: >
  Comprehensive security scanner for filesystems, container images,
  and IaC. Detects known CVEs in dependencies, hardcoded secrets,
  and IaC misconfigurations.
applyTo: "**"
---

# Trivy Security Scan Skill

Trivy scans for CVEs, secrets, and misconfigurations.

Core commands: trivy fs --scanners vuln,secret ./
Filter by severity: trivy fs --severity HIGH,CRITICAL ./
IaC scan: trivy config ./
```

---

## Writing Effective Skills

### Description is Critical

The `description` field determines when Copilot loads the skill. Write it from Copilot's perspective — describe the **trigger conditions**, not just what the skill does:

```yaml
# ✅ Good — tells Copilot WHEN to use it
description: >
  Guide for debugging failing GitHub Actions workflows.
  Use this when asked to debug failing CI, fix build errors,
  or investigate workflow run failures.

# ❌ Vague — Copilot won't know when to activate it
description: "Helps with CI/CD"
```

### Include Concrete Commands

Skills work best when they give Copilot exact commands to run rather than general advice:

```markdown
## Running the Scan

1. Run `bandit -r ./src -f json -o bandit-report.json`
2. If bandit is not installed, run `pip install bandit` first
3. Parse the JSON output and group findings by severity
```

### Keep Skills Focused

One skill per task. Don't combine "testing" and "deployment" into a single skill — create separate ones so Copilot loads only what's relevant.

---

## Skills vs Custom Instructions

| Feature | Skills | Custom Instructions |
|---------|--------|---------------------|
| **Loaded** | Only when relevant (based on description match) | Always included in every prompt |
| **Scope** | Specific tasks or tools | Repository-wide conventions |
| **Format** | `SKILL.md` in a named directory | `.instructions.md` or `copilot-instructions.md` |
| **Best for** | Detailed tool guides, multi-step procedures | Coding standards, project context, build commands |

Use custom instructions for things every agent should know (coding standards, build commands). Use skills for specialized knowledge that only some agents need.

---

## Implementation Status

| Feature | Status |
|---------|--------|
| Workflow-level `skills` field | Parsed from YAML |
| Step-level `skills` field | Parsed from YAML |
| Passing skills to Copilot CLI | Via `--add-dir` (directories containing skills are added to CLI discovery) |
| Copilot CLI skill auto-discovery | Handled by Copilot CLI from standard paths |

!!! note
    The `skills` field in the workflow YAML is parsed and stored. Skills referenced via `--add-dir` step directories are passed to Copilot CLI for discovery. Copilot CLI handles the actual skill loading and injection into agent context.

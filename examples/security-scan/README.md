# Security Scan Workflow

Multi-scanner security audit that runs 5 security tools in parallel, aggregates findings, and generates a prioritized remediation plan.

## Architecture

```
discover → ┌─ scan-python (Bandit)
            ├─ scan-supply-chain (GuardDog)
            ├─ scan-shell (ShellCheck)        → aggregate → remediation-plan
            ├─ scan-patterns (Graudit)
            └─ scan-vulnerabilities (Trivy)
```

**Phase 1 — Discovery:** Classifies files by language, checks which security tools are installed, and identifies dependency manifests and IaC configs.

**Phase 2 — Parallel Scanning (fan-out):** Runs up to 5 scanners concurrently, each focused on a specific threat domain. Each scanner step has a **condition** that checks the discovery output for a `[RUN:X]` marker — if the tool is not installed or has no applicable files, the step is structurally skipped (never starts), saving tokens and time. Steps also use `on_error: continue` as a safety net.

**Phase 3 — Aggregation (fan-in):** Deduplicates findings across scanners, normalizes severity levels, and produces a unified security report.

**Phase 4 — Remediation Planning:** Generates a prioritized fix plan with SLA timelines, code fix patterns, and verification commands.

## Scanners

| Scanner | Skill | Detects |
|---------|-------|---------|
| [Bandit](https://bandit.readthedocs.io/) | `bandit-security-scan` | Python code vulnerabilities (eval, exec, SQL injection, hardcoded creds) |
| [GuardDog](https://github.com/DataDog/guarddog) | `guarddog-security-scan` | Malicious packages, supply chain attacks, typosquatting |
| [ShellCheck](https://www.shellcheck.net/) | `shellcheck-security-scan` | Shell script injection, unquoted variables, dangerous patterns |
| [Graudit](https://github.com/wireghoul/graudit) | `graudit-security-scan` | Multi-language pattern matching (secrets, command injection, XSS) |
| [Trivy](https://trivy.dev/) | `trivy-security-scan` | CVEs in dependencies, hardcoded secrets, IaC misconfigurations |

## Usage

```bash
# Scan current directory
workflow-runner run --workflow examples/security-scan/security-scan.yaml

# Scan a specific directory
workflow-runner run --workflow examples/security-scan/security-scan.yaml \
  --inputs target=./src

# Only report HIGH and CRITICAL findings
workflow-runner run --workflow examples/security-scan/security-scan.yaml \
  --inputs target=./src \
  --inputs severity=HIGH
```

## Installing Security Tools

The workflow gracefully handles missing tools — each scanner step skips if its tool is unavailable. For full coverage, install:

```bash
# Python security scanner
pip install bandit

# Supply chain scanner (requires Python 3.10+)
pip install guarddog

# Shell script analyzer
brew install shellcheck          # macOS
# apt-get install shellcheck     # Ubuntu/Debian

# Multi-language pattern scanner
git clone https://github.com/wireghoul/graudit ~/graudit
export PATH="$HOME/graudit:$PATH"

# Comprehensive CVE/secret/IaC scanner
brew install trivy               # macOS
# sudo apt-get install trivy     # Ubuntu/Debian
```

## Directory Structure

```
security-scan/
├── security-scan.yaml              # Workflow definition
├── README.md                       # This file
├── agents/
│   ├── discovery.agent.md          # File/tool discovery
│   ├── bandit-scanner.agent.md     # Python security scanning
│   ├── guarddog-scanner.agent.md   # Supply chain scanning
│   ├── shellcheck-scanner.agent.md # Shell script scanning
│   ├── graudit-scanner.agent.md    # Pattern-based scanning
│   ├── trivy-scanner.agent.md      # CVE/secret/IaC scanning
│   ├── security-aggregator.agent.md # Report aggregation
│   └── remediation-planner.agent.md # Fix planning
└── skills/
    ├── bandit-security-scan/SKILL.md
    ├── guarddog-security-scan/SKILL.md
    ├── shellcheck-security-scan/SKILL.md
    ├── graudit-security-scan/SKILL.md
    └── trivy-security-scan/SKILL.md
```

## Adapting the Workflow

Based on the [sec-check](https://github.com/alxayo/sec-check) security agent. The original sec-check repo includes additional scanners (ESLint, Checkov, OWASP Dependency-Check) that can be added by:

1. Creating a new agent file in `agents/`
2. Adding a streamlined skill in `skills/`
3. Adding a new parallel step in the workflow YAML with `depends_on: [discover]`
4. Adding the step's output reference to the aggregator's prompt

The workflow follows the sec-check safety model: agents are restricted to running approved scanner commands only and never execute code from the scanned project.

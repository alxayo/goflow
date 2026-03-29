# Security Scan Workflow

A focused security analysis workflow with controlled tool access.

---

## Overview

This example demonstrates:

- **Tool restrictions** — Limiting agent capabilities for security
- **Escalation patterns** — Conditional deep-dive on critical findings
- **Severity-based output** — Structured vulnerability reporting
- **Skills integration** — Using skill modules for specialized tasks

---

## The Workflow

```yaml title="examples/security-scan/security-scan.yaml"
name: "security-scan"
description: "Comprehensive security vulnerability scan with escalation"

inputs:
  files:
    description: "Files to scan (glob pattern)"
    default: "src/**/*.go"
  severity_threshold:
    description: "Minimum severity to report: low, medium, high, critical"
    default: "medium"

config:
  truncate:
    strategy: lines
    limit: 200

agents:
  scanner:
    inline:
      description: "Fast initial security scan"
      prompt: |
        You perform quick security scans to identify potential vulnerabilities.
        Focus on common issues: injection, auth, exposure.
        Mark findings as CRITICAL, HIGH, MEDIUM, or LOW.
      tools:
        - grep
        - semantic_search
        # Note: No run_in_terminal — scanner can't execute code

  deep-analyzer:
    inline:
      description: "Deep security analysis"
      prompt: |
        You perform detailed security analysis on specific vulnerabilities.
        Trace data flows, analyze attack vectors, assess exploitability.
        Provide proof-of-concept descriptions (not actual exploits).
      tools:
        - grep
        - semantic_search
        - read_file
        # Still no execution capabilities

  reporter:
    inline:
      description: "Security report generator"
      prompt: |
        You create actionable security reports for development teams.
        Prioritize by risk, provide clear remediation steps.
        Use standard vulnerability categories (CWE when applicable).

steps:
  # Phase 1: Quick scan
  - id: initial-scan
    agent: scanner
    prompt: |
      Perform a security scan of {{inputs.files}}.
      
      Look for:
      - SQL/Command injection
      - XSS vulnerabilities  
      - Hardcoded credentials
      - Insecure data handling
      - Missing input validation
      
      Severity threshold: {{inputs.severity_threshold}}
      
      Format each finding as:
      [SEVERITY] Category: Description
        Location: file:line
      
      If you find CRITICAL issues, clearly mark them.

  # Phase 2: Deep analysis (conditional)
  - id: deep-analysis
    agent: deep-analyzer
    prompt: |
      Critical vulnerabilities were found. Perform deep analysis:
      
      {{steps.initial-scan.output}}
      
      For each CRITICAL and HIGH finding:
      1. Trace the vulnerable data flow
      2. Assess exploitability (easy/moderate/difficult)
      3. Identify related vulnerable patterns
      4. Provide specific remediation code
    depends_on: [initial-scan]
    condition:
      step: initial-scan
      contains: "CRITICAL"

  # Phase 3: Report generation
  - id: report
    agent: reporter
    prompt: |
      Generate a security scan report:
      
      ## Scan Results
      {{steps.initial-scan.output}}
      
      ## Deep Analysis (if performed)
      {{steps.deep-analysis.output}}
      
      Create a report with:
      1. Executive Summary (2-3 sentences)
      2. Risk Assessment (overall exposure level)
      3. Critical Findings (table format)
      4. Remediation Priority List
      5. Quick Wins (easy fixes)
    depends_on: [initial-scan, deep-analysis]

output:
  steps: [report]
  format: markdown
```

---

## Execution Pattern

```
                          ┌─→ deep-analysis ─┐
initial-scan ─────────────┤                  ├─→ report
                          └──────────────────┘
                          (only if CRITICAL found)
```

---

## Key Security Patterns

### 1. Tool Restrictions

Agents are limited to read-only operations:

```yaml
agents:
  scanner:
    inline:
      tools:
        - grep
        - semantic_search
        # NO run_in_terminal
        # NO create_file
        # NO replace_string_in_file
```

**Why?** Security scanners shouldn't be able to:
- Execute arbitrary commands
- Modify files (could hide evidence)
- Make network calls (data exfiltration risk)

### 2. Escalation Pattern

Deep analysis is triggered only when needed:

```yaml
- id: deep-analysis
  condition:
    step: initial-scan
    contains: "CRITICAL"
```

This saves time and tokens on clean code.

### 3. Structured Severity Levels

Consistent severity markers enable filtering:

```yaml
prompt: |
  Mark findings as CRITICAL, HIGH, MEDIUM, or LOW.
```

Teams can filter by severity in reports.

---

## Running the Example

### Quick Scan

```bash
goflow run \
  --workflow examples/security-scan/security-scan.yaml \
  --inputs files='pkg/**/*.go' \
  --verbose
```

### Full Scan (Lower Threshold)

```bash
goflow run \
  --workflow examples/security-scan/security-scan.yaml \
  --inputs files='**/*.go' \
  --inputs severity_threshold='low' \
  --verbose
```

### Mock Mode (Structure Test)

```bash
goflow run \
  --workflow examples/security-scan/security-scan.yaml \
  --inputs files='pkg/*.go' \
  --mock \
  --verbose
```

---

## Sample Output

```markdown
# Security Scan Report

## Executive Summary

Security scan of pkg/**/*.go identified 3 issues: 
1 CRITICAL SQL injection, 1 HIGH credential exposure, 1 MEDIUM input validation.

## Risk Assessment

**Overall Exposure: HIGH** — Critical vulnerability requires immediate attention.

## Critical Findings

| Severity | Category | Location | Status |
|----------|----------|----------|--------|
| 🔴 CRITICAL | SQL Injection | db/query.go:45 | Unpatched |
| 🟠 HIGH | Credential | config/auth.go:12 | Unpatched |
| 🟡 MEDIUM | Input Validation | api/handler.go:89 | Unpatched |

## Remediation Priority

1. **db/query.go:45** — Use parameterized queries
2. **config/auth.go:12** — Move credentials to environment
3. **api/handler.go:89** — Add input sanitization

## Quick Wins

- [ ] Replace string concatenation with prepared statements
- [ ] Use environment variables for secrets
```

---

## Variations

### Add Compliance Check

```yaml
- id: compliance-check
  agent: compliance-reviewer
  prompt: "Check against OWASP Top 10..."
  depends_on: [initial-scan]
```

### Add Auto-Fix Suggestions

For trusted environments, add file modification capability:

```yaml
agents:
  fixer:
    inline:
      prompt: "Generate fix patches..."
      tools:
        - grep
        - read_file
        - create_file  # Creates patch files, not in-place edits
```

---

## See Also

- [Code Review Pipeline](code-review.md) — Broader review with multiple experts
- [Reference: Agent Format](../reference/agent-format.md) — Tool configuration

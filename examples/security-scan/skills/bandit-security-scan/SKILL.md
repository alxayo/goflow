---
name: bandit-security-scan
description: >
  Python AST-based security analysis using Bandit. Detects exec/eval code execution,
  pickle/yaml deserialization, subprocess shell injection, SQL injection, hardcoded
  credentials, weak cryptography. Use for Python security audits, malicious code triage.
applyTo: "**/*.py"
---

# Bandit Security Scanner Skill

Bandit is a security linter for Python code. It builds an AST from each file and runs plugins against the nodes to detect common vulnerabilities.

## Core Commands

```bash
# Scan directory recursively
bandit -r /path/to/project

# JSON output (recommended for parsing)
bandit -r . -f json -o bandit-results.json

# Only HIGH severity findings
bandit -r . -lll

# Only MEDIUM and above
bandit -r . -ll

# Run specific critical tests only
bandit -r . -t B102,B307,B602,B605,B301 -lll

# Exclude test directories
bandit -r . --exclude "*/tests/*,*/venv/*"
```

## Critical Detection Rules

| Test ID | Detection | Severity | MITRE ATT&CK |
|---------|-----------|----------|---------------|
| B102 | `exec()` usage | MEDIUM | T1059 |
| B307 | `eval()` usage | MEDIUM | T1059 |
| B602 | `subprocess(shell=True)` | HIGH | T1059.004 |
| B605 | `os.system()` | HIGH | T1059.004 |
| B301 | `pickle.load()` | MEDIUM | T1059 |
| B105-B107 | Hardcoded passwords | LOW | T1552.001 |
| B310 | `urllib.urlopen` | MEDIUM | T1071 |
| B312 | `telnetlib` | HIGH | T1071 |
| B501 | `request_with_no_cert_validation` | HIGH | T1557 |
| B506 | `yaml.load()` | MEDIUM | T1059 |
| B608 | Hardcoded SQL expressions | MEDIUM | T1190 |

## Recommended Workflows

**Quick triage (< 30 seconds):**
```bash
bandit -r . -t B102,B307,B602,B605 -lll
```

**Standard audit:**
```bash
bandit -r . -ll --exclude "*/tests/*,*/venv/*"
```

**Full malicious code scan:**
```bash
bandit -r . -f json -o full-scan.json
```

## Interpreting Results

- **HIGH severity + HIGH confidence**: Almost certainly a real issue
- **MEDIUM severity**: Likely an issue, verify context
- **LOW severity**: May be a false positive, review manually

## Common False Positives

| Test ID | Scenario | Mitigation |
|---------|----------|------------|
| B101 | `assert` in test files | `--exclude "*/tests/*"` |
| B311 | `random` for non-security use | Skip with `-s B311` |
| B105 | Variables named `password` that aren't credentials | Manual review |

## Limitations

- Static analysis only — cannot detect runtime vulnerabilities
- Python only — does not scan other languages
- No dependency scanning — use `guarddog` or `pip-audit` for that

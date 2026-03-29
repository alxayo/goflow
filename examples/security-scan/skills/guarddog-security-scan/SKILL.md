---
name: guarddog-security-scan
description: >
  Detect malicious packages and supply chain attacks in Python (PyPI) and Node.js (npm).
  Scans requirements.txt, package.json, package-lock.json for malware, typosquatting,
  data exfiltration, and compromised maintainers.
applyTo: "**/requirements*.txt,**/package.json,**/package-lock.json"
---

# GuardDog Supply Chain Security Skill

GuardDog by DataDog detects malicious PyPI and npm packages using Semgrep source code rules and metadata heuristics. It identifies malware, supply chain attacks, typosquatting, and compromised packages.

## Core Commands

```bash
# Verify Python dependencies (recommended first step)
guarddog pypi verify requirements.txt

# Verify npm dependencies
guarddog npm verify package-lock.json

# Scan local Python project source for malicious patterns
guarddog pypi scan /path/to/project/

# Scan local Node.js project source
guarddog npm scan /path/to/project/

# Check a specific package before installing
guarddog pypi scan <package-name>
guarddog npm scan <package-name>

# JSON output for automation
guarddog pypi scan /path --output-format=json
```

## Critical Detection Rules

| Rule | Ecosystem | Threat | MITRE ATT&CK |
|------|-----------|--------|---------------|
| `exec-base64` | Both | Obfuscated code execution | T1027, T1059 |
| `exfiltrate-sensitive-data` | Both | Steals credentials/keys | T1005, T1041 |
| `code-execution` | Python | OS commands in setup.py | T1059 |
| `download-executable` | Python | Downloads and runs malware | T1105 |
| `typosquatting` | Both | Impersonates popular packages | — |
| `npm-install-script` | npm | Malicious install hooks | T1059 |
| `npm-serialize-environment` | npm | Exfiltrates env vars | T1082, T1041 |
| `obfuscation` | Both | Deliberately obscured code | T1027 |
| `repository_integrity_mismatch` | Python | Package differs from source | — |

## Selective Scanning

```bash
# Scan with specific rules only
guarddog pypi scan /path --rules exec-base64 --rules code-execution

# Exclude specific rules
guarddog pypi scan /path --exclude-rules repository_integrity_mismatch
```

## Interpreting Results

```
Found 2 potentially malicious indicators:
  - exec-base64: Identified base64-encoded code execution in setup.py
  - exfiltrate-sensitive-data: Package reads SSH keys and sends to external URL
```

Each rule match indicates a potential malicious pattern that needs investigation.

## Limitations

- Semgrep-based detection may produce false positives
- Cannot detect logic vulnerabilities or runtime-only behavior
- Does not scan your own source code for vulnerabilities (use `bandit` or `graudit`)

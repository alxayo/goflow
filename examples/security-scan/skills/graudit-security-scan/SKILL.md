---
name: graudit-security-scan
description: >
  Multi-language pattern-based security scanner using grep signature databases.
  Primary use for secrets detection, command injection patterns, and first-pass
  security scans on unknown codebases. Supports 15+ languages.
applyTo: "**"
---

# Graudit Security Scan Skill

Graudit is a grep-based source code auditing tool that uses regex signature databases to detect dangerous patterns across many languages.

## Core Commands

```bash
# Scan with default database
graudit /path/to/scan

# Language-specific scan
graudit -d python /path/to/scan
graudit -d js /path/to/scan
graudit -d go /path/to/scan

# Critical: always run for untrusted code
graudit -d exec /path/to/scan     # Command injection, reverse shells
graudit -d secrets /path/to/scan  # Hardcoded credentials, API keys

# Additional databases
graudit -d sql /path/to/scan      # SQL injection
graudit -d xss /path/to/scan      # Cross-site scripting

# Exclude noise
graudit -x "*.min.js,node_modules/*,vendor/*,.git/*" -d secrets /path

# Show more context lines
graudit -c 3 -d exec /path/to/scan

# List available databases
graudit -l
```

## Available Databases

| Database | Detects |
|----------|---------|
| `exec` | Command injection, reverse shells, system calls, process spawning |
| `secrets` | Hardcoded credentials, API keys, tokens, passwords |
| `sql` | SQL injection patterns, unsafe query construction |
| `xss` | Cross-site scripting, DOM manipulation, unsafe output |
| `python` | Python-specific dangerous functions |
| `js` | JavaScript security issues |
| `go` | Go language security issues |
| `php` | PHP security flaws |
| `c` | C/C++ dangerous patterns |
| `ruby` | Ruby security patterns |
| `dotnet` | .NET/C# security issues |
| `default` | General security patterns |

## Malicious Code Indicators

| Pattern | Database | Threat |
|---------|----------|--------|
| `socket.connect`, `/dev/tcp/` | `exec` | Reverse shell |
| `base64.b64decode`, `eval(atob())` | `exec` | Obfuscation |
| `curl` + encoded data | `exec` | Data exfiltration |
| `crontab`, startup scripts | `exec` | Persistence |
| `/etc/passwd`, keychain | `secrets` | Credential theft |
| API keys, tokens | `secrets` | Exposed credentials |

## Interpreting Results

Output format: `filename:line_number:matched_line`

| Pattern Type | Typical Severity | Action |
|-------------|------------------|--------|
| `secrets` (real credentials) | CRITICAL | Immediate rotation |
| `exec` with user input | CRITICAL | Code remediation |
| `sql` string concatenation | HIGH | Parameterize queries |
| `xss` innerHTML | HIGH | Sanitize output |
| Generic `default` | VARIABLE | Manual review |

## Limitations

- Pattern matching (regex) — will produce false positives
- Cannot detect logic flaws or context-dependent vulnerabilities
- Cannot analyze obfuscated/encrypted code
- Does not execute code — purely static analysis
- For Python-specific deep analysis, prefer Bandit
- For shell script analysis, prefer ShellCheck

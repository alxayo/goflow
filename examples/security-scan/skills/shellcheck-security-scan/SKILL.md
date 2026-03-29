---
name: shellcheck-security-scan
description: >
  Shell script static analysis using ShellCheck. Detects command injection, unquoted
  variables, reverse shell patterns, dangerous rm operations, obfuscated payloads.
  Combine with graudit for patterns ShellCheck may miss.
applyTo: "**/*.sh,**/*.bash"
---

# ShellCheck Security Scan Skill

ShellCheck is a static analysis tool for shell scripts (bash, sh, dash, ksh). It identifies bugs, security vulnerabilities, and dangerous patterns.

## Core Commands

```bash
# Scan a single script
shellcheck script.sh

# Scan with all checks enabled and warning threshold
shellcheck --enable=all --severity=warning script.sh

# JSON output for parsing
shellcheck --enable=all --format=json script.sh

# Scan all scripts recursively
find . -name "*.sh" -exec shellcheck --enable=all --severity=warning {} +

# SARIF output for security tools
shellcheck --format=sarif --enable=all script.sh

# Specify shell dialect
shellcheck --shell=bash script.sh
```

## Security-Critical Checks

| Code | Pattern | Risk | MITRE ATT&CK |
|------|---------|------|---------------|
| SC2086 | Unquoted variable in rm/curl | Command injection | T1059.004 |
| SC2046 | Unquoted command substitution | Command injection | T1059.004 |
| SC2091 | Executing command output `$(...)` | Arbitrary code execution | T1059.004 |
| SC2115 | Empty var + rm | Root filesystem wipe | T1485 |
| SC2216 | Pipe to rm | Arbitrary file deletion | T1070.004 |
| SC2029 | SSH command injection | Remote command confusion | T1021.004 |
| SC2087 | Unquoted heredoc | Data injection | T1059.004 |

## Supplementary Pattern Detection

ShellCheck misses obfuscated/malicious patterns. Also check:

```bash
# Reverse shells
grep -rn --include="*.sh" -E "(bash\s+-i|/dev/tcp/|nc\s+(-e|-c)|mkfifo)" .

# Download-and-execute
grep -rn --include="*.sh" -E "(curl|wget).*\|.*(ba)?sh" .

# Data exfiltration
grep -rn --include="*.sh" -E "curl.*-d.*\$|wget.*--post-data" .

# Obfuscated payloads
grep -rn --include="*.sh" -E "(base64\s+(-d|--decode)).*\|\s*(ba)?sh" .

# Persistence mechanisms
grep -rn --include="*.sh" -E "(crontab|/etc/cron|systemctl.*enable|update-rc.d)" .
```

## Interpreting Results

- **error**: Definite bugs or syntax errors
- **warning**: Likely bugs or security-relevant patterns
- **info**: Suggestions for improvement
- **style**: Stylistic issues

## Triage Priority

1. **CRITICAL**: SC2091 (executing output), SC2115 (empty var + rm)
2. **HIGH**: SC2086/SC2046 with user input in rm/curl/eval
3. **MEDIUM**: General quoting issues, permission problems
4. **LOW**: Style issues, minor suggestions

## Limitations

- Cannot detect obfuscated reverse shells (base64 | bash)
- No taint tracking — misses data flow from untrusted sources
- Shell-only analysis — embedded Python/Perl/Ruby is ignored
- Cannot detect time bombs or environment-triggered payloads

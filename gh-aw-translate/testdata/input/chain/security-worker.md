---
name: Security Worker
on:
  workflow_call:
permissions:
  contents: read
engine: copilot
timeout-minutes: 15
tools:
  github:
    toolsets: [default, code_security]
safe-outputs:
  create-issue:
    title-prefix: "[security] "
    labels: [security, automated]
    max: 3
---

# Security Worker

Perform a security review of the repository.

Focus on:
- Dependency vulnerabilities
- Secret exposure risks
- Injection attack surfaces

Create issues for any findings with severity HIGH or CRITICAL.

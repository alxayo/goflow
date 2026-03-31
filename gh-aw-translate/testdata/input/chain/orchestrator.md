---
name: Orchestrator
on:
  workflow_dispatch:
permissions:
  contents: read
engine: copilot
timeout-minutes: 20
tools:
  github:
    toolsets: [default]
safe-outputs:
  call-workflow:
    workflows: [security-worker, perf-worker]
    max: 1
---

# Orchestrator

Analyze repository ${{ github.repository }} and decide which review to run.

If the repository contains Go code, call the security-worker.
If the repository has performance-sensitive code, call the perf-worker.

---
name: Perf Worker
on:
  workflow_call:
permissions:
  contents: read
engine: copilot
timeout-minutes: 15
tools:
  github:
    toolsets: [default]
  bash: ["go", "test", "benchmark"]
safe-outputs:
  create-issue:
    title-prefix: "[perf] "
    labels: [performance, automated]
    max: 3
---

# Performance Worker

Perform a performance review of the repository.

Focus on:
- Hot paths and algorithmic complexity
- Memory allocation patterns
- Benchmark regressions

Create issues for any findings with measurable impact.

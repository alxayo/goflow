---
name: Daily Status Report
on:
  schedule: daily on weekdays
permissions:
  contents: read
  issues: read
  pull-requests: read
engine: copilot
timeout-minutes: 15
tools:
  github:
    toolsets: [default]
  web-search:
safe-outputs:
  create-issue:
    title-prefix: "[team-status] "
    labels: [report, daily-status]
    close-older-issues: true
    max: 1
---

# Daily Status Report

Create an upbeat daily status report for the team as a GitHub issue.

## What to include

- Recent repository activity for ${{ github.repository }}
  - Issues opened/closed in the last 24 hours
  - PRs merged
  - Notable commits to main branch
- Progress tracking and highlights
- Actionable next steps for maintainers

## Style

Keep the tone positive and motivating. Use emoji sparingly but effectively.
Focus on what was accomplished, not just what's pending.

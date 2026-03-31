---
name: Issue Triage Bot
on:
  issues:
    types: [opened, reopened]
permissions:
  contents: read
  issues: read
engine:
  id: copilot
  model: gpt-5
  max-turns: 10
timeout-minutes: 10
tools:
  github:
    toolsets: [default]
  edit:
safe-outputs:
  add-labels:
    allowed: [bug, enhancement, question, documentation]
    max: 3
  add-comment:
    max: 1
---

# Issue Triage Bot

Analyze issue #${{ github.event.issue.number }} in repository ${{ github.repository }}.

The issue was created by ${{ github.actor }}.

## Your Tasks

1. Read the issue content using the GitHub tools
2. Categorize the issue type (bug, enhancement, question, documentation)
3. Add the appropriate label from the allowed list
4. Post a helpful triage comment with next steps

## Guidelines

- Be concise and helpful
- If unsure about categorization, default to "question"
- Always cite the specific part of the issue that informed your decision

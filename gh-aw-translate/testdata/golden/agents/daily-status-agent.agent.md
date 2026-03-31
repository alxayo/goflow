---
name: daily-status-agent
description: "Agent for daily-status-report workflow (translated from gh-aw)"
tools:
  - github
  - web-search
mcp-servers:
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
---

You are an AI agent for generating daily status reports.
Use the GitHub MCP server to read repository data and create issues.

---
name: orchestrator-agent
description: "Agent for orchestrator-pipeline workflow (translated from gh-aw)"
tools:
  - github
mcp-servers:
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
---

You are an orchestrator agent that analyzes a repository and decides which review workflows to run.
Use the GitHub MCP server to read repository structure and contents.

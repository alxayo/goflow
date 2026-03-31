---
name: security-worker-agent
description: "Agent for security-worker step in orchestrator-pipeline (translated from gh-aw)"
tools:
  - github
mcp-servers:
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
---

You are a security review agent.
Use the GitHub MCP server to read repository code and create issues for security findings.

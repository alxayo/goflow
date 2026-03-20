package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentMarkdown_FullFrontmatter(t *testing.T) {
	input := `---
name: security-reviewer
description: Reviews code for vulnerabilities
tools:
  - grep
  - view
agents:
  - helper-agent
model: gpt-5
mcp-servers:
  sec-tools:
    command: docker
    args: ["run", "security:latest"]
    env:
      TOKEN: abc
handoffs:
  - label: Send to Aggregator
    agent: aggregator
    prompt: "Aggregate findings..."
hooks:
  onPreToolUse: "check-permissions"
  onPostToolUse: "log-usage"
---

# Security Reviewer

You are an expert security reviewer.
`

	agent, err := ParseAgentMarkdown([]byte(input), "/path/to/security-reviewer.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Name", agent.Name, "security-reviewer")
	assertEqual(t, "Description", agent.Description, "Reviews code for vulnerabilities")
	assertEqual(t, "SourceFile", agent.SourceFile, "/path/to/security-reviewer.agent.md")

	assertSliceEqual(t, "Tools", agent.Tools, []string{"grep", "view"})
	assertSliceEqual(t, "Agents", agent.Agents, []string{"helper-agent"})
	assertSliceEqual(t, "Model.Models", agent.Model.Models, []string{"gpt-5"})

	if len(agent.MCPServers) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(agent.MCPServers))
	}
	srv := agent.MCPServers["sec-tools"]
	assertEqual(t, "MCPServers.command", srv.Command, "docker")
	assertSliceEqual(t, "MCPServers.args", srv.Args, []string{"run", "security:latest"})
	assertEqual(t, "MCPServers.env.TOKEN", srv.Env["TOKEN"], "abc")

	if len(agent.Handoffs) != 1 {
		t.Fatalf("expected 1 handoff, got %d", len(agent.Handoffs))
	}
	assertEqual(t, "Handoff.Label", agent.Handoffs[0].Label, "Send to Aggregator")
	assertEqual(t, "Handoff.Agent", agent.Handoffs[0].Agent, "aggregator")

	if agent.Hooks == nil {
		t.Fatal("expected hooks to be set")
	}
	assertEqual(t, "Hooks.OnPreToolUse", agent.Hooks.OnPreToolUse, "check-permissions")
	assertEqual(t, "Hooks.OnPostToolUse", agent.Hooks.OnPostToolUse, "log-usage")

	if agent.Prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(agent.Prompt, "expert security reviewer") {
		t.Errorf("prompt missing expected content, got: %q", agent.Prompt)
	}
}

func TestParseAgentMarkdown_MinimalNameOnly(t *testing.T) {
	input := `---
name: minimal-agent
---
`
	agent, err := ParseAgentMarkdown([]byte(input), "/agents/minimal-agent.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "Name", agent.Name, "minimal-agent")
	assertSliceEqual(t, "Tools", agent.Tools, []string{})
	assertSliceEqual(t, "Agents", agent.Agents, []string{})
	if agent.Model.Models != nil {
		t.Errorf("expected nil Models, got %v", agent.Model.Models)
	}
}

func TestParseAgentMarkdown_NoFrontmatter(t *testing.T) {
	input := `# Just Markdown

No frontmatter here.
`
	_, err := ParseAgentMarkdown([]byte(input), "/agents/bad.md")
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
	if !contains(err.Error(), "missing frontmatter") {
		t.Errorf("expected 'missing frontmatter' in error, got: %v", err)
	}
}

func TestParseAgentMarkdown_ModelAsString(t *testing.T) {
	input := `---
name: test
model: gpt-5
---
`
	agent, err := ParseAgentMarkdown([]byte(input), "/test.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSliceEqual(t, "Model.Models", agent.Model.Models, []string{"gpt-5"})
}

func TestParseAgentMarkdown_ModelAsArray(t *testing.T) {
	input := `---
name: test
model:
  - gpt-5
  - gpt-4
---
`
	agent, err := ParseAgentMarkdown([]byte(input), "/test.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSliceEqual(t, "Model.Models", agent.Model.Models, []string{"gpt-5", "gpt-4"})
}

func TestParseAgentMarkdown_DefaultNameFromFilename(t *testing.T) {
	tests := []struct {
		path     string
		wantName string
	}{
		{"/agents/perf-checker.agent.md", "perf-checker"},
		{"/agents/my-agent.md", "my-agent"},
	}
	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			input := `---
description: no name field
---
`
			agent, err := ParseAgentMarkdown([]byte(input), tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, "Name", agent.Name, tt.wantName)
		})
	}
}

func TestParseAgentMarkdown_UnknownFieldsIgnored(t *testing.T) {
	input := `---
name: test
totally_unknown_field: should be ignored
another_one: 42
---

Body content.
`
	agent, err := ParseAgentMarkdown([]byte(input), "/test.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "Name", agent.Name, "test")
}

func TestParseAgentMarkdown_EmptyBody(t *testing.T) {
	input := `---
name: empty-body
---`
	agent, err := ParseAgentMarkdown([]byte(input), "/test.agent.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "Prompt", agent.Prompt, "")
}

func TestLoadAgentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-agent.agent.md")
	content := `---
name: loaded-agent
tools:
  - bash
---

Hello from file.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	agent, err := LoadAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "Name", agent.Name, "loaded-agent")
	absPath, _ := filepath.Abs(path)
	assertEqual(t, "SourceFile", agent.SourceFile, absPath)
}

func TestLoadAgentFile_NotFound(t *testing.T) {
	_, err := LoadAgentFile("/nonexistent/agent.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- Claude normalization tests ---

func TestNormalizeClaudeAgent_CommaSeparatedTools(t *testing.T) {
	agent := &Agent{
		Name:       "claude-agent",
		Tools:      []string{"Read, Grep, Bash"},
		SourceFile: "/workspace/.claude/agents/review.md",
	}
	NormalizeClaudeAgent(agent)
	assertSliceEqual(t, "Tools", agent.Tools, []string{"view", "grep", "bash"})
}

func TestNormalizeClaudeAgent_MixedKnownUnknown(t *testing.T) {
	agent := &Agent{
		Name:       "claude-agent",
		Tools:      []string{"Read", "CustomTool", "Bash"},
		SourceFile: "/workspace/.claude/agents/review.md",
	}
	NormalizeClaudeAgent(agent)
	assertSliceEqual(t, "Tools", agent.Tools, []string{"view", "CustomTool", "bash"})
}

func TestNormalizeClaudeAgent_AlreadyProperArray(t *testing.T) {
	agent := &Agent{
		Name:       "claude-agent",
		Tools:      []string{"view", "grep"},
		SourceFile: "/workspace/.claude/agents/review.md",
	}
	NormalizeClaudeAgent(agent)
	// Unknown names stay as-is; view/grep are not in claudeToolMap so kept.
	assertSliceEqual(t, "Tools", agent.Tools, []string{"view", "grep"})
}

func TestNormalizeClaudeAgent_NonClaudePath(t *testing.T) {
	agent := &Agent{
		Name:       "normal-agent",
		Tools:      []string{"Read", "Grep"},
		SourceFile: "/workspace/.github/agents/review.agent.md",
	}
	NormalizeClaudeAgent(agent)
	// Non-Claude path: no changes.
	assertSliceEqual(t, "Tools", agent.Tools, []string{"Read", "Grep"})
}

// --- Helpers ---

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func assertSliceEqual(t *testing.T, field string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: length %d, want %d; got %v", field, len(got), len(want), got)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", field, i, got[i], want[i])
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

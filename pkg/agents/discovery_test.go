package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// writeAgentFile is a test helper that creates an .agent.md file with the
// given content and returns its absolute path.
func writeAgentFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("creating dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

const minimalAgent = `---
name: %s
tools:
  - grep
---

Prompt for %s.
`

func TestDiscoverAgents_GithubDir(t *testing.T) {
	workspace := t.TempDir()
	githubDir := filepath.Join(workspace, ".github", "agents")
	os.MkdirAll(githubDir, 0755)

	writeAgentFile(t, githubDir, "security.agent.md", `---
name: security
tools:
  - grep
---

Security prompt.
`)

	agents, err := DiscoverAgents(workspace, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := agents["security"]; !ok {
		t.Fatalf("expected agent 'security' to be discovered, got keys: %v", agentKeys(agents))
	}
	assertEqual(t, "Name", agents["security"].Name, "security")
}

func TestDiscoverAgents_ClaudeDirWithNormalization(t *testing.T) {
	workspace := t.TempDir()
	claudeDir := filepath.Join(workspace, ".claude", "agents")
	os.MkdirAll(claudeDir, 0755)

	writeAgentFile(t, claudeDir, "reviewer.md", `---
name: reviewer
tools:
  - Read
  - Grep
---

Claude reviewer prompt.
`)

	agents, err := DiscoverAgents(workspace, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agent, ok := agents["reviewer"]
	if !ok {
		t.Fatalf("expected agent 'reviewer', got keys: %v", agentKeys(agents))
	}
	// Tools should be normalized from Claude format.
	assertSliceEqual(t, "Tools", agent.Tools, []string{"view", "grep"})
}

func TestDiscoverAgents_GithubWinsOverClaude(t *testing.T) {
	workspace := t.TempDir()
	githubDir := filepath.Join(workspace, ".github", "agents")
	claudeDir := filepath.Join(workspace, ".claude", "agents")
	os.MkdirAll(githubDir, 0755)
	os.MkdirAll(claudeDir, 0755)

	writeAgentFile(t, githubDir, "reviewer.agent.md", `---
name: reviewer
description: GitHub version
tools:
  - view
---

GitHub reviewer.
`)
	writeAgentFile(t, claudeDir, "reviewer.md", `---
name: reviewer
description: Claude version
tools:
  - Read
---

Claude reviewer.
`)

	agents, err := DiscoverAgents(workspace, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agent := agents["reviewer"]
	assertEqual(t, "Description", agent.Description, "GitHub version")
}

func TestDiscoverAgents_EmptyMissingDirs(t *testing.T) {
	workspace := t.TempDir()
	agents, err := DiscoverAgents(workspace, []string{"/nonexistent/path"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected empty map, got %d agents", len(agents))
	}
}

func TestDiscoverAgents_AgentNameFromFrontmatterVsFilename(t *testing.T) {
	workspace := t.TempDir()
	githubDir := filepath.Join(workspace, ".github", "agents")
	os.MkdirAll(githubDir, 0755)

	// Agent with name in frontmatter.
	writeAgentFile(t, githubDir, "file-name.agent.md", `---
name: frontmatter-name
---

Prompt.
`)

	// Agent without name in frontmatter → uses filename.
	writeAgentFile(t, githubDir, "no-name.agent.md", `---
description: no name set
---

Prompt.
`)

	agents, err := DiscoverAgents(workspace, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := agents["frontmatter-name"]; !ok {
		t.Errorf("expected agent keyed by frontmatter name 'frontmatter-name', got: %v", agentKeys(agents))
	}
	if _, ok := agents["no-name"]; !ok {
		t.Errorf("expected agent keyed by filename 'no-name', got: %v", agentKeys(agents))
	}
}

func TestDiscoverAgents_ExtraPaths(t *testing.T) {
	workspace := t.TempDir()
	extraDir := t.TempDir()

	writeAgentFile(t, extraDir, "extra-agent.agent.md", `---
name: extra-agent
---

Extra agent prompt.
`)

	agents, err := DiscoverAgents(workspace, []string{extraDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := agents["extra-agent"]; !ok {
		t.Fatalf("expected agent 'extra-agent' from extra path, got: %v", agentKeys(agents))
	}
}

func TestResolveAgents_ExplicitFileHighestPriority(t *testing.T) {
	workspace := t.TempDir()
	githubDir := filepath.Join(workspace, ".github", "agents")
	os.MkdirAll(githubDir, 0755)

	// Discovered agent.
	writeAgentFile(t, githubDir, "reviewer.agent.md", `---
name: reviewer
description: discovered
---

Discovered.
`)

	// Explicit file agent.
	explicitPath := writeAgentFile(t, workspace, "custom-reviewer.agent.md", `---
name: explicit-reviewer
description: explicit
---

Explicit.
`)

	wf := &workflow.Workflow{
		Agents: map[string]workflow.AgentRef{
			"reviewer": {File: explicitPath},
		},
		Steps: []workflow.Step{
			{ID: "step1", Agent: "reviewer"},
		},
	}

	agents, err := ResolveAgents(wf, workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Explicit ref should override discovered.
	assertEqual(t, "Description", agents["reviewer"].Description, "explicit")
}

func TestResolveAgents_InlineAgent(t *testing.T) {
	workspace := t.TempDir()
	wf := &workflow.Workflow{
		Agents: map[string]workflow.AgentRef{
			"aggregator": {
				Inline: &workflow.InlineAgent{
					Description: "Aggregates reviews",
					Prompt:      "You are an aggregator",
					Tools:       []string{"grep", "view"},
					Model:       "gpt-5",
				},
			},
		},
		Steps: []workflow.Step{
			{ID: "agg-step", Agent: "aggregator"},
		},
	}

	agents, err := ResolveAgents(wf, workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agent := agents["aggregator"]
	assertEqual(t, "Name", agent.Name, "aggregator")
	assertEqual(t, "Description", agent.Description, "Aggregates reviews")
	assertEqual(t, "Prompt", agent.Prompt, "You are an aggregator")
	assertSliceEqual(t, "Tools", agent.Tools, []string{"grep", "view"})
	assertSliceEqual(t, "Model.Models", agent.Model.Models, []string{"gpt-5"})
}

func TestResolveAgents_MissingAgentError(t *testing.T) {
	workspace := t.TempDir()
	wf := &workflow.Workflow{
		Agents: map[string]workflow.AgentRef{},
		Steps: []workflow.Step{
			{ID: "step1", Agent: "nonexistent-agent"},
		},
	}

	_, err := ResolveAgents(wf, workspace)
	if err == nil {
		t.Fatal("expected error for missing agent reference")
	}
	if !contains(err.Error(), "unknown agent") {
		t.Errorf("expected 'unknown agent' in error, got: %v", err)
	}
}

func TestResolveAgents_RelativeFilePath(t *testing.T) {
	workspace := t.TempDir()
	writeAgentFile(t, workspace, "agents/custom.agent.md", `---
name: custom
---

Custom prompt.
`)

	wf := &workflow.Workflow{
		Agents: map[string]workflow.AgentRef{
			"custom": {File: "agents/custom.agent.md"},
		},
		Steps: []workflow.Step{
			{ID: "step1", Agent: "custom"},
		},
	}

	agents, err := ResolveAgents(wf, workspace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, "Name", agents["custom"].Name, "custom")
}

// agentKeys returns a slice of agent map keys for debugging.
func agentKeys(m map[string]*Agent) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Package agents: discovery.go implements multi-path agent file discovery with
// priority resolution. Agents are discovered from standard locations and merged
// with explicit references from the workflow YAML.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// DiscoverAgents scans all configured agent search paths and returns a map
// of agent-name → Agent. When agents with the same name exist in multiple
// locations, higher-priority sources win.
//
// Priority order (highest first):
//  1. .github/agents/*.agent.md and .github/agents/*.md
//  2. .claude/agents/*.md (Claude format, auto-normalized)
//  3. ~/.copilot/agents/*.agent.md
//  4. config.agent_search_paths entries
func DiscoverAgents(workspaceDir string, extraPaths []string) (map[string]*Agent, error) {
	agents := make(map[string]*Agent)

	// Scan in reverse priority order so higher-priority sources overwrite.

	// Priority 4 (lowest): extra search paths from config.
	for i := len(extraPaths) - 1; i >= 0; i-- {
		if err := scanDir(extraPaths[i], false, agents); err != nil {
			return nil, fmt.Errorf("scanning extra path %q: %w", extraPaths[i], err)
		}
	}

	// Priority 3: ~/.copilot/agents/
	homeDir, err := os.UserHomeDir()
	if err == nil {
		copilotDir := filepath.Join(homeDir, ".copilot", "agents")
		if err := scanDir(copilotDir, false, agents); err != nil {
			return nil, fmt.Errorf("scanning copilot agents dir: %w", err)
		}
	}

	// Priority 2: .claude/agents/ (Claude format with normalization).
	claudeDir := filepath.Join(workspaceDir, ".claude", "agents")
	if err := scanDir(claudeDir, true, agents); err != nil {
		return nil, fmt.Errorf("scanning claude agents dir: %w", err)
	}

	// Priority 1 (highest): .github/agents/
	githubDir := filepath.Join(workspaceDir, ".github", "agents")
	if err := scanDir(githubDir, false, agents); err != nil {
		return nil, fmt.Errorf("scanning github agents dir: %w", err)
	}

	return agents, nil
}

// scanDir loads all agent files from dir and adds them to the agents map.
// If the directory does not exist, it is silently skipped.
// If claude is true, loaded agents are normalized via NormalizeClaudeAgent.
func scanDir(dir string, claude bool, agents map[string]*Agent) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".agent.md") && !strings.HasSuffix(name, ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		agent, err := LoadAgentFile(path)
		if err != nil {
			return fmt.Errorf("loading agent %q: %w", path, err)
		}

		if claude {
			NormalizeClaudeAgent(agent)
		}

		agents[agent.Name] = agent
	}
	return nil
}

// ResolveAgents loads agents from explicit workflow YAML refs and merges them
// with discovered agents. Explicit refs always take highest priority.
// Inline agents are also resolved (created from InlineAgent definitions).
// Returns error if a step references an agent that cannot be found.
func ResolveAgents(wf *workflow.Workflow, workspaceDir string) (map[string]*Agent, error) {
	// Discover agents from standard locations.
	discovered, err := DiscoverAgents(workspaceDir, wf.Config.AgentSearchPaths)
	if err != nil {
		return nil, fmt.Errorf("discovering agents: %w", err)
	}

	// Load explicit file refs and inline agents from workflow YAML.
	// These take highest priority and overwrite discovered agents.
	for name, ref := range wf.Agents {
		if ref.File != "" {
			path := ref.File
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}
			agent, err := LoadAgentFile(path)
			if err != nil {
				return nil, fmt.Errorf("loading explicit agent %q from %q: %w", name, ref.File, err)
			}
			// Override name to match the key used in workflow YAML.
			agent.Name = name
			discovered[name] = agent
		} else if ref.Inline != nil {
			agent := &Agent{
				Name:        name,
				Description: ref.Inline.Description,
				Prompt:      ref.Inline.Prompt,
				Tools:       ref.Inline.Tools,
				Agents:      []string{},
			}
			if ref.Inline.Model != "" {
				agent.Model = ModelSpec{Models: []string{ref.Inline.Model}}
			}
			if agent.Tools == nil {
				agent.Tools = []string{}
			}
			discovered[name] = agent
		}
	}

	// Validate that every step references a resolvable agent.
	for _, step := range wf.Steps {
		if _, ok := discovered[step.Agent]; !ok {
			return nil, fmt.Errorf("step %q references unknown agent %q", step.ID, step.Agent)
		}
	}

	return discovered, nil
}

// Package agents: loader.go parses .agent.md files with YAML frontmatter and
// markdown body. The frontmatter is parsed into Agent fields, and the markdown
// body becomes the agent's system prompt.
package agents

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// rawAgent is an intermediate struct for YAML unmarshaling that allows the
// model field to be either a string or a list of strings.
type rawAgent struct {
	Name                   string                     `yaml:"name"`
	Description            string                     `yaml:"description"`
	Tools                  []string                   `yaml:"tools"`
	Agents                 []string                   `yaml:"agents"`
	Model                  interface{}                `yaml:"model"`
	MCPServers             map[string]MCPServerConfig `yaml:"mcp-servers"`
	Handoffs               []Handoff                  `yaml:"handoffs"`
	Hooks                  *HooksConfig               `yaml:"hooks,omitempty"`
	ArgumentHint           string                     `yaml:"argument-hint"`
	UserInvocable          *bool                      `yaml:"user-invocable"`
	DisableModelInvocation *bool                      `yaml:"disable-model-invocation"`
	Target                 string                     `yaml:"target"`
}

// claudeToolMap maps Claude tool names to VS Code equivalents.
var claudeToolMap = map[string]string{
	"Read":      "view",
	"Grep":      "grep",
	"Glob":      "glob",
	"Bash":      "bash",
	"Write":     "create_file",
	"Edit":      "replace_string_in_file",
	"MultiEdit": "multi_replace_string_in_file",
}

// LoadAgentFile parses a .agent.md file and returns an Agent.
func LoadAgentFile(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %q: %w", path, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path for %q: %w", path, err)
	}
	return ParseAgentMarkdown(data, absPath)
}

// ParseAgentMarkdown splits a markdown file into YAML frontmatter and body,
// parses the frontmatter into Agent fields, and stores the body as Prompt.
func ParseAgentMarkdown(data []byte, sourcePath string) (*Agent, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing agent file %q: %w", sourcePath, err)
	}

	var raw rawAgent
	if err := yaml.Unmarshal(frontmatter, &raw); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML in %q: %w", sourcePath, err)
	}

	agent := &Agent{
		Name:                   raw.Name,
		Description:            raw.Description,
		Tools:                  raw.Tools,
		Agents:                 raw.Agents,
		MCPServers:             raw.MCPServers,
		Handoffs:               raw.Handoffs,
		Hooks:                  raw.Hooks,
		Prompt:                 body,
		SourceFile:             sourcePath,
		ArgumentHint:           raw.ArgumentHint,
		UserInvocable:          raw.UserInvocable,
		DisableModelInvocation: raw.DisableModelInvocation,
		Target:                 raw.Target,
	}

	agent.Model, err = parseModelSpec(raw.Model)
	if err != nil {
		return nil, fmt.Errorf("parsing model field in %q: %w", sourcePath, err)
	}

	// Default name to filename stem if not set in frontmatter.
	if agent.Name == "" {
		agent.Name = agentNameFromPath(sourcePath)
	}

	// Ensure nil slices are empty slices for consistency.
	if agent.Tools == nil {
		agent.Tools = []string{}
	}
	if agent.Agents == nil {
		agent.Agents = []string{}
	}

	return agent, nil
}

// splitFrontmatter extracts YAML frontmatter delimited by "---" lines and
// the remaining markdown body from raw file data.
func splitFrontmatter(data []byte) (frontmatter []byte, body string, err error) {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return nil, "", fmt.Errorf("missing frontmatter")
	}

	closingIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			closingIdx = i
			break
		}
	}
	if closingIdx == -1 {
		return nil, "", fmt.Errorf("missing frontmatter")
	}

	fm := bytes.Join(lines[1:closingIdx], []byte("\n"))
	bodyLines := lines[closingIdx+1:]

	// Trim one leading empty line from the body if present.
	bodyStr := string(bytes.Join(bodyLines, []byte("\n")))
	bodyStr = strings.TrimLeft(bodyStr, "\n")

	return fm, bodyStr, nil
}

// parseModelSpec converts the raw model YAML value (string or []interface{})
// into a ModelSpec.
func parseModelSpec(raw interface{}) (ModelSpec, error) {
	if raw == nil {
		return ModelSpec{}, nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ModelSpec{}, nil
		}
		return ModelSpec{Models: []string{v}}, nil
	case []interface{}:
		models := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return ModelSpec{}, fmt.Errorf("model list contains non-string value: %v", item)
			}
			models = append(models, s)
		}
		return ModelSpec{Models: models}, nil
	default:
		return ModelSpec{}, fmt.Errorf("unsupported model type: %T", raw)
	}
}

// agentNameFromPath derives an agent name from the file path by stripping
// known extensions (.agent.md, .md).
func agentNameFromPath(path string) string {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".agent.md") {
		return strings.TrimSuffix(base, ".agent.md")
	}
	return strings.TrimSuffix(base, ".md")
}

// NormalizeClaudeAgent detects Claude-format conventions and maps them
// to VS Code-compatible fields.
// Only applies when the agent's SourceFile is under a .claude/agents/ directory.
// Heuristics:
//   - If tools contains comma-separated values in a single string, split into array
//   - Map tool names via claudeToolMap (known → mapped, unknown → kept as-is)
func NormalizeClaudeAgent(agent *Agent) {
	if !isClaudeAgent(agent) {
		return
	}

	// Expand comma-separated tool strings and map tool names.
	expanded := make([]string, 0, len(agent.Tools))
	for _, tool := range agent.Tools {
		if strings.Contains(tool, ",") {
			parts := strings.Split(tool, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					expanded = append(expanded, mapClaudeTool(p))
				}
			}
		} else {
			expanded = append(expanded, mapClaudeTool(strings.TrimSpace(tool)))
		}
	}
	agent.Tools = expanded
}

// isClaudeAgent returns true if the agent's SourceFile is under a
// .claude/agents/ directory.
func isClaudeAgent(agent *Agent) bool {
	// Normalize path separators for consistent matching.
	normalized := filepath.ToSlash(agent.SourceFile)
	return strings.Contains(normalized, ".claude/agents/")
}

// mapClaudeTool maps a single Claude tool name to its VS Code equivalent,
// returning the original name if no mapping exists.
func mapClaudeTool(name string) string {
	if mapped, ok := claudeToolMap[name]; ok {
		return mapped
	}
	return name
}

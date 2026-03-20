// Package agents handles loading, parsing, and discovery of .agent.md files
// compatible with the VS Code custom agents format. It also supports
// Claude-format agent files with automatic tool name mapping.
package agents

// Agent represents a fully resolved agent definition ready for session creation.
type Agent struct {
	Name        string                     `yaml:"name"`
	Description string                     `yaml:"description"`
	Tools       []string                   `yaml:"tools"`
	Agents      []string                   `yaml:"agents"`
	Model       ModelSpec                  `yaml:"-"`
	Prompt      string                     `yaml:"-"`
	MCPServers  map[string]MCPServerConfig `yaml:"mcp-servers"`
	Handoffs    []Handoff                  `yaml:"handoffs"`
	Hooks       *HooksConfig               `yaml:"hooks,omitempty"`
	SourceFile  string                     `yaml:"-"`

	// Fields parsed but ignored at runtime (interactive-only).
	ArgumentHint           string `yaml:"argument-hint"`
	UserInvocable          *bool  `yaml:"user-invocable"`
	DisableModelInvocation *bool  `yaml:"disable-model-invocation"`
	Target                 string `yaml:"target"`
}

// ModelSpec holds either a single model string or a priority list.
type ModelSpec struct {
	Models []string
}

// MCPServerConfig mirrors the VS Code MCP server definition.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// Handoff defines a transition from one agent to another.
type Handoff struct {
	Label  string `yaml:"label"`
	Agent  string `yaml:"agent"`
	Prompt string `yaml:"prompt"`
	Send   *bool  `yaml:"send,omitempty"`
	Model  string `yaml:"model,omitempty"`
}

// HooksConfig mirrors the VS Code agent hooks structure.
type HooksConfig struct {
	OnPreToolUse  string `yaml:"onPreToolUse"`
	OnPostToolUse string `yaml:"onPostToolUse"`
}

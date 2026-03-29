// Package workflow defines the core data model for workflow definitions,
// including steps, agent references, conditions, and configuration.
// It is the shared vocabulary used by the parser, DAG builder, template
// engine, executor, and orchestrator.
package workflow

// Workflow is the top-level representation of a parsed workflow YAML file.
type Workflow struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Inputs      map[string]Input    `yaml:"inputs"`
	Config      Config              `yaml:"config"`
	Agents      map[string]AgentRef `yaml:"agents"`
	Skills      []string            `yaml:"skills"`
	Steps       []Step              `yaml:"steps"`
	Output      OutputConfig        `yaml:"output"`
}

// Input defines a workflow-level input variable with optional default.
type Input struct {
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
}

// Config holds global workflow configuration.
type Config struct {
	Model            string             `yaml:"model"`
	AuditDir         string             `yaml:"audit_dir"`
	AuditRetention   int                `yaml:"audit_retention"`
	SharedMemory     SharedMemoryConfig `yaml:"shared_memory"`
	Provider         *ProviderConfig    `yaml:"provider,omitempty"`
	Streaming        bool               `yaml:"streaming"`
	LogLevel         string             `yaml:"log_level"`
	AgentSearchPaths []string           `yaml:"agent_search_paths"`
	MaxConcurrency   int                `yaml:"max_concurrency"`

	// Interactive enables the ask_user tool for all steps in this workflow,
	// allowing agents to pause and request clarification from the user.
	// When false (default), agents run autonomously without user interaction.
	Interactive bool `yaml:"interactive"`
}

// SharedMemoryConfig controls shared memory between parallel agents.
type SharedMemoryConfig struct {
	Enabled          bool   `yaml:"enabled"`
	InjectIntoPrompt bool   `yaml:"inject_into_prompt"`
	InitialContent   string `yaml:"initial_content"`
	InitialFile      string `yaml:"initial_file"`
}

// ProviderConfig holds BYOK provider settings.
type ProviderConfig struct {
	Type      string `yaml:"type"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}

// AgentRef is a reference to an agent — either a file path or an inline definition.
type AgentRef struct {
	File   string       `yaml:"file,omitempty"`
	Inline *InlineAgent `yaml:"inline,omitempty"`
}

// InlineAgent defines an agent directly in the workflow YAML.
type InlineAgent struct {
	Description string   `yaml:"description"`
	Prompt      string   `yaml:"prompt"`
	Tools       []string `yaml:"tools"`
	Model       string   `yaml:"model"`
}

// Step represents a single execution unit in the workflow DAG.
type Step struct {
	ID         string     `yaml:"id"`
	Agent      string     `yaml:"agent"`
	Prompt     string     `yaml:"prompt"`
	DependsOn  []string   `yaml:"depends_on"`
	Condition  *Condition `yaml:"condition,omitempty"`
	Skills     []string   `yaml:"skills"`
	OnError    string     `yaml:"on_error"`
	RetryCount int        `yaml:"retry_count"`
	Timeout    string     `yaml:"timeout"`

	// Model overrides the agent's model for this step only. If empty,
	// the agent's model is used; if the agent has no model, the workflow's
	// config.model is used; if that's also empty, the CLI picks the default.
	Model string `yaml:"model"`

	// ExtraDirs lists directories whose agents, skills, MCP servers,
	// instructions, and hooks should be discovered and added for this
	// step only, extending the CLI baseline.
	ExtraDirs []string `yaml:"extra_dirs"`

	// Interactive controls whether this step's agent can ask the user for
	// clarification via the ask_user tool. Uses a pointer to support
	// three states:
	//   - nil (unset): inherit from workflow config or CLI flag
	//   - true: enable user input for this step regardless of config
	//   - false: disable user input for this step regardless of config
	Interactive *bool `yaml:"interactive,omitempty"`
}

// Condition defines when a step should execute based on a prior step's output.
type Condition struct {
	Step        string `yaml:"step"`
	Contains    string `yaml:"contains,omitempty"`
	NotContains string `yaml:"not_contains,omitempty"`
	Equals      string `yaml:"equals,omitempty"`
}

// OutputConfig controls what gets reported after workflow completion.
type OutputConfig struct {
	Steps    []string        `yaml:"steps"`
	Format   string          `yaml:"format"`
	Truncate *TruncateConfig `yaml:"truncate,omitempty"`
}

// TruncateConfig limits the size of injected step outputs.
type TruncateConfig struct {
	Strategy string `yaml:"strategy"`
	Limit    int    `yaml:"limit"`
}

// StepStatus represents the current execution state of a step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusFailed    StepStatus = "failed"
)

// StepResult holds the outcome of executing a single step.
type StepResult struct {
	StepID    string     `json:"step_id"`
	Status    StepStatus `json:"status"`
	Output    string     `json:"output"`
	Error     error      `json:"-"`
	ErrorMsg  string     `json:"error,omitempty"`
	StartedAt string     `json:"started_at"`
	EndedAt   string     `json:"ended_at"`
	SessionID string     `json:"session_id,omitempty"`
}

// IsInteractive resolves whether a specific step should allow user input.
// The resolution priority (highest to lowest) is:
//  1. Step-level Interactive field (explicit per-step override)
//  2. Workflow-level config.interactive setting
//  3. CLI-level --interactive flag
//
// If the step has an explicit Interactive value (non-nil), it takes precedence.
// Otherwise, user input is enabled if either the workflow config or CLI flag
// enables it. This allows users to opt in globally while opting out specific
// steps, or vice versa.
func IsInteractive(step Step, wfInteractive, cliInteractive bool) bool {
	// Step-level override takes highest precedence when explicitly set.
	if step.Interactive != nil {
		return *step.Interactive
	}
	// Fall back to workflow config or CLI flag — either one enables it.
	return wfInteractive || cliInteractive
}

// BoolPtr returns a pointer to the given bool value.
// This is a convenience helper for constructing Step literals with an
// explicit Interactive field in tests and YAML parsing.
func BoolPtr(b bool) *bool {
	return &b
}

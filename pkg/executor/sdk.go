// Package executor handles the execution of individual workflow steps via
// the Copilot SDK. It defines a SessionExecutor interface that abstracts
// the SDK lifecycle, allowing testability through mock implementations.
package executor

import "context"

// SessionExecutor abstracts the Copilot SDK session lifecycle.
// This interface allows testing without a real SDK/CLI connection.
type SessionExecutor interface {
	// CreateSession starts a new SDK session with the given configuration.
	CreateSession(ctx context.Context, cfg SessionConfig) (Session, error)
}

// Session represents an active SDK session.
type Session interface {
	// Send submits a prompt and blocks until the session reaches idle.
	// Returns the final assistant message content.
	Send(ctx context.Context, prompt string) (string, error)

	// SessionID returns the unique session identifier (for resume support).
	SessionID() string

	// Close terminates the session and releases resources.
	Close() error
}

// SessionConfig holds the configuration for creating a new SDK session.
type SessionConfig struct {
	SystemPrompt string

	// Models is a priority-ordered list of model names to try. The executor
	// attempts each model in order; if unavailable, it falls back to the next.
	// If all models fail or the list is empty, the CLI picks the default model.
	Models []string

	Tools      []string
	MCPServers map[string]interface{}

	// ExtraDirs lists directories whose agents, skills, MCP servers,
	// instructions, and hooks are added to CLI discovery for this session.
	ExtraDirs []string
}

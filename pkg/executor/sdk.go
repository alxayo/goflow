// Package executor handles the execution of individual workflow steps via
// the Copilot SDK. It defines a SessionExecutor interface that abstracts
// the SDK lifecycle, allowing testability through mock implementations.
package executor

import "context"

// UserInputHandler is a callback function that the executor calls when the
// LLM requests clarification from the user during step execution.
//
// The handler receives:
//   - question: the text the LLM wants to ask the user
//   - choices: optional list of predefined answer choices (may be empty)
//
// The handler should present the question to the user (e.g., print to
// terminal), wait for their answer, and return it. Blocking until the
// user provides input is expected and normal.
//
// If the handler returns an error (e.g., stdin is closed, user presses
// Ctrl+C), the step execution will fail with that error.
type UserInputHandler func(question string, choices []string) (answer string, err error)

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

	// Interactive enables the ask_user tool for this session, allowing the
	// LLM to pause execution and request clarification from the user.
	// When false (default), the CLI runs with --no-ask-user, suppressing
	// any user interaction.
	Interactive bool

	// OnUserInput is the callback invoked when the LLM uses the ask_user
	// tool to request clarification. It is only used when Interactive is
	// true. If Interactive is true but OnUserInput is nil, user input
	// requests will fail with an error.
	OnUserInput UserInputHandler
}

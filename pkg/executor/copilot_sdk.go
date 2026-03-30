// copilot_sdk.go provides a SessionExecutor implementation backed by the
// Copilot SDK Go library. It communicates with the Copilot CLI via JSON-RPC,
// unlocking BYOK provider support, streaming events, session resume, and
// efficient single-process management compared to the subprocess-per-call
// approach in copilot_cli.go.
//
// The SDK wraps the CLI runtime — all built-in tools (grep, view,
// semantic_search, etc.) remain available regardless of provider choice.
package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	copilot "github.com/github/copilot-sdk/go"
)

// CopilotSDKExecutor uses the Copilot SDK Go library for session management.
// It supports BYOK providers (OpenAI, Anthropic, Azure, Ollama) while
// preserving access to all CLI built-in tools via the underlying CLI runtime.
type CopilotSDKExecutor struct {
	// client is the shared SDK client. Created once, reused across sessions.
	// The SDK manages the CLI process lifecycle internally via JSON-RPC.
	client *copilot.Client

	// provider holds the BYOK provider configuration from the workflow YAML.
	// Nil means "use GitHub Models" (the default Copilot provider).
	provider *ProviderConfig
}

// ProviderConfig holds BYOK provider settings for the SDK executor.
// It mirrors workflow.ProviderConfig to avoid a circular import between
// the executor and workflow packages. The main.go maps between them.
type ProviderConfig struct {
	Type      string // "openai", "anthropic", "azure", "ollama"
	BaseURL   string // e.g., "https://api.openai.com/v1"
	APIKeyEnv string // env var name holding the API key (never the key itself)
}

// NewCopilotSDKExecutor creates an SDK-backed executor with optional BYOK provider.
// If provider is nil, the SDK uses GitHub Models (default Copilot provider) —
// you still get streaming, session resume, and JSON-RPC efficiency.
//
// When provider is non-nil, the API key is resolved from the environment variable
// named in provider.APIKeyEnv. It is never stored in the struct — only passed to
// the SDK client at construction time.
func NewCopilotSDKExecutor(provider *ProviderConfig) (*CopilotSDKExecutor, error) {
	if provider != nil {
		if provider.Type == "" {
			return nil, errors.New("BYOK provider type is required (openai, anthropic, azure, ollama)")
		}
		// Validate API key from environment. API keys are optional for local
		// providers like Ollama, so only require it when the env var name is
		// explicitly set.
		if provider.APIKeyEnv != "" {
			if os.Getenv(provider.APIKeyEnv) == "" {
				return nil, fmt.Errorf("BYOK provider %q: env var %q is empty or not set",
					provider.Type, provider.APIKeyEnv)
			}
		}
	}

	// Create SDK client. AutoStart is true by default, so the CLI process
	// is spawned automatically on first CreateSession call.
	client := copilot.NewClient(nil)

	// Start the client explicitly so we fail fast if the CLI is missing.
	if err := client.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("starting Copilot SDK client: %w", err)
	}

	return &CopilotSDKExecutor{
		client:   client,
		provider: provider,
		// byokProvider is stored for use in CreateSession via closure, not as a
		// field — we pass it per-session to support future per-step providers.
		// For now it's captured in the executor and applied to every session.
	}, nil
}

// sdkProviderConfig returns the copilot.ProviderConfig for session creation,
// or nil if no BYOK provider is configured.
func (e *CopilotSDKExecutor) sdkProviderConfig() *copilot.ProviderConfig {
	if e.provider == nil {
		return nil
	}
	// Re-resolve API key each time to pick up env changes (unlikely but safe).
	var apiKey string
	if e.provider.APIKeyEnv != "" {
		apiKey = os.Getenv(e.provider.APIKeyEnv)
	}
	return &copilot.ProviderConfig{
		Type:    e.provider.Type,
		BaseURL: e.provider.BaseURL,
		APIKey:  apiKey,
	}
}

// CreateSession creates a new SDK session with the given configuration.
// Model fallback is handled at the CreateSession level: each model in the
// priority list is tried in order, creating a new session for each attempt.
func (e *CopilotSDKExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
	models := append([]string{}, cfg.Models...)
	if len(models) == 0 {
		models = []string{""} // empty = SDK/CLI default model
	}

	var lastErr error
	for _, model := range models {
		sdkCfg := e.buildSessionConfig(cfg, model)
		sdkSession, err := e.client.CreateSession(ctx, sdkCfg)
		if err != nil {
			if isSDKModelUnavailable(err) {
				lastErr = err
				continue
			}
			return nil, fmt.Errorf("creating SDK session: %w", err)
		}
		return &CopilotSDKSession{
			session: sdkSession,
			cfg:     cfg,
			models:  models,
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all models unavailable: %w", lastErr)
	}
	return nil, errors.New("no models configured and SDK default failed")
}

// buildSessionConfig maps our SessionConfig to the SDK's SessionConfig for a
// specific model.
func (e *CopilotSDKExecutor) buildSessionConfig(cfg SessionConfig, model string) *copilot.SessionConfig {
	sdkCfg := &copilot.SessionConfig{
		// Approve all tool permission requests automatically. Workflow steps
		// run autonomously; pausing for permission would break the DAG.
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	}

	if model != "" {
		sdkCfg.Model = model
	}

	// System prompt injection via SystemMessage config. We use "append" mode
	// so the agent's prompt is added after the SDK's built-in system sections.
	if cfg.SystemPrompt != "" {
		sdkCfg.SystemMessage = &copilot.SystemMessageConfig{
			Content: cfg.SystemPrompt,
		}
	}

	// Tool restriction: if the agent specifies an explicit tools list,
	// restrict the session to only those tools. Otherwise all CLI built-in
	// tools remain available (default).
	if len(cfg.Tools) > 0 {
		sdkCfg.AvailableTools = cfg.Tools
	}

	// MCP servers from agent config.
	if len(cfg.MCPServers) > 0 {
		mcpServers := make(map[string]copilot.MCPServerConfig, len(cfg.MCPServers))
		for name, config := range cfg.MCPServers {
			if m, ok := config.(map[string]interface{}); ok {
				mcpServers[name] = copilot.MCPServerConfig(m)
			}
		}
		sdkCfg.MCPServers = mcpServers
	}

	// Extra directories for per-step resource discovery. The SDK exposes
	// this via SkillDirectories (which also triggers agent/MCP discovery).
	if len(cfg.ExtraDirs) > 0 {
		sdkCfg.SkillDirectories = cfg.ExtraDirs
	}

	// BYOK provider configuration.
	sdkCfg.Provider = e.sdkProviderConfig()

	// Interactive mode: wire up the user input handler so the LLM can
	// use the ask_user tool to request clarification.
	if cfg.Interactive && cfg.OnUserInput != nil {
		handler := cfg.OnUserInput
		sdkCfg.OnUserInputRequest = func(req copilot.UserInputRequest, inv copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
			question := req.Question
			if question == "" {
				question = "Please provide the information needed for this step."
			}
			answer, err := handler(question, req.Choices)
			if err != nil {
				return copilot.UserInputResponse{}, err
			}
			return copilot.UserInputResponse{
				Answer: answer,
			}, nil
		}
	}

	return sdkCfg
}

// CopilotSDKSession wraps an SDK session to satisfy our Session interface.
type CopilotSDKSession struct {
	session *copilot.Session
	cfg     SessionConfig
	models  []string
}

// Send submits a prompt via the SDK and blocks until the session reaches idle.
// Returns the final assistant message content.
func (s *CopilotSDKSession) Send(ctx context.Context, prompt string) (string, error) {
	// For non-interactive mode, compose system + user prompt into a single
	// message. The SDK separately handles system prompts via SessionConfig,
	// but we also compose here for consistency with CLI executor behavior
	// and to support template-resolved prompts.
	finalPrompt := prompt

	// Interactive steps: the SDK's OnUserInputRequest handler was set in
	// buildSessionConfig, so the LLM can natively ask the user. The SDK
	// handles the ask_user flow internally — no pre-ask pattern needed.

	// SendAndWait blocks until the session reaches idle and returns the
	// final assistant message event.
	event, err := s.session.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: finalPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("SDK session send: %w", err)
	}

	// Extract the assistant message content from the response event.
	output := extractSDKOutput(event)
	if output == "" {
		return "", errors.New("SDK session returned empty output")
	}
	return output, nil
}

// extractSDKOutput extracts the text content from a session event.
func extractSDKOutput(event *copilot.SessionEvent) string {
	if event == nil {
		return ""
	}
	// The Data.Content field holds the assistant's text response.
	if event.Data.Content != nil {
		return strings.TrimSpace(*event.Data.Content)
	}
	return ""
}

// SessionID returns the unique session identifier for resume support.
func (s *CopilotSDKSession) SessionID() string {
	return s.session.SessionID
}

// Close terminates the session and releases resources.
func (s *CopilotSDKSession) Close() error {
	if s.session != nil {
		return s.session.Disconnect()
	}
	return nil
}

// Close releases the SDK client resources. Should be called when the
// workflow run completes (via defer in main.go).
func (e *CopilotSDKExecutor) Close() error {
	if e.client != nil {
		return e.client.Stop()
	}
	return nil
}

// isSDKModelUnavailable checks if an error indicates the requested model
// is not available. This mirrors isModelUnavailableError in copilot_cli.go
// but for SDK-returned errors.
func isSDKModelUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not available") ||
		strings.Contains(msg, "model not found") ||
		strings.Contains(msg, "does not exist")
}

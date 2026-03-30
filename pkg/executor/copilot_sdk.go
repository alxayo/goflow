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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	// We wrap the handler to emit stream events for the audit trail,
	// enabling TUI/CLI to show context when the LLM asks for input.
	if cfg.Interactive && cfg.OnUserInput != nil {
		handler := cfg.OnUserInput
		onProgress := cfg.OnProgress
		stepID := cfg.StepID
		sdkCfg.OnUserInputRequest = func(req copilot.UserInputRequest, inv copilot.UserInputInvocation) (copilot.UserInputResponse, error) {
			question := req.Question
			if question == "" {
				question = "Please provide the information needed for this step."
			}

			// Emit user.input_requested event for the audit trail.
			// This allows TUIs to show the stream context before the question.
			if onProgress != nil {
				onProgress(SessionEventInfo{
					Type:      "user.input_requested",
					StepID:    stepID,
					SessionID: "", // Session ID not available here, filled by monitor if needed
					Timestamp: time.Now(),
					Data: UserInputRequest{
						Prompt:  question,
						Choices: req.Choices,
					},
				})
			}

			answer, err := handler(question, req.Choices)
			if err != nil {
				return copilot.UserInputResponse{}, err
			}

			// Emit user.input_response event for the audit trail.
			if onProgress != nil {
				onProgress(SessionEventInfo{
					Type:      "user.input_response",
					StepID:    stepID,
					SessionID: "",
					Timestamp: time.Now(),
					Data:      answer,
				})
			}

			return copilot.UserInputResponse{
				Answer: answer,
			}, nil
		}
	}

	// Streaming mode: enable real-time event delivery for progress monitoring.
	// When true, the SDK emits assistant.message_delta events as the LLM
	// generates output, plus detailed tool execution events.
	sdkCfg.Streaming = cfg.Streaming

	return sdkCfg
}

// CopilotSDKSession wraps an SDK session to satisfy our Session interface.
type CopilotSDKSession struct {
	session *copilot.Session
	cfg     SessionConfig
	models  []string
}

// Send submits a prompt via the SDK and waits for the session to become idle.
// Returns the final assistant message content.
//
// Event-Based Monitoring:
// Instead of relying on SendAndWait's timeout, this method uses the SDK's event
// system to detect completion. The session stays alive indefinitely until:
//   - session.idle event is received (success)
//   - session.error event is received (failure)
//   - The context is cancelled (caller-initiated abort)
//
// This approach eliminates timeout configuration requirements and provides
// real-time progress visibility when Streaming is enabled in SessionConfig.
func (s *CopilotSDKSession) Send(ctx context.Context, prompt string) (string, error) {
	// Create monitor for progress tracking
	monitor := NewSessionMonitor(s.cfg.StepID, s.session.SessionID, s.cfg.OnProgress)

	// Channels for completion signaling
	doneCh := make(chan struct{})
	var finalOutput string
	var sendErr error

	// Register event handler for session monitoring
	unsubscribe := s.session.On(func(event copilot.SessionEvent) {
		s.handleSessionEvent(event, monitor, &finalOutput, &sendErr, doneCh)
	})
	defer unsubscribe()

	// Non-blocking send - the event handler will signal completion
	// Send returns a message ID which we don't need.
	_, err := s.session.Send(ctx, copilot.MessageOptions{
		Prompt: prompt,
	})
	if err != nil {
		return "", fmt.Errorf("sending prompt: %w", err)
	}

	monitor.EmitTurnStart()

	// Wait for completion OR context cancellation
	// Note: No timeout here - we rely on events to signal completion
	select {
	case <-doneCh:
		if sendErr != nil {
			return "", sendErr
		}
		if finalOutput == "" {
			// Fallback: try to get output from streamed text
			if streamed := monitor.GetStreamedText(); streamed != "" {
				return strings.TrimSpace(streamed), nil
			}
			return "", errors.New("session completed but returned empty output")
		}
		return finalOutput, nil

	case <-ctx.Done():
		// Context cancelled - could be user interrupt or caller-set deadline
		return "", fmt.Errorf("session interrupted: %w", ctx.Err())
	}
}

// handleSessionEvent processes SDK events and updates the monitor state.
func (s *CopilotSDKSession) handleSessionEvent(
	event copilot.SessionEvent,
	monitor *SessionMonitor,
	finalOutput *string,
	sendErr *error,
	doneCh chan struct{},
) {
	switch event.Type {
	case copilot.SessionEventTypeAssistantTurnStart:
		monitor.EmitTurnStart()

	case copilot.SessionEventTypeAssistantTurnEnd:
		monitor.EmitTurnEnd()

	case copilot.SessionEventTypeAssistantMessage:
		// Final assistant message - capture the full content
		if event.Data.Content != nil {
			*finalOutput = strings.TrimSpace(*event.Data.Content)
		}

	case copilot.SessionEventTypeAssistantMessageDelta:
		// Streaming text delta
		if event.Data.DeltaContent != nil {
			monitor.AppendStreamedText(*event.Data.DeltaContent)
		}

	case copilot.SessionEventTypeToolExecutionStart:
		// Tool starting execution
		toolName := ""
		if event.Data.ToolName != nil {
			toolName = *event.Data.ToolName
		}
		argsJSON := ""
		if event.Data.Arguments != nil {
			// Arguments is interface{}, marshal to string for logging
			if argsBytes, err := marshalJSONSafe(event.Data.Arguments); err == nil {
				argsJSON = string(argsBytes)
			}
		}
		monitor.StartToolCall(toolName, argsJSON)

	case copilot.SessionEventTypeToolExecutionComplete:
		// Tool finished execution
		toolName := ""
		if event.Data.ToolName != nil {
			toolName = *event.Data.ToolName
		}
		result := ""
		if event.Data.Result != nil && event.Data.Result.Content != nil {
			result = *event.Data.Result.Content
		}
		monitor.CompleteToolCall(toolName, result, "completed")

	case copilot.SessionEventTypeSessionIdle:
		// Session completed successfully
		monitor.EmitIdle()
		safeClose(doneCh)

	case copilot.SessionEventTypeSessionError:
		// Session error occurred
		errMsg := "unknown session error"
		if event.Data.Message != nil {
			errMsg = *event.Data.Message
		}
		monitor.SetError(errMsg)
		*sendErr = fmt.Errorf("session error: %s", errMsg)
		safeClose(doneCh)

	case copilot.SessionEventTypeSubagentStarted:
		// Subagent delegation started
		if s.cfg.OnProgress != nil {
			agentName := ""
			if event.Data.AgentName != nil {
				agentName = *event.Data.AgentName
			}
			s.cfg.OnProgress(SessionEventInfo{
				Type:      "subagent.started",
				StepID:    s.cfg.StepID,
				SessionID: s.session.SessionID,
				Timestamp: event.Timestamp,
				Data:      map[string]string{"agent": agentName},
			})
		}

	case copilot.SessionEventTypeSubagentCompleted:
		// Subagent delegation completed
		if s.cfg.OnProgress != nil {
			agentName := ""
			if event.Data.AgentName != nil {
				agentName = *event.Data.AgentName
			}
			s.cfg.OnProgress(SessionEventInfo{
				Type:      "subagent.completed",
				StepID:    s.cfg.StepID,
				SessionID: s.session.SessionID,
				Timestamp: event.Timestamp,
				Data:      map[string]string{"agent": agentName},
			})
		}
	}
}

// marshalJSONSafe marshals an interface{} value to JSON bytes.
// Returns an empty byte slice on error rather than failing.
func marshalJSONSafe(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	return json.Marshal(v)
}

// safeClose closes a channel only if it hasn't been closed already.
func safeClose(ch chan struct{}) {
	select {
	case <-ch:
		// Already closed
	default:
		close(ch)
	}
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

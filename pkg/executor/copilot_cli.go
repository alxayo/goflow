// copilot_cli.go provides a real SessionExecutor implementation backed by the
// local Copilot CLI in non-interactive prompt mode.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// CopilotCLIExecutor executes prompts via the local `copilot` CLI.
type CopilotCLIExecutor struct {
	// BinaryPath is the CLI binary to invoke. Defaults to "copilot" when empty.
	BinaryPath string
}

var copilotSessionCounter atomic.Uint64

// CreateSession returns a lightweight session wrapper that runs one CLI command
// per Send call.
func (e *CopilotCLIExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
	binary := e.BinaryPath
	if binary == "" {
		binary = "copilot"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("copilot CLI not found (%q): %w", binary, err)
	}
	id := fmt.Sprintf("copilot-cli-%d", copilotSessionCounter.Add(1))
	return &CopilotCLISession{
		binaryPath: binary,
		cfg:        cfg,
		sessionID:  id,
	}, nil
}

// CopilotCLISession maps Session calls to copilot CLI process execution.
type CopilotCLISession struct {
	binaryPath string
	cfg        SessionConfig
	sessionID  string
}

// Send executes a single non-interactive Copilot CLI prompt.
// If multiple models are configured, it tries each in order until one succeeds.
func (s *CopilotCLISession) Send(ctx context.Context, prompt string) (string, error) {
	finalPrompt := composePrompt(s.cfg.SystemPrompt, prompt)

	// Try each model in the priority list, then fall back to CLI default.
	modelsToTry := append([]string{}, s.cfg.Models...)
	modelsToTry = append(modelsToTry, "") // Empty string = CLI default model

	var lastErr error
	for _, model := range modelsToTry {
		output, err := s.tryWithModel(ctx, finalPrompt, model)
		if err == nil {
			return output, nil
		}
		// If this model is unavailable, try the next one.
		if isModelUnavailableError(err) {
			lastErr = err
			continue
		}
		// For other errors, fail immediately.
		return "", err
	}

	// All models exhausted.
	if lastErr != nil {
		return "", fmt.Errorf("all models unavailable: %w", lastErr)
	}
	return "", errors.New("copilot CLI returned empty output")
}

// tryWithModel attempts to run the prompt with a specific model.
// If model is empty, runs without --model flag (CLI picks default).
//
// When the session is configured as interactive (cfg.Interactive == true),
// the --no-ask-user flag is omitted, allowing the Copilot CLI's built-in
// ask_user tool to function. In this mode, the CLI may pause and prompt
// the user for input via the terminal's stdin/stdout.
//
// When the session is NOT interactive (the default), --no-ask-user is
// passed to suppress any user interaction, making the step fully autonomous.
func (s *CopilotCLISession) tryWithModel(ctx context.Context, prompt, model string) (string, error) {
	args := []string{
		"--no-color",
		"-s",
		"-p",
		prompt,
	}

	// Only suppress user interaction when the step is NOT interactive.
	// When interactive, the CLI's ask_user tool is available, allowing
	// the LLM to pause and ask the user for clarification.
	if !s.cfg.Interactive {
		args = append(args, "--no-ask-user")
	}

	// If the agent specifies an explicit tools list, restrict to those tools.
	// Otherwise allow all tools (CLI default discovery).
	if len(s.cfg.Tools) > 0 {
		args = append(args, "--available-tools", strings.Join(s.cfg.Tools, ","))
	} else {
		args = append(args, "--allow-all-tools")
	}

	if model != "" {
		args = append(args, "--model", strings.ToLower(model))
	}

	// Add extra directories for per-step resource discovery.
	for _, dir := range s.cfg.ExtraDirs {
		args = append(args, "--add-dir", dir)
	}

	out, errOut, runErr := s.runCopilot(ctx, args)
	if runErr != nil {
		msg := strings.TrimSpace(errOut)
		if msg == "" {
			msg = strings.TrimSpace(out)
		}
		if msg == "" {
			msg = runErr.Error()
		}
		return "", fmt.Errorf("copilot CLI execution failed: %s", msg)
	}

	trimmed := strings.TrimSpace(out)
	if trimmed != "" {
		return trimmed, nil
	}

	trimmedErr := strings.TrimSpace(errOut)
	if trimmedErr != "" {
		// Check if this is a model unavailability error.
		if model != "" && strings.Contains(trimmedErr, "from --model flag is not available") {
			return "", &modelUnavailableError{model: model, msg: trimmedErr}
		}
		return "", fmt.Errorf("copilot CLI returned no output: %s", trimmedErr)
	}
	return "", errors.New("copilot CLI returned empty output")
}

// modelUnavailableError indicates the specified model is not available.
type modelUnavailableError struct {
	model string
	msg   string
}

func (e *modelUnavailableError) Error() string {
	return fmt.Sprintf("model %q unavailable: %s", e.model, e.msg)
}

// isModelUnavailableError returns true if the error indicates model unavailability.
func isModelUnavailableError(err error) bool {
	var mue *modelUnavailableError
	return errors.As(err, &mue)
}

func (s *CopilotCLISession) runCopilot(ctx context.Context, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, s.binaryPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// When running in interactive mode, connect the process's stdin to the
	// terminal so the user can respond to ask_user prompts from the CLI.
	// In non-interactive mode (the default), stdin is not connected, which
	// prevents the CLI from blocking on user input.
	if s.cfg.Interactive {
		cmd.Stdin = os.Stdin
	}

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (s *CopilotCLISession) SessionID() string { return s.sessionID }

func (s *CopilotCLISession) Close() error { return nil }

func composePrompt(systemPrompt, userPrompt string) string {
	sp := strings.TrimSpace(systemPrompt)
	up := strings.TrimSpace(userPrompt)
	if sp == "" {
		return up
	}
	return fmt.Sprintf("System instructions:\n%s\n\nUser task:\n%s", sp, up)
}

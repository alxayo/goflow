// copilot_cli.go provides a real SessionExecutor implementation backed by the
// local Copilot CLI in non-interactive prompt mode.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	// Interactive workflow steps are handled by explicitly asking the user for
	// one round of input through the provided callback, then continuing in
	// scripted mode. This guarantees the workflow pauses for terminal input.
	if s.cfg.Interactive {
		if s.cfg.OnUserInput == nil {
			return "", errors.New("interactive mode enabled but no user input handler configured")
		}

		question := interactiveQuestionFromPrompt(prompt)
		answer, err := s.cfg.OnUserInput(question, nil)
		if err != nil {
			return "", fmt.Errorf("getting user input: %w", err)
		}

		finalPrompt = fmt.Sprintf(
			"%s\n\nUser clarification:\n%s",
			finalPrompt,
			strings.TrimSpace(answer),
		)
	}

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
// For interactive steps, the CLI is launched with -i (interactive REPL mode).
// The ask_user tool is ONLY available in -i mode; in -p mode it is not
// registered. The -i flag starts a multi-turn conversation where the LLM
// can call ask_user to pause and get user input. The user exits the session
// via Ctrl+D or /exit when the conversation is done.
//
// For non-interactive steps, -p -s --no-ask-user are used for single-shot
// autonomous execution with clean scripting output.
func (s *CopilotCLISession) tryWithModel(ctx context.Context, prompt, model string) (string, error) {
	args := []string{
		"--no-color",
		"-s",
		"-p",
		prompt,
		"--no-ask-user",
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

	// All modes are executed as scripted commands with captured output.
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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

func interactiveQuestionFromPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "Please provide the information needed for this step."
	}

	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate != "" {
			return candidate
		}
	}

	return "Please provide the information needed for this step."
}

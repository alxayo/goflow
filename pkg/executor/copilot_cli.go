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
func (s *CopilotCLISession) Send(ctx context.Context, prompt string) (string, error) {
	finalPrompt := composePrompt(s.cfg.SystemPrompt, prompt)

	args := []string{
		"--allow-all-tools",
		"--no-ask-user",
		"--no-color",
		"-s",
		"-p",
		finalPrompt,
	}
	if s.cfg.Model != "" {
		args = append(args, "--model", s.cfg.Model)
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
	if s.cfg.Model != "" && strings.Contains(trimmedErr, "from --model flag is not available") {
		fallbackArgs := []string{
			"--allow-all-tools",
			"--no-ask-user",
			"--no-color",
			"-s",
			"-p",
			finalPrompt,
		}
		fallbackOut, fallbackErrOut, fallbackRunErr := s.runCopilot(ctx, fallbackArgs)
		if fallbackRunErr != nil {
			fallbackMsg := strings.TrimSpace(fallbackErrOut)
			if fallbackMsg == "" {
				fallbackMsg = strings.TrimSpace(fallbackOut)
			}
			if fallbackMsg == "" {
				fallbackMsg = fallbackRunErr.Error()
			}
			return "", fmt.Errorf("copilot CLI fallback without model failed: %s", fallbackMsg)
		}
		fallbackTrimmed := strings.TrimSpace(fallbackOut)
		if fallbackTrimmed != "" {
			return fallbackTrimmed, nil
		}
		if strings.TrimSpace(fallbackErrOut) != "" {
			return "", fmt.Errorf("copilot CLI fallback without model returned no output: %s", strings.TrimSpace(fallbackErrOut))
		}
		return "", errors.New("copilot CLI fallback without model returned empty output")
	}

	if trimmedErr != "" {
		return "", fmt.Errorf("copilot CLI returned no output: %s", trimmedErr)
	}
	return "", errors.New("copilot CLI returned empty output")
}

func (s *CopilotCLISession) runCopilot(ctx context.Context, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, s.binaryPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
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

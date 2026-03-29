// interactive_test.go contains tests for the interactive user-input feature
// across the executor and mock layers. These tests verify that:
// - The Interactive and OnUserInput fields are correctly threaded through
// - The mock executor simulates user-input questions
// - The SessionConfig receives the interactive flag
// - User input errors are properly propagated
package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// TestExecute_InteractiveFlagPassedToSession verifies that when the executor
// has Interactive=true and OnUserInput set, these values are forwarded into
// the SessionConfig used to create the SDK session.
func TestExecute_InteractiveFlagPassedToSession(t *testing.T) {
	mock := &MockSessionExecutor{DefaultResponse: "output"}
	exec := &StepExecutor{
		SDK:         mock,
		Interactive: true,
		OnUserInput: func(q string, c []string) (string, error) {
			return "answer", nil
		},
	}

	step := workflow.Step{
		ID:     "interactive-step",
		Agent:  "bot",
		Prompt: "Do the work",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the SessionConfig received the interactive flag.
	if !mock.LastConfig.Interactive {
		t.Error("expected SessionConfig.Interactive=true, got false")
	}
	if mock.LastConfig.OnUserInput == nil {
		t.Error("expected SessionConfig.OnUserInput to be set, got nil")
	}
}

// TestExecute_NonInteractiveByDefault verifies that when the executor
// has Interactive=false (default), the SessionConfig does NOT include
// the interactive flag or user-input handler.
func TestExecute_NonInteractiveByDefault(t *testing.T) {
	mock := &MockSessionExecutor{DefaultResponse: "output"}
	exec := &StepExecutor{SDK: mock}

	step := workflow.Step{
		ID:     "auto-step",
		Agent:  "bot",
		Prompt: "Do the work",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.LastConfig.Interactive {
		t.Error("expected SessionConfig.Interactive=false, got true")
	}
	if mock.LastConfig.OnUserInput != nil {
		t.Error("expected SessionConfig.OnUserInput=nil for non-interactive step")
	}
}

// TestMockSession_SimulatedUserInput verifies that the mock executor
// correctly simulates the LLM asking a question and receiving an answer
// through the UserInputHandler callback.
func TestMockSession_SimulatedUserInput(t *testing.T) {
	// Track what question the handler received and what answer we return.
	var receivedQuestion string
	handler := func(question string, choices []string) (string, error) {
		receivedQuestion = question
		return "use Go 1.21", nil
	}

	mock := &MockSessionExecutor{
		DefaultResponse: "fallback",
		SimulatedQuestions: map[string]string{
			"Which version": "What Go version should I target?",
		},
	}

	exec := &StepExecutor{
		SDK:         mock,
		Interactive: true,
		OnUserInput: handler,
	}

	step := workflow.Step{
		ID:     "ask-step",
		Agent:  "bot",
		Prompt: "Which version of Go should I use?",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The handler should have been called with the simulated question.
	if receivedQuestion != "What Go version should I target?" {
		t.Errorf("handler received question %q, want %q", receivedQuestion, "What Go version should I target?")
	}

	// The result should include the user's answer.
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	if result.Output == "" {
		t.Error("expected non-empty output incorporating user answer")
	}
}

// TestMockSession_SimulatedQuestionNoMatch verifies that when the prompt
// does not match any simulated question, the mock falls back to the
// normal response matching behavior.
func TestMockSession_SimulatedQuestionNoMatch(t *testing.T) {
	handler := func(question string, choices []string) (string, error) {
		t.Error("handler should not be called when prompt doesn't match")
		return "", nil
	}

	mock := &MockSessionExecutor{
		DefaultResponse: "normal response",
		SimulatedQuestions: map[string]string{
			"unrelated prompt": "This question won't match",
		},
	}

	exec := &StepExecutor{
		SDK:         mock,
		Interactive: true,
		OnUserInput: handler,
	}

	step := workflow.Step{
		ID:     "no-question-step",
		Agent:  "bot",
		Prompt: "Just do the analysis",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output != "normal response" {
		t.Errorf("want output %q, got %q", "normal response", result.Output)
	}
}

// TestMockSession_UserInputError verifies that when the user-input handler
// returns an error (e.g., stdin closed), the step fails properly.
func TestMockSession_UserInputError(t *testing.T) {
	handler := func(question string, choices []string) (string, error) {
		return "", errors.New("stdin closed")
	}

	mock := &MockSessionExecutor{
		SimulatedQuestions: map[string]string{
			"Review": "What files to review?",
		},
	}

	exec := &StepExecutor{
		SDK:         mock,
		Interactive: true,
		OnUserInput: handler,
	}

	step := workflow.Step{
		ID:     "fail-input-step",
		Agent:  "bot",
		Prompt: "Review the code",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err == nil {
		t.Fatal("expected error when user input handler fails, got nil")
	}
	if result.Status != workflow.StepStatusFailed {
		t.Errorf("want status=failed, got %s", result.Status)
	}
}

// TestMockSession_NonInteractiveIgnoresSimulatedQuestions verifies that
// even if SimulatedQuestions are configured on the mock, they are NOT
// triggered when the session is non-interactive. This ensures the
// autonomous mode is truly separate from interactive mode.
func TestMockSession_NonInteractiveIgnoresSimulatedQuestions(t *testing.T) {
	handlerCalled := false
	handler := func(question string, choices []string) (string, error) {
		handlerCalled = true
		return "should not happen", nil
	}

	mock := &MockSessionExecutor{
		DefaultResponse: "autonomous output",
		SimulatedQuestions: map[string]string{
			"Review": "What files?",
		},
	}

	// Interactive is false (default), so the handler should NOT be set on
	// the session, even though we provide it on the executor.
	exec := &StepExecutor{
		SDK:         mock,
		Interactive: false,
		OnUserInput: handler,
	}

	step := workflow.Step{
		ID:     "autonomous-step",
		Agent:  "bot",
		Prompt: "Review the code",
	}
	agent := &agents.Agent{
		Name:   "bot",
		Prompt: "You are a bot",
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handlerCalled {
		t.Error("user input handler should NOT be called in non-interactive mode")
	}
	if result.Output != "autonomous output" {
		t.Errorf("want output %q, got %q", "autonomous output", result.Output)
	}
}

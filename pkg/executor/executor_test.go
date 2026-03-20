package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/audit"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// newTestExecutor creates a StepExecutor with a MockSessionExecutor.
// The caller can override fields on the returned mock after creation.
func newTestExecutor(responses map[string]string) (*StepExecutor, *MockSessionExecutor) {
	mock := &MockSessionExecutor{Responses: responses, DefaultResponse: "default output"}
	return &StepExecutor{SDK: mock}, mock
}

func testAgent() *agents.Agent {
	return &agents.Agent{
		Name:   "test-agent",
		Prompt: "You are a test agent.",
		Tools:  []string{"grep", "view"},
		Model:  agents.ModelSpec{Models: []string{"gpt-5"}},
	}
}

func TestExecute_HappyPath(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Analyze the code": "analysis result from mock",
	})

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "test-agent",
		Prompt: "Analyze the code",
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	if result.Output != "analysis result from mock" {
		t.Errorf("unexpected output: %q", result.Output)
	}
	if result.StepID != "analyze" {
		t.Errorf("want step_id=analyze, got %s", result.StepID)
	}
	if result.SessionID == "" {
		t.Error("expected a session ID, got empty string")
	}
	if result.StartedAt == "" || result.EndedAt == "" {
		t.Error("expected timestamps, got empty strings")
	}
	if mock.SessionsCreated.Load() != 1 {
		t.Errorf("want 1 session created, got %d", mock.SessionsCreated.Load())
	}
}

func TestExecute_ConditionNotMet(t *testing.T) {
	exec, mock := newTestExecutor(nil)

	step := workflow.Step{
		ID:     "decide",
		Agent:  "test-agent",
		Prompt: "Make a decision",
		Condition: &workflow.Condition{
			Step:     "prior",
			Contains: "APPROVE",
		},
	}
	results := map[string]string{"prior": "REJECTED by reviewer"}

	result, err := exec.Execute(context.Background(), step, testAgent(), results, nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusSkipped {
		t.Errorf("want status=skipped, got %s", result.Status)
	}
	if result.Output != "" {
		t.Errorf("skipped step should have no output, got %q", result.Output)
	}
	// No SDK session should have been created for a skipped step.
	if mock.SessionsCreated.Load() != 0 {
		t.Errorf("want 0 sessions created for skipped step, got %d", mock.SessionsCreated.Load())
	}
}

func TestExecute_ConditionMet(t *testing.T) {
	exec, _ := newTestExecutor(map[string]string{
		"Make a decision": "decision made",
	})

	step := workflow.Step{
		ID:     "decide",
		Agent:  "test-agent",
		Prompt: "Make a decision",
		Condition: &workflow.Condition{
			Step:     "prior",
			Contains: "APPROVE",
		},
	}
	results := map[string]string{"prior": "APPROVE this change"}

	result, err := exec.Execute(context.Background(), step, testAgent(), results, nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	if result.Output == "" {
		t.Error("expected output from executed step")
	}
}

func TestExecute_TemplateResolution(t *testing.T) {
	exec, _ := newTestExecutor(map[string]string{
		"output from step-a": "response based on step-a output",
	})

	step := workflow.Step{
		ID:     "step-b",
		Agent:  "test-agent",
		Prompt: "Process this: {{steps.step-a.output}}",
	}
	results := map[string]string{"step-a": "output from step-a"}

	result, err := exec.Execute(context.Background(), step, testAgent(), results, nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	// The mock matches "output from step-a" as a substring of the resolved prompt.
	if result.Output != "response based on step-a output" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestExecute_TemplateResolutionWithInputs(t *testing.T) {
	exec, _ := newTestExecutor(map[string]string{
		"src/main.go": "reviewed main.go",
	})

	step := workflow.Step{
		ID:     "review",
		Agent:  "test-agent",
		Prompt: "Review {{inputs.files}}",
	}
	inputs := map[string]string{"files": "src/main.go"}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, inputs, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	if result.Output != "reviewed main.go" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestExecute_SDKCreateSessionFailure(t *testing.T) {
	exec, _ := newTestExecutor(nil)
	exec.SDK.(*MockSessionExecutor).CreateErr = errors.New("connection refused")

	step := workflow.Step{
		ID:     "failing-step",
		Agent:  "test-agent",
		Prompt: "Do something",
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result == nil {
		t.Fatal("expected result even on error, got nil")
	}
	if result.Status != workflow.StepStatusFailed {
		t.Errorf("want status=failed, got %s", result.Status)
	}
	if result.ErrorMsg != "connection refused" {
		t.Errorf("want error msg 'connection refused', got %q", result.ErrorMsg)
	}
}

func TestExecute_SDKSendFailure(t *testing.T) {
	exec, _ := newTestExecutor(nil)
	exec.SDK.(*MockSessionExecutor).SendErr = errors.New("model overloaded")

	step := workflow.Step{
		ID:     "send-fail",
		Agent:  "test-agent",
		Prompt: "Do something",
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Status != workflow.StepStatusFailed {
		t.Errorf("want status=failed, got %s", result.Status)
	}
	if result.ErrorMsg != "model overloaded" {
		t.Errorf("want error msg 'model overloaded', got %q", result.ErrorMsg)
	}
	if result.SessionID == "" {
		t.Error("expected session ID to be set even on send failure")
	}
}

func TestExecute_AuditFilesWritten(t *testing.T) {
	tmpDir := t.TempDir()
	runLogger, err := audit.NewRunLogger(tmpDir, "test-workflow")
	if err != nil {
		t.Fatalf("creating run logger: %v", err)
	}

	exec, _ := newTestExecutor(map[string]string{
		"Analyze": "analysis complete",
	})
	exec.AuditLogger = runLogger

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "test-agent",
		Prompt: "Analyze the code",
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}

	// Verify audit files exist.
	stepDir := filepath.Join(runLogger.RunDir, "steps", "01_analyze")

	promptPath := filepath.Join(stepDir, "prompt.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		t.Error("prompt.md not created")
	}
	promptData, _ := os.ReadFile(promptPath)
	if string(promptData) != "Analyze the code" {
		t.Errorf("prompt.md content = %q, want %q", string(promptData), "Analyze the code")
	}

	outputPath := filepath.Join(stepDir, "output.md")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output.md not created")
	}
	outputData, _ := os.ReadFile(outputPath)
	if string(outputData) != "analysis complete" {
		t.Errorf("output.md content = %q, want %q", string(outputData), "analysis complete")
	}

	metaPath := filepath.Join(stepDir, "step.meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("step.meta.json not created")
	}
}

func TestExecute_NoAuditLogger(t *testing.T) {
	exec, _ := newTestExecutor(map[string]string{
		"Do it": "done",
	})
	// AuditLogger is nil by default from newTestExecutor.

	step := workflow.Step{
		ID:     "no-audit",
		Agent:  "test-agent",
		Prompt: "Do it",
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
	if result.Output != "done" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestExecute_SkippedStepAudit(t *testing.T) {
	tmpDir := t.TempDir()
	runLogger, err := audit.NewRunLogger(tmpDir, "test-workflow")
	if err != nil {
		t.Fatalf("creating run logger: %v", err)
	}

	exec, mock := newTestExecutor(nil)
	exec.AuditLogger = runLogger

	step := workflow.Step{
		ID:     "skippable",
		Agent:  "test-agent",
		Prompt: "Should be skipped",
		Condition: &workflow.Condition{
			Step:     "prior",
			Contains: "APPROVE",
		},
	}
	results := map[string]string{"prior": "REJECTED"}

	result, err := exec.Execute(context.Background(), step, testAgent(), results, nil, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusSkipped {
		t.Errorf("want status=skipped, got %s", result.Status)
	}

	// Verify step.meta.json exists for skipped step.
	stepDir := filepath.Join(runLogger.RunDir, "steps", "03_skippable")
	metaPath := filepath.Join(stepDir, "step.meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("step.meta.json not created for skipped step")
	}

	// No session should have been created.
	if mock.SessionsCreated.Load() != 0 {
		t.Errorf("want 0 sessions for skipped step, got %d", mock.SessionsCreated.Load())
	}
}

func TestExecute_AgentWithNoModel(t *testing.T) {
	exec, _ := newTestExecutor(map[string]string{
		"Do stuff": "done without model",
	})

	agent := &agents.Agent{
		Name:   "no-model-agent",
		Prompt: "You are an agent.",
		Tools:  []string{"grep"},
		// Model is empty — no models set.
	}

	step := workflow.Step{
		ID:     "no-model",
		Agent:  "no-model-agent",
		Prompt: "Do stuff",
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
}

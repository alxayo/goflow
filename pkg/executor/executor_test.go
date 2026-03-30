package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestExecute_RetryCountTransientTimeoutSucceeds(t *testing.T) {
	exec := &StepExecutor{SDK: &flakyTimeoutExecutor{failuresLeft: 1}}
	step := workflow.Step{
		ID:         "retry-step",
		Agent:      "test-agent",
		Prompt:     "Do something",
		RetryCount: 1,
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}
}

func TestExecute_RetryCountExhaustedFails(t *testing.T) {
	exec := &StepExecutor{SDK: &flakyTimeoutExecutor{failuresLeft: 2}}
	step := workflow.Step{
		ID:         "retry-step",
		Agent:      "test-agent",
		Prompt:     "Do something",
		RetryCount: 1,
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Status != workflow.StepStatusFailed {
		t.Errorf("want status=failed, got %s", result.Status)
	}
	if result.ErrorMsg == "" {
		t.Error("expected non-empty error message")
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

type flakyTimeoutExecutor struct {
	failuresLeft int32
}

func (f *flakyTimeoutExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
	_ = ctx
	_ = cfg
	return &flakyTimeoutSession{executor: f}, nil
}

type flakyTimeoutSession struct {
	executor *flakyTimeoutExecutor
}

func (s *flakyTimeoutSession) Send(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	_ = prompt
	for {
		left := atomic.LoadInt32(&s.executor.failuresLeft)
		if left <= 0 {
			return "recovered output", nil
		}
		if atomic.CompareAndSwapInt32(&s.executor.failuresLeft, left, left-1) {
			return "", errors.New("SDK session send: waiting for session.idle: context deadline exceeded")
		}
	}
}

func (s *flakyTimeoutSession) SessionID() string {
	return "flaky-session"
}

func (s *flakyTimeoutSession) Close() error {
	return nil
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

func TestExecute_ToolsPassedToSession(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Scan": "scan complete",
	})

	agent := &agents.Agent{
		Name:   "restricted-agent",
		Prompt: "You scan code.",
		Tools:  []string{"grep", "view"},
		Model:  agents.ModelSpec{Models: []string{"gpt-5"}},
	}

	step := workflow.Step{
		ID:     "scan",
		Agent:  "restricted-agent",
		Prompt: "Scan the code",
	}

	result, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	if len(cfg.Tools) != 2 || cfg.Tools[0] != "grep" || cfg.Tools[1] != "view" {
		t.Errorf("tools = %v, want [grep view]", cfg.Tools)
	}
	if len(cfg.Models) != 1 || cfg.Models[0] != "gpt-5" {
		t.Errorf("models = %v, want [gpt-5]", cfg.Models)
	}
}

func TestExecute_ExtraDirsPassedToSession(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Analyze": "analysis done",
	})

	step := workflow.Step{
		ID:        "analyze",
		Agent:     "test-agent",
		Prompt:    "Analyze the code",
		ExtraDirs: []string{"./extra/security", "./extra/common"},
	}

	result, err := exec.Execute(context.Background(), step, testAgent(), nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != workflow.StepStatusCompleted {
		t.Errorf("want status=completed, got %s", result.Status)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	if len(cfg.ExtraDirs) != 2 {
		t.Fatalf("extra_dirs count = %d, want 2", len(cfg.ExtraDirs))
	}
	if cfg.ExtraDirs[0] != "./extra/security" || cfg.ExtraDirs[1] != "./extra/common" {
		t.Errorf("extra_dirs = %v, want [./extra/security ./extra/common]", cfg.ExtraDirs)
	}
}

func TestExecute_NoToolsAllowsAll(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Work": "done",
	})

	agent := &agents.Agent{
		Name:   "unrestricted-agent",
		Prompt: "You do everything.",
		Tools:  nil, // No tool restriction — all tools available.
		Model:  agents.ModelSpec{Models: []string{"gpt-5"}},
	}

	step := workflow.Step{
		ID:     "work",
		Agent:  "unrestricted-agent",
		Prompt: "Work on it",
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	if cfg.Tools != nil {
		t.Errorf("tools should be nil for unrestricted agent, got %v", cfg.Tools)
	}
}

// TestExecute_StepModelOverridesAgent verifies that step-level model takes
// highest priority over agent's model.
func TestExecute_StepModelOverridesAgent(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Analyze": "done",
	})

	agent := &agents.Agent{
		Name:   "test-agent",
		Prompt: "Instructions",
		Model:  agents.ModelSpec{Models: []string{"gpt-4", "gpt-3.5"}},
	}

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "test-agent",
		Prompt: "Analyze code",
		Model:  "claude-sonnet-4.5", // Step override
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	// Step model should be first, then agent models.
	want := []string{"claude-sonnet-4.5", "gpt-4", "gpt-3.5"}
	if !slicesEqual(cfg.Models, want) {
		t.Errorf("models = %v, want %v", cfg.Models, want)
	}
}

// TestExecute_WorkflowDefaultModelFallback verifies that the workflow-level
// default model is used as the last fallback.
func TestExecute_WorkflowDefaultModelFallback(t *testing.T) {
	mock := &MockSessionExecutor{
		Responses: map[string]string{"Analyze": "done"},
	}
	exec := &StepExecutor{
		SDK:          mock,
		DefaultModel: "workflow-default-model", // Workflow-level default
	}

	agent := &agents.Agent{
		Name:   "no-model-agent",
		Prompt: "Instructions",
		Model:  agents.ModelSpec{}, // No model specified
	}

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "no-model-agent",
		Prompt: "Analyze code",
		// No step model either
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	// Only the workflow default should be in the list.
	want := []string{"workflow-default-model"}
	if !slicesEqual(cfg.Models, want) {
		t.Errorf("models = %v, want %v", cfg.Models, want)
	}
}

// TestExecute_ModelDeduplication verifies that duplicate models are removed
// while preserving priority order.
func TestExecute_ModelDeduplication(t *testing.T) {
	mock := &MockSessionExecutor{
		Responses: map[string]string{"Analyze": "done"},
	}
	exec := &StepExecutor{
		SDK:          mock,
		DefaultModel: "gpt-5", // Same as agent's first model — should be deduped
	}

	agent := &agents.Agent{
		Name:   "test-agent",
		Prompt: "Instructions",
		Model:  agents.ModelSpec{Models: []string{"gpt-5", "gpt-4"}},
	}

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "test-agent",
		Prompt: "Analyze code",
		Model:  "gpt-4", // Same as agent's second model — should be deduped
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	// Order: step (gpt-4), agent (gpt-5, gpt-4 deduped), workflow (gpt-5 deduped)
	// Final: [gpt-4, gpt-5]
	want := []string{"gpt-4", "gpt-5"}
	if !slicesEqual(cfg.Models, want) {
		t.Errorf("models = %v, want %v", cfg.Models, want)
	}
}

// TestExecute_EmptyModelsWhenNoneSpecified verifies that an empty models list
// is passed when no model is specified anywhere.
func TestExecute_EmptyModelsWhenNoneSpecified(t *testing.T) {
	exec, mock := newTestExecutor(map[string]string{
		"Analyze": "done",
	})
	// newTestExecutor creates executor with no DefaultModel.

	agent := &agents.Agent{
		Name:   "no-model-agent",
		Prompt: "Instructions",
		Model:  agents.ModelSpec{}, // No model
	}

	step := workflow.Step{
		ID:     "analyze",
		Agent:  "no-model-agent",
		Prompt: "Analyze code",
	}

	_, err := exec.Execute(context.Background(), step, agent, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.mu.Lock()
	cfg := mock.LastConfig
	mock.mu.Unlock()

	// Models list should be empty — CLI will pick the default.
	if len(cfg.Models) != 0 {
		t.Errorf("models = %v, want empty", cfg.Models)
	}
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

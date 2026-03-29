// interactive_test.go tests the orchestrator's interactive-mode behavior:
// - Per-step interactive resolution using the three-level priority
// - Parallel-level handling where interactive steps run sequentially
// - Integration of the user-input handler through the full stack
package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/executor"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// TestRun_InteractiveStep verifies that the orchestrator correctly resolves
// the interactive flag for each step and passes it to the executor.
func TestRun_InteractiveStep(t *testing.T) {
	// Track which steps had interactive=true when the mock was called.
	// We use the LastConfig on the mock to check the most recent session.
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "output",
	}
	se := &executor.StepExecutor{SDK: mock}

	handlerCalled := false
	orch := &Orchestrator{
		Executor: se,
		Agents: map[string]*agents.Agent{
			"bot": makeAgent("bot"),
		},
		Inputs:         map[string]string{},
		CLIInteractive: true,
		OnUserInput: func(q string, c []string) (string, error) {
			handlerCalled = true
			return "yes", nil
		},
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("step-a", "bot", "Do work"),
		},
	}

	_, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Since CLIInteractive=true and step has no override, the executor
	// should have been set to interactive.
	if !se.Interactive {
		t.Error("expected executor.Interactive=true for step with CLI flag")
	}
	if se.OnUserInput == nil {
		t.Error("expected executor.OnUserInput to be set")
	}
	_ = handlerCalled // handler is only called if mock simulates a question
}

// TestRun_StepOverridesWorkflowInteractive verifies that a step with
// interactive=false overrides the workflow-level config.interactive=true.
func TestRun_StepOverridesWorkflowInteractive(t *testing.T) {
	mock := &executor.MockSessionExecutor{DefaultResponse: "output"}
	se := &executor.StepExecutor{SDK: mock}

	orch := &Orchestrator{
		Executor: se,
		Agents: map[string]*agents.Agent{
			"bot": makeAgent("bot"),
		},
		Inputs: map[string]string{},
	}

	// Workflow has config.interactive=true, but the step overrides to false.
	wf := &workflow.Workflow{
		Config: workflow.Config{
			Interactive: true,
		},
		Steps: []workflow.Step{
			{
				ID:          "opt-out",
				Agent:       "bot",
				Prompt:      "Work autonomously",
				Interactive: workflow.BoolPtr(false),
			},
		},
	}

	_, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The step explicitly set interactive=false, overriding the workflow config.
	if se.Interactive {
		t.Error("expected executor.Interactive=false when step overrides to false")
	}
}

// TestRun_StepEnablesInteractive verifies that a step with interactive=true
// enables interactivity even when neither CLI nor workflow config has it.
func TestRun_StepEnablesInteractive(t *testing.T) {
	mock := &executor.MockSessionExecutor{DefaultResponse: "output"}
	se := &executor.StepExecutor{SDK: mock}

	orch := &Orchestrator{
		Executor: se,
		Agents: map[string]*agents.Agent{
			"bot": makeAgent("bot"),
		},
		Inputs: map[string]string{},
		OnUserInput: func(q string, c []string) (string, error) {
			return "answer", nil
		},
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			{
				ID:          "opt-in",
				Agent:       "bot",
				Prompt:      "Ask me questions",
				Interactive: workflow.BoolPtr(true),
			},
		},
	}

	_, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !se.Interactive {
		t.Error("expected executor.Interactive=true when step explicitly enables it")
	}
}

// TestRunParallel_InteractiveStepsSerialized verifies that in RunParallel,
// interactive steps within a parallel level are separated and run
// sequentially after non-interactive steps complete.
func TestRunParallel_InteractiveStepsSerialized(t *testing.T) {
	// Use a send hook to track execution order.
	var executionOrder []string
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "output",
		SendHook: func(prompt string) {
			// Extract step hint from prompt.
			executionOrder = append(executionOrder, prompt)
		},
	}
	se := &executor.StepExecutor{SDK: mock}

	orch := &Orchestrator{
		Executor: se,
		Agents: map[string]*agents.Agent{
			"bot": makeAgent("bot"),
		},
		Inputs:         map[string]string{},
		CLIInteractive: false, // Default off
		OnUserInput: func(q string, c []string) (string, error) {
			return "answer", nil
		},
	}

	// Three steps at the same level (no dependencies):
	// - step-a and step-b are non-interactive → parallel
	// - step-c is interactive → runs after parallel batch
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			{ID: "step-a", Agent: "bot", Prompt: "parallel-A"},
			{ID: "step-b", Agent: "bot", Prompt: "parallel-B"},
			{ID: "step-c", Agent: "bot", Prompt: "interactive-C", Interactive: workflow.BoolPtr(true)},
		},
	}

	_, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// step-c should be the last one executed since it's interactive.
	if len(executionOrder) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(executionOrder))
	}

	// The last executed prompt should contain the interactive step's text.
	// Note: the prompt is composed with the agent's system prompt prefix,
	// so we check for substring containment rather than exact match.
	lastPrompt := executionOrder[len(executionOrder)-1]
	if !strings.Contains(lastPrompt, "interactive-C") {
		t.Errorf("expected interactive step to run last, but last prompt was %q", lastPrompt)
	}
}

// TestRunParallel_AllNonInteractive verifies that when no steps are
// interactive, RunParallel behaves identically to before (all run
// in parallel within their level).
func TestRunParallel_AllNonInteractive(t *testing.T) {
	mock := &executor.MockSessionExecutor{DefaultResponse: "output"}
	se := &executor.StepExecutor{SDK: mock}

	orch := &Orchestrator{
		Executor: se,
		Agents: map[string]*agents.Agent{
			"bot": makeAgent("bot"),
		},
		Inputs: map[string]string{},
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "do A"),
			makeStep("B", "bot", "do B"),
			makeStep("C", "bot", "do C"),
		},
	}

	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three should have completed.
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for _, id := range []string{"A", "B", "C"} {
		if r, ok := results[id]; !ok || r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %v", id, r)
		}
	}
}

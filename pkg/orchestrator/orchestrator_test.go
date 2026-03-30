package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/executor"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// makeAgent builds a minimal Agent for testing.
func makeAgent(name string) *agents.Agent {
	return &agents.Agent{
		Name:   name,
		Prompt: "You are " + name,
		Model:  agents.ModelSpec{Models: []string{"test-model"}},
	}
}

// makeStep builds a Step with optional dependencies.
func makeStep(id, agent, prompt string, deps ...string) workflow.Step {
	return workflow.Step{
		ID:        id,
		Agent:     agent,
		Prompt:    prompt,
		DependsOn: deps,
	}
}

// newOrchestrator wires up an Orchestrator with a MockSessionExecutor.
func newOrchestrator(mock *executor.MockSessionExecutor, agentMap map[string]*agents.Agent, inputs map[string]string) *Orchestrator {
	se := &executor.StepExecutor{
		SDK: mock,
	}
	return &Orchestrator{
		Executor: se,
		Agents:   agentMap,
		Inputs:   inputs,
	}
}

func TestLinearPipeline(t *testing.T) {
	// A → B → C, each step references the previous output via template.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"start":                "output-A",
			"{{steps.A.output}}":   "output-B from output-A",
			"output-A":             "output-B from output-A",
			"output-B from output": "output-C from output-B from output-A",
		},
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.B.output}}", "B"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, id := range []string{"A", "B", "C"} {
		r, ok := results[id]
		if !ok {
			t.Errorf("missing result for step %s", id)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}

	// Verify chaining: B's output should reference A's output.
	if results["B"].Output != "output-B from output-A" {
		t.Errorf("step B output = %q, want %q", results["B"].Output, "output-B from output-A")
	}

	// 3 sessions should have been created.
	if got := mock.SessionsCreated.Load(); got != 3 {
		t.Errorf("sessions created = %d, want 3", got)
	}
}

func TestConditionMet(t *testing.T) {
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"check":   "Result: APPROVE this PR",
			"APPROVE": "Approved and merged",
		},
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("check", "bot", "check"),
			{
				ID:        "act",
				Agent:     "bot",
				Prompt:    "{{steps.check.output}}",
				DependsOn: []string{"check"},
				Condition: &workflow.Condition{
					Step:     "check",
					Contains: "APPROVE",
				},
			},
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results["act"].Status != workflow.StepStatusCompleted {
		t.Errorf("step act: expected completed, got %s", results["act"].Status)
	}
}

func TestConditionNotMet(t *testing.T) {
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"check": "Result: REJECT this PR",
		},
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("check", "bot", "check"),
			{
				ID:        "act",
				Agent:     "bot",
				Prompt:    "{{steps.check.output}}",
				DependsOn: []string{"check"},
				Condition: &workflow.Condition{
					Step:     "check",
					Contains: "APPROVE",
				},
			},
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results["act"].Status != workflow.StepStatusSkipped {
		t.Errorf("step act: expected skipped, got %s", results["act"].Status)
	}
}

func TestStepFailure(t *testing.T) {
	mock := &executor.MockSessionExecutor{
		SendErr: fmt.Errorf("LLM timeout"),
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "do something"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Partial results should contain step A's failed result.
	r, ok := results["A"]
	if !ok {
		t.Fatal("expected result for step A in partial results")
	}
	if r.Status != workflow.StepStatusFailed {
		t.Errorf("step A: expected failed, got %s", r.Status)
	}

	// Step B should not be in results.
	if _, ok := results["B"]; ok {
		t.Error("step B should not be in results after A failed")
	}
}

func TestEmptyWorkflow(t *testing.T) {
	mock := &executor.MockSessionExecutor{}
	agentMap := map[string]*agents.Agent{}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFanOutFanIn(t *testing.T) {
	// A → {B, C} → D
	// B and C run in the same DAG level (sequentially in this implementation).
	// D depends on both and gets both outputs.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"start":       "root-output",
			"root-out":    "branch-B-output",
			"root-output": "branch-B-output",
		},
		DefaultResponse: "default-output",
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.A.output}}", "A"),
			makeStep("D", "bot", "{{steps.B.output}} and {{steps.C.output}}", "B", "C"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	for _, id := range []string{"A", "B", "C", "D"} {
		r, ok := results[id]
		if !ok {
			t.Errorf("missing result for step %s", id)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}

	// D should have received non-empty output.
	if results["D"].Output == "" {
		t.Error("step D should have non-empty output")
	}

	// 4 sessions total.
	if got := mock.SessionsCreated.Load(); got != 4 {
		t.Errorf("sessions created = %d, want 4", got)
	}
}

func TestMissingAgent(t *testing.T) {
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "ok",
	}

	agentMap := map[string]*agents.Agent{
		"bot": makeAgent("bot"),
	}

	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "nonexistent-agent", "do something"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	_, err := orch.Run(context.Background(), wf)
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}

	wantSubstr := `agent "nonexistent-agent" not found`
	if got := err.Error(); !contains(got, wantSubstr) {
		t.Errorf("error = %q, want substring %q", got, wantSubstr)
	}
}

// contains checks if s contains substr (avoids importing strings in test).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- RunParallel tests ---

func TestParallelLinearPipeline(t *testing.T) {
	// A → B → C — same as sequential, verifies RunParallel handles linear DAGs.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"start":    "output-A",
			"output-A": "output-B from output-A",
		},
		DefaultResponse: "default-output",
	}

	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.B.output}}", "B"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, id := range []string{"A", "B", "C"} {
		if r, ok := results[id]; !ok {
			t.Errorf("missing result for %s", id)
		} else if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}
}

func TestParallelFanOut(t *testing.T) {
	// A → {B, C, D}: all three fan-out steps complete.
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "fan-out-result",
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.A.output}}", "A"),
			makeStep("D", "bot", "{{steps.A.output}}", "A"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []string{"A", "B", "C", "D"} {
		r, ok := results[id]
		if !ok {
			t.Errorf("missing result for %s", id)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}
}

func TestParallelFanIn(t *testing.T) {
	// {B,C,D} → E: E gets outputs from all three.
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "fan-result",
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("B", "bot", "work-b"),
			makeStep("C", "bot", "work-c"),
			makeStep("D", "bot", "work-d"),
			makeStep("E", "bot", "{{steps.B.output}} {{steps.C.output}} {{steps.D.output}}", "B", "C", "D"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if results["E"].Status != workflow.StepStatusCompleted {
		t.Errorf("step E: expected completed, got %s", results["E"].Status)
	}
}

func TestParallelDiamond(t *testing.T) {
	// A → {B, C} → D: D gets B and C outputs.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"start": "root",
		},
		DefaultResponse: "diamond-result",
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.A.output}}", "A"),
			makeStep("D", "bot", "{{steps.B.output}} and {{steps.C.output}}", "B", "C"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []string{"A", "B", "C", "D"} {
		r, ok := results[id]
		if !ok {
			t.Errorf("missing result for %s", id)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}
	if results["D"].Output == "" {
		t.Error("step D should have non-empty output")
	}
}

func TestParallelActuallyConcurrent(t *testing.T) {
	// A → {B, C, D}: verify that B, C, D all start before any finishes.
	// Use a barrier pattern: each goroutine signals arrival, then waits
	// until all have arrived before proceeding.
	const fanOutCount = 3
	arrived := make(chan struct{}, fanOutCount)
	gate := make(chan struct{})

	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"root-start": "root-output",
		},
		DefaultResponse: "concurrent-result",
		SendHook: func(prompt string) {
			// Only synchronize on the fan-out level (B, C, D). Their
			// resolved prompts contain "root-output" (A's output), but
			// A's own prompt is "root-start" which does not.
			if !contains(prompt, "root-output") {
				return
			}
			arrived <- struct{}{}
			<-gate // block until test releases the gate
		},
	}

	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "root-start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.A.output}}", "A"),
			makeStep("D", "bot", "{{steps.A.output}}", "A"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)

	resultCh := make(chan struct {
		results map[string]*workflow.StepResult
		err     error
	}, 1)

	go func() {
		results, err := orch.RunParallel(context.Background(), wf)
		resultCh <- struct {
			results map[string]*workflow.StepResult
			err     error
		}{results, err}
	}()

	// Wait for all fan-out steps to arrive (proves they started concurrently).
	for i := 0; i < fanOutCount; i++ {
		<-arrived
	}
	// Release all of them.
	close(gate)

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if len(res.results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(res.results))
	}
}

func TestParallelMaxConcurrency1(t *testing.T) {
	// With MaxConcurrency=1, fan-out steps complete but run one at a time.
	mock := &executor.MockSessionExecutor{
		DefaultResponse: "serial-result",
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "{{steps.A.output}}", "A"),
			makeStep("C", "bot", "{{steps.A.output}}", "A"),
			makeStep("D", "bot", "{{steps.A.output}}", "A"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	orch.MaxConcurrency = 1
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []string{"A", "B", "C", "D"} {
		r, ok := results[id]
		if !ok {
			t.Errorf("missing result for %s", id)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %s: expected completed, got %s", id, r.Status)
		}
	}
}

func TestParallelStepFailure(t *testing.T) {
	// A → {B, C}: C fails but RunParallel continues in best-effort mode.
	// D fans in from B and C and should still run with C's output as empty.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"start":              "seed-output",
			"succeed-b":          "output-from-b",
			"fan-in":             "aggregated-output",
			"B=output-from-b|C=": "aggregated-output",
		},
		SendErrForStep: map[string]error{
			"fail-c": fmt.Errorf("LLM timeout on C"),
		},
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "start"),
			makeStep("B", "bot", "succeed-b", "A"),
			makeStep("C", "bot", "fail-c", "A"),
			makeStep("D", "bot", "fan-in B={{steps.B.output}}|C={{steps.C.output}}", "B", "C"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A should have completed before the fan-out level.
	if r, ok := results["A"]; !ok || r.Status != workflow.StepStatusCompleted {
		t.Errorf("step A should be completed, got %v", results["A"])
	}
	// C should be failed in results.
	if r, ok := results["C"]; !ok || r.Status != workflow.StepStatusFailed {
		t.Errorf("step C should be failed in results, got %v", results["C"])
	}
	// D should still complete using best-effort fan-in context.
	if r, ok := results["D"]; !ok || r.Status != workflow.StepStatusCompleted {
		t.Errorf("step D should be completed in best-effort mode, got %v", results["D"])
	}
}

func TestParallelEmptyWorkflow(t *testing.T) {
	mock := &executor.MockSessionExecutor{}
	agentMap := map[string]*agents.Agent{}
	wf := &workflow.Workflow{Steps: []workflow.Step{}}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParallelSingleStepFailureFailsWorkflow(t *testing.T) {
	// Single-step level is sequential/critical and should fail fast.
	mock := &executor.MockSessionExecutor{
		SendErrForStep: map[string]error{
			"fail-now": fmt.Errorf("critical failure"),
		},
	}
	agentMap := map[string]*agents.Agent{"bot": makeAgent("bot")}
	wf := &workflow.Workflow{
		Steps: []workflow.Step{
			makeStep("A", "bot", "fail-now"),
			makeStep("B", "bot", "should-not-run", "A"),
		},
	}

	orch := newOrchestrator(mock, agentMap, nil)
	results, err := orch.RunParallel(context.Background(), wf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := results["B"]; ok {
		t.Error("step B should not run after critical sequential failure")
	}
}

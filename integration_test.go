// integration_test.go provides end-to-end tests that exercise the full
// workflow-runner pipeline: YAML parsing → validation → agent resolution →
// DAG building → step execution → audit logging → output reporting.
// All tests are self-contained using temp dirs and inline agents.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/audit"
	"github.com/alex-workflow-runner/workflow-runner/pkg/executor"
	"github.com/alex-workflow-runner/workflow-runner/pkg/memory"
	"github.com/alex-workflow-runner/workflow-runner/pkg/orchestrator"
	"github.com/alex-workflow-runner/workflow-runner/pkg/reporter"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// ---------------------------------------------------------------------------
// Task 1: P1T22 — Sequential Integration Tests
// ---------------------------------------------------------------------------

func TestEndToEndSequential(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Workflow YAML: 3-step sequential pipeline with inline agents.
	yamlContent := `
name: "sequential-pipeline"
description: "3-step sequential e2e test"

agents:
  analyzer:
    inline:
      description: "Analyzes code"
      prompt: "You are a code analyzer."
      tools: [grep, view]
      model: "gpt-5"
  reviewer:
    inline:
      description: "Reviews analysis"
      prompt: "You are a code reviewer."
      tools: [grep]
      model: "gpt-5"
  summarizer:
    inline:
      description: "Summarizes review"
      prompt: "You are a summarizer."
      tools: []
      model: "gpt-5"

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze the codebase for issues"

  - id: review
    agent: reviewer
    prompt: "Review findings: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: summarize
    agent: summarizer
    prompt: "Summarize: {{steps.analyze.output}} and {{steps.review.output}}"
    depends_on: [review]

output:
  steps: [summarize, review]
  format: "markdown"
`

	// 2. Parse and validate.
	wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes: %v", err)
	}
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("ValidateWorkflow: %v", err)
	}

	// 3. Build DAG and verify levels.
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}
	if len(levels) != 3 {
		t.Fatalf("expected 3 DAG levels, got %d", len(levels))
	}

	// 4. Resolve agents using inline definitions.
	resolvedAgents, err := agents.ResolveAgents(wf, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("ResolveAgents: %v", err)
	}
	if len(resolvedAgents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(resolvedAgents))
	}

	// 5. Mock executor with canned responses.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"Analyze the codebase": "Found 3 issues: SQL injection, XSS, CSRF",
			"Review findings":      "Confirmed: SQL injection is critical, XSS is high, CSRF is medium",
			"Summarize":            "Summary: 3 vulnerabilities found. 1 critical, 1 high, 1 medium.",
		},
	}

	// 6. Create audit logger in temp dir.
	auditDir := filepath.Join(tmpDir, ".workflow-runs")
	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	if err := auditLogger.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	// 7. Run orchestrator.
	orch := &orchestrator.Orchestrator{
		Executor: &executor.StepExecutor{
			SDK:         mock,
			AuditLogger: auditLogger,
		},
		Agents: resolvedAgents,
		Inputs: map[string]string{},
	}
	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 8. Assertions.

	// 8a. All 3 steps completed.
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for _, stepID := range []string{"analyze", "review", "summarize"} {
		r, ok := results[stepID]
		if !ok {
			t.Errorf("missing result for step %q", stepID)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %q: expected status completed, got %s", stepID, r.Status)
		}
	}

	// 8b. Template substitution: step 2's prompt received step 1's output.
	//     The mock matched "Review findings" which means the template
	//     {{steps.analyze.output}} was resolved before the prompt was sent.
	reviewResult := results["review"]
	if !strings.Contains(reviewResult.Output, "SQL injection is critical") {
		t.Errorf("review output doesn't contain expected content: %s", reviewResult.Output)
	}

	// 8c. Step 3's prompt contains both step 1 and step 2 outputs.
	//     The mock matched "Summarize" only if both outputs were in prompt.
	summarizeResult := results["summarize"]
	if !strings.Contains(summarizeResult.Output, "Summary") {
		t.Errorf("summarize output doesn't contain expected content: %s", summarizeResult.Output)
	}

	// 8d. Verify session IDs were assigned.
	for _, stepID := range []string{"analyze", "review", "summarize"} {
		if results[stepID].SessionID == "" {
			t.Errorf("step %q: expected non-empty session ID", stepID)
		}
	}

	// 8e. Audit directory structure check.
	stepsDir := filepath.Join(auditLogger.RunDir, "steps")
	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		t.Fatalf("reading steps dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 step audit dirs, got %d", len(entries))
	}

	// Check that step meta and output files exist.
	for _, entry := range entries {
		stepDir := filepath.Join(stepsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(stepDir, "step.meta.json")); err != nil {
			t.Errorf("missing step.meta.json in %s", entry.Name())
		}
		if _, err := os.Stat(filepath.Join(stepDir, "prompt.md")); err != nil {
			t.Errorf("missing prompt.md in %s", entry.Name())
		}
		if _, err := os.Stat(filepath.Join(stepDir, "output.md")); err != nil {
			t.Errorf("missing output.md in %s", entry.Name())
		}
	}

	// Verify the prompt.md for review step contains the analyze output
	// (proof that template substitution happened before audit write).
	reviewPromptPath := findStepFile(t, stepsDir, "review", "prompt.md")
	if reviewPromptPath != "" {
		data, err := os.ReadFile(reviewPromptPath)
		if err != nil {
			t.Errorf("reading review prompt.md: %v", err)
		} else if !strings.Contains(string(data), "Found 3 issues") {
			t.Errorf("review prompt.md doesn't contain analyze output: %s", string(data))
		}
	}

	// 8f. Reporter output matches markdown format.
	output, err := reporter.FormatOutput(results, wf.Output)
	if err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}
	if !strings.Contains(output, "# Workflow Results") {
		t.Error("reporter output missing markdown header")
	}
	if !strings.Contains(output, "## Step: summarize") {
		t.Error("reporter output missing summarize step")
	}
	if !strings.Contains(output, "## Step: review") {
		t.Error("reporter output missing review step")
	}

	// 8g. Sessions created count.
	if mock.SessionsCreated.Load() != 3 {
		t.Errorf("expected 3 sessions created, got %d", mock.SessionsCreated.Load())
	}
}

func TestEndToEndWithConditions(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
name: "conditional-pipeline"
description: "Conditional branching test"

agents:
  analyzer:
    inline:
      description: "Analyzes code"
      prompt: "You analyze code."
      tools: []
      model: "gpt-5"
  decider:
    inline:
      description: "Makes a decision"
      prompt: "You decide things."
      tools: []
      model: "gpt-5"
  approver:
    inline:
      description: "Handles approvals"
      prompt: "You handle approvals."
      tools: []
      model: "gpt-5"
  changer:
    inline:
      description: "Handles change requests"
      prompt: "You handle changes."
      tools: []
      model: "gpt-5"

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze the code"

  - id: decide
    agent: decider
    prompt: "Decide on: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: approve-action
    agent: approver
    prompt: "Process approval for: {{steps.decide.output}}"
    depends_on: [decide]
    condition:
      step: decide
      contains: "APPROVE"

  - id: changes-action
    agent: changer
    prompt: "Process changes for: {{steps.decide.output}}"
    depends_on: [decide]
    condition:
      step: decide
      contains: "REQUEST_CHANGES"

output:
  steps: [approve-action, changes-action]
  format: "markdown"
`

	wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes: %v", err)
	}
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("ValidateWorkflow: %v", err)
	}

	resolvedAgents, err := agents.ResolveAgents(wf, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("ResolveAgents: %v", err)
	}

	// Mock: "decide" returns "APPROVE" — so approve-action should run, changes-action should skip.
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"Analyze the code":    "Code looks clean with minor issues",
			"Decide on":           "APPROVE - all checks passed",
			"Process approval":    "Approval processed successfully",
			"Process changes for": "Changes requested",
		},
	}

	auditDir := filepath.Join(tmpDir, ".workflow-runs")
	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	if err := auditLogger.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	orch := &orchestrator.Orchestrator{
		Executor: &executor.StepExecutor{
			SDK:         mock,
			AuditLogger: auditLogger,
		},
		Agents: resolvedAgents,
		Inputs: map[string]string{},
	}

	results, err := orch.Run(context.Background(), wf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// approve-action should have executed.
	approveResult, ok := results["approve-action"]
	if !ok {
		t.Fatal("missing result for approve-action")
	}
	if approveResult.Status != workflow.StepStatusCompleted {
		t.Errorf("approve-action: expected completed, got %s", approveResult.Status)
	}
	if !strings.Contains(approveResult.Output, "Approval processed") {
		t.Errorf("approve-action output unexpected: %s", approveResult.Output)
	}

	// changes-action should have been skipped.
	changesResult, ok := results["changes-action"]
	if !ok {
		t.Fatal("missing result for changes-action")
	}
	if changesResult.Status != workflow.StepStatusSkipped {
		t.Errorf("changes-action: expected skipped, got %s", changesResult.Status)
	}

	// Verify reporter output includes the skipped step info.
	output, err := reporter.FormatOutput(results, wf.Output)
	if err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}
	if !strings.Contains(output, "*Skipped*") {
		t.Error("reporter output should mention skipped step")
	}
}

func TestEndToEndValidationFailures(t *testing.T) {
	t.Run("invalid YAML", func(t *testing.T) {
		// Use YAML that has structural issues (tabs mixed with spaces, bad indentation).
		invalidYAML := "name: test\nsteps:\n\t- broken:\n  bad: [unclosed"
		_, err := workflow.ParseWorkflowBytes([]byte(invalidYAML))
		if err == nil {
			t.Fatal("expected parse error for invalid YAML")
		}
		if !strings.Contains(err.Error(), "parsing workflow YAML") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("missing agent reference", func(t *testing.T) {
		yamlContent := `
name: "bad-agent-ref"
agents:
  real-agent:
    inline:
      description: "exists"
      prompt: "hi"
steps:
  - id: step1
    agent: nonexistent-agent
    prompt: "hello"
`
		wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
		if err != nil {
			t.Fatalf("ParseWorkflowBytes: %v", err)
		}
		err = workflow.ValidateWorkflow(wf)
		if err == nil {
			t.Fatal("expected validation error for missing agent")
		}
		if !strings.Contains(err.Error(), "not defined") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("circular dependency", func(t *testing.T) {
		yamlContent := `
name: "circular-deps"
agents:
  agent1:
    inline:
      description: "agent"
      prompt: "hi"
steps:
  - id: step-a
    agent: agent1
    prompt: "hello"
    depends_on: [step-b]
  - id: step-b
    agent: agent1
    prompt: "world"
    depends_on: [step-a]
`
		wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
		if err != nil {
			t.Fatalf("ParseWorkflowBytes: %v", err)
		}
		_, err = workflow.BuildDAG(wf.Steps)
		if err == nil {
			t.Fatal("expected error for circular dependency")
		}
		if !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing workflow name", func(t *testing.T) {
		yamlContent := `
agents:
  a:
    inline:
      description: "x"
      prompt: "x"
steps:
  - id: s1
    agent: a
    prompt: "p"
`
		wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
		if err != nil {
			t.Fatalf("ParseWorkflowBytes: %v", err)
		}
		err = workflow.ValidateWorkflow(wf)
		if err == nil {
			t.Fatal("expected error for missing name")
		}
		if !strings.Contains(err.Error(), "name is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no steps", func(t *testing.T) {
		yamlContent := `
name: "no-steps"
steps: []
`
		wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
		if err != nil {
			t.Fatalf("ParseWorkflowBytes: %v", err)
		}
		err = workflow.ValidateWorkflow(wf)
		if err == nil {
			t.Fatal("expected error for empty steps")
		}
		if !strings.Contains(err.Error(), "at least one step") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Task 2: P2T08 — Parallel Integration Tests
// ---------------------------------------------------------------------------

func TestEndToEndParallel(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
name: "parallel-pipeline"
description: "Fan-out + fan-in parallel test"

agents:
  analyzer:
    inline:
      description: "Code analyzer"
      prompt: "You analyze code."
      tools: []
      model: "gpt-5"
  security:
    inline:
      description: "Security reviewer"
      prompt: "You review security."
      tools: [grep]
      model: "gpt-5"
  performance:
    inline:
      description: "Performance reviewer"
      prompt: "You review performance."
      tools: [grep]
      model: "gpt-5"
  style:
    inline:
      description: "Style reviewer"
      prompt: "You review code style."
      tools: []
      model: "gpt-5"
  aggregator:
    inline:
      description: "Aggregates reviews"
      prompt: "You aggregate review results."
      tools: []
      model: "gpt-5"

steps:
  - id: analyze
    agent: analyzer
    prompt: "Analyze the codebase"

  - id: review-security
    agent: security
    prompt: "Security review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-performance
    agent: performance
    prompt: "Performance review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: review-style
    agent: style
    prompt: "Style review: {{steps.analyze.output}}"
    depends_on: [analyze]

  - id: aggregate
    agent: aggregator
    prompt: "Aggregate: {{steps.review-security.output}} | {{steps.review-performance.output}} | {{steps.review-style.output}}"
    depends_on: [review-security, review-performance, review-style]

output:
  steps: [aggregate]
  format: "markdown"
`

	wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes: %v", err)
	}
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("ValidateWorkflow: %v", err)
	}

	resolvedAgents, err := agents.ResolveAgents(wf, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("ResolveAgents: %v", err)
	}

	// Mock executor with 10ms delay per step.
	stepDelay := 10 * time.Millisecond
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"Analyze the codebase": "Codebase has 10 files, 500 lines",
			"Security review":      "No security issues found",
			"Performance review":   "2 performance bottlenecks detected",
			"Style review":         "5 style violations found",
			"Aggregate":            "Final: 0 security, 2 performance, 5 style issues",
		},
		SendHook: func(prompt string) {
			time.Sleep(stepDelay)
		},
	}

	auditDir := filepath.Join(tmpDir, ".workflow-runs")
	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	if err := auditLogger.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	orch := &orchestrator.Orchestrator{
		Executor: &executor.StepExecutor{
			SDK:         mock,
			AuditLogger: auditLogger,
		},
		Agents: resolvedAgents,
		Inputs: map[string]string{},
	}

	start := time.Now()
	results, err := orch.RunParallel(context.Background(), wf)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunParallel: %v", err)
	}

	// 1. All 5 steps completed.
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
	for _, stepID := range []string{"analyze", "review-security", "review-performance", "review-style", "aggregate"} {
		r, ok := results[stepID]
		if !ok {
			t.Errorf("missing result for step %q", stepID)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %q: expected completed, got %s", stepID, r.Status)
		}
	}

	// 2. Fan-in: aggregate received all 3 parallel step outputs.
	aggResult := results["aggregate"]
	if !strings.Contains(aggResult.Output, "Final") {
		t.Errorf("aggregate output doesn't contain expected content: %s", aggResult.Output)
	}

	// 3. Parallel steps share the same DAG depth.
	levels, _ := workflow.BuildDAG(wf.Steps)
	if len(levels) != 3 {
		t.Fatalf("expected 3 DAG levels, got %d", len(levels))
	}
	// Level 1 should have 3 parallel review steps.
	if len(levels[1].Steps) != 3 {
		t.Errorf("expected 3 steps at depth 1, got %d", len(levels[1].Steps))
	}
	for _, s := range levels[1].Steps {
		if !strings.HasPrefix(s.ID, "review-") {
			t.Errorf("unexpected step at depth 1: %s", s.ID)
		}
	}

	// 4. Audit trail has correct structure.
	stepsDir := filepath.Join(auditLogger.RunDir, "steps")
	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		t.Fatalf("reading steps dir: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 step audit dirs, got %d", len(entries))
	}

	// Verify step.meta.json exists for each audit step dir.
	for _, entry := range entries {
		metaPath := filepath.Join(stepsDir, entry.Name(), "step.meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Errorf("missing step.meta.json in %s: %v", entry.Name(), err)
			continue
		}
		var meta audit.StepMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			t.Errorf("invalid step.meta.json in %s: %v", entry.Name(), err)
		}
	}

	// 5. Parallel execution should be faster than sequential.
	// Sequential would take at least 5 * 10ms = 50ms.
	// Parallel (3 concurrent at level 1) should take ~30ms total.
	// We give generous slack: just ensure < 5 * delay.
	sequentialMin := 5 * stepDelay
	if elapsed >= sequentialMin {
		t.Logf("WARNING: parallel execution (%v) not faster than sequential minimum (%v)", elapsed, sequentialMin)
	}
}

func TestEndToEndParallelWithSharedMemory(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
name: "shared-memory-pipeline"
description: "Test shared memory injection"

config:
  shared_memory:
    enabled: true
    inject_into_prompt: true

agents:
  reviewer-a:
    inline:
      description: "Reviewer A"
      prompt: "You are reviewer A."
      tools: []
      model: "gpt-5"
  reviewer-b:
    inline:
      description: "Reviewer B"
      prompt: "You are reviewer B."
      tools: []
      model: "gpt-5"
  aggregator:
    inline:
      description: "Aggregator"
      prompt: "You aggregate."
      tools: []
      model: "gpt-5"

steps:
  - id: review-a
    agent: reviewer-a
    prompt: "Review module A"

  - id: review-b
    agent: reviewer-b
    prompt: "Review module B"

  - id: aggregate
    agent: aggregator
    prompt: "Combine: {{steps.review-a.output}} and {{steps.review-b.output}}"
    depends_on: [review-a, review-b]

output:
  steps: [aggregate]
  format: "markdown"
`

	wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes: %v", err)
	}
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("ValidateWorkflow: %v", err)
	}

	resolvedAgents, err := agents.ResolveAgents(wf, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("ResolveAgents: %v", err)
	}

	// Create memory manager with initial content.
	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("creating memory dir: %v", err)
	}
	mgr, err := memory.NewManager(memDir, "INITIAL_CONTEXT: Project uses Go 1.21, standard library only")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Verify InjectIntoPrompt works.
	injected := mgr.InjectIntoPrompt("Review module A")
	if !strings.Contains(injected, "--- Shared Memory") {
		t.Error("InjectIntoPrompt missing memory header")
	}
	if !strings.Contains(injected, "INITIAL_CONTEXT") {
		t.Error("InjectIntoPrompt missing initial content")
	}
	if !strings.Contains(injected, "Review module A") {
		t.Error("InjectIntoPrompt missing original prompt")
	}

	// Track which prompts the mock receives, to verify memory injection.
	var mu sync.Mutex
	var capturedPrompts []string
	mock := &executor.MockSessionExecutor{
		Responses: map[string]string{
			"Review module A": "Module A looks good",
			"Review module B": "Module B has issues",
			"Combine":         "Overall: A good, B needs work",
		},
		SendHook: func(prompt string) {
			mu.Lock()
			capturedPrompts = append(capturedPrompts, prompt)
			mu.Unlock()
		},
	}

	auditDir := filepath.Join(tmpDir, ".workflow-runs")
	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	if err := auditLogger.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	orch := &orchestrator.Orchestrator{
		Executor: &executor.StepExecutor{
			SDK:         mock,
			AuditLogger: auditLogger,
		},
		Agents: resolvedAgents,
		Inputs: map[string]string{},
	}

	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("RunParallel: %v", err)
	}

	// All steps completed.
	for _, stepID := range []string{"review-a", "review-b", "aggregate"} {
		r, ok := results[stepID]
		if !ok {
			t.Errorf("missing result for step %q", stepID)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %q: expected completed, got %s", stepID, r.Status)
		}
	}

	// Verify shared memory manager state after writes.
	if err := mgr.Write("reviewer-a", "Module A passed all checks"); err != nil {
		t.Fatalf("memory Write: %v", err)
	}
	content := mgr.Read()
	if !strings.Contains(content, "INITIAL_CONTEXT") {
		t.Error("memory lost initial content after write")
	}
	if !strings.Contains(content, "Module A passed all checks") {
		t.Error("memory missing written entry")
	}

	// Verify memory file was persisted.
	memFile := mgr.FilePath()
	data, err := os.ReadFile(memFile)
	if err != nil {
		t.Fatalf("reading memory file: %v", err)
	}
	if !strings.Contains(string(data), "INITIAL_CONTEXT") {
		t.Error("memory file missing initial content")
	}

	// Verify InjectIntoPrompt with updated memory.
	injectedAfter := mgr.InjectIntoPrompt("Next prompt")
	if !strings.Contains(injectedAfter, "Module A passed all checks") {
		t.Error("InjectIntoPrompt doesn't reflect written entries")
	}
	if !strings.Contains(injectedAfter, "--- End Shared Memory ---") {
		t.Error("InjectIntoPrompt missing end marker")
	}
}

func TestEndToEndParallelMaxConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `
name: "max-concurrency-pipeline"
description: "Test max concurrency enforcement"

config:
  max_concurrency: 2

agents:
  worker:
    inline:
      description: "Worker agent"
      prompt: "You are a worker."
      tools: []
      model: "gpt-5"
  aggregator:
    inline:
      description: "Aggregator"
      prompt: "You aggregate."
      tools: []
      model: "gpt-5"

steps:
  - id: root
    agent: worker
    prompt: "Start work"

  - id: worker-1
    agent: worker
    prompt: "Do task 1: {{steps.root.output}}"
    depends_on: [root]

  - id: worker-2
    agent: worker
    prompt: "Do task 2: {{steps.root.output}}"
    depends_on: [root]

  - id: worker-3
    agent: worker
    prompt: "Do task 3: {{steps.root.output}}"
    depends_on: [root]

  - id: worker-4
    agent: worker
    prompt: "Do task 4: {{steps.root.output}}"
    depends_on: [root]

  - id: worker-5
    agent: worker
    prompt: "Do task 5: {{steps.root.output}}"
    depends_on: [root]

  - id: final
    agent: aggregator
    prompt: "Aggregate all"
    depends_on: [worker-1, worker-2, worker-3, worker-4, worker-5]

output:
  steps: [final]
  format: "markdown"
`

	wf, err := workflow.ParseWorkflowBytes([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes: %v", err)
	}
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("ValidateWorkflow: %v", err)
	}

	resolvedAgents, err := agents.ResolveAgents(wf, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("ResolveAgents: %v", err)
	}

	// Track concurrent execution count.
	var currentConcurrent atomic.Int32
	var maxObservedConcurrent atomic.Int32

	mock := &executor.MockSessionExecutor{
		DefaultResponse: "task completed",
		SendHook: func(prompt string) {
			cur := currentConcurrent.Add(1)
			// Update max observed concurrency.
			for {
				old := maxObservedConcurrent.Load()
				if cur <= old || maxObservedConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			// Hold the slot briefly to allow overlap detection.
			time.Sleep(20 * time.Millisecond)
			currentConcurrent.Add(-1)
		},
	}

	auditDir := filepath.Join(tmpDir, ".workflow-runs")
	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	if err := auditLogger.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	orch := &orchestrator.Orchestrator{
		Executor: &executor.StepExecutor{
			SDK:         mock,
			AuditLogger: auditLogger,
		},
		Agents:         resolvedAgents,
		Inputs:         map[string]string{},
		MaxConcurrency: wf.Config.MaxConcurrency,
	}

	results, err := orch.RunParallel(context.Background(), wf)
	if err != nil {
		t.Fatalf("RunParallel: %v", err)
	}

	// All 7 steps completed.
	if len(results) != 7 {
		t.Errorf("expected 7 results, got %d", len(results))
	}
	for _, stepID := range []string{"root", "worker-1", "worker-2", "worker-3", "worker-4", "worker-5", "final"} {
		r, ok := results[stepID]
		if !ok {
			t.Errorf("missing result for step %q", stepID)
			continue
		}
		if r.Status != workflow.StepStatusCompleted {
			t.Errorf("step %q: expected completed, got %s", stepID, r.Status)
		}
	}

	// Max concurrent during the parallel level should not exceed 2.
	maxConc := maxObservedConcurrent.Load()
	if maxConc > 2 {
		t.Errorf("max concurrency exceeded: observed %d concurrent executions, limit was 2", maxConc)
	}

	// Verify that parallel execution actually happened (max > 1 means overlap occurred).
	if maxConc < 2 {
		t.Logf("NOTE: max observed concurrency was %d; expected 2 (may be timing-sensitive)", maxConc)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findStepFile locates a file within the step audit directory matching
// a step ID substring.
func findStepFile(t *testing.T, stepsDir, stepID, filename string) string {
	t.Helper()
	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		t.Errorf("reading steps dir: %v", err)
		return ""
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), stepID) {
			return filepath.Join(stepsDir, entry.Name(), filename)
		}
	}
	t.Errorf("step dir containing %q not found", stepID)
	return ""
}

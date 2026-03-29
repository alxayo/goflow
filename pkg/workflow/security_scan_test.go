package workflow_test

import (
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

func TestSecurityScanWorkflow(t *testing.T) {
	wf, err := workflow.ParseWorkflow("../../examples/security-scan/security-scan.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if wf.Name != "security-scan" {
		t.Errorf("name = %q, want %q", wf.Name, "security-scan")
	}
	if len(wf.Steps) != 8 {
		t.Errorf("steps count = %d, want 8", len(wf.Steps))
	}
	if len(wf.Agents) != 8 {
		t.Errorf("agents count = %d, want 8", len(wf.Agents))
	}
	if len(wf.Skills) != 5 {
		t.Errorf("skills count = %d, want 5", len(wf.Skills))
	}
	if wf.Config.Interactive {
		t.Error("config.interactive should be false")
	}
	if wf.Config.MaxConcurrency != 5 {
		t.Errorf("max_concurrency = %d, want 5", wf.Config.MaxConcurrency)
	}

	// Validate semantic correctness.
	if err := workflow.ValidateWorkflow(wf); err != nil {
		t.Fatalf("validation error: %v", err)
	}

	// Build DAG and verify levels.
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		t.Fatalf("DAG error: %v", err)
	}

	// Expect 4 levels: discover → 5 scanners → aggregate → remediation-plan
	if len(levels) != 4 {
		t.Fatalf("DAG levels = %d, want 4", len(levels))
	}

	// Level 0: discover (1 step)
	if len(levels[0].Steps) != 1 || levels[0].Steps[0].ID != "discover" {
		ids := stepIDs(levels[0].Steps)
		t.Errorf("level 0 = %v, want [discover]", ids)
	}

	// Level 1: 5 parallel scanners
	if len(levels[1].Steps) != 5 {
		ids := stepIDs(levels[1].Steps)
		t.Errorf("level 1 = %v, want 5 parallel scanners", ids)
	}
	scannerIDs := map[string]bool{
		"scan-python":          false,
		"scan-supply-chain":    false,
		"scan-shell":           false,
		"scan-patterns":        false,
		"scan-vulnerabilities": false,
	}
	for _, s := range levels[1].Steps {
		if _, ok := scannerIDs[s.ID]; !ok {
			t.Errorf("unexpected step %q in level 1", s.ID)
		}
		scannerIDs[s.ID] = true
	}
	for id, found := range scannerIDs {
		if !found {
			t.Errorf("missing scanner %q in level 1", id)
		}
	}

	// Level 2: aggregate (1 step, depends on all 5 scanners)
	if len(levels[2].Steps) != 1 || levels[2].Steps[0].ID != "aggregate" {
		ids := stepIDs(levels[2].Steps)
		t.Errorf("level 2 = %v, want [aggregate]", ids)
	}

	// Level 3: remediation-plan (1 step, depends on aggregate)
	if len(levels[3].Steps) != 1 || levels[3].Steps[0].ID != "remediation-plan" {
		ids := stepIDs(levels[3].Steps)
		t.Errorf("level 3 = %v, want [remediation-plan]", ids)
	}

	// Verify on_error=continue and conditions for scanner steps.
	conditionMarkers := map[string]string{
		"scan-python":          "[RUN:BANDIT]",
		"scan-supply-chain":    "[RUN:GUARDDOG]",
		"scan-shell":           "[RUN:SHELLCHECK]",
		"scan-patterns":        "[RUN:GRAUDIT]",
		"scan-vulnerabilities": "[RUN:TRIVY]",
	}
	for _, s := range wf.Steps {
		marker, isScanner := conditionMarkers[s.ID]
		if !isScanner {
			continue
		}
		if s.OnError != "continue" {
			t.Errorf("step %q on_error = %q, want %q", s.ID, s.OnError, "continue")
		}
		if s.Condition == nil {
			t.Errorf("step %q missing condition", s.ID)
			continue
		}
		if s.Condition.Step != "discover" {
			t.Errorf("step %q condition.step = %q, want %q", s.ID, s.Condition.Step, "discover")
		}
		if s.Condition.Contains != marker {
			t.Errorf("step %q condition.contains = %q, want %q", s.ID, s.Condition.Contains, marker)
		}
	}

	// Verify output config.
	if len(wf.Output.Steps) != 2 {
		t.Errorf("output steps = %d, want 2", len(wf.Output.Steps))
	}
}

func stepIDs(steps []workflow.Step) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}

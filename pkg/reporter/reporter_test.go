package reporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

func TestFormatOutput(t *testing.T) {
	tests := []struct {
		name      string
		results   map[string]*workflow.StepResult
		outputCfg workflow.OutputConfig
		// wantContains lists substrings that must appear in the output.
		wantContains []string
		// wantNotContains lists substrings that must NOT appear.
		wantNotContains []string
		wantError       bool
	}{
		{
			name: "markdown format with 2 completed steps",
			results: map[string]*workflow.StepResult{
				"analyze": {
					StepID: "analyze",
					Status: workflow.StepStatusCompleted,
					Output: "Found 3 issues",
				},
				"review": {
					StepID: "review",
					Status: workflow.StepStatusCompleted,
					Output: "All issues addressed",
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{"analyze", "review"},
				Format: "markdown",
			},
			wantContains: []string{
				"# Workflow Results",
				"## Step: analyze",
				"Found 3 issues",
				"## Step: review",
				"All issues addressed",
			},
		},
		{
			name: "JSON format with completed and skipped step",
			results: map[string]*workflow.StepResult{
				"analyze": {
					StepID: "analyze",
					Status: workflow.StepStatusCompleted,
					Output: "Found 3 issues",
				},
				"optional": {
					StepID: "optional",
					Status: workflow.StepStatusSkipped,
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{"analyze", "optional"},
				Format: "json",
			},
			wantContains: []string{
				`"analyze"`,
				`"completed"`,
				`"Found 3 issues"`,
				`"optional"`,
				`"skipped"`,
			},
		},
		{
			name: "plain text format",
			results: map[string]*workflow.StepResult{
				"analyze": {
					StepID: "analyze",
					Status: workflow.StepStatusCompleted,
					Output: "Found 3 issues",
				},
				"review": {
					StepID: "review",
					Status: workflow.StepStatusCompleted,
					Output: "All clear",
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{"analyze", "review"},
				Format: "plain",
			},
			wantContains: []string{
				"=== analyze ===",
				"Found 3 issues",
				"=== review ===",
				"All clear",
			},
		},
		{
			name: "empty output steps includes all completed",
			results: map[string]*workflow.StepResult{
				"step-a": {
					StepID: "step-a",
					Status: workflow.StepStatusCompleted,
					Output: "output A",
				},
				"step-b": {
					StepID: "step-b",
					Status: workflow.StepStatusSkipped,
				},
				"step-c": {
					StepID: "step-c",
					Status: workflow.StepStatusCompleted,
					Output: "output C",
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{},
				Format: "markdown",
			},
			wantContains: []string{
				"## Step: step-a",
				"output A",
				"## Step: step-c",
				"output C",
			},
			wantNotContains: []string{
				"## Step: step-b",
			},
		},
		{
			name: "missing step in results silently skipped",
			results: map[string]*workflow.StepResult{
				"analyze": {
					StepID: "analyze",
					Status: workflow.StepStatusCompleted,
					Output: "Found 3 issues",
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{"analyze", "nonexistent"},
				Format: "markdown",
			},
			wantContains: []string{
				"## Step: analyze",
				"Found 3 issues",
			},
			wantNotContains: []string{
				"nonexistent",
			},
		},
		{
			name: "unknown format defaults to markdown",
			results: map[string]*workflow.StepResult{
				"analyze": {
					StepID: "analyze",
					Status: workflow.StepStatusCompleted,
					Output: "Found 3 issues",
				},
			},
			outputCfg: workflow.OutputConfig{
				Steps:  []string{"analyze"},
				Format: "yaml",
			},
			wantContains: []string{
				"# Workflow Results",
				"## Step: analyze",
				"Found 3 issues",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatOutput(tt.results, tt.outputCfg)
			if (err != nil) != tt.wantError {
				t.Fatalf("FormatOutput() error = %v, wantError = %v", err, tt.wantError)
			}
			if tt.wantError {
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing expected substring %q\ngot:\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("output should not contain %q\ngot:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestFormatOutputJSON_Structure(t *testing.T) {
	results := map[string]*workflow.StepResult{
		"analyze": {
			StepID: "analyze",
			Status: workflow.StepStatusCompleted,
			Output: "Found 3 issues",
		},
		"optional": {
			StepID: "optional",
			Status: workflow.StepStatusSkipped,
		},
	}
	cfg := workflow.OutputConfig{
		Steps:  []string{"analyze", "optional"},
		Format: "json",
	}

	got, err := FormatOutput(results, cfg)
	if err != nil {
		t.Fatalf("FormatOutput() error = %v", err)
	}

	// Verify it's valid JSON with expected structure
	var parsed struct {
		Steps map[string]struct {
			Status string `json:"status"`
			Output string `json:"output"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot:\n%s", err, got)
	}

	if s, ok := parsed.Steps["analyze"]; !ok {
		t.Error("missing 'analyze' step in JSON")
	} else {
		if s.Status != "completed" {
			t.Errorf("analyze status = %q, want %q", s.Status, "completed")
		}
		if s.Output != "Found 3 issues" {
			t.Errorf("analyze output = %q, want %q", s.Output, "Found 3 issues")
		}
	}

	if s, ok := parsed.Steps["optional"]; !ok {
		t.Error("missing 'optional' step in JSON")
	} else {
		if s.Status != "skipped" {
			t.Errorf("optional status = %q, want %q", s.Status, "skipped")
		}
		if s.Output != "" {
			t.Errorf("optional output should be empty for skipped step, got %q", s.Output)
		}
	}
}

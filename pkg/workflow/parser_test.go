package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// P1T03 — Parser happy-path tests
// ---------------------------------------------------------------------------

func TestParseWorkflowBytes(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		check   func(t *testing.T, wf *Workflow)
		wantErr string
	}{
		{
			name: "valid minimal workflow",
			yaml: `
name: minimal
steps:
  - id: greet
    agent: bot
    prompt: "Say hello"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Name != "minimal" {
					t.Errorf("name = %q, want %q", wf.Name, "minimal")
				}
				if len(wf.Steps) != 1 {
					t.Fatalf("steps count = %d, want 1", len(wf.Steps))
				}
				if wf.Steps[0].ID != "greet" {
					t.Errorf("step id = %q, want %q", wf.Steps[0].ID, "greet")
				}
			},
		},
		{
			name: "valid full workflow with all fields",
			yaml: `
name: full-pipeline
description: "End-to-end pipeline"
inputs:
  files:
    description: "glob pattern"
    default: "src/**/*.go"
config:
  model: gpt-5
  audit_dir: ".runs"
  audit_retention: 5
  shared_memory:
    enabled: true
    inject_into_prompt: true
  log_level: debug
  max_concurrency: 4
agents:
  reviewer:
    file: "./agents/reviewer.agent.md"
  summarizer:
    inline:
      description: "Summarizes output"
      prompt: "Summarize"
      tools: [grep, view]
      model: gpt-5
steps:
  - id: analyze
    agent: reviewer
    prompt: "Analyze code"
  - id: summarize
    agent: summarizer
    prompt: "Summarize {{steps.analyze.output}}"
    depends_on: [analyze]
    condition:
      step: analyze
      contains: "ISSUE"
output:
  steps: [summarize]
  format: json
  truncate:
    strategy: chars
    limit: 2000
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Name != "full-pipeline" {
					t.Errorf("name = %q", wf.Name)
				}
				if wf.Description != "End-to-end pipeline" {
					t.Errorf("description = %q", wf.Description)
				}
				if wf.Config.AuditDir != ".runs" {
					t.Errorf("audit_dir = %q, want %q", wf.Config.AuditDir, ".runs")
				}
				if wf.Config.AuditRetention != 5 {
					t.Errorf("audit_retention = %d", wf.Config.AuditRetention)
				}
				if !wf.Config.SharedMemory.Enabled {
					t.Error("shared_memory.enabled should be true")
				}
				if wf.Config.LogLevel != "debug" {
					t.Errorf("log_level = %q, want %q", wf.Config.LogLevel, "debug")
				}
				if wf.Config.MaxConcurrency != 4 {
					t.Errorf("max_concurrency = %d", wf.Config.MaxConcurrency)
				}
				if len(wf.Agents) != 2 {
					t.Fatalf("agents count = %d, want 2", len(wf.Agents))
				}
				if wf.Agents["reviewer"].File != "./agents/reviewer.agent.md" {
					t.Errorf("reviewer file = %q", wf.Agents["reviewer"].File)
				}
				if wf.Agents["summarizer"].Inline == nil {
					t.Fatal("summarizer inline is nil")
				}
				if len(wf.Steps) != 2 {
					t.Fatalf("steps count = %d, want 2", len(wf.Steps))
				}
				if wf.Steps[1].Condition == nil {
					t.Fatal("step[1] condition is nil")
				}
				if wf.Steps[1].Condition.Contains != "ISSUE" {
					t.Errorf("condition.contains = %q", wf.Steps[1].Condition.Contains)
				}
				if wf.Output.Format != "json" {
					t.Errorf("output.format = %q", wf.Output.Format)
				}
				if wf.Output.Truncate == nil || wf.Output.Truncate.Limit != 2000 {
					t.Error("truncate config not parsed correctly")
				}
			},
		},
		{
			name: "default audit_dir when config omitted",
			yaml: `
name: defaults
steps:
  - id: s1
    agent: a
    prompt: "do stuff"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Config.AuditDir != ".workflow-runs" {
					t.Errorf("audit_dir = %q, want %q", wf.Config.AuditDir, ".workflow-runs")
				}
			},
		},
		{
			name: "default format when output.format omitted",
			yaml: `
name: defaults
steps:
  - id: s1
    agent: a
    prompt: "do stuff"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Output.Format != "markdown" {
					t.Errorf("format = %q, want %q", wf.Output.Format, "markdown")
				}
			},
		},
		{
			name: "default log_level when not set",
			yaml: `
name: defaults
steps:
  - id: s1
    agent: a
    prompt: "do stuff"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Config.LogLevel != "info" {
					t.Errorf("log_level = %q, want %q", wf.Config.LogLevel, "info")
				}
			},
		},
		{
			name:    "invalid YAML",
			yaml:    `name: [broken`,
			wantErr: "parsing workflow YAML",
		},
		{
			name: "step with extra_dirs",
			yaml: `
name: scoped
steps:
  - id: s1
    agent: a
    prompt: "do work"
    extra_dirs:
      - "./extra/security"
      - "./extra/common"
`,
			check: func(t *testing.T, wf *Workflow) {
				if len(wf.Steps[0].ExtraDirs) != 2 {
					t.Fatalf("extra_dirs count = %d, want 2", len(wf.Steps[0].ExtraDirs))
				}
				if wf.Steps[0].ExtraDirs[0] != "./extra/security" {
					t.Errorf("extra_dirs[0] = %q", wf.Steps[0].ExtraDirs[0])
				}
				if wf.Steps[0].ExtraDirs[1] != "./extra/common" {
					t.Errorf("extra_dirs[1] = %q", wf.Steps[0].ExtraDirs[1])
				}
			},
		},
		{
			name: "step without extra_dirs defaults to nil",
			yaml: `
name: no-extra
steps:
  - id: s1
    agent: a
    prompt: "do stuff"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].ExtraDirs != nil {
					t.Errorf("extra_dirs should be nil, got %v", wf.Steps[0].ExtraDirs)
				}
			},
		},
		{
			name: "step with model override",
			yaml: `
name: model-test
steps:
  - id: s1
    agent: a
    prompt: "analyze"
    model: claude-sonnet-4.5
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].Model != "claude-sonnet-4.5" {
					t.Errorf("step model = %q, want %q", wf.Steps[0].Model, "claude-sonnet-4.5")
				}
			},
		},
		{
			name: "step without model defaults to empty",
			yaml: `
name: no-model
steps:
  - id: s1
    agent: a
    prompt: "work"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].Model != "" {
					t.Errorf("step model should be empty, got %q", wf.Steps[0].Model)
				}
			},
		},
		{
			name: "config interactive parsed",
			yaml: `
name: interactive-wf
config:
  interactive: true
steps:
  - id: s1
    agent: a
    prompt: "ask me things"
`,
			check: func(t *testing.T, wf *Workflow) {
				if !wf.Config.Interactive {
					t.Error("config.interactive should be true")
				}
			},
		},
		{
			name: "config interactive defaults to false",
			yaml: `
name: non-interactive-wf
steps:
  - id: s1
    agent: a
    prompt: "autonomous work"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Config.Interactive {
					t.Error("config.interactive should default to false")
				}
			},
		},
		{
			name: "step interactive field parsed as true",
			yaml: `
name: step-interactive
steps:
  - id: s1
    agent: a
    prompt: "ask me"
    interactive: true
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].Interactive == nil {
					t.Fatal("step.interactive should not be nil")
				}
				if !*wf.Steps[0].Interactive {
					t.Error("step.interactive should be true")
				}
			},
		},
		{
			name: "step interactive field parsed as false",
			yaml: `
name: step-no-interactive
steps:
  - id: s1
    agent: a
    prompt: "no questions"
    interactive: false
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].Interactive == nil {
					t.Fatal("step.interactive should not be nil when explicitly false")
				}
				if *wf.Steps[0].Interactive {
					t.Error("step.interactive should be false")
				}
			},
		},
		{
			name: "step interactive unset defaults to nil",
			yaml: `
name: step-unset
steps:
  - id: s1
    agent: a
    prompt: "inherit"
`,
			check: func(t *testing.T, wf *Workflow) {
				if wf.Steps[0].Interactive != nil {
					t.Errorf("step.interactive should be nil when unset, got %v", *wf.Steps[0].Interactive)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := ParseWorkflowBytes([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, wf)
			}
		})
	}
}

func TestParseWorkflow_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	content := `
name: from-file
steps:
  - id: step1
    agent: bot
    prompt: "Hello"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	wf, err := ParseWorkflow(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "from-file" {
		t.Errorf("name = %q, want %q", wf.Name, "from-file")
	}
}

func TestParseWorkflow_FileNotFound(t *testing.T) {
	_, err := ParseWorkflow("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading workflow file") {
		t.Errorf("error %q missing expected prefix", err)
	}
}

// ---------------------------------------------------------------------------
// P1T04 — Validation tests
// ---------------------------------------------------------------------------

func TestValidateWorkflow(t *testing.T) {
	// helper builds a minimal valid workflow for mutation in each test case.
	minimal := func() *Workflow {
		return &Workflow{
			Name: "valid",
			Agents: map[string]AgentRef{
				"bot": {File: "./agents/bot.agent.md"},
			},
			Steps: []Step{
				{ID: "s1", Agent: "bot", Prompt: "do work"},
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(wf *Workflow)
		wantErr string
	}{
		{
			name:    "empty name",
			mutate:  func(wf *Workflow) { wf.Name = "" },
			wantErr: "workflow name is required",
		},
		{
			name:    "no steps",
			mutate:  func(wf *Workflow) { wf.Steps = nil },
			wantErr: "workflow must have at least one step",
		},
		{
			name: "duplicate step ID",
			mutate: func(wf *Workflow) {
				wf.Steps = append(wf.Steps, Step{ID: "s1", Agent: "bot", Prompt: "dup"})
			},
			wantErr: `duplicate step ID "s1"`,
		},
		{
			name: "step missing agent",
			mutate: func(wf *Workflow) {
				wf.Steps[0].Agent = ""
			},
			wantErr: `step "s1": agent is required`,
		},
		{
			name: "step missing prompt",
			mutate: func(wf *Workflow) {
				wf.Steps[0].Prompt = ""
			},
			wantErr: `step "s1": prompt is required`,
		},
		{
			name: "unknown agent ref",
			mutate: func(wf *Workflow) {
				wf.Steps[0].Agent = "ghost"
			},
			wantErr: `step "s1": agent "ghost" not defined in workflow agents`,
		},
		{
			name: "unknown depends_on",
			mutate: func(wf *Workflow) {
				wf.Steps[0].DependsOn = []string{"nonexistent"}
			},
			wantErr: `step "s1": depends_on references unknown step "nonexistent"`,
		},
		{
			name: "self dependency",
			mutate: func(wf *Workflow) {
				wf.Steps[0].DependsOn = []string{"s1"}
			},
			wantErr: `step "s1": cannot depend on itself`,
		},
		{
			name: "unknown condition step",
			mutate: func(wf *Workflow) {
				wf.Steps[0].Condition = &Condition{Step: "phantom", Contains: "x"}
			},
			wantErr: `step "s1": condition references unknown step "phantom"`,
		},
		{
			name: "condition step not in transitive dependencies",
			mutate: func(wf *Workflow) {
				wf.Steps = append(wf.Steps, Step{
					ID: "s2", Agent: "bot", Prompt: "extra",
				})
				// s1 has condition on s2 but does NOT depend on s2
				wf.Steps[0].Condition = &Condition{Step: "s2", Contains: "x"}
			},
			wantErr: `step "s1": condition step "s2" must be an upstream dependency`,
		},
		{
			name: "invalid template ref",
			mutate: func(wf *Workflow) {
				wf.Steps[0].Prompt = "Use {{steps.missing.output}}"
			},
			wantErr: `step "s1": template references unknown step "missing"`,
		},
		{
			name: "agent has neither file nor inline",
			mutate: func(wf *Workflow) {
				wf.Agents["bot"] = AgentRef{}
			},
			wantErr: `agent "bot": must have either 'file' or 'inline' defined`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := minimal()
			tt.mutate(wf)
			err := ValidateWorkflow(wf)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}

	// Positive case: minimal valid workflow passes validation.
	t.Run("valid workflow passes", func(t *testing.T) {
		if err := ValidateWorkflow(minimal()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

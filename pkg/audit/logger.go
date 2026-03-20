// Package audit manages the audit trail for workflow runs. Each run creates
// a timestamped directory with metadata files, step folders, and output
// files for full transparency and debugging.
package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// RunLogger manages the audit trail for a single workflow run.
type RunLogger struct {
	RunDir       string
	WorkflowName string
	startedAt    time.Time
}

// runMeta is the JSON schema for workflow.meta.json.
type runMeta struct {
	WorkflowName string            `json:"workflow_name"`
	StartedAt    string            `json:"started_at"`
	CompletedAt  string            `json:"completed_at,omitempty"`
	Status       string            `json:"status"`
	Inputs       map[string]string `json:"inputs"`
	ConfigHash   string            `json:"config_hash"`
}

// NewRunLogger creates a timestamped audit directory for a workflow run.
// Directory format: <audit_dir>/<timestamp>_<workflow-name>/
// Timestamp uses "-" instead of ":" for filesystem compatibility.
func NewRunLogger(auditDir, workflowName string) (*RunLogger, error) {
	now := time.Now()
	ts := now.Format("2006-01-02T15-04-05")
	dirName := fmt.Sprintf("%s_%s", ts, workflowName)
	runDir := filepath.Join(auditDir, dirName)

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating audit run directory: %w", err)
	}

	return &RunLogger{
		RunDir:       runDir,
		WorkflowName: workflowName,
		startedAt:    now,
	}, nil
}

// WriteWorkflowMeta writes the initial workflow.meta.json with run metadata.
// Contains: workflow_name, started_at, status ("running"), inputs, config_hash.
func (rl *RunLogger) WriteWorkflowMeta(wf *workflow.Workflow, inputs map[string]string) error {
	meta := runMeta{
		WorkflowName: wf.Name,
		StartedAt:    rl.startedAt.Format(time.RFC3339),
		Status:       "running",
		Inputs:       inputs,
		ConfigHash:   configHash(wf.Config),
	}
	return writeJSON(filepath.Join(rl.RunDir, "workflow.meta.json"), meta)
}

// SnapshotWorkflow copies the workflow YAML into the audit directory as workflow.yaml.
func (rl *RunLogger) SnapshotWorkflow(yamlPath string) error {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("reading workflow file for snapshot: %w", err)
	}
	dst := filepath.Join(rl.RunDir, "workflow.yaml")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("writing workflow snapshot: %w", err)
	}
	return nil
}

// FinalizeRun writes final_output.md and updates workflow.meta.json with
// end time and final status.
func (rl *RunLogger) FinalizeRun(status string, outputs map[string]string, outputSteps []string) error {
	// Write final_output.md aggregating specified steps.
	var sb strings.Builder
	for _, stepID := range outputSteps {
		sb.WriteString(fmt.Sprintf("## %s\n\n", stepID))
		if out, ok := outputs[stepID]; ok {
			sb.WriteString(out)
		}
		sb.WriteString("\n\n")
	}
	outputPath := filepath.Join(rl.RunDir, "final_output.md")
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing final output: %w", err)
	}

	// Read existing workflow.meta.json and update it.
	metaPath := filepath.Join(rl.RunDir, "workflow.meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("reading workflow meta for finalize: %w", err)
	}
	var meta runMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing workflow meta: %w", err)
	}
	meta.CompletedAt = time.Now().Format(time.RFC3339)
	meta.Status = status

	return writeJSON(metaPath, meta)
}

// StepLogger creates a per-step audit logger.
type StepLogger struct {
	StepDir string
	StepID  string
	SeqNum  int
}

// StepMeta is the structured metadata for a single step execution.
type StepMeta struct {
	StepID       string              `json:"step_id"`
	Agent        string              `json:"agent"`
	AgentFile    string              `json:"agent_file"`
	Model        string              `json:"model"`
	Status       string              `json:"status"`
	StartedAt    string              `json:"started_at"`
	CompletedAt  string              `json:"completed_at"`
	DurationSecs float64             `json:"duration_seconds"`
	OutputFile   string              `json:"output_file"`
	DependsOn    []string            `json:"depends_on"`
	Condition    *workflow.Condition `json:"condition,omitempty"`
	ConditionMet *bool               `json:"condition_result,omitempty"`
	SessionID    string              `json:"session_id,omitempty"`
	Error        string              `json:"error,omitempty"`
}

// NewStepLogger creates the step subdirectory and returns a StepLogger.
// Directory name: steps/<seq>_<step-id>/ (e.g., "steps/01_analyze/")
// seq is zero-padded to 2 digits.
func (rl *RunLogger) NewStepLogger(stepID string, seqNum int) (*StepLogger, error) {
	dirName := fmt.Sprintf("%02d_%s", seqNum, stepID)
	stepDir := filepath.Join(rl.RunDir, "steps", dirName)

	if err := os.MkdirAll(stepDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating step directory %q: %w", dirName, err)
	}

	return &StepLogger{
		StepDir: stepDir,
		StepID:  stepID,
		SeqNum:  seqNum,
	}, nil
}

// WriteStepMeta writes step.meta.json for a completed or failed step.
func (sl *StepLogger) WriteStepMeta(meta StepMeta) error {
	return writeJSON(filepath.Join(sl.StepDir, "step.meta.json"), meta)
}

// WritePrompt writes the resolved prompt to prompt.md in the step directory.
func (sl *StepLogger) WritePrompt(prompt string) error {
	p := filepath.Join(sl.StepDir, "prompt.md")
	if err := os.WriteFile(p, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("writing prompt for step %q: %w", sl.StepID, err)
	}
	return nil
}

// WriteOutput writes the step's final output to output.md.
func (sl *StepLogger) WriteOutput(output string) error {
	p := filepath.Join(sl.StepDir, "output.md")
	if err := os.WriteFile(p, []byte(output), 0o644); err != nil {
		return fmt.Errorf("writing output for step %q: %w", sl.StepID, err)
	}
	return nil
}

// writeJSON marshals v as indented JSON and writes it to path.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	return nil
}

// configHash returns a short SHA-256 hex digest of the JSON-serialized config,
// providing a lightweight way to detect config changes between runs.
func configHash(cfg workflow.Config) string {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "unknown"
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

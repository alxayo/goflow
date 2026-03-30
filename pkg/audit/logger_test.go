package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// --- P1T13: RunLogger & StepLogger tests ---

func TestNewRunLogger_DirectoryFormat(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "code-review-pipeline")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	base := filepath.Base(rl.RunDir)
	// Expect format: YYYY-MM-DDTHH-MM-SS_<name>
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}_code-review-pipeline$`)
	if !re.MatchString(base) {
		t.Errorf("directory name %q does not match expected timestamp format", base)
	}

	info, err := os.Stat(rl.RunDir)
	if err != nil {
		t.Fatalf("stat run dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("run dir is not a directory")
	}
}

func TestWriteWorkflowMeta(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	wf := &workflow.Workflow{
		Name: "test-wf",
		Config: workflow.Config{
			Model:    "gpt-5",
			AuditDir: ".workflow-runs",
		},
	}
	inputs := map[string]string{"files": "src/**/*.go"}

	if err := rl.WriteWorkflowMeta(wf, inputs); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rl.RunDir, "workflow.meta.json"))
	if err != nil {
		t.Fatalf("reading meta: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if meta["workflow_name"] != "test-wf" {
		t.Errorf("workflow_name = %v, want test-wf", meta["workflow_name"])
	}
	if meta["status"] != "running" {
		t.Errorf("status = %v, want running", meta["status"])
	}
	if meta["started_at"] == nil || meta["started_at"] == "" {
		t.Error("started_at is missing")
	}
	if meta["config_hash"] == nil || meta["config_hash"] == "" {
		t.Error("config_hash is missing")
	}

	inputsMap, ok := meta["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("inputs not a map")
	}
	if inputsMap["files"] != "src/**/*.go" {
		t.Errorf("inputs.files = %v, want src/**/*.go", inputsMap["files"])
	}
}

func TestSnapshotWorkflow(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	yamlContent := "name: test-wf\nsteps: []\n"
	yamlPath := filepath.Join(tmp, "original.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("writing yaml: %v", err)
	}

	if err := rl.SnapshotWorkflow(yamlPath); err != nil {
		t.Fatalf("SnapshotWorkflow: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rl.RunDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if string(data) != yamlContent {
		t.Errorf("snapshot mismatch: got %q", string(data))
	}
}

func TestNewStepLogger_DirectoryFormat(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl1, err := rl.NewStepLogger("analyze", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	sl2, err := rl.NewStepLogger("review", 2)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	if filepath.Base(sl1.StepDir) != "01_analyze" {
		t.Errorf("step1 dir = %q, want 01_analyze", filepath.Base(sl1.StepDir))
	}
	if filepath.Base(sl2.StepDir) != "02_review" {
		t.Errorf("step2 dir = %q, want 02_review", filepath.Base(sl2.StepDir))
	}

	// Both directories should exist.
	for _, sl := range []*StepLogger{sl1, sl2} {
		info, err := os.Stat(sl.StepDir)
		if err != nil {
			t.Errorf("step dir %s does not exist: %v", sl.StepDir, err)
		} else if !info.IsDir() {
			t.Errorf("step dir %s is not a directory", sl.StepDir)
		}
	}
}

func TestWriteStepMeta(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl, err := rl.NewStepLogger("analyze", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	condMet := true
	meta := StepMeta{
		StepID:       "analyze",
		Agent:        "security-reviewer",
		AgentFile:    "./agents/security-reviewer.agent.md",
		Model:        "gpt-5",
		Status:       "completed",
		StartedAt:    "2026-03-20T14:32:05Z",
		CompletedAt:  "2026-03-20T14:32:15Z",
		DurationSecs: 10.0,
		OutputFile:   "output.md",
		DependsOn:    []string{},
		Condition: &workflow.Condition{
			Step:     "decide",
			Contains: "APPROVE",
		},
		ConditionMet: &condMet,
		SessionID:    "sess-123",
	}

	if err := sl.WriteStepMeta(meta); err != nil {
		t.Fatalf("WriteStepMeta: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sl.StepDir, "step.meta.json"))
	if err != nil {
		t.Fatalf("reading step meta: %v", err)
	}

	var got StepMeta
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got.StepID != "analyze" {
		t.Errorf("step_id = %q, want analyze", got.StepID)
	}
	if got.Agent != "security-reviewer" {
		t.Errorf("agent = %q, want security-reviewer", got.Agent)
	}
	if got.DurationSecs != 10.0 {
		t.Errorf("duration_seconds = %v, want 10", got.DurationSecs)
	}
	if got.Condition == nil || got.Condition.Contains != "APPROVE" {
		t.Errorf("condition not serialized correctly")
	}
	if got.ConditionMet == nil || *got.ConditionMet != true {
		t.Errorf("condition_result not serialized correctly")
	}
}

func TestMultipleStepLoggers(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "multi-step")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	stepIDs := []string{"parse", "lint", "test", "build", "deploy"}
	loggers := make([]*StepLogger, len(stepIDs))
	for i, id := range stepIDs {
		sl, err := rl.NewStepLogger(id, i+1)
		if err != nil {
			t.Fatalf("NewStepLogger(%s): %v", id, err)
		}
		loggers[i] = sl
	}

	// All directories should exist under steps/.
	stepsDir := filepath.Join(rl.RunDir, "steps")
	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		t.Fatalf("reading steps dir: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 step dirs, got %d", len(entries))
	}
}

// --- P1T14: Prompt, Output, FinalizeRun tests ---

func TestWritePrompt(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	sl, err := rl.NewStepLogger("analyze", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	prompt := "Analyze the following files for security issues:\n- src/auth.go\n- src/handler.go"
	if err := sl.WritePrompt(prompt); err != nil {
		t.Fatalf("WritePrompt: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sl.StepDir, "prompt.md"))
	if err != nil {
		t.Fatalf("reading prompt.md: %v", err)
	}
	if string(data) != prompt {
		t.Errorf("prompt.md content mismatch:\ngot:  %q\nwant: %q", string(data), prompt)
	}
}

func TestWriteOutput(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}
	sl, err := rl.NewStepLogger("analyze", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	output := "## Security Review\n\nNo critical issues found.\nSeverity: LOW"
	if err := sl.WriteOutput(output); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sl.StepDir, "output.md"))
	if err != nil {
		t.Fatalf("reading output.md: %v", err)
	}
	if string(data) != output {
		t.Errorf("output.md content mismatch:\ngot:  %q\nwant: %q", string(data), output)
	}
}

func TestFinalizeRun(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "test-wf")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	// Write initial meta so FinalizeRun can update it.
	wf := &workflow.Workflow{Name: "test-wf"}
	if err := rl.WriteWorkflowMeta(wf, nil); err != nil {
		t.Fatalf("WriteWorkflowMeta: %v", err)
	}

	outputs := map[string]string{
		"decide":    "APPROVED",
		"aggregate": "All reviews passed.",
	}
	outputSteps := []string{"decide", "aggregate"}

	if err := rl.FinalizeRun("completed", outputs, outputSteps); err != nil {
		t.Fatalf("FinalizeRun: %v", err)
	}

	// Check final_output.md contains both steps in order.
	data, err := os.ReadFile(filepath.Join(rl.RunDir, "final_output.md"))
	if err != nil {
		t.Fatalf("reading final_output.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## decide") {
		t.Error("final_output.md missing ## decide header")
	}
	if !strings.Contains(content, "APPROVED") {
		t.Error("final_output.md missing decide output")
	}
	if !strings.Contains(content, "## aggregate") {
		t.Error("final_output.md missing ## aggregate header")
	}
	if !strings.Contains(content, "All reviews passed.") {
		t.Error("final_output.md missing aggregate output")
	}
	// Ensure ordering: decide before aggregate.
	decideIdx := strings.Index(content, "## decide")
	aggIdx := strings.Index(content, "## aggregate")
	if decideIdx > aggIdx {
		t.Error("final_output.md steps are out of order")
	}

	// Check workflow.meta.json was updated.
	metaData, err := os.ReadFile(filepath.Join(rl.RunDir, "workflow.meta.json"))
	if err != nil {
		t.Fatalf("reading updated meta: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if meta["status"] != "completed" {
		t.Errorf("status = %v, want completed", meta["status"])
	}
	if meta["completed_at"] == nil || meta["completed_at"] == "" {
		t.Error("completed_at is missing after finalize")
	}
}

// --- Stream Recording Tests ---

// TestAppendStreamEvent_SingleEvent verifies that a single stream event
// is correctly written to stream.jsonl in JSON Lines format.
func TestAppendStreamEvent_SingleEvent(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "stream-test")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl, err := rl.NewStepLogger("analyze", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	event := StreamEvent{
		Timestamp: "2026-03-30T14:32:05.001Z",
		Type:      "assistant.turn_start",
		Data:      nil,
	}

	if err := sl.AppendStreamEvent(event); err != nil {
		t.Fatalf("AppendStreamEvent: %v", err)
	}

	// Read and verify the stream.jsonl file.
	data, err := os.ReadFile(filepath.Join(sl.StepDir, "stream.jsonl"))
	if err != nil {
		t.Fatalf("reading stream.jsonl: %v", err)
	}

	// Should be exactly one line (plus a trailing newline).
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}

	// Parse the JSON line.
	var parsed StreamEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed.Type != "assistant.turn_start" {
		t.Errorf("type = %q, want assistant.turn_start", parsed.Type)
	}
	if parsed.Timestamp != "2026-03-30T14:32:05.001Z" {
		t.Errorf("ts = %q, want 2026-03-30T14:32:05.001Z", parsed.Timestamp)
	}
}

// TestAppendStreamEvent_MultipleEvents verifies that multiple events are
// correctly appended to stream.jsonl, each on its own line.
func TestAppendStreamEvent_MultipleEvents(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "stream-multi")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl, err := rl.NewStepLogger("review", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	// Simulate a typical streaming sequence.
	events := []StreamEvent{
		{Timestamp: "2026-03-30T14:32:05.001Z", Type: "assistant.turn_start"},
		{Timestamp: "2026-03-30T14:32:05.050Z", Type: "assistant.message_delta", Data: "I'll analyze"},
		{Timestamp: "2026-03-30T14:32:05.080Z", Type: "assistant.message_delta", Data: " the code"},
		{Timestamp: "2026-03-30T14:32:05.200Z", Type: "tool.execution_start", Data: map[string]string{"tool": "grep", "args": `{"query":"password"}`}},
		{Timestamp: "2026-03-30T14:32:06.500Z", Type: "tool.execution_complete", Data: map[string]string{"tool": "grep", "status": "completed"}},
		{Timestamp: "2026-03-30T14:32:07.000Z", Type: "assistant.turn_end"},
		{Timestamp: "2026-03-30T14:32:07.100Z", Type: "session.idle"},
	}

	for _, e := range events {
		if err := sl.AppendStreamEvent(e); err != nil {
			t.Fatalf("AppendStreamEvent: %v", err)
		}
	}

	// Read and verify the stream.jsonl file.
	data, err := os.ReadFile(filepath.Join(sl.StepDir, "stream.jsonl"))
	if err != nil {
		t.Fatalf("reading stream.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(events) {
		t.Errorf("expected %d lines, got %d", len(events), len(lines))
	}

	// Verify each line is valid JSON and has the expected type.
	for i, line := range lines {
		var parsed StreamEvent
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d invalid JSON: %v", i, err)
			continue
		}
		if parsed.Type != events[i].Type {
			t.Errorf("line %d type = %q, want %q", i, parsed.Type, events[i].Type)
		}
	}
}

// TestAppendStreamEvent_WithData verifies that event data is correctly
// serialized for different data types (strings, maps).
func TestAppendStreamEvent_WithData(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "stream-data")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl, err := rl.NewStepLogger("process", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	// Test string data (message delta).
	deltaEvent := StreamEvent{
		Timestamp: "2026-03-30T14:32:05.050Z",
		Type:      "assistant.message_delta",
		Data:      "Hello, I'll analyze the code for you.",
	}
	if err := sl.AppendStreamEvent(deltaEvent); err != nil {
		t.Fatalf("AppendStreamEvent (delta): %v", err)
	}

	// Test map data (tool execution).
	toolEvent := StreamEvent{
		Timestamp: "2026-03-30T14:32:05.200Z",
		Type:      "tool.execution_start",
		Data: map[string]string{
			"tool": "semantic_search",
			"args": `{"query":"authentication"}`,
		},
	}
	if err := sl.AppendStreamEvent(toolEvent); err != nil {
		t.Fatalf("AppendStreamEvent (tool): %v", err)
	}

	// Read and parse the stream.jsonl file.
	data, err := os.ReadFile(filepath.Join(sl.StepDir, "stream.jsonl"))
	if err != nil {
		t.Fatalf("reading stream.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Verify delta event data.
	var delta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &delta); err != nil {
		t.Fatalf("line 0 invalid JSON: %v", err)
	}
	if delta["data"] != "Hello, I'll analyze the code for you." {
		t.Errorf("delta data = %v, want string", delta["data"])
	}

	// Verify tool event data.
	var tool map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &tool); err != nil {
		t.Fatalf("line 1 invalid JSON: %v", err)
	}
	toolData, ok := tool["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("tool data is not a map")
	}
	if toolData["tool"] != "semantic_search" {
		t.Errorf("tool name = %v, want semantic_search", toolData["tool"])
	}
}

// TestAppendStreamEvent_UserInput verifies that user input request/response
// events are correctly serialized with their structured data.
func TestAppendStreamEvent_UserInput(t *testing.T) {
	tmp := t.TempDir()
	rl, err := NewRunLogger(tmp, "stream-input")
	if err != nil {
		t.Fatalf("NewRunLogger: %v", err)
	}

	sl, err := rl.NewStepLogger("interactive", 1)
	if err != nil {
		t.Fatalf("NewStepLogger: %v", err)
	}

	// Simulate user input request with choices.
	requestEvent := StreamEvent{
		Timestamp: "2026-03-30T14:32:10.000Z",
		Type:      "user.input_requested",
		Data: map[string]interface{}{
			"prompt":  "Should I continue with the security scan?",
			"choices": []string{"yes", "no", "skip"},
		},
	}
	if err := sl.AppendStreamEvent(requestEvent); err != nil {
		t.Fatalf("AppendStreamEvent (request): %v", err)
	}

	// Simulate user response.
	responseEvent := StreamEvent{
		Timestamp: "2026-03-30T14:32:15.000Z",
		Type:      "user.input_response",
		Data:      "yes",
	}
	if err := sl.AppendStreamEvent(responseEvent); err != nil {
		t.Fatalf("AppendStreamEvent (response): %v", err)
	}

	// Read and verify the stream.jsonl file.
	data, err := os.ReadFile(filepath.Join(sl.StepDir, "stream.jsonl"))
	if err != nil {
		t.Fatalf("reading stream.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Verify request event.
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &request); err != nil {
		t.Fatalf("line 0 invalid JSON: %v", err)
	}
	if request["type"] != "user.input_requested" {
		t.Errorf("request type = %v, want user.input_requested", request["type"])
	}
	requestData, _ := request["data"].(map[string]interface{})
	if requestData["prompt"] != "Should I continue with the security scan?" {
		t.Errorf("request prompt mismatch")
	}

	// Verify response event.
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &response); err != nil {
		t.Fatalf("line 1 invalid JSON: %v", err)
	}
	if response["type"] != "user.input_response" {
		t.Errorf("response type = %v, want user.input_response", response["type"])
	}
	if response["data"] != "yes" {
		t.Errorf("response data = %v, want yes", response["data"])
	}
}

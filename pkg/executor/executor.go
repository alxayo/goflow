// executor.go implements single-step execution: resolves prompt templates,
// evaluates conditions, creates SDK sessions, sends prompts, captures output,
// and writes audit files.
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/audit"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// StepExecutor executes a single workflow step: resolves its prompt template,
// evaluates conditions, creates an SDK session, sends the prompt, captures
// the output, and writes audit files.
type StepExecutor struct {
	SDK         SessionExecutor
	AuditLogger *audit.RunLogger
	Truncate    *workflow.TruncateConfig
}

// Execute runs a single step and returns its result.
// It is the caller's responsibility to ensure dependencies are satisfied.
func (se *StepExecutor) Execute(
	ctx context.Context,
	step workflow.Step,
	agent *agents.Agent,
	results map[string]string,
	inputs map[string]string,
	seqNum int,
) (*workflow.StepResult, error) {
	startedAt := time.Now()
	result := &workflow.StepResult{
		StepID:    step.ID,
		StartedAt: startedAt.UTC().Format(time.RFC3339),
	}

	// 1. Evaluate condition — skip if not met.
	condMet, err := workflow.EvaluateCondition(step.Condition, results)
	if err != nil {
		return nil, fmt.Errorf("evaluating condition for step %q: %w", step.ID, err)
	}
	if !condMet {
		result.Status = workflow.StepStatusSkipped
		result.EndedAt = time.Now().UTC().Format(time.RFC3339)
		if se.AuditLogger != nil {
			se.writeSkippedAudit(step, agent, result, seqNum)
		}
		return result, nil
	}

	// 2. Resolve prompt template.
	resolvedPrompt, err := workflow.ResolveTemplate(step.Prompt, results, inputs)
	if err != nil {
		return nil, fmt.Errorf("resolving template for step %q: %w", step.ID, err)
	}

	// 3. Create step audit logger and write the resolved prompt.
	var stepLogger *audit.StepLogger
	if se.AuditLogger != nil {
		stepLogger, err = se.AuditLogger.NewStepLogger(step.ID, seqNum)
		if err != nil {
			return nil, fmt.Errorf("creating step logger for %q: %w", step.ID, err)
		}
		if writeErr := stepLogger.WritePrompt(resolvedPrompt); writeErr != nil {
			return nil, fmt.Errorf("writing prompt for step %q: %w", step.ID, writeErr)
		}
	}

	// 4. Build session config from agent.
	sessionCfg := SessionConfig{
		SystemPrompt: agent.Prompt,
		Tools:        agent.Tools,
	}
	if len(agent.Model.Models) > 0 {
		sessionCfg.Model = agent.Model.Models[0]
	}

	// 5. Create SDK session.
	session, err := se.SDK.CreateSession(ctx, sessionCfg)
	if err != nil {
		result.Status = workflow.StepStatusFailed
		result.Error = err
		result.ErrorMsg = err.Error()
		result.EndedAt = time.Now().UTC().Format(time.RFC3339)
		if stepLogger != nil {
			se.writeFailedAudit(stepLogger, step, agent, result, startedAt)
		}
		return result, fmt.Errorf("creating session for step %q: %w", step.ID, err)
	}
	defer session.Close()

	result.SessionID = session.SessionID()

	// 6. Send resolved prompt and get output.
	output, err := session.Send(ctx, resolvedPrompt)
	if err != nil {
		result.Status = workflow.StepStatusFailed
		result.Error = err
		result.ErrorMsg = err.Error()
		result.EndedAt = time.Now().UTC().Format(time.RFC3339)
		if stepLogger != nil {
			se.writeFailedAudit(stepLogger, step, agent, result, startedAt)
		}
		return result, fmt.Errorf("executing step %q: %w", step.ID, err)
	}

	// 7. Record success.
	result.Status = workflow.StepStatusCompleted
	result.Output = output
	result.EndedAt = time.Now().UTC().Format(time.RFC3339)

	// 8. Write audit files.
	if stepLogger != nil {
		se.writeCompletedAudit(stepLogger, step, agent, result, startedAt)
	}

	return result, nil
}

// writeSkippedAudit records step.meta.json for a step whose condition was not met.
func (se *StepExecutor) writeSkippedAudit(step workflow.Step, agent *agents.Agent, result *workflow.StepResult, seqNum int) {
	sl, err := se.AuditLogger.NewStepLogger(step.ID, seqNum)
	if err != nil {
		return // best-effort audit; don't block execution
	}
	condMet := false
	meta := audit.StepMeta{
		StepID:       step.ID,
		Agent:        agent.Name,
		AgentFile:    agent.SourceFile,
		Model:        firstModel(agent),
		Status:       string(workflow.StepStatusSkipped),
		StartedAt:    result.StartedAt,
		CompletedAt:  result.EndedAt,
		DurationSecs: 0,
		DependsOn:    step.DependsOn,
		Condition:    step.Condition,
		ConditionMet: &condMet,
	}
	_ = sl.WriteStepMeta(meta)
}

// writeFailedAudit records step.meta.json (and output.md if output exists)
// for a step that failed during session creation or prompt execution.
func (se *StepExecutor) writeFailedAudit(sl *audit.StepLogger, step workflow.Step, agent *agents.Agent, result *workflow.StepResult, startedAt time.Time) {
	if result.Output != "" {
		_ = sl.WriteOutput(result.Output)
	}
	condMet := true
	meta := audit.StepMeta{
		StepID:       step.ID,
		Agent:        agent.Name,
		AgentFile:    agent.SourceFile,
		Model:        firstModel(agent),
		Status:       string(workflow.StepStatusFailed),
		StartedAt:    result.StartedAt,
		CompletedAt:  result.EndedAt,
		DurationSecs: time.Since(startedAt).Seconds(),
		OutputFile:   "output.md",
		DependsOn:    step.DependsOn,
		Condition:    step.Condition,
		ConditionMet: &condMet,
		SessionID:    result.SessionID,
		Error:        result.ErrorMsg,
	}
	_ = sl.WriteStepMeta(meta)
}

// writeCompletedAudit records output.md and step.meta.json for a successful step.
func (se *StepExecutor) writeCompletedAudit(sl *audit.StepLogger, step workflow.Step, agent *agents.Agent, result *workflow.StepResult, startedAt time.Time) {
	_ = sl.WriteOutput(result.Output)
	condMet := true
	meta := audit.StepMeta{
		StepID:       step.ID,
		Agent:        agent.Name,
		AgentFile:    agent.SourceFile,
		Model:        firstModel(agent),
		Status:       string(workflow.StepStatusCompleted),
		StartedAt:    result.StartedAt,
		CompletedAt:  result.EndedAt,
		DurationSecs: time.Since(startedAt).Seconds(),
		OutputFile:   "output.md",
		DependsOn:    step.DependsOn,
		Condition:    step.Condition,
		ConditionMet: &condMet,
		SessionID:    result.SessionID,
	}
	_ = sl.WriteStepMeta(meta)
}

// firstModel returns the first model from the agent's model spec, or "".
func firstModel(agent *agents.Agent) string {
	if len(agent.Model.Models) > 0 {
		return agent.Model.Models[0]
	}
	return ""
}

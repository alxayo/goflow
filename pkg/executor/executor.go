// executor.go implements single-step execution: resolves prompt templates,
// evaluates conditions, creates SDK sessions, sends prompts, captures output,
// and writes audit files.
package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	// DefaultModel is the workflow-level fallback model from config.model.
	// Used when neither the step nor the agent specifies a model.
	DefaultModel string

	// Interactive is the resolved interactive flag for the current step.
	// Set by the orchestrator before each Execute() call based on the
	// three-level resolution (step > workflow config > CLI flag).
	// When true, the session allows the LLM to ask the user questions.
	Interactive bool

	// OnUserInput is the callback invoked when the LLM requests user
	// clarification. Only used when Interactive is true.
	OnUserInput UserInputHandler

	// Streaming enables real-time event monitoring for session progress.
	// When true, the SDK emits events as the LLM works, eliminating the
	// need for timeout configuration on long-running steps.
	Streaming bool

	// OnProgress is called for each significant session event when Streaming
	// is enabled. Use this for real-time CLI output or audit logging.
	OnProgress ProgressHandler
}

// Execute runs a single step and returns its result.
// It is the caller's responsibility to ensure dependencies are satisfied.
//
// Timeout Handling:
// If step.Timeout is set (e.g., "120s"), a context deadline is applied. This allows
// you to override or extend the SDK's internal timeout limits. For example:
//   - SDK default: ~60 seconds
//   - Set timeout: "120s" for slow operations
//   - Set timeout: "300s" (5 minutes) for complex multi-agent orchestration
//
// The context deadline applies to both session creation and the Send call.
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

	// 0. Apply step-level timeout if specified.
	if step.Timeout != "" {
		timeout, err := time.ParseDuration(step.Timeout)
		if err == nil && timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		// If timeout is malformed, log but continue (non-fatal).
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
	// Include the interactive flag and user-input handler so the session
	// knows whether to allow the LLM to ask clarification questions.
	// Also include streaming config for event-based progress monitoring.
	sessionCfg := SessionConfig{
		SystemPrompt: agent.Prompt,
		Tools:        agent.Tools,
		ExtraDirs:    step.ExtraDirs,
		Models:       se.resolveModels(step, agent),
		Interactive:  se.Interactive,
		OnUserInput:  se.OnUserInput,
		Streaming:    se.Streaming,
		OnProgress:   se.OnProgress,
		StepID:       step.ID,
	}

	// 5. Create SDK session.
	attempts := step.RetryCount + 1
	if attempts < 1 {
		attempts = 1
	}

	var output string
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		session, err := se.SDK.CreateSession(ctx, sessionCfg)
		if err != nil {
			lastErr = err
			if attempt < attempts && isTransientSDKTimeoutError(err) {
				if !sleepWithContext(ctx, retryBackoff(attempt)) {
					break
				}
				continue
			}

			result.Status = workflow.StepStatusFailed
			result.Error = err
			result.ErrorMsg = err.Error()
			result.EndedAt = time.Now().UTC().Format(time.RFC3339)
			if stepLogger != nil {
				se.writeFailedAudit(stepLogger, step, agent, result, startedAt)
			}
			return result, fmt.Errorf("creating session for step %q: %w", step.ID, err)
		}

		result.SessionID = session.SessionID()

		output, err = session.Send(ctx, resolvedPrompt)
		_ = session.Close()
		if err != nil {
			lastErr = err
			if attempt < attempts && isTransientSDKTimeoutError(err) {
				if !sleepWithContext(ctx, retryBackoff(attempt)) {
					break
				}
				continue
			}

			result.Status = workflow.StepStatusFailed
			result.Error = err
			result.ErrorMsg = err.Error()
			result.EndedAt = time.Now().UTC().Format(time.RFC3339)
			if stepLogger != nil {
				se.writeFailedAudit(stepLogger, step, agent, result, startedAt)
			}
			return result, fmt.Errorf("executing step %q: %w", step.ID, err)
		}

		// Successful send ends retry loop.
		break
	}

	if output == "" {
		err := lastErr
		if err == nil {
			err = errors.New("step execution failed after retries")
		}
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
		Model:        se.resolvedModel(step, agent),
		Status:       string(workflow.StepStatusSkipped),
		StartedAt:    result.StartedAt,
		CompletedAt:  result.EndedAt,
		DurationSecs: 0,
		DependsOn:    step.DependsOn,
		Condition:    step.Condition,
		ConditionMet: &condMet,
		Interactive:  se.Interactive,
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
		Model:        se.resolvedModel(step, agent),
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
		Interactive:  se.Interactive,
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
		Model:        se.resolvedModel(step, agent),
		Status:       string(workflow.StepStatusCompleted),
		StartedAt:    result.StartedAt,
		CompletedAt:  result.EndedAt,
		DurationSecs: time.Since(startedAt).Seconds(),
		OutputFile:   "output.md",
		DependsOn:    step.DependsOn,
		Condition:    step.Condition,
		ConditionMet: &condMet,
		SessionID:    result.SessionID,
		Interactive:  se.Interactive,
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

// resolveModels builds a priority-ordered list of models to try for this step.
// Returns: step.Model → agent.Model.Models → se.DefaultModel (workflow config).
// If all are empty, returns nil and the CLI will pick the default model.
func (se *StepExecutor) resolveModels(step workflow.Step, agent *agents.Agent) []string {
	var models []string

	// Highest priority: step-level model override.
	if step.Model != "" {
		models = append(models, step.Model)
	}

	// Second: agent's model list (may have multiple fallbacks).
	models = append(models, agent.Model.Models...)

	// Third: workflow-level default model.
	if se.DefaultModel != "" {
		models = append(models, se.DefaultModel)
	}

	// Deduplicate while preserving order.
	return dedupeStrings(models)
}

// resolvedModel returns the first (highest-priority) model for audit logging.
func (se *StepExecutor) resolvedModel(step workflow.Step, agent *agents.Agent) string {
	models := se.resolveModels(step, agent)
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

// dedupeStrings removes duplicate strings while preserving order.
func dedupeStrings(input []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(input))
	for _, s := range input {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// isTransientSDKTimeoutError returns true for timeout-style SDK errors that
// are typically transient and safe to retry.
func isTransientSDKTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "waiting for session.idle") ||
		strings.Contains(msg, "timeout")
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	// Short linear backoff to avoid immediately hammering the backend.
	return time.Duration(attempt) * 500 * time.Millisecond
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

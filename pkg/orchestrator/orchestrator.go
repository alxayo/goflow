// Package orchestrator executes a workflow's DAG by processing levels in
// order. In the sequential implementation, steps within a level are also
// executed one at a time. The parallel implementation (Phase 2) upgrades
// this to concurrent execution within levels.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/executor"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// Orchestrator executes a workflow's DAG by processing levels sequentially
// or in parallel.
type Orchestrator struct {
	Executor       *executor.StepExecutor
	Agents         map[string]*agents.Agent
	Inputs         map[string]string
	MaxConcurrency int // 0 = unlimited

	// CLIInteractive is the global interactive flag from the --interactive
	// CLI flag. Combined with the workflow's config.interactive and each
	// step's interactive field to resolve the effective interactive state.
	CLIInteractive bool

	// OnUserInput is the callback for handling user-input requests from
	// the LLM. Passed through to the executor for interactive steps.
	OnUserInput executor.UserInputHandler
}

// Run executes the workflow, processing DAG levels in order.
// Returns a map of step ID → StepResult for all executed steps.
//
// For each step, the interactive flag is resolved using the three-level
// priority: step.Interactive > wf.Config.Interactive > CLIInteractive.
// The resolved flag is set on the executor before running each step.
func (o *Orchestrator) Run(ctx context.Context, wf *workflow.Workflow) (map[string]*workflow.StepResult, error) {
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	results := make(map[string]*workflow.StepResult)
	outputMap := make(map[string]string) // step ID → output text for template resolution

	for _, level := range levels {
		for _, step := range level.Steps {
			agent, ok := o.Agents[step.Agent]
			if !ok {
				return results, fmt.Errorf("step %q: agent %q not found", step.ID, step.Agent)
			}

			// Resolve the interactive flag for this specific step.
			// This uses the three-level resolution: step > config > CLI.
			o.Executor.Interactive = workflow.IsInteractive(step, wf.Config.Interactive, o.CLIInteractive)
			o.Executor.OnUserInput = o.OnUserInput

			result, err := o.Executor.Execute(ctx, step, agent, outputMap, o.Inputs, level.Depth)
			if err != nil {
				if result != nil {
					results[step.ID] = result
				}
				return results, fmt.Errorf("step %q failed: %w", step.ID, err)
			}

			results[step.ID] = result
			if result.Status == workflow.StepStatusCompleted {
				outputMap[step.ID] = result.Output
			} else if result.Status == workflow.StepStatusSkipped {
				// Skipped steps get an empty output so downstream
				// {{steps.X.output}} references resolve to "" rather
				// than failing with "unknown step".
				outputMap[step.ID] = ""
			}
		}
	}

	return results, nil
}

// RunParallel executes the workflow DAG with concurrent step execution
// within each level. Uses goroutines + sync.WaitGroup for fan-out.
// MaxConcurrency limits how many steps run simultaneously (0 = unlimited).
//
// Interactive steps within a parallel level are executed sequentially AFTER
// all non-interactive steps in that level complete. This prevents confusing
// interleaved user prompts from multiple agents asking questions at once.
func (o *Orchestrator) RunParallel(ctx context.Context, wf *workflow.Workflow) (map[string]*workflow.StepResult, error) {
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	store := NewResultsStore()
	sem := NewSemaphore(o.MaxConcurrency)

	for _, level := range levels {
		bestEffortLevel := len(level.Steps) > 1

		// Separate interactive and non-interactive steps. Interactive steps
		// are run sequentially after the parallel batch to avoid interleaved
		// user prompts on the terminal.
		var parallelSteps []workflow.Step
		var interactiveSteps []workflow.Step
		for _, step := range level.Steps {
			if workflow.IsInteractive(step, wf.Config.Interactive, o.CLIInteractive) {
				interactiveSteps = append(interactiveSteps, step)
			} else {
				parallelSteps = append(parallelSteps, step)
			}
		}

		// Warn if interactive steps are in a parallel level with other steps.
		if len(interactiveSteps) > 0 && len(parallelSteps) > 0 {
			fmt.Fprintf(os.Stderr, "warning: level %d has %d interactive step(s) that will run sequentially after %d parallel step(s)\n",
				level.Depth, len(interactiveSteps), len(parallelSteps))
		}

		// Phase 1: Run non-interactive steps in parallel.
		if len(parallelSteps) > 0 {
			var wg sync.WaitGroup
			var levelErrMu sync.Mutex
			var levelErrs []error

			for _, step := range parallelSteps {
				wg.Add(1)
				go func(s workflow.Step) {
					defer wg.Done()
					sem.Acquire()
					defer sem.Release()

					agent, ok := o.Agents[s.Agent]
					if !ok {
						err := fmt.Errorf("step %q: agent %q not found", s.ID, s.Agent)
						store.Store(s.ID, failedStepResult(s.ID, err))
						levelErrMu.Lock()
						levelErrs = append(levelErrs, err)
						levelErrMu.Unlock()
						return
					}

					// Create a per-goroutine executor copy to avoid data races
					// on the Interactive field, since non-interactive steps run
					// concurrently.
					stepExec := *o.Executor
					stepExec.Interactive = false
					stepExec.OnUserInput = nil

					result, err := stepExec.Execute(
						ctx, s, agent, store.OutputMap(), o.Inputs, level.Depth,
					)
					if err != nil {
						if result != nil {
							store.Store(s.ID, result)
						} else {
							store.Store(s.ID, failedStepResult(s.ID, err))
						}
						levelErrMu.Lock()
						levelErrs = append(levelErrs, err)
						levelErrMu.Unlock()
						return
					}
					store.Store(s.ID, result)
				}(step)
			}

			wg.Wait()
			if !bestEffortLevel && len(levelErrs) > 0 {
				return store.All(), levelErrs[0]
			}
		}

		// Phase 2: Run interactive steps sequentially so user prompts
		// don't interleave on the terminal.
		for _, step := range interactiveSteps {
			agent, ok := o.Agents[step.Agent]
			if !ok {
				err := fmt.Errorf("step %q: agent %q not found", step.ID, step.Agent)
				store.Store(step.ID, failedStepResult(step.ID, err))
				if !bestEffortLevel {
					return store.All(), err
				}
				continue
			}

			o.Executor.Interactive = true
			o.Executor.OnUserInput = o.OnUserInput

			result, err := o.Executor.Execute(
				ctx, step, agent, store.OutputMap(), o.Inputs, level.Depth,
			)
			if err != nil {
				if result != nil {
					store.Store(step.ID, result)
				} else {
					store.Store(step.ID, failedStepResult(step.ID, err))
				}
				if !bestEffortLevel {
					return store.All(), err
				}
				continue
			}
			store.Store(step.ID, result)
		}
	}

	return store.All(), nil
}

func failedStepResult(stepID string, err error) *workflow.StepResult {
	now := time.Now().UTC().Format(time.RFC3339)
	return &workflow.StepResult{
		StepID:    stepID,
		Status:    workflow.StepStatusFailed,
		Error:     err,
		ErrorMsg:  err.Error(),
		StartedAt: now,
		EndedAt:   now,
	}
}

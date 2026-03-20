// Package orchestrator executes a workflow's DAG by processing levels in
// order. In the sequential implementation, steps within a level are also
// executed one at a time. The parallel implementation (Phase 2) upgrades
// this to concurrent execution within levels.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

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
}

// Run executes the workflow, processing DAG levels in order.
// Returns a map of step ID → StepResult for all executed steps.
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

			result, err := o.Executor.Execute(ctx, step, agent, outputMap, o.Inputs, level.Depth)
			if err != nil {
				results[step.ID] = result
				return results, fmt.Errorf("step %q failed: %w", step.ID, err)
			}

			results[step.ID] = result
			if result.Status == workflow.StepStatusCompleted {
				outputMap[step.ID] = result.Output
			}
		}
	}

	return results, nil
}

// RunParallel executes the workflow DAG with concurrent step execution
// within each level. Uses goroutines + sync.WaitGroup for fan-out.
// MaxConcurrency limits how many steps run simultaneously (0 = unlimited).
func (o *Orchestrator) RunParallel(ctx context.Context, wf *workflow.Workflow) (map[string]*workflow.StepResult, error) {
	levels, err := workflow.BuildDAG(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	store := NewResultsStore()
	sem := NewSemaphore(o.MaxConcurrency)

	for _, level := range levels {
		var wg sync.WaitGroup
		errCh := make(chan error, len(level.Steps))

		for _, step := range level.Steps {
			wg.Add(1)
			go func(s workflow.Step) {
				defer wg.Done()
				sem.Acquire()
				defer sem.Release()

				agent, ok := o.Agents[s.Agent]
				if !ok {
					errCh <- fmt.Errorf("step %q: agent %q not found", s.ID, s.Agent)
					return
				}

				result, err := o.Executor.Execute(
					ctx, s, agent, store.OutputMap(), o.Inputs, level.Depth,
				)
				if err != nil {
					if result != nil {
						store.Store(s.ID, result)
					}
					errCh <- fmt.Errorf("step %q: %w", s.ID, err)
					return
				}
				store.Store(s.ID, result)
			}(step)
		}

		wg.Wait()
		close(errCh)

		// Fail fast: return on first error from this level.
		for err := range errCh {
			return store.All(), err
		}
	}

	return store.All(), nil
}

// main.go is the CLI entry point for the workflow runner. It provides the
// "run" command that loads a workflow YAML, discovers agents, and executes
// the workflow DAG.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alex-workflow-runner/workflow-runner/pkg/agents"
	"github.com/alex-workflow-runner/workflow-runner/pkg/audit"
	"github.com/alex-workflow-runner/workflow-runner/pkg/executor"
	"github.com/alex-workflow-runner/workflow-runner/pkg/orchestrator"
	"github.com/alex-workflow-runner/workflow-runner/pkg/reporter"
	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// inputsFlag collects repeatable --inputs key=value flags into a map.
type inputsFlag struct {
	values map[string]string
}

func (f *inputsFlag) String() string { return fmt.Sprintf("%v", f.values) }

func (f *inputsFlag) Set(val string) error {
	parts := strings.SplitN(val, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid input format %q, expected key=value", val)
	}
	f.values[parts[0]] = parts[1]
	return nil
}

const usage = `Usage: workflow-runner run [options]

Options:
  --workflow    Path to workflow YAML file (required)
  --inputs      Key=value input pairs (repeatable)
  --audit-dir   Override audit directory (default from workflow config)
  --verbose     Enable verbose logging
`

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	// Parse flags after the "run" subcommand.
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	workflowPath := fs.String("workflow", "", "Path to workflow YAML file (required)")
	auditDirFlag := fs.String("audit-dir", "", "Override audit directory")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	inputs := &inputsFlag{values: make(map[string]string)}
	fs.Var(inputs, "inputs", "Key=value input pair (repeatable)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return 1
	}

	if *workflowPath == "" {
		fmt.Fprintln(os.Stderr, "error: --workflow is required")
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	// 1. Load and parse workflow YAML.
	if *verbose {
		fmt.Fprintf(os.Stderr, "Loading workflow: %s\n", *workflowPath)
	}

	wf, err := workflow.ParseWorkflow(*workflowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// 2. Validate workflow.
	if err := workflow.ValidateWorkflow(wf); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// 3. Merge CLI inputs with workflow input defaults.
	mergedInputs := make(map[string]string, len(wf.Inputs))
	for name, inp := range wf.Inputs {
		if v, ok := inputs.values[name]; ok {
			mergedInputs[name] = v
		} else if inp.Default != "" {
			mergedInputs[name] = inp.Default
		}
	}
	// Include CLI inputs not declared in workflow (pass-through).
	for k, v := range inputs.values {
		if _, exists := mergedInputs[k]; !exists {
			mergedInputs[k] = v
		}
	}

	// 4. Resolve agents.
	workspaceDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: getting working directory: %v\n", err)
		return 1
	}

	resolvedAgents, err := agents.ResolveAgents(wf, workspaceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Resolved %d agents\n", len(resolvedAgents))
	}

	// 5. Set up audit directory.
	auditDir := wf.Config.AuditDir
	if *auditDirFlag != "" {
		auditDir = *auditDirFlag
	}

	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating audit logger: %v\n", err)
		return 1
	}

	// 6. Apply retention policy.
	if err := audit.ApplyRetention(auditDir, wf.Config.AuditRetention); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit retention: %v\n", err)
		// Non-fatal — continue execution.
	}

	// 7. Write workflow meta and snapshot.
	if err := auditLogger.WriteWorkflowMeta(wf, mergedInputs); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing workflow meta: %v\n", err)
		return 1
	}
	if err := auditLogger.SnapshotWorkflow(*workflowPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: snapshotting workflow: %v\n", err)
		return 1
	}

	// 8. Create mock SDK executor (real SDK integration coming later).
	fmt.Fprintln(os.Stderr, "NOTE: Using mock SDK executor. Real Copilot SDK integration coming in a future phase.")
	mockSDK := &executor.MockSessionExecutor{
		DefaultResponse: "mock output",
	}

	// 9. Build StepExecutor.
	stepExec := &executor.StepExecutor{
		SDK:         mockSDK,
		AuditLogger: auditLogger,
		Truncate:    wf.Output.Truncate,
	}

	// 10. Build and run Orchestrator.
	orch := &orchestrator.Orchestrator{
		Executor: stepExec,
		Agents:   resolvedAgents,
		Inputs:   mergedInputs,
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Executing workflow: %s\n", wf.Name)
	}

	startTime := time.Now()
	ctx := context.Background()
	results, runErr := orch.Run(ctx, wf)
	elapsed := time.Since(startTime)

	// 11. Print step statuses in verbose mode.
	if *verbose {
		for _, step := range wf.Steps {
			if r, ok := results[step.ID]; ok {
				fmt.Fprintf(os.Stderr, "Step %s: %s\n", step.ID, r.Status)
			}
		}
		fmt.Fprintf(os.Stderr, "Workflow completed in %.1fs\n", elapsed.Seconds())
	}

	// 12. Determine final status.
	failed := runErr != nil
	finalStatus := "completed"
	if failed {
		finalStatus = "failed"
	}

	// 13. Collect outputs for reporter and audit finalization.
	outputMap := make(map[string]string, len(results))
	for id, r := range results {
		if r.Status == workflow.StepStatusCompleted {
			outputMap[id] = r.Output
		}
	}

	// 14. Format output via reporter.
	output, err := reporter.FormatOutput(results, wf.Output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: formatting output: %v\n", err)
		return 1
	}

	// 15. Finalize audit trail.
	outputSteps := wf.Output.Steps
	if len(outputSteps) == 0 {
		for _, step := range wf.Steps {
			if r, ok := results[step.ID]; ok && r.Status == workflow.StepStatusCompleted {
				outputSteps = append(outputSteps, step.ID)
			}
		}
	}
	if err := auditLogger.FinalizeRun(finalStatus, outputMap, outputSteps); err != nil {
		fmt.Fprintf(os.Stderr, "warning: finalizing audit: %v\n", err)
	}

	// 16. Print output to stdout.
	fmt.Print(output)

	if failed {
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		return 1
	}
	return 0
}

// main.go is the CLI entry point for goflow. It provides the
// "run" command that loads a workflow YAML, discovers agents, and executes
// the workflow DAG.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

const usage = `Usage: goflow run [options]

       goflow version

Options:
  --workflow      Path to workflow YAML file (required)
  --inputs        Key=value input pairs (repeatable)
  --audit-dir     Override audit directory (default from workflow config)
  --mock          Use mock executor instead of Copilot CLI
  --interactive   Allow agents to ask for user input during execution
  --verbose       Enable verbose logging
`

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	return runArgs(os.Args[1:], os.Stdout, os.Stderr)
}

func runArgs(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:], stdout, stderr)
	case "version", "--version", "-version":
		fmt.Fprintln(stdout, buildInfo())
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 1
	}
}

func runCommand(args []string, stdout, stderr io.Writer) int {
	_ = stdout

	// Parse flags after the "run" subcommand.
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	workflowPath := fs.String("workflow", "", "Path to workflow YAML file (required)")
	auditDirFlag := fs.String("audit-dir", "", "Override audit directory")
	useMock := fs.Bool("mock", false, "Use mock executor instead of Copilot CLI")
	interactive := fs.Bool("interactive", false, "Allow agents to ask for user input during execution")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	inputs := &inputsFlag{values: make(map[string]string)}
	fs.Var(inputs, "inputs", "Key=value input pair (repeatable)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *workflowPath == "" {
		fmt.Fprintln(stderr, "error: --workflow is required")
		fmt.Fprint(stderr, usage)
		return 1
	}

	// 1. Load and parse workflow YAML.
	if *verbose {
		fmt.Fprintf(stderr, "Loading workflow: %s\n", *workflowPath)
	}

	wf, err := workflow.ParseWorkflow(*workflowPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// 2. Validate workflow.
	if err := workflow.ValidateWorkflow(wf); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
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
	// workspaceDir is used for agent discovery in standard locations.
	workspaceDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "error: getting working directory: %v\n", err)
		return 1
	}

	// workflowDir is used for resolving relative agent file paths.
	// This allows workflows to reference agents relative to their own location.
	absWorkflowPath, err := filepath.Abs(*workflowPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving workflow path: %v\n", err)
		return 1
	}
	workflowDir := filepath.Dir(absWorkflowPath)

	resolvedAgents, err := agents.ResolveAgents(wf, workspaceDir, workflowDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *verbose {
		fmt.Fprintf(stderr, "Resolved %d agents\n", len(resolvedAgents))
	}

	// 5. Set up audit directory.
	auditDir := wf.Config.AuditDir
	if *auditDirFlag != "" {
		auditDir = *auditDirFlag
	}

	auditLogger, err := audit.NewRunLogger(auditDir, wf.Name)
	if err != nil {
		fmt.Fprintf(stderr, "error: creating audit logger: %v\n", err)
		return 1
	}

	// 6. Apply retention policy.
	if err := audit.ApplyRetention(auditDir, wf.Config.AuditRetention); err != nil {
		fmt.Fprintf(stderr, "warning: audit retention: %v\n", err)
		// Non-fatal — continue execution.
	}

	// 7. Write workflow meta and snapshot.
	if err := auditLogger.WriteWorkflowMeta(wf, mergedInputs); err != nil {
		fmt.Fprintf(stderr, "error: writing workflow meta: %v\n", err)
		return 1
	}
	if err := auditLogger.SnapshotWorkflow(*workflowPath); err != nil {
		fmt.Fprintf(stderr, "error: snapshotting workflow: %v\n", err)
		return 1
	}

	// 8. Create executor.
	var sessionExecutor executor.SessionExecutor
	if *useMock {
		fmt.Fprintln(stderr, "NOTE: Using mock executor.")
		sessionExecutor = &executor.MockSessionExecutor{DefaultResponse: "mock output"}
	} else {
		sessionExecutor = &executor.CopilotCLIExecutor{}
	}

	// 9. Build StepExecutor.
	stepExec := &executor.StepExecutor{
		SDK:          sessionExecutor,
		AuditLogger:  auditLogger,
		Truncate:     wf.Output.Truncate,
		DefaultModel: wf.Config.Model,
	}

	// 10. Build user-input handler for interactive mode.
	// The handler is only set when interactive mode is enabled (via CLI
	// flag or workflow config). It reads user answers from the terminal.
	var userInputHandler executor.UserInputHandler
	isInteractive := *interactive || wf.Config.Interactive
	if isInteractive {
		userInputHandler = terminalInputHandler
		if *verbose {
			fmt.Fprintln(stderr, "Interactive mode enabled — agents may ask for clarification")
		}
	}

	// 11. Build and run Orchestrator.
	orch := &orchestrator.Orchestrator{
		Executor:       stepExec,
		Agents:         resolvedAgents,
		Inputs:         mergedInputs,
		MaxConcurrency: wf.Config.MaxConcurrency,
		CLIInteractive: *interactive,
		OnUserInput:    userInputHandler,
	}

	if *verbose {
		fmt.Fprintf(stderr, "Executing workflow: %s\n", wf.Name)
	}

	startTime := time.Now()
	ctx := context.Background()
	results, runErr := orch.Run(ctx, wf)
	elapsed := time.Since(startTime)

	// 12. Print step statuses in verbose mode.
	if *verbose {
		for _, step := range wf.Steps {
			if r, ok := results[step.ID]; ok && r != nil {
				fmt.Fprintf(stderr, "Step %s: %s\n", step.ID, r.Status)
			}
		}
		fmt.Fprintf(stderr, "Workflow completed in %.1fs\n", elapsed.Seconds())
	}

	// 13. Determine final status.
	failed := runErr != nil
	finalStatus := "completed"
	if failed {
		finalStatus = "failed"
	}

	// 14. Collect outputs for reporter and audit finalization.
	outputMap := make(map[string]string, len(results))
	for id, r := range results {
		if r.Status == workflow.StepStatusCompleted {
			outputMap[id] = r.Output
		}
	}

	// 15. Format output via reporter.
	output, err := reporter.FormatOutput(results, wf.Output)
	if err != nil {
		fmt.Fprintf(stderr, "error: formatting output: %v\n", err)
		return 1
	}

	// 16. Finalize audit trail.
	outputSteps := wf.Output.Steps
	if len(outputSteps) == 0 {
		for _, step := range wf.Steps {
			if r, ok := results[step.ID]; ok && r.Status == workflow.StepStatusCompleted {
				outputSteps = append(outputSteps, step.ID)
			}
		}
	}
	if err := auditLogger.FinalizeRun(finalStatus, outputMap, outputSteps); err != nil {
		fmt.Fprintf(stderr, "warning: finalizing audit: %v\n", err)
	}

	// 17. Print output to stdout.
	fmt.Fprint(stdout, output)

	if failed {
		fmt.Fprintf(stderr, "error: %v\n", runErr)
		return 1
	}
	return 0
}

func buildInfo() string {
	return fmt.Sprintf("goflow %s\ncommit: %s\nbuilt: %s", version, commit, buildDate)
}

// terminalInputHandler is the interactive-mode callback that presents
// the LLM's clarification question to the user on stderr and reads
// their answer from stdin.
//
// When the LLM provides predefined choices, they are displayed as a
// numbered list. The user can either type a choice number or provide
// a freeform answer.
//
// This function blocks until the user provides input, which is the
// expected behavior — the workflow step pauses while waiting.
//
// If stdin is closed (e.g., piped input exhausted) or the user sends
// an interrupt, an error is returned, which will cause the step to fail.
func terminalInputHandler(question string, choices []string) (string, error) {
	fmt.Fprintf(os.Stderr, "\n--- Agent needs clarification ---\n")
	fmt.Fprintf(os.Stderr, "%s\n", question)

	if len(choices) > 0 {
		// Display numbered choices so the user can pick by number.
		for i, c := range choices {
			fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, c)
		}
		fmt.Fprintf(os.Stderr, "Enter choice number or type your answer: ")
	} else {
		fmt.Fprintf(os.Stderr, "> ")
	}

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading user input: %w", err)
	}

	return strings.TrimSpace(answer), nil
}

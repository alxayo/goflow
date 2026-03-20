// Package reporter formats workflow execution results for display.
// It collects outputs from specified output steps and formats them
// as markdown, JSON, or plain text.
package reporter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// FormatOutput collects results from the specified output steps and formats
// them according to the output configuration. If outputCfg.Steps is empty,
// all completed steps are included. Steps not found in results are silently
// skipped. The default (and fallback for unknown) format is markdown.
func FormatOutput(
	results map[string]*workflow.StepResult,
	outputCfg workflow.OutputConfig,
) (string, error) {
	stepIDs := outputCfg.Steps
	if len(stepIDs) == 0 {
		stepIDs = completedStepIDs(results)
	}

	format := strings.ToLower(strings.TrimSpace(outputCfg.Format))

	switch format {
	case "json":
		return formatJSON(results, stepIDs)
	case "plain", "text":
		return formatPlain(results, stepIDs), nil
	default:
		// "markdown", "", or any unknown format → markdown
		return formatMarkdown(results, stepIDs), nil
	}
}

// completedStepIDs returns IDs of all steps with status "completed" in
// arbitrary but deterministic (alphabetical) order.
func completedStepIDs(results map[string]*workflow.StepResult) []string {
	ids := make([]string, 0, len(results))
	for id, r := range results {
		if r.Status == workflow.StepStatusCompleted {
			ids = append(ids, id)
		}
	}
	// Sort for deterministic output.
	sortStrings(ids)
	return ids
}

// sortStrings sorts a slice of strings in place (insertion sort to avoid
// importing sort for a small helper).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// formatMarkdown renders results as a markdown document.
func formatMarkdown(results map[string]*workflow.StepResult, stepIDs []string) string {
	var b strings.Builder
	b.WriteString("# Workflow Results\n")

	for _, id := range stepIDs {
		r, ok := results[id]
		if !ok {
			continue
		}
		b.WriteString("\n## Step: ")
		b.WriteString(id)
		b.WriteString("\n\n")
		if r.Status == workflow.StepStatusSkipped {
			b.WriteString("*Skipped*\n")
		} else {
			b.WriteString(r.Output)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatPlain renders results as plain text with === delimiters.
func formatPlain(results map[string]*workflow.StepResult, stepIDs []string) string {
	var b strings.Builder

	for i, id := range stepIDs {
		r, ok := results[id]
		if !ok {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("=== ")
		b.WriteString(id)
		b.WriteString(" ===\n\n")
		if r.Status == workflow.StepStatusSkipped {
			b.WriteString("(skipped)\n")
		} else {
			b.WriteString(r.Output)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// jsonOutput is the top-level JSON structure for workflow results.
type jsonOutput struct {
	Steps map[string]jsonStep `json:"steps"`
}

// jsonStep represents a single step in the JSON output.
type jsonStep struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

// formatJSON renders results as a JSON document.
func formatJSON(results map[string]*workflow.StepResult, stepIDs []string) (string, error) {
	out := jsonOutput{Steps: make(map[string]jsonStep, len(stepIDs))}

	for _, id := range stepIDs {
		r, ok := results[id]
		if !ok {
			continue
		}
		js := jsonStep{Status: string(r.Status)}
		if r.Status != workflow.StepStatusSkipped {
			js.Output = r.Output
		}
		out.Steps[id] = js
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling JSON output: %w", err)
	}
	return string(data), nil
}

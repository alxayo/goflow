// Package workflow — parser.go
//
// ParseWorkflow reads a workflow YAML file and returns a validated Workflow
// struct. It performs structural parsing and basic type checking. Deep
// semantic validation (cycle detection, agent resolution) is handled by
// dedicated components.
package workflow

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// ParseWorkflow reads a YAML file at the given path and returns a Workflow.
func ParseWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %q: %w", path, err)
	}
	return ParseWorkflowBytes(data)
}

// ParseWorkflowBytes parses YAML bytes into a Workflow struct.
func ParseWorkflowBytes(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	// Apply defaults
	if wf.Config.AuditDir == "" {
		wf.Config.AuditDir = ".workflow-runs"
	}
	if wf.Config.LogLevel == "" {
		wf.Config.LogLevel = "info"
	}
	if wf.Output.Format == "" {
		wf.Output.Format = "markdown"
	}

	return &wf, nil
}

// templateRefPattern matches {{steps.<stepID>.output}} references in prompts.
var templateRefPattern = regexp.MustCompile(`\{\{steps\.([a-zA-Z0-9_-]+)\.output\}\}`)

// ValidateWorkflow performs semantic validation on a parsed workflow.
func ValidateWorkflow(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// Build lookup sets for steps and agents.
	stepIDs := make(map[string]bool, len(wf.Steps))
	for _, s := range wf.Steps {
		if stepIDs[s.ID] {
			return fmt.Errorf("duplicate step ID %q", s.ID)
		}
		stepIDs[s.ID] = true
	}

	for _, s := range wf.Steps {
		if s.Agent == "" {
			return fmt.Errorf("step %q: agent is required", s.ID)
		}
		if s.Prompt == "" {
			return fmt.Errorf("step %q: prompt is required", s.ID)
		}

		// Agent must be declared in workflow agents map.
		if wf.Agents != nil {
			if _, ok := wf.Agents[s.Agent]; !ok {
				return fmt.Errorf("step %q: agent %q not defined in workflow agents", s.ID, s.Agent)
			}
		}

		// Validate depends_on references.
		for _, dep := range s.DependsOn {
			if dep == s.ID {
				return fmt.Errorf("step %q: cannot depend on itself", s.ID)
			}
			if !stepIDs[dep] {
				return fmt.Errorf("step %q: depends_on references unknown step %q", s.ID, dep)
			}
		}

		// Validate condition references.
		if s.Condition != nil {
			condStep := s.Condition.Step
			if !stepIDs[condStep] {
				return fmt.Errorf("step %q: condition references unknown step %q", s.ID, condStep)
			}
			if !isTransitiveDependency(s.ID, condStep, wf.Steps) {
				return fmt.Errorf("step %q: condition step %q must be an upstream dependency", s.ID, condStep)
			}
		}

		// Validate template references in prompt.
		matches := templateRefPattern.FindAllStringSubmatch(s.Prompt, -1)
		for _, m := range matches {
			ref := m[1]
			if !stepIDs[ref] {
				return fmt.Errorf("step %q: template references unknown step %q", s.ID, ref)
			}
		}
	}

	// Validate agent definitions: each must have file or inline.
	for name, ref := range wf.Agents {
		if ref.File == "" && ref.Inline == nil {
			return fmt.Errorf("agent %q: must have either 'file' or 'inline' defined", name)
		}
	}

	return nil
}

// isTransitiveDependency returns true if condStep is reachable via the
// depends_on chain starting from the step identified by stepID.
func isTransitiveDependency(stepID, condStep string, steps []Step) bool {
	// Build a map from step ID to its depends_on list.
	deps := make(map[string][]string, len(steps))
	for _, s := range steps {
		deps[s.ID] = s.DependsOn
	}

	// BFS from stepID through depends_on edges.
	visited := make(map[string]bool)
	queue := make([]string, 0, len(deps[stepID]))
	queue = append(queue, deps[stepID]...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		if cur == condStep {
			return true
		}
		queue = append(queue, deps[cur]...)
	}
	return false
}

// condition.go evaluates step conditions to decide whether a step should
// execute. Supports contains, not_contains, and equals operators against
// a prior step's output.
package workflow

import (
	"fmt"
	"strings"
)

// EvaluateCondition checks whether a step's condition is met based on the
// referenced step's output. Returns true if the step should execute.
// If the step has no condition, always returns true.
func EvaluateCondition(cond *Condition, results map[string]string) (bool, error) {
	if cond == nil {
		return true, nil
	}

	output, ok := results[cond.Step]
	if !ok {
		return false, fmt.Errorf("condition references step %q which has no result", cond.Step)
	}

	switch {
	case cond.Contains != "":
		return strings.Contains(output, cond.Contains), nil
	case cond.NotContains != "":
		return !strings.Contains(output, cond.NotContains), nil
	case cond.Equals != "":
		return strings.TrimSpace(output) == strings.TrimSpace(cond.Equals), nil
	default:
		// Condition struct present but no operator specified → treat as always true
		return true, nil
	}
}

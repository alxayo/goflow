// dag.go builds an execution plan from workflow steps using Kahn's algorithm
// (BFS topological sort). Steps are grouped into levels — steps within a
// level share the same topological depth and can run concurrently.
package workflow

import (
	"fmt"
	"sort"
	"strings"
)

// DAGLevel is a set of steps that can execute concurrently — they share
// the same topological depth (all dependencies already satisfied).
type DAGLevel struct {
	Steps []Step
	Depth int // 0-indexed topological depth
}

// BuildDAG constructs an execution plan from the workflow steps.
// Steps are grouped into levels using Kahn's algorithm (BFS topo sort).
// Returns an error if a cycle is detected.
func BuildDAG(steps []Step) ([]DAGLevel, error) {
	if len(steps) == 0 {
		return nil, nil
	}

	// Index steps by ID for fast lookup.
	stepByID := make(map[string]Step, len(steps))
	for _, s := range steps {
		stepByID[s.ID] = s
	}

	// Compute in-degree for each step and build a reverse adjacency list
	// (dependency → list of dependents).
	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string) // dep ID → IDs that depend on it
	for _, s := range steps {
		if _, exists := inDegree[s.ID]; !exists {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++
			dependents[dep] = append(dependents[dep], s.ID)
		}
	}

	// Seed the queue with all zero-in-degree steps (roots).
	queue := make([]string, 0)
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var levels []DAGLevel
	processed := 0

	for len(queue) > 0 {
		// Drain the entire queue into the current level.
		level := DAGLevel{Depth: len(levels)}
		currentIDs := queue
		queue = nil

		// Sort IDs for deterministic level ordering.
		sort.Strings(currentIDs)

		for _, id := range currentIDs {
			level.Steps = append(level.Steps, stepByID[id])
			processed++

			// Decrement in-degree for each dependent; enqueue if it reaches 0.
			for _, depID := range dependents[id] {
				inDegree[depID]--
				if inDegree[depID] == 0 {
					queue = append(queue, depID)
				}
			}
		}

		levels = append(levels, level)
	}

	// If we didn't process every step, a cycle exists among the remainder.
	if processed < len(steps) {
		remaining := make([]string, 0, len(steps)-processed)
		for _, s := range steps {
			if inDegree[s.ID] > 0 {
				remaining = append(remaining, s.ID)
			}
		}
		sort.Strings(remaining)
		return nil, fmt.Errorf("cycle detected among steps: %s", strings.Join(remaining, ", "))
	}

	return levels, nil
}

// ValidateDAG checks for structural issues beyond cycles:
//   - Orphan dependencies (steps referencing non-existent step IDs)
func ValidateDAG(steps []Step) error {
	known := make(map[string]struct{}, len(steps))
	for _, s := range steps {
		known[s.ID] = struct{}{}
	}

	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, exists := known[dep]; !exists {
				return fmt.Errorf("step %q depends on non-existent step %q", s.ID, dep)
			}
		}
	}
	return nil
}

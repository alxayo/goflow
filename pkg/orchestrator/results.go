// results.go provides a concurrent-safe map for storing step execution
// results during parallel DAG execution.
package orchestrator

import (
	"sync"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

// ResultsStore is a concurrent-safe map for storing step execution results.
// All methods are safe for concurrent use from multiple goroutines.
type ResultsStore struct {
	mu      sync.RWMutex
	results map[string]*workflow.StepResult
}

// NewResultsStore creates an empty ResultsStore.
func NewResultsStore() *ResultsStore {
	return &ResultsStore{results: make(map[string]*workflow.StepResult)}
}

// Store saves a step result. Thread-safe.
func (rs *ResultsStore) Store(stepID string, result *workflow.StepResult) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.results[stepID] = result
}

// Get retrieves a step result by ID. Thread-safe.
func (rs *ResultsStore) Get(stepID string) (*workflow.StepResult, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	r, ok := rs.results[stepID]
	return r, ok
}

// OutputMap returns a snapshot (copy) of step ID → output string for all
// completed steps. This snapshot is safe to pass to template resolution
// without holding locks.
func (rs *ResultsStore) OutputMap() map[string]string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]string, len(rs.results))
	for id, r := range rs.results {
		if r.Status == workflow.StepStatusCompleted {
			out[id] = r.Output
		}
	}
	return out
}

// All returns a copy of all results regardless of status.
func (rs *ResultsStore) All() map[string]*workflow.StepResult {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	cp := make(map[string]*workflow.StepResult, len(rs.results))
	for id, r := range rs.results {
		cp[id] = r
	}
	return cp
}

// Semaphore limits concurrent step execution within a DAG level.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a semaphore with the given max concurrency.
// If max <= 0, creates an unlimited semaphore (Acquire/Release are no-ops).
func NewSemaphore(max int) *Semaphore {
	if max <= 0 {
		return &Semaphore{ch: nil}
	}
	return &Semaphore{ch: make(chan struct{}, max)}
}

// Acquire blocks until a slot is available.
func (s *Semaphore) Acquire() {
	if s.ch == nil {
		return
	}
	s.ch <- struct{}{}
}

// Release releases a slot.
func (s *Semaphore) Release() {
	if s.ch == nil {
		return
	}
	<-s.ch
}

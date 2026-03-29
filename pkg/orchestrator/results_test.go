package orchestrator

import (
	"fmt"
	"sync"
	"testing"

	"github.com/alex-workflow-runner/workflow-runner/pkg/workflow"
)

func TestStoreAndGet(t *testing.T) {
	store := NewResultsStore()

	r := &workflow.StepResult{
		StepID: "A",
		Status: workflow.StepStatusCompleted,
		Output: "hello",
	}
	store.Store("A", r)

	got, ok := store.Get("A")
	if !ok {
		t.Fatal("expected result for A")
	}
	if got.Output != "hello" {
		t.Errorf("output = %q, want %q", got.Output, "hello")
	}
}

func TestGetMissing(t *testing.T) {
	store := NewResultsStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestOutputMapOnlyCompleted(t *testing.T) {
	store := NewResultsStore()
	store.Store("completed", &workflow.StepResult{
		StepID: "completed",
		Status: workflow.StepStatusCompleted,
		Output: "result-c",
	})
	store.Store("failed", &workflow.StepResult{
		StepID: "failed",
		Status: workflow.StepStatusFailed,
		Output: "result-f",
	})
	store.Store("skipped", &workflow.StepResult{
		StepID: "skipped",
		Status: workflow.StepStatusSkipped,
	})
	store.Store("running", &workflow.StepResult{
		StepID: "running",
		Status: workflow.StepStatusRunning,
		Output: "partial",
	})

	out := store.OutputMap()
	if len(out) != 2 {
		t.Fatalf("expected 2 entries in OutputMap (completed + skipped), got %d", len(out))
	}
	if out["completed"] != "result-c" {
		t.Errorf("completed output = %q, want %q", out["completed"], "result-c")
	}
	if out["skipped"] != "" {
		t.Errorf("skipped output = %q, want empty string", out["skipped"])
	}
}

func TestOutputMapReturnsSnapshot(t *testing.T) {
	store := NewResultsStore()
	store.Store("A", &workflow.StepResult{
		StepID: "A",
		Status: workflow.StepStatusCompleted,
		Output: "original",
	})

	snap := store.OutputMap()

	// Modifying the snapshot must not affect the store.
	snap["A"] = "modified"
	snap["B"] = "injected"

	got, _ := store.Get("A")
	if got.Output != "original" {
		t.Errorf("store was mutated via snapshot: Output = %q", got.Output)
	}

	_, ok := store.Get("B")
	if ok {
		t.Error("store gained entry 'B' via snapshot mutation")
	}
}

func TestAllReturnsAllStatuses(t *testing.T) {
	store := NewResultsStore()
	store.Store("ok", &workflow.StepResult{StepID: "ok", Status: workflow.StepStatusCompleted})
	store.Store("bad", &workflow.StepResult{StepID: "bad", Status: workflow.StepStatusFailed})
	store.Store("skip", &workflow.StepResult{StepID: "skip", Status: workflow.StepStatusSkipped})

	all := store.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 results, got %d", len(all))
	}
	for _, id := range []string{"ok", "bad", "skip"} {
		if _, ok := all[id]; !ok {
			t.Errorf("missing result for %q", id)
		}
	}
}

func TestConcurrentStore(t *testing.T) {
	store := NewResultsStore()

	const n = 100
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("step-%d", idx)
			store.Store(id, &workflow.StepResult{
				StepID: id,
				Status: workflow.StepStatusCompleted,
				Output: fmt.Sprintf("output-%d", idx),
			})
		}(i)
	}

	wg.Wait()

	all := store.All()
	if len(all) != n {
		t.Fatalf("expected %d results, got %d", n, len(all))
	}
}

// --- Semaphore tests ---

func TestSemaphoreUnlimited(t *testing.T) {
	sem := NewSemaphore(0)
	// Unlimited: Acquire/Release should be no-ops, never block.
	sem.Acquire()
	sem.Release()
	sem.Acquire()
	sem.Release()
}

func TestSemaphoreLimited(t *testing.T) {
	sem := NewSemaphore(2)
	sem.Acquire()
	sem.Acquire()
	// Both slots taken; release one so a goroutine can proceed.
	sem.Release()
	sem.Acquire()
	sem.Release()
	sem.Release()
}

func TestSemaphoreConcurrency(t *testing.T) {
	const max = 3
	sem := NewSemaphore(max)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.Acquire()
			defer sem.Release()
			// no-op work; the race detector validates correctness.
		}()
	}
	wg.Wait()
}

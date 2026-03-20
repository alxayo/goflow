package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewManager_WithInitialContent(t *testing.T) {
	dir := t.TempDir()
	initial := "# Shared Memory\nSome initial notes."

	m, err := NewManager(dir, initial)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got := m.Read()
	// Initial content should have a trailing newline appended.
	want := initial + "\n"
	if got != want {
		t.Errorf("Read() = %q, want %q", got, want)
	}
}

func TestNewManager_EmptyContent(t *testing.T) {
	dir := t.TempDir()

	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got := m.Read()
	if got != "" {
		t.Errorf("Read() = %q, want empty string", got)
	}
}

func TestNewManager_InvalidDir(t *testing.T) {
	_, err := NewManager("/nonexistent/path/that/should/fail", "")
	if err == nil {
		t.Fatal("NewManager() expected error for invalid directory, got nil")
	}
	if !strings.Contains(err.Error(), "initializing shared memory") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "initializing shared memory")
	}
}

func TestWrite_AppendsTimestampedEntry(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := m.Write("security-reviewer", "Found SQL injection in auth.go"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := m.Read()
	// Verify the entry contains expected components.
	if !strings.Contains(got, "[security-reviewer]") {
		t.Errorf("Read() = %q, want it to contain agent name", got)
	}
	if !strings.Contains(got, "Found SQL injection in auth.go") {
		t.Errorf("Read() = %q, want it to contain entry text", got)
	}
	// Verify RFC3339 timestamp bracket pattern is present (e.g. [2026-...Z]).
	if !strings.Contains(got, "Z] [") {
		t.Errorf("Read() = %q, want it to contain RFC3339 timestamp", got)
	}
	// Entry should end with a newline.
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Read() = %q, want trailing newline", got)
	}
}

func TestWrite_MultipleAgents_PreservesOrder(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	entries := []struct {
		agent string
		text  string
	}{
		{"agent-a", "first entry"},
		{"agent-b", "second entry"},
		{"agent-a", "third entry"},
		{"agent-c", "fourth entry"},
	}

	for _, e := range entries {
		if err := m.Write(e.agent, e.text); err != nil {
			t.Fatalf("Write(%q, %q) error = %v", e.agent, e.text, err)
		}
	}

	got := m.Read()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != len(entries) {
		t.Fatalf("got %d lines, want %d", len(lines), len(entries))
	}

	for i, e := range entries {
		if !strings.Contains(lines[i], fmt.Sprintf("[%s]", e.agent)) {
			t.Errorf("line %d = %q, want it to contain [%s]", i, lines[i], e.agent)
		}
		if !strings.Contains(lines[i], e.text) {
			t.Errorf("line %d = %q, want it to contain %q", i, lines[i], e.text)
		}
	}
}

func TestWrite_ConcurrentWrites_NoCorruption(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	const numGoroutines = 20
	const writesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			defer wg.Done()
			agent := fmt.Sprintf("agent-%d", id)
			for w := 0; w < writesPerGoroutine; w++ {
				entry := fmt.Sprintf("entry-%d", w)
				if err := m.Write(agent, entry); err != nil {
					t.Errorf("Write(%q, %q) error = %v", agent, entry, err)
				}
			}
		}(g)
	}

	wg.Wait()

	got := m.Read()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	totalExpected := numGoroutines * writesPerGoroutine
	if len(lines) != totalExpected {
		t.Errorf("got %d lines, want %d", len(lines), totalExpected)
	}

	// Every line should be well-formed: starts with '[' and contains agent marker.
	for i, line := range lines {
		if !strings.HasPrefix(line, "[") {
			t.Errorf("line %d = %q, want it to start with '['", i, line)
		}
		if !strings.Contains(line, "] [agent-") {
			t.Errorf("line %d = %q, want it to contain agent marker", i, line)
		}
	}
}

func TestFlush_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "initial seed")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := m.Write("test-agent", "disk persistence check"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read the file directly from disk.
	data, err := os.ReadFile(filepath.Join(dir, "memory.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	diskContent := string(data)
	memContent := m.Read()
	if diskContent != memContent {
		t.Errorf("disk content = %q, memory content = %q, want them to match", diskContent, memContent)
	}

	// Both should contain the initial seed and the written entry.
	if !strings.Contains(diskContent, "initial seed") {
		t.Errorf("disk content = %q, want it to contain %q", diskContent, "initial seed")
	}
	if !strings.Contains(diskContent, "disk persistence check") {
		t.Errorf("disk content = %q, want it to contain %q", diskContent, "disk persistence check")
	}
}

func TestRead_ConcurrentWithWriters(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	const numWriters = 5
	const numReaders = 5
	const iterations = 20

	var wg sync.WaitGroup
	wg.Add(numWriters + numReaders)

	// Spawn concurrent writers.
	for w := 0; w < numWriters; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if err := m.Write(fmt.Sprintf("writer-%d", id), fmt.Sprintf("msg-%d", i)); err != nil {
					t.Errorf("Write() error = %v", err)
				}
			}
		}(w)
	}

	// Spawn concurrent readers.
	for r := 0; r < numReaders; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				content := m.Read()
				// Content should always be valid (no partial writes).
				if strings.Count(content, "[") != strings.Count(content, "]") {
					t.Errorf("Read() returned content with mismatched brackets, possible corruption")
				}
			}
		}()
	}

	wg.Wait()
}

func TestInjectIntoPrompt_EmptyMemory(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	prompt := "Analyze the code for security issues."
	got := m.InjectIntoPrompt(prompt)
	if got != prompt {
		t.Errorf("InjectIntoPrompt() = %q, want original prompt unchanged %q", got, prompt)
	}
}

func TestInjectIntoPrompt_NonEmptyMemory(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "some initial context")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	prompt := "Analyze the code."
	got := m.InjectIntoPrompt(prompt)

	if !strings.Contains(got, "--- Shared Memory (read-only context from other agents) ---") {
		t.Error("want opening delimiter in output")
	}
	if !strings.Contains(got, "--- End Shared Memory ---") {
		t.Error("want closing delimiter in output")
	}
	if !strings.Contains(got, "some initial context") {
		t.Error("want memory content in output")
	}
	if !strings.HasSuffix(got, prompt) {
		t.Errorf("want output to end with original prompt %q", prompt)
	}
}

func TestInjectIntoPrompt_AfterWrites(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := m.Write("agent-a", "found a bug"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := m.Write("agent-b", "fixed the bug"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	prompt := "Review the changes."
	got := m.InjectIntoPrompt(prompt)

	if !strings.Contains(got, "[agent-a] found a bug") {
		t.Error("want first write entry in injected prompt")
	}
	if !strings.Contains(got, "[agent-b] fixed the bug") {
		t.Error("want second write entry in injected prompt")
	}
	if !strings.HasSuffix(got, prompt) {
		t.Errorf("want output to end with original prompt %q", prompt)
	}
}

func TestInjectIntoPrompt_PreservesPromptExactly(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "context")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	prompt := "Line 1\nLine 2\n  indented\ttabbed\nspecial chars: !@#$%^&*()"
	got := m.InjectIntoPrompt(prompt)

	// The original prompt must appear verbatim after the delimiter block.
	idx := strings.Index(got, "--- End Shared Memory ---\n\n")
	if idx == -1 {
		t.Fatal("missing End Shared Memory delimiter")
	}
	afterDelimiter := got[idx+len("--- End Shared Memory ---\n\n"):]
	if afterDelimiter != prompt {
		t.Errorf("prompt after injection = %q, want exact original %q", afterDelimiter, prompt)
	}
}

func TestFilePath(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, "")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	want := filepath.Join(dir, "memory.md")
	if got := m.FilePath(); got != want {
		t.Errorf("FilePath() = %q, want %q", got, want)
	}
}

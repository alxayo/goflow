// Package memory provides a shared memory file that parallel agents can read
// from and write to during workflow execution. Writes are serialized via a
// mutex to prevent corruption. The memory content can be injected into prompts
// or exposed as tools.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager provides thread-safe read/write access to a shared memory file.
type Manager struct {
	mu       sync.RWMutex
	content  strings.Builder
	filePath string
}

// NewManager creates a shared memory manager. If initialContent is non-empty,
// it seeds the memory with that content. The memory file is created in the
// given directory as "memory.md".
func NewManager(dir string, initialContent string) (*Manager, error) {
	m := &Manager{
		filePath: filepath.Join(dir, "memory.md"),
	}
	if initialContent != "" {
		m.content.WriteString(initialContent)
		m.content.WriteString("\n")
	}
	// Persist initial content to disk.
	if err := m.Flush(); err != nil {
		return nil, fmt.Errorf("initializing shared memory: %w", err)
	}
	return m, nil
}

// Read returns the current memory content. Thread-safe.
func (m *Manager) Read() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.content.String()
}

// Write appends a timestamped, agent-attributed entry. Thread-safe.
// Format: [2026-03-20T14:32:15Z] [agent-name] entry text
func (m *Manager) Write(agentName string, entry string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, agentName, entry)
	m.content.WriteString(line)

	return m.flush()
}

// Flush persists the current memory content to disk.
func (m *Manager) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flush()
}

// flush is the non-locking internal flush.
func (m *Manager) flush() error {
	return os.WriteFile(m.filePath, []byte(m.content.String()), 0644)
}

// InjectIntoPrompt prepends the current shared memory content to a prompt.
// The memory is formatted as a clearly delimited section so the agent can
// distinguish it from the actual task prompt.
//
// Format:
//
//	--- Shared Memory (read-only context from other agents) ---
//	<memory content>
//	--- End Shared Memory ---
//
//	<original prompt>
func (m *Manager) InjectIntoPrompt(prompt string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content := m.content.String()
	if content == "" {
		return prompt
	}

	return fmt.Sprintf("--- Shared Memory (read-only context from other agents) ---\n%s\n--- End Shared Memory ---\n\n%s", content, prompt)
}

// FilePath returns the path to the memory file on disk.
func (m *Manager) FilePath() string {
	return m.filePath
}

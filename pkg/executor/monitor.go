// monitor.go provides event-based session monitoring for long-running workflow steps.
// It replaces timeout-based blocking with event-driven completion detection, giving
// real-time visibility into session progress and eliminating the need for timeout
// configuration.
//
// The SessionMonitor tracks:
//   - Session state (starting, running, idle, error)
//   - Tool execution progress
//   - Streaming assistant output
//   - Last activity timestamp (for debugging)
package executor

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// SessionState represents the current state of a monitored session.
type SessionState int

const (
	// StateStarting means the session is being initialized.
	StateStarting SessionState = iota
	// StateRunning means the session is actively processing (LLM thinking or tools running).
	StateRunning
	// StateIdle means the session has completed its current turn.
	StateIdle
	// StateError means the session encountered an error.
	StateError
)

// String returns a human-readable state name.
func (s SessionState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateIdle:
		return "idle"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ToolCallInfo tracks a single tool invocation during session execution.
type ToolCallInfo struct {
	Name      string
	Args      string // JSON-encoded arguments (for logging)
	StartedAt time.Time
	EndedAt   time.Time
	Status    string // "running", "completed", "failed"
	Result    string // Truncated result for logging
}

// SessionEventInfo is a simplified event structure passed to progress callbacks.
// It abstracts the SDK's SessionEvent to avoid leaking SDK types outside the executor.
type SessionEventInfo struct {
	Type      string      // Event type (e.g., "tool.execution_start", "assistant.message_delta")
	StepID    string      // Workflow step ID
	SessionID string      // SDK session ID
	Timestamp time.Time   // When the event occurred
	Data      interface{} // Event-specific payload (tool name, delta text, etc.)
}

// ProgressHandler is a callback for session progress events.
// Used to stream output to the CLI or write to audit logs.
type ProgressHandler func(event SessionEventInfo)

// SessionMonitor tracks the state and progress of a single session.
// It is goroutine-safe and can be updated from event handler callbacks.
type SessionMonitor struct {
	StepID       string
	SessionID    string
	State        SessionState
	LastActivity time.Time
	ToolCalls    []ToolCallInfo
	StreamedText strings.Builder
	ErrorMsg     string
	StartedAt    time.Time
	EndedAt      time.Time

	// onProgress is called for each significant event (if set).
	onProgress ProgressHandler

	mu sync.Mutex
}

// NewSessionMonitor creates a monitor for tracking a session's progress.
func NewSessionMonitor(stepID, sessionID string, onProgress ProgressHandler) *SessionMonitor {
	return &SessionMonitor{
		StepID:       stepID,
		SessionID:    sessionID,
		State:        StateStarting,
		LastActivity: time.Now(),
		StartedAt:    time.Now(),
		ToolCalls:    make([]ToolCallInfo, 0),
		onProgress:   onProgress,
	}
}

// SetState updates the session state with proper locking.
func (m *SessionMonitor) SetState(state SessionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.State = state
	m.LastActivity = time.Now()
	if state == StateIdle || state == StateError {
		m.EndedAt = time.Now()
	}
}

// GetState returns the current session state.
func (m *SessionMonitor) GetState() SessionState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.State
}

// SetError records an error state with message.
func (m *SessionMonitor) SetError(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.State = StateError
	m.ErrorMsg = msg
	m.LastActivity = time.Now()
	m.EndedAt = time.Now()
}

// GetError returns the error message if in error state.
func (m *SessionMonitor) GetError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ErrorMsg
}

// StartToolCall records the start of a tool execution.
func (m *SessionMonitor) StartToolCall(name, args string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ToolCalls = append(m.ToolCalls, ToolCallInfo{
		Name:      name,
		Args:      args,
		StartedAt: time.Now(),
		Status:    "running",
	})
	m.LastActivity = time.Now()
	m.State = StateRunning

	if m.onProgress != nil {
		m.onProgress(SessionEventInfo{
			Type:      "tool.execution_start",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
			Data:      map[string]string{"tool": name, "args": args},
		})
	}
}

// CompleteToolCall marks the most recent tool call with the given name as complete.
func (m *SessionMonitor) CompleteToolCall(name, result, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the most recent running tool call with this name
	for i := len(m.ToolCalls) - 1; i >= 0; i-- {
		if m.ToolCalls[i].Name == name && m.ToolCalls[i].Status == "running" {
			m.ToolCalls[i].EndedAt = time.Now()
			m.ToolCalls[i].Status = status
			// Truncate result for logging
			if len(result) > 500 {
				m.ToolCalls[i].Result = result[:500] + "..."
			} else {
				m.ToolCalls[i].Result = result
			}
			break
		}
	}
	m.LastActivity = time.Now()

	if m.onProgress != nil {
		m.onProgress(SessionEventInfo{
			Type:      "tool.execution_complete",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
			Data:      map[string]string{"tool": name, "status": status},
		})
	}
}

// AppendStreamedText adds streaming text delta to the accumulated output.
func (m *SessionMonitor) AppendStreamedText(delta string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StreamedText.WriteString(delta)
	m.LastActivity = time.Now()

	if m.onProgress != nil {
		m.onProgress(SessionEventInfo{
			Type:      "assistant.message_delta",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
			Data:      delta,
		})
	}
}

// GetStreamedText returns all accumulated streamed text.
func (m *SessionMonitor) GetStreamedText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.StreamedText.String()
}

// Duration returns how long the session has been running.
func (m *SessionMonitor) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.EndedAt.IsZero() {
		return m.EndedAt.Sub(m.StartedAt)
	}
	return time.Since(m.StartedAt)
}

// EmitTurnStart signals the start of an assistant turn.
func (m *SessionMonitor) EmitTurnStart() {
	m.mu.Lock()
	m.State = StateRunning
	m.LastActivity = time.Now()
	onProgress := m.onProgress
	m.mu.Unlock()

	if onProgress != nil {
		onProgress(SessionEventInfo{
			Type:      "assistant.turn_start",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
		})
	}
}

// EmitTurnEnd signals the end of an assistant turn.
func (m *SessionMonitor) EmitTurnEnd() {
	m.mu.Lock()
	m.LastActivity = time.Now()
	onProgress := m.onProgress
	m.mu.Unlock()

	if onProgress != nil {
		onProgress(SessionEventInfo{
			Type:      "assistant.turn_end",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
		})
	}
}

// EmitIdle signals that the session has become idle (completed).
func (m *SessionMonitor) EmitIdle() {
	m.SetState(StateIdle)

	m.mu.Lock()
	onProgress := m.onProgress
	m.mu.Unlock()

	if onProgress != nil {
		onProgress(SessionEventInfo{
			Type:      "session.idle",
			StepID:    m.StepID,
			SessionID: m.SessionID,
			Timestamp: time.Now(),
		})
	}
}

// Summary returns a brief summary of the session for logging.
func (m *SessionMonitor) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString("state=")
	b.WriteString(m.State.String())
	b.WriteString(" duration=")
	if !m.EndedAt.IsZero() {
		b.WriteString(m.EndedAt.Sub(m.StartedAt).Round(time.Millisecond).String())
	} else {
		b.WriteString(time.Since(m.StartedAt).Round(time.Millisecond).String())
	}
	b.WriteString(" tools=")
	b.WriteString(strconv.Itoa(len(m.ToolCalls)))
	if m.ErrorMsg != "" {
		b.WriteString(" error=")
		b.WriteString(m.ErrorMsg)
	}
	return b.String()
}

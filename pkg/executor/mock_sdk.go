package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// MockSessionExecutor returns pre-configured responses for testing.
// It matches prompts against response keys using substring matching.
type MockSessionExecutor struct {
	// Responses maps prompt substrings to mock outputs.
	// When Send is called, the first key found as a substring of the prompt
	// is used to look up the response.
	Responses map[string]string

	// DefaultResponse is returned when no matching key is found.
	DefaultResponse string

	// CreateErr causes CreateSession to return this error if set.
	CreateErr error

	// SendErr causes Session.Send to return this error if set.
	SendErr error

	// SendErrForStep maps step prompt substrings to errors, allowing
	// selective failures in parallel tests.
	SendErrForStep map[string]error

	// SendHook, if set, is called during Send before returning.
	// Use this to inject synchronization logic (e.g., channels) for
	// concurrency verification in parallel tests.
	SendHook func(prompt string)

	// SessionsCreated tracks how many sessions were created (for assertions).
	SessionsCreated atomic.Int32

	// LastConfig records the most recent SessionConfig passed to CreateSession.
	LastConfig SessionConfig

	mu sync.Mutex
}

func (m *MockSessionExecutor) CreateSession(ctx context.Context, cfg SessionConfig) (Session, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	m.mu.Lock()
	m.LastConfig = cfg
	m.mu.Unlock()
	m.SessionsCreated.Add(1)
	return &MockSession{
		id:             fmt.Sprintf("mock-session-%d", m.SessionsCreated.Load()),
		responses:      m.Responses,
		defaultResp:    m.DefaultResponse,
		sendErr:        m.SendErr,
		sendErrForStep: m.SendErrForStep,
		sendHook:       m.SendHook,
	}, nil
}

// MockSession holds a single mock session's state.
type MockSession struct {
	id             string
	responses      map[string]string
	defaultResp    string
	sendErr        error
	sendErrForStep map[string]error
	sendHook       func(prompt string)
	closed         bool
}

func (ms *MockSession) Send(ctx context.Context, prompt string) (string, error) {
	if ms.sendErr != nil {
		return "", ms.sendErr
	}
	// Check per-step errors.
	for key, err := range ms.sendErrForStep {
		if strings.Contains(prompt, key) {
			return "", err
		}
	}
	// Call hook if set (used for concurrency verification).
	if ms.sendHook != nil {
		ms.sendHook(prompt)
	}
	// Match prompt against response keys
	for key, response := range ms.responses {
		if strings.Contains(prompt, key) {
			return response, nil
		}
	}
	if ms.defaultResp != "" {
		return ms.defaultResp, nil
	}
	return fmt.Sprintf("mock response for: %s", prompt), nil
}

func (ms *MockSession) SessionID() string { return ms.id }

func (ms *MockSession) Close() error {
	ms.closed = true
	return nil
}

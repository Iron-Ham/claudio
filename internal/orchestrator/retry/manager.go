// Package retry provides retry state management for task execution.
//
// This package tracks retry attempts per task, determines whether tasks
// should be retried based on configuration, and maintains retry history
// for debugging and auditing purposes.
package retry

import (
	"sync"
)

// TaskState tracks retry attempts for a task.
type TaskState struct {
	TaskID       string `json:"task_id"`
	RetryCount   int    `json:"retry_count"`
	MaxRetries   int    `json:"max_retries"`
	LastError    string `json:"last_error,omitempty"`
	CommitCounts []int  `json:"commit_counts,omitempty"` // Commits per attempt (for debugging)
	Succeeded    bool   `json:"succeeded,omitempty"`     // True if task eventually succeeded
}

// Manager manages retry state for tasks.
// It is thread-safe and can be used concurrently.
type Manager struct {
	mu     sync.RWMutex
	states map[string]*TaskState
}

// NewManager creates a new retry manager.
func NewManager() *Manager {
	return &Manager{
		states: make(map[string]*TaskState),
	}
}

// GetOrCreateState returns or creates retry state for a task.
// If the state doesn't exist, it creates one with the given maxRetries.
func (m *Manager) GetOrCreateState(taskID string, maxRetries int) *TaskState {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[taskID]
	if !exists {
		state = &TaskState{
			TaskID:       taskID,
			MaxRetries:   maxRetries,
			CommitCounts: make([]int, 0),
		}
		m.states[taskID] = state
	}
	return state
}

// GetState returns the retry state for a task, or nil if not found.
func (m *Manager) GetState(taskID string) *TaskState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[taskID]
}

// ShouldRetry returns whether a task should be retried.
// A task should be retried if it has retry state and hasn't exceeded max retries.
func (m *Manager) ShouldRetry(taskID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[taskID]
	if !exists {
		return false
	}
	return state.RetryCount < state.MaxRetries && !state.Succeeded
}

// RecordAttempt records an attempt for a task.
// If success is true, the task is marked as succeeded and no more retries will be allowed.
// If success is false, the retry count is incremented.
func (m *Manager) RecordAttempt(taskID string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[taskID]
	if !exists {
		return
	}

	if success {
		state.Succeeded = true
	} else {
		state.RetryCount++
	}
}

// RecordCommitCount records the number of commits for an attempt.
func (m *Manager) RecordCommitCount(taskID string, commitCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[taskID]
	if !exists {
		return
	}
	state.CommitCounts = append(state.CommitCounts, commitCount)
}

// SetLastError sets the last error message for a task.
func (m *Manager) SetLastError(taskID string, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[taskID]
	if !exists {
		return
	}
	state.LastError = errMsg
}

// GetFailedTasks returns the IDs of all tasks that have exhausted their retries
// without succeeding.
func (m *Manager) GetFailedTasks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var failed []string
	for taskID, state := range m.states {
		if !state.Succeeded && state.RetryCount >= state.MaxRetries {
			failed = append(failed, taskID)
		}
	}
	return failed
}

// GetRetryingTasks returns the IDs of all tasks that are still eligible for retry.
func (m *Manager) GetRetryingTasks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var retrying []string
	for taskID, state := range m.states {
		if !state.Succeeded && state.RetryCount < state.MaxRetries {
			retrying = append(retrying, taskID)
		}
	}
	return retrying
}

// Reset clears the retry state for a task.
func (m *Manager) Reset(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, taskID)
}

// ResetAll clears all retry state.
func (m *Manager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states = make(map[string]*TaskState)
}

// GetAllStates returns a copy of all task retry states.
// This is useful for serialization/persistence.
func (m *Manager) GetAllStates() map[string]*TaskState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*TaskState, len(m.states))
	for k, v := range m.states {
		stateCopy := *v
		// Copy the slice to avoid sharing
		if v.CommitCounts != nil {
			stateCopy.CommitCounts = make([]int, len(v.CommitCounts))
			copy(stateCopy.CommitCounts, v.CommitCounts)
		}
		result[k] = &stateCopy
	}
	return result
}

// LoadStates loads retry states from a map.
// This is useful for restoring from persistence.
func (m *Manager) LoadStates(states map[string]*TaskState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states = make(map[string]*TaskState, len(states))
	for k, v := range states {
		if v != nil {
			stateCopy := *v
			if v.CommitCounts != nil {
				stateCopy.CommitCounts = make([]int, len(v.CommitCounts))
				copy(stateCopy.CommitCounts, v.CommitCounts)
			}
			m.states[k] = &stateCopy
		}
	}
}

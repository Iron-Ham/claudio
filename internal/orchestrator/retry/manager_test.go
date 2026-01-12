package retry

import (
	"sort"
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.states == nil {
		t.Error("NewManager() states map is nil")
	}
}

func TestGetOrCreateState(t *testing.T) {
	tests := []struct {
		name       string
		taskID     string
		maxRetries int
		callTwice  bool
	}{
		{
			name:       "create new state",
			taskID:     "task-1",
			maxRetries: 3,
			callTwice:  false,
		},
		{
			name:       "get existing state",
			taskID:     "task-2",
			maxRetries: 5,
			callTwice:  true,
		},
		{
			name:       "zero max retries",
			taskID:     "task-3",
			maxRetries: 0,
			callTwice:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()

			state1 := m.GetOrCreateState(tt.taskID, tt.maxRetries)
			if state1 == nil {
				t.Fatal("GetOrCreateState() returned nil")
			}
			if state1.TaskID != tt.taskID {
				t.Errorf("TaskID = %q, want %q", state1.TaskID, tt.taskID)
			}
			if state1.MaxRetries != tt.maxRetries {
				t.Errorf("MaxRetries = %d, want %d", state1.MaxRetries, tt.maxRetries)
			}
			if state1.RetryCount != 0 {
				t.Errorf("RetryCount = %d, want 0", state1.RetryCount)
			}
			if state1.CommitCounts == nil {
				t.Error("CommitCounts is nil")
			}

			if tt.callTwice {
				state2 := m.GetOrCreateState(tt.taskID, tt.maxRetries+10) // different maxRetries
				if state2 != state1 {
					t.Error("second call returned different state")
				}
				// maxRetries should NOT change on second call
				if state2.MaxRetries != tt.maxRetries {
					t.Errorf("MaxRetries changed on second call: got %d, want %d", state2.MaxRetries, tt.maxRetries)
				}
			}
		})
	}
}

func TestGetState(t *testing.T) {
	m := NewManager()

	// Non-existent task
	state := m.GetState("nonexistent")
	if state != nil {
		t.Error("GetState() for nonexistent task should return nil")
	}

	// Create and get
	m.GetOrCreateState("task-1", 3)
	state = m.GetState("task-1")
	if state == nil {
		t.Fatal("GetState() for existing task returned nil")
	}
	if state.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", state.TaskID, "task-1")
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(m *Manager)
		taskID      string
		expected    bool
		description string
	}{
		{
			name:        "nonexistent task",
			setup:       func(_ *Manager) {},
			taskID:      "nonexistent",
			expected:    false,
			description: "should not retry if task doesn't exist",
		},
		{
			name: "new task with retries available",
			setup: func(m *Manager) {
				m.GetOrCreateState("task-1", 3)
			},
			taskID:      "task-1",
			expected:    true,
			description: "should retry new task",
		},
		{
			name: "task at max retries",
			setup: func(m *Manager) {
				m.GetOrCreateState("task-2", 3)
				m.RecordAttempt("task-2", false)
				m.RecordAttempt("task-2", false)
				m.RecordAttempt("task-2", false)
			},
			taskID:      "task-2",
			expected:    false,
			description: "should not retry when max retries reached",
		},
		{
			name: "task succeeded",
			setup: func(m *Manager) {
				m.GetOrCreateState("task-3", 3)
				m.RecordAttempt("task-3", true)
			},
			taskID:      "task-3",
			expected:    false,
			description: "should not retry succeeded task",
		},
		{
			name: "task with some retries used",
			setup: func(m *Manager) {
				m.GetOrCreateState("task-4", 3)
				m.RecordAttempt("task-4", false)
				m.RecordAttempt("task-4", false)
			},
			taskID:      "task-4",
			expected:    true,
			description: "should retry if retries remain",
		},
		{
			name: "zero max retries",
			setup: func(m *Manager) {
				m.GetOrCreateState("task-5", 0)
			},
			taskID:      "task-5",
			expected:    false,
			description: "should not retry with zero max retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			tt.setup(m)

			result := m.ShouldRetry(tt.taskID)
			if result != tt.expected {
				t.Errorf("%s: ShouldRetry() = %v, want %v", tt.description, result, tt.expected)
			}
		})
	}
}

func TestRecordAttempt(t *testing.T) {
	t.Run("record failure", func(t *testing.T) {
		m := NewManager()
		m.GetOrCreateState("task-1", 3)

		m.RecordAttempt("task-1", false)
		state := m.GetState("task-1")
		if state.RetryCount != 1 {
			t.Errorf("RetryCount = %d, want 1", state.RetryCount)
		}
		if state.Succeeded {
			t.Error("Succeeded should be false")
		}

		m.RecordAttempt("task-1", false)
		state = m.GetState("task-1")
		if state.RetryCount != 2 {
			t.Errorf("RetryCount = %d, want 2", state.RetryCount)
		}
	})

	t.Run("record success", func(t *testing.T) {
		m := NewManager()
		m.GetOrCreateState("task-2", 3)
		m.RecordAttempt("task-2", false) // First attempt fails

		m.RecordAttempt("task-2", true) // Second attempt succeeds
		state := m.GetState("task-2")
		if !state.Succeeded {
			t.Error("Succeeded should be true")
		}
		// RetryCount should remain at 1 (only failures increment)
		if state.RetryCount != 1 {
			t.Errorf("RetryCount = %d, want 1", state.RetryCount)
		}
	})

	t.Run("nonexistent task", func(t *testing.T) {
		m := NewManager()
		// Should not panic
		m.RecordAttempt("nonexistent", false)
		m.RecordAttempt("nonexistent", true)
	})
}

func TestRecordCommitCount(t *testing.T) {
	m := NewManager()
	m.GetOrCreateState("task-1", 3)

	m.RecordCommitCount("task-1", 0)
	m.RecordCommitCount("task-1", 2)
	m.RecordCommitCount("task-1", 1)

	state := m.GetState("task-1")
	expected := []int{0, 2, 1}
	if len(state.CommitCounts) != len(expected) {
		t.Fatalf("CommitCounts length = %d, want %d", len(state.CommitCounts), len(expected))
	}
	for i, v := range expected {
		if state.CommitCounts[i] != v {
			t.Errorf("CommitCounts[%d] = %d, want %d", i, state.CommitCounts[i], v)
		}
	}

	// Non-existent task should not panic
	m.RecordCommitCount("nonexistent", 5)
}

func TestSetLastError(t *testing.T) {
	m := NewManager()
	m.GetOrCreateState("task-1", 3)

	m.SetLastError("task-1", "first error")
	state := m.GetState("task-1")
	if state.LastError != "first error" {
		t.Errorf("LastError = %q, want %q", state.LastError, "first error")
	}

	m.SetLastError("task-1", "second error")
	state = m.GetState("task-1")
	if state.LastError != "second error" {
		t.Errorf("LastError = %q, want %q", state.LastError, "second error")
	}

	// Non-existent task should not panic
	m.SetLastError("nonexistent", "some error")
}

func TestGetFailedTasks(t *testing.T) {
	m := NewManager()

	// Empty manager
	failed := m.GetFailedTasks()
	if len(failed) != 0 {
		t.Errorf("GetFailedTasks() on empty manager = %v, want empty", failed)
	}

	// Create various states
	m.GetOrCreateState("task-success", 3)
	m.RecordAttempt("task-success", true)

	m.GetOrCreateState("task-failed", 2)
	m.RecordAttempt("task-failed", false)
	m.RecordAttempt("task-failed", false)

	m.GetOrCreateState("task-retrying", 3)
	m.RecordAttempt("task-retrying", false)

	m.GetOrCreateState("task-new", 3)

	failed = m.GetFailedTasks()
	if len(failed) != 1 {
		t.Errorf("GetFailedTasks() length = %d, want 1", len(failed))
	}
	if len(failed) > 0 && failed[0] != "task-failed" {
		t.Errorf("GetFailedTasks()[0] = %q, want %q", failed[0], "task-failed")
	}
}

func TestGetRetryingTasks(t *testing.T) {
	m := NewManager()

	// Empty manager
	retrying := m.GetRetryingTasks()
	if len(retrying) != 0 {
		t.Errorf("GetRetryingTasks() on empty manager = %v, want empty", retrying)
	}

	// Create various states
	m.GetOrCreateState("task-success", 3)
	m.RecordAttempt("task-success", true)

	m.GetOrCreateState("task-failed", 2)
	m.RecordAttempt("task-failed", false)
	m.RecordAttempt("task-failed", false)

	m.GetOrCreateState("task-retrying", 3)
	m.RecordAttempt("task-retrying", false)

	m.GetOrCreateState("task-new", 3)

	retrying = m.GetRetryingTasks()
	sort.Strings(retrying)

	expected := []string{"task-new", "task-retrying"}
	if len(retrying) != len(expected) {
		t.Errorf("GetRetryingTasks() length = %d, want %d", len(retrying), len(expected))
	}
	for i, v := range expected {
		if i < len(retrying) && retrying[i] != v {
			t.Errorf("GetRetryingTasks()[%d] = %q, want %q", i, retrying[i], v)
		}
	}
}

func TestReset(t *testing.T) {
	m := NewManager()
	m.GetOrCreateState("task-1", 3)
	m.RecordAttempt("task-1", false)

	m.Reset("task-1")

	state := m.GetState("task-1")
	if state != nil {
		t.Error("Reset() should remove task state")
	}

	// Reset non-existent should not panic
	m.Reset("nonexistent")
}

func TestResetAll(t *testing.T) {
	m := NewManager()
	m.GetOrCreateState("task-1", 3)
	m.GetOrCreateState("task-2", 3)
	m.GetOrCreateState("task-3", 3)

	m.ResetAll()

	states := m.GetAllStates()
	if len(states) != 0 {
		t.Errorf("ResetAll() should clear all states, got %d", len(states))
	}
}

func TestGetAllStates(t *testing.T) {
	m := NewManager()
	m.GetOrCreateState("task-1", 3)
	m.RecordAttempt("task-1", false)
	m.RecordCommitCount("task-1", 0)
	m.SetLastError("task-1", "error")

	m.GetOrCreateState("task-2", 5)
	m.RecordAttempt("task-2", true)

	states := m.GetAllStates()
	if len(states) != 2 {
		t.Fatalf("GetAllStates() length = %d, want 2", len(states))
	}

	// Verify task-1
	s1 := states["task-1"]
	if s1 == nil {
		t.Fatal("states[task-1] is nil")
	}
	if s1.TaskID != "task-1" {
		t.Errorf("task-1 TaskID = %q, want %q", s1.TaskID, "task-1")
	}
	if s1.RetryCount != 1 {
		t.Errorf("task-1 RetryCount = %d, want 1", s1.RetryCount)
	}
	if s1.LastError != "error" {
		t.Errorf("task-1 LastError = %q, want %q", s1.LastError, "error")
	}

	// Verify task-2
	s2 := states["task-2"]
	if s2 == nil {
		t.Fatal("states[task-2] is nil")
	}
	if !s2.Succeeded {
		t.Error("task-2 should be succeeded")
	}

	// Verify it's a copy (modifying returned state shouldn't affect manager)
	s1.RetryCount = 999
	original := m.GetState("task-1")
	if original.RetryCount == 999 {
		t.Error("GetAllStates() should return a copy, not the original")
	}
}

func TestLoadStates(t *testing.T) {
	m := NewManager()

	states := map[string]*TaskState{
		"task-1": {
			TaskID:       "task-1",
			RetryCount:   2,
			MaxRetries:   3,
			LastError:    "some error",
			CommitCounts: []int{0, 1},
			Succeeded:    false,
		},
		"task-2": {
			TaskID:     "task-2",
			RetryCount: 0,
			MaxRetries: 5,
			Succeeded:  true,
		},
	}

	m.LoadStates(states)

	loaded := m.GetAllStates()
	if len(loaded) != 2 {
		t.Fatalf("LoadStates() resulted in %d states, want 2", len(loaded))
	}

	// Verify task-1
	s1 := loaded["task-1"]
	if s1.RetryCount != 2 {
		t.Errorf("task-1 RetryCount = %d, want 2", s1.RetryCount)
	}
	if len(s1.CommitCounts) != 2 {
		t.Errorf("task-1 CommitCounts length = %d, want 2", len(s1.CommitCounts))
	}

	// Verify modification of original doesn't affect loaded
	states["task-1"].RetryCount = 999
	s1 = m.GetState("task-1")
	if s1.RetryCount == 999 {
		t.Error("LoadStates() should copy states, not reference them")
	}
}

func TestLoadStatesWithNilValues(t *testing.T) {
	m := NewManager()

	states := map[string]*TaskState{
		"task-1": {
			TaskID:     "task-1",
			MaxRetries: 3,
		},
		"task-nil": nil, // nil value should be handled gracefully
	}

	m.LoadStates(states)

	loaded := m.GetAllStates()
	if len(loaded) != 1 {
		t.Errorf("LoadStates() with nil should result in 1 state, got %d", len(loaded))
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			taskID := "task"

			for j := range numOperations {
				// Mix of operations
				m.GetOrCreateState(taskID, 10)
				m.ShouldRetry(taskID)
				m.RecordAttempt(taskID, j%2 == 0)
				m.RecordCommitCount(taskID, j)
				m.SetLastError(taskID, "error")
				m.GetState(taskID)
				m.GetAllStates()
				m.GetFailedTasks()
				m.GetRetryingTasks()
			}
		}(i)
	}

	wg.Wait()

	// Should complete without data race (run with -race flag)
	state := m.GetState("task")
	if state == nil {
		t.Error("state should exist after concurrent operations")
	}
}

func TestConcurrentResetAndAccess(t *testing.T) {
	m := NewManager()
	const numGoroutines = 5
	const numOperations = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Half doing operations, half doing resets

	// Goroutines doing normal operations
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := range numOperations {
				m.GetOrCreateState("task-1", 5)
				m.RecordAttempt("task-1", false)
				m.ShouldRetry("task-1")
				_ = j // use j to avoid unused variable warning
			}
		}()
	}

	// Goroutines doing resets
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := range numOperations {
				if j%10 == 0 {
					m.Reset("task-1")
				}
				if j%50 == 0 {
					m.ResetAll()
				}
			}
		}()
	}

	wg.Wait()
	// Should complete without panic or race condition
}

func TestRetryWorkflow(t *testing.T) {
	// Simulate a realistic retry workflow
	m := NewManager()
	taskID := "build-task"
	maxRetries := 3

	// First attempt: task fails (no commits)
	state := m.GetOrCreateState(taskID, maxRetries)
	m.RecordCommitCount(taskID, 0)

	if !m.ShouldRetry(taskID) {
		t.Error("should retry after first failure")
	}
	m.RecordAttempt(taskID, false)
	m.SetLastError(taskID, "task produced no commits")

	if state.RetryCount != 1 {
		t.Errorf("RetryCount = %d after first failure, want 1", state.RetryCount)
	}

	// Second attempt: task fails again
	m.RecordCommitCount(taskID, 0)
	if !m.ShouldRetry(taskID) {
		t.Error("should retry after second failure")
	}
	m.RecordAttempt(taskID, false)
	m.SetLastError(taskID, "task produced no commits again")

	if state.RetryCount != 2 {
		t.Errorf("RetryCount = %d after second failure, want 2", state.RetryCount)
	}

	// Third attempt: task succeeds!
	m.RecordCommitCount(taskID, 2)
	if !m.ShouldRetry(taskID) {
		t.Error("should still be eligible for retry before recording success")
	}
	m.RecordAttempt(taskID, true)

	if !state.Succeeded {
		t.Error("task should be marked as succeeded")
	}
	if m.ShouldRetry(taskID) {
		t.Error("should not retry succeeded task")
	}

	// Verify commit counts tracked
	if len(state.CommitCounts) != 3 {
		t.Errorf("CommitCounts length = %d, want 3", len(state.CommitCounts))
	}
	expectedCounts := []int{0, 0, 2}
	for i, v := range expectedCounts {
		if state.CommitCounts[i] != v {
			t.Errorf("CommitCounts[%d] = %d, want %d", i, state.CommitCounts[i], v)
		}
	}
}

func TestMaxRetriesExhausted(t *testing.T) {
	m := NewManager()
	taskID := "failing-task"
	maxRetries := 2

	m.GetOrCreateState(taskID, maxRetries)

	// First failure
	m.RecordAttempt(taskID, false)
	if !m.ShouldRetry(taskID) {
		t.Error("should retry after first failure")
	}

	// Second failure (exhausts retries)
	m.RecordAttempt(taskID, false)
	if m.ShouldRetry(taskID) {
		t.Error("should not retry after exhausting max retries")
	}

	// Verify task is in failed tasks
	failed := m.GetFailedTasks()
	if len(failed) != 1 || failed[0] != taskID {
		t.Errorf("GetFailedTasks() = %v, want [%s]", failed, taskID)
	}
}

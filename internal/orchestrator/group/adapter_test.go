package group

import (
	"testing"
)

func TestNewSessionAdapter(t *testing.T) {
	callCounts := make(map[string]int)

	adapter := NewSessionAdapter(
		func() PlanData {
			callCounts["getPlan"]++
			return nil
		},
		func() []string {
			callCounts["getCompletedTasks"]++
			return []string{"task-1"}
		},
		func() []string {
			callCounts["getFailedTasks"]++
			return []string{"task-2"}
		},
		func() map[string]int {
			callCounts["getTaskCommitCounts"]++
			return map[string]int{"task-1": 2}
		},
		func() int {
			callCounts["getCurrentGroup"]++
			return 3
		},
	)

	if adapter == nil {
		t.Fatal("NewSessionAdapter returned nil")
	}

	// Call each method and verify it delegates correctly
	_ = adapter.GetPlan()
	if callCounts["getPlan"] != 1 {
		t.Error("GetPlan did not call getPlan function")
	}

	completed := adapter.GetCompletedTasks()
	if callCounts["getCompletedTasks"] != 1 {
		t.Error("GetCompletedTasks did not call getCompletedTasks function")
	}
	if len(completed) != 1 || completed[0] != "task-1" {
		t.Errorf("GetCompletedTasks returned %v, want [task-1]", completed)
	}

	failed := adapter.GetFailedTasks()
	if callCounts["getFailedTasks"] != 1 {
		t.Error("GetFailedTasks did not call getFailedTasks function")
	}
	if len(failed) != 1 || failed[0] != "task-2" {
		t.Errorf("GetFailedTasks returned %v, want [task-2]", failed)
	}

	commitCounts := adapter.GetTaskCommitCounts()
	if callCounts["getTaskCommitCounts"] != 1 {
		t.Error("GetTaskCommitCounts did not call getTaskCommitCounts function")
	}
	if commitCounts["task-1"] != 2 {
		t.Errorf("GetTaskCommitCounts returned %v, want map[task-1:2]", commitCounts)
	}

	currentGroup := adapter.GetCurrentGroup()
	if callCounts["getCurrentGroup"] != 1 {
		t.Error("GetCurrentGroup did not call getCurrentGroup function")
	}
	if currentGroup != 3 {
		t.Errorf("GetCurrentGroup returned %d, want 3", currentGroup)
	}
}

func TestSessionAdapter_NilFunctions(t *testing.T) {
	// Create adapter with nil functions
	adapter := NewSessionAdapter(nil, nil, nil, nil, nil)

	// All methods should return nil/zero values without panicking
	t.Run("GetPlan with nil function", func(t *testing.T) {
		result := adapter.GetPlan()
		if result != nil {
			t.Errorf("GetPlan with nil function = %v, want nil", result)
		}
	})

	t.Run("GetCompletedTasks with nil function", func(t *testing.T) {
		result := adapter.GetCompletedTasks()
		if result != nil {
			t.Errorf("GetCompletedTasks with nil function = %v, want nil", result)
		}
	})

	t.Run("GetFailedTasks with nil function", func(t *testing.T) {
		result := adapter.GetFailedTasks()
		if result != nil {
			t.Errorf("GetFailedTasks with nil function = %v, want nil", result)
		}
	})

	t.Run("GetTaskCommitCounts with nil function", func(t *testing.T) {
		result := adapter.GetTaskCommitCounts()
		if result != nil {
			t.Errorf("GetTaskCommitCounts with nil function = %v, want nil", result)
		}
	})

	t.Run("GetCurrentGroup with nil function", func(t *testing.T) {
		result := adapter.GetCurrentGroup()
		if result != 0 {
			t.Errorf("GetCurrentGroup with nil function = %d, want 0", result)
		}
	})
}

func TestNewPlanAdapter(t *testing.T) {
	callCounts := make(map[string]int)
	testTask := &Task{
		ID:    "task-1",
		Title: "Test Task",
	}

	adapter := NewPlanAdapter(
		func() [][]string {
			callCounts["getExecutionOrder"]++
			return [][]string{{"task-1", "task-2"}, {"task-3"}}
		},
		func(taskID string) *Task {
			callCounts["getTask"]++
			if taskID == "task-1" {
				return testTask
			}
			return nil
		},
	)

	if adapter == nil {
		t.Fatal("NewPlanAdapter returned nil")
	}

	// Test GetExecutionOrder
	order := adapter.GetExecutionOrder()
	if callCounts["getExecutionOrder"] != 1 {
		t.Error("GetExecutionOrder did not call getExecutionOrder function")
	}
	if len(order) != 2 {
		t.Errorf("GetExecutionOrder returned %d groups, want 2", len(order))
	}
	if len(order[0]) != 2 {
		t.Errorf("GetExecutionOrder group 0 has %d tasks, want 2", len(order[0]))
	}

	// Test GetTask with existing task
	task := adapter.GetTask("task-1")
	if callCounts["getTask"] != 1 {
		t.Error("GetTask did not call getTask function")
	}
	if task != testTask {
		t.Error("GetTask did not return expected task")
	}

	// Test GetTask with non-existing task
	task = adapter.GetTask("task-99")
	if callCounts["getTask"] != 2 {
		t.Error("GetTask did not call getTask function for second call")
	}
	if task != nil {
		t.Error("GetTask should return nil for non-existing task")
	}
}

func TestPlanAdapter_NilFunctions(t *testing.T) {
	// Create adapter with nil functions
	adapter := NewPlanAdapter(nil, nil)

	t.Run("GetExecutionOrder with nil function", func(t *testing.T) {
		result := adapter.GetExecutionOrder()
		if result != nil {
			t.Errorf("GetExecutionOrder with nil function = %v, want nil", result)
		}
	})

	t.Run("GetTask with nil function", func(t *testing.T) {
		result := adapter.GetTask("any-task")
		if result != nil {
			t.Errorf("GetTask with nil function = %v, want nil", result)
		}
	})
}

func TestSessionAdapterImplementsSessionData(t *testing.T) {
	// Compile-time check that SessionAdapter implements SessionData
	var _ SessionData = (*SessionAdapter)(nil)
}

func TestPlanAdapterImplementsPlanData(t *testing.T) {
	// Compile-time check that PlanAdapter implements PlanData
	var _ PlanData = (*PlanAdapter)(nil)
}

func TestAdapterIntegration(t *testing.T) {
	// Test that adapters can be used together with Tracker
	planAdapter := NewPlanAdapter(
		func() [][]string {
			return [][]string{{"task-1", "task-2"}, {"task-3"}}
		},
		func(taskID string) *Task {
			return &Task{ID: taskID, Title: "Task " + taskID}
		},
	)

	sessionAdapter := NewSessionAdapter(
		func() PlanData {
			return planAdapter
		},
		func() []string {
			return []string{"task-1"}
		},
		func() []string {
			return []string{"task-2"}
		},
		func() map[string]int {
			return map[string]int{"task-1": 1}
		},
		func() int {
			return 0
		},
	)

	// Create a tracker using the adapters
	tracker := NewTracker(sessionAdapter)

	// Verify the tracker works with the adapters
	groupIdx := tracker.GetTaskGroupIndex("task-1")
	if groupIdx != 0 {
		t.Errorf("GetTaskGroupIndex(task-1) = %d, want 0", groupIdx)
	}

	isComplete := tracker.IsGroupComplete(0)
	if !isComplete {
		t.Error("Group 0 should be complete (task-1 completed, task-2 failed)")
	}

	hasPartial := tracker.HasPartialFailure(0)
	if !hasPartial {
		t.Error("Group 0 should have partial failure (task-1 success, task-2 failure)")
	}
}

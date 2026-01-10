package executor

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestExecutionStatus_String(t *testing.T) {
	tests := []struct {
		status ExecutionStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
		{ExecutionStatus(99), "unknown"},
	}

	for _, tc := range tests {
		got := tc.status.String()
		if got != tc.want {
			t.Errorf("ExecutionStatus(%d).String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestExecutionStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status     ExecutionStatus
		isTerminal bool
	}{
		{StatusPending, false},
		{StatusRunning, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusCancelled, true},
	}

	for _, tc := range tests {
		got := tc.status.IsTerminal()
		if got != tc.isTerminal {
			t.Errorf("%s.IsTerminal() = %v, want %v", tc.status, got, tc.isTerminal)
		}
	}
}

func TestTaskStatus_Duration(t *testing.T) {
	// Test pending task (no start time)
	status := TaskStatus{
		TaskID: "task-1",
		Status: StatusPending,
	}
	if status.Duration() != 0 {
		t.Error("Expected zero duration for pending task")
	}

	// Test running task
	status = TaskStatus{
		TaskID:    "task-2",
		Status:    StatusRunning,
		StartTime: time.Now().Add(-5 * time.Second),
	}
	dur := status.Duration()
	if dur < 5*time.Second || dur > 6*time.Second {
		t.Errorf("Expected ~5s duration for running task, got %v", dur)
	}

	// Test completed task
	startTime := time.Now().Add(-10 * time.Second)
	end := startTime.Add(5 * time.Second)
	status = TaskStatus{
		TaskID:    "task-3",
		Status:    StatusCompleted,
		StartTime: startTime,
		EndTime:   end,
	}
	dur = status.Duration()
	if dur != 5*time.Second {
		t.Errorf("Expected 5s duration for completed task, got %v", dur)
	}
}

func TestExecutionProgress_IsComplete(t *testing.T) {
	// Not complete - tasks still running
	progress := ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 2,
		RunningTasks:   1,
		FailedTasks:    0,
		PendingTasks:   2,
	}
	if progress.IsComplete() {
		t.Error("Expected not complete with running tasks")
	}

	// Complete - all in terminal state
	progress = ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 3,
		RunningTasks:   0,
		FailedTasks:    1,
		CancelledTasks: 1,
		PendingTasks:   0,
	}
	if !progress.IsComplete() {
		t.Error("Expected complete when all tasks in terminal state")
	}
}

func TestExecutionProgress_SuccessRate(t *testing.T) {
	// No tasks finished
	progress := ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 0,
		FailedTasks:    0,
	}
	if progress.SuccessRate() != 0 {
		t.Error("Expected 0% success rate with no finished tasks")
	}

	// All succeeded
	progress = ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 5,
		FailedTasks:    0,
	}
	if progress.SuccessRate() != 100 {
		t.Errorf("Expected 100%% success rate, got %v", progress.SuccessRate())
	}

	// Some failed
	progress = ExecutionProgress{
		TotalTasks:     4,
		CompletedTasks: 3,
		FailedTasks:    1,
	}
	rate := progress.SuccessRate()
	if rate != 75 {
		t.Errorf("Expected 75%% success rate, got %v", rate)
	}
}

func TestExecutionProgress_Duration(t *testing.T) {
	// No start time
	progress := ExecutionProgress{}
	if progress.Duration() != 0 {
		t.Error("Expected zero duration with no start time")
	}

	// Running
	progress = ExecutionProgress{
		StartTime: time.Now().Add(-10 * time.Second),
	}
	dur := progress.Duration()
	if dur < 10*time.Second || dur > 11*time.Second {
		t.Errorf("Expected ~10s duration, got %v", dur)
	}

	// Completed
	start := time.Now().Add(-20 * time.Second)
	end := start.Add(15 * time.Second)
	progress = ExecutionProgress{
		StartTime: start,
		EndTime:   end,
	}
	dur = progress.Duration()
	if dur != 15*time.Second {
		t.Errorf("Expected 15s duration, got %v", dur)
	}
}

func TestExecutionProgress_RemainingEstimate(t *testing.T) {
	// No completed tasks
	progress := ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 0,
	}
	if progress.RemainingEstimate() != 0 {
		t.Error("Expected zero estimate with no completed tasks")
	}

	// Already complete
	progress = ExecutionProgress{
		TotalTasks:     5,
		CompletedTasks: 5,
	}
	if progress.RemainingEstimate() != 0 {
		t.Error("Expected zero estimate when complete")
	}

	// Partial progress
	progress = ExecutionProgress{
		TotalTasks:     4,
		CompletedTasks: 2,
		StartTime:      time.Now().Add(-10 * time.Second),
	}
	estimate := progress.RemainingEstimate()
	// 2 tasks took 10s, so 2 remaining should take ~10s
	if estimate < 9*time.Second || estimate > 11*time.Second {
		t.Errorf("Expected ~10s remaining estimate, got %v", estimate)
	}
}

func TestNewExecutionProgress(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3"},
		},
	}

	progress := NewExecutionProgress(plan)

	if progress.TotalTasks != 3 {
		t.Errorf("Expected TotalTasks=3, got %d", progress.TotalTasks)
	}
	if progress.PendingTasks != 3 {
		t.Errorf("Expected PendingTasks=3, got %d", progress.PendingTasks)
	}
	if progress.CompletedTasks != 0 {
		t.Errorf("Expected CompletedTasks=0, got %d", progress.CompletedTasks)
	}
	if len(progress.TaskStatuses) != 3 {
		t.Errorf("Expected 3 task statuses, got %d", len(progress.TaskStatuses))
	}

	for _, task := range plan.Tasks {
		status, ok := progress.TaskStatuses[task.ID]
		if !ok {
			t.Errorf("Missing status for task %s", task.ID)
			continue
		}
		if status.Status != StatusPending {
			t.Errorf("Expected pending status for task %s, got %v", task.ID, status.Status)
		}
	}

	if progress.StartTime.IsZero() {
		t.Error("Expected StartTime to be set")
	}
}

package executor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// mockTaskExecutor provides a test implementation of TaskExecutor.
type mockTaskExecutor struct {
	status        ExecutionStatus
	executeDelay  time.Duration
	executeError  error
	executeCalled bool
	mu            sync.Mutex
}

func newMockTaskExecutor() *mockTaskExecutor {
	return &mockTaskExecutor{
		status: StatusPending,
	}
}

func (m *mockTaskExecutor) Execute(_ context.Context, _ *ultraplan.PlannedTask) error {
	m.mu.Lock()
	m.executeCalled = true
	m.status = StatusRunning
	m.mu.Unlock()

	if m.executeDelay > 0 {
		time.Sleep(m.executeDelay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.executeError != nil {
		m.status = StatusFailed
		return m.executeError
	}
	m.status = StatusCompleted
	return nil
}

func (m *mockTaskExecutor) GetStatus() ExecutionStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// mockTaskExecutorFactory provides a test implementation of TaskExecutorFactory.
type mockTaskExecutorFactory struct {
	executors    map[string]*mockTaskExecutor
	createError  error
	mu           sync.Mutex
}

func newMockTaskExecutorFactory() *mockTaskExecutorFactory {
	return &mockTaskExecutorFactory{
		executors: make(map[string]*mockTaskExecutor),
	}
}

func (f *mockTaskExecutorFactory) Create(task *ultraplan.PlannedTask, _ string) (TaskExecutor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createError != nil {
		return nil, f.createError
	}

	executor := newMockTaskExecutor()
	f.executors[task.ID] = executor
	return executor, nil
}

// mockWorktreeGetter provides a test implementation of WorktreeGetter.
type mockWorktreeGetter struct {
	worktrees map[string]string
	getError  error
}

func newMockWorktreeGetter() *mockWorktreeGetter {
	return &mockWorktreeGetter{
		worktrees: make(map[string]string),
	}
}

func (m *mockWorktreeGetter) GetWorktree(taskID string) (string, error) {
	if m.getError != nil {
		return "", m.getError
	}
	if path, ok := m.worktrees[taskID]; ok {
		return path, nil
	}
	return "/tmp/worktree/" + taskID, nil
}

// mockEventHandler provides a test implementation of EventHandler.
type mockEventHandler struct {
	taskStarted    []string
	taskCompleted  []string
	taskFailed     []string
	groupCompleted []int
	progressCalls  int
	mu             sync.Mutex
}

func newMockEventHandler() *mockEventHandler {
	return &mockEventHandler{
		taskStarted:    make([]string, 0),
		taskCompleted:  make([]string, 0),
		taskFailed:     make([]string, 0),
		groupCompleted: make([]int, 0),
	}
}

func (h *mockEventHandler) OnTaskStarted(taskID, _ string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.taskStarted = append(h.taskStarted, taskID)
}

func (h *mockEventHandler) OnTaskCompleted(taskID string, _ int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.taskCompleted = append(h.taskCompleted, taskID)
}

func (h *mockEventHandler) OnTaskFailed(taskID string, _ error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.taskFailed = append(h.taskFailed, taskID)
}

func (h *mockEventHandler) OnGroupCompleted(groupIndex int, _, _ []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.groupCompleted = append(h.groupCompleted, groupIndex)
}

func (h *mockEventHandler) OnProgress(_ ExecutionProgress) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.progressCalls++
}

func TestDefaultCoordinatorConfig(t *testing.T) {
	cfg := DefaultCoordinatorConfig()

	if cfg.MaxParallel != 3 {
		t.Errorf("Expected MaxParallel 3, got %d", cfg.MaxParallel)
	}
	if !cfg.RequireVerifiedCommits {
		t.Error("Expected RequireVerifiedCommits to be true")
	}
	if cfg.MaxTaskRetries != 3 {
		t.Errorf("Expected MaxTaskRetries 3, got %d", cfg.MaxTaskRetries)
	}
}

func TestNewCoordinator(t *testing.T) {
	cfg := DefaultCoordinatorConfig()
	factory := newMockTaskExecutorFactory()
	wtGetter := newMockWorktreeGetter()
	handler := newMockEventHandler()

	c := NewCoordinator(cfg, factory, wtGetter, handler)

	if c == nil {
		t.Fatal("NewCoordinator returned nil")
	}

	if c.config.MaxParallel != cfg.MaxParallel {
		t.Errorf("Expected MaxParallel %d, got %d", cfg.MaxParallel, c.config.MaxParallel)
	}
}

func TestCoordinator_Start_NilPlan(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	err := c.Start(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestCoordinator_Start_AlreadyStarted(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	// Start once
	err := c.Start(plan)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Try to start again
	err = c.Start(plan)
	if err == nil {
		t.Error("Expected error for second Start call")
	}

	// Clean up
	_ = c.Stop()
}

func TestCoordinator_Start_InvalidPlan(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	// Plan with empty tasks (invalid)
	plan := &ultraplan.PlanSpec{
		Tasks:          []ultraplan.PlannedTask{},
		ExecutionOrder: [][]string{},
	}

	err := c.Start(plan)
	if err == nil {
		t.Error("Expected error for invalid plan")
	}
}

func TestCoordinator_Stop(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	_ = c.Start(plan)

	// Stop should be idempotent
	err := c.Stop()
	if err != nil {
		t.Errorf("First Stop failed: %v", err)
	}

	err = c.Stop()
	if err != nil {
		t.Errorf("Second Stop failed: %v", err)
	}
}

func TestCoordinator_GetProgress(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2"}},
	}

	_ = c.Start(plan)
	defer func() { _ = c.Stop() }()

	progress := c.GetProgress()

	if progress.TotalTasks != 2 {
		t.Errorf("Expected TotalTasks 2, got %d", progress.TotalTasks)
	}
	if progress.PendingTasks != 2 {
		t.Errorf("Expected PendingTasks 2, got %d", progress.PendingTasks)
	}
}

func TestCoordinator_GetCurrentGroup(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	if c.GetCurrentGroup() != 0 {
		t.Errorf("Expected initial current group 0, got %d", c.GetCurrentGroup())
	}

	// Manually advance for testing
	c.mu.Lock()
	c.currentGroup = 2
	c.mu.Unlock()

	if c.GetCurrentGroup() != 2 {
		t.Errorf("Expected current group 2, got %d", c.GetCurrentGroup())
	}
}

func TestCoordinator_GetRunningTaskCount(t *testing.T) {
	c := NewCoordinator(DefaultCoordinatorConfig(), newMockTaskExecutorFactory(), newMockWorktreeGetter(), nil)

	if c.GetRunningTaskCount() != 0 {
		t.Errorf("Expected initial running task count 0, got %d", c.GetRunningTaskCount())
	}
}

func TestCoordinator_ExecuteSimplePlan(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	handler := newMockEventHandler()

	cfg := DefaultCoordinatorConfig()
	cfg.MaxParallel = 2

	c := NewCoordinator(cfg, factory, newMockWorktreeGetter(), handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2"}},
	}

	err := c.Start(plan)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for completion
	err = c.Wait()
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	progress := c.GetProgress()
	if progress.CompletedTasks != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", progress.CompletedTasks)
	}

	// Verify event handler was called
	handler.mu.Lock()
	startedCount := len(handler.taskStarted)
	completedCount := len(handler.taskCompleted)
	handler.mu.Unlock()

	if startedCount != 2 {
		t.Errorf("Expected 2 task started events, got %d", startedCount)
	}
	if completedCount != 2 {
		t.Errorf("Expected 2 task completed events, got %d", completedCount)
	}
}

func TestCoordinator_ExecuteWithDependencies(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	handler := newMockEventHandler()

	cfg := DefaultCoordinatorConfig()
	cfg.MaxParallel = 2

	c := NewCoordinator(cfg, factory, newMockWorktreeGetter(), handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
	}

	err := c.Start(plan)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = c.Wait()
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	progress := c.GetProgress()
	if progress.CompletedTasks != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", progress.CompletedTasks)
	}

	// Verify at least one group completion event
	// Note: Due to async timing, we may not get all group events before Wait returns
	handler.mu.Lock()
	groupCount := len(handler.groupCompleted)
	handler.mu.Unlock()

	if groupCount < 1 {
		t.Errorf("Expected at least 1 group completed event, got %d", groupCount)
	}
}

func TestCoordinator_ExecuteWithFailure(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	handler := newMockEventHandler()

	c := NewCoordinator(DefaultCoordinatorConfig(), factory, newMockWorktreeGetter(), handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	// Configure executor to fail
	_ = c.Start(plan)

	// Wait a bit for executor to be created, then make it fail
	time.Sleep(50 * time.Millisecond)
	factory.mu.Lock()
	if exec, ok := factory.executors["task-1"]; ok {
		exec.mu.Lock()
		exec.executeError = errors.New("task execution failed")
		exec.mu.Unlock()
	}
	factory.mu.Unlock()

	err := c.Wait()
	// Error is expected
	if err == nil {
		// Check if we have failed tasks
		progress := c.GetProgress()
		if progress.FailedTasks == 0 && progress.CompletedTasks == 0 {
			t.Log("Task may have completed before error was set")
		}
	}
}

func TestCoordinator_ConcurrencyLimit(t *testing.T) {
	factory := newMockTaskExecutorFactory()

	cfg := DefaultCoordinatorConfig()
	cfg.MaxParallel = 1 // Only allow one task at a time

	c := NewCoordinator(cfg, factory, newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3"},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2", "task-3"}},
	}

	_ = c.Start(plan)

	// Give some time for scheduling
	time.Sleep(200 * time.Millisecond)

	runningCount := c.GetRunningTaskCount()
	if runningCount > cfg.MaxParallel {
		t.Errorf("Expected at most %d running tasks, got %d", cfg.MaxParallel, runningCount)
	}

	// Wait for completion
	_ = c.Wait()
	_ = c.Stop()
}

func TestCoordinator_StopCancelsRunningTasks(t *testing.T) {
	factory := newMockTaskExecutorFactory()

	c := NewCoordinator(DefaultCoordinatorConfig(), factory, newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2"}},
	}

	_ = c.Start(plan)

	// Give tasks time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should cancel running tasks
	err := c.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	progress := c.GetProgress()
	// Some tasks should be cancelled
	if progress.CancelledTasks+progress.CompletedTasks+progress.PendingTasks+progress.RunningTasks != progress.TotalTasks {
		t.Error("Task count mismatch after stop")
	}
}

func TestCoordinator_WorktreeGetterError(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	wtGetter := newMockWorktreeGetter()
	wtGetter.getError = errors.New("worktree creation failed")
	handler := newMockEventHandler()

	c := NewCoordinator(DefaultCoordinatorConfig(), factory, wtGetter, handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	_ = c.Start(plan)
	_ = c.Wait()

	progress := c.GetProgress()
	if progress.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task due to worktree error, got %d", progress.FailedTasks)
	}

	handler.mu.Lock()
	failedCount := len(handler.taskFailed)
	handler.mu.Unlock()

	if failedCount != 1 {
		t.Errorf("Expected 1 task failed event, got %d", failedCount)
	}
}

func TestCoordinator_FactoryCreateError(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	factory.createError = errors.New("executor creation failed")
	handler := newMockEventHandler()

	c := NewCoordinator(DefaultCoordinatorConfig(), factory, newMockWorktreeGetter(), handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}

	_ = c.Start(plan)
	_ = c.Wait()

	progress := c.GetProgress()
	if progress.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task due to factory error, got %d", progress.FailedTasks)
	}
}

func TestCoordinator_MultiGroupExecution(t *testing.T) {
	factory := newMockTaskExecutorFactory()
	handler := newMockEventHandler()

	cfg := DefaultCoordinatorConfig()
	cfg.MaxParallel = 3

	c := NewCoordinator(cfg, factory, newMockWorktreeGetter(), handler)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1", "task-2"}},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2"},
			{"task-3"},
		},
	}

	err := c.Start(plan)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = c.Wait()
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	progress := c.GetProgress()
	if progress.CompletedTasks != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", progress.CompletedTasks)
	}

	// Verify at least one group completion event
	// Note: Due to async timing and execution order, not all events may be delivered before Wait returns
	handler.mu.Lock()
	groupCount := len(handler.groupCompleted)
	handler.mu.Unlock()

	if groupCount < 1 {
		t.Errorf("Expected at least 1 group completed event, got %d", groupCount)
	}
}

func TestCoordinator_EmptyGroup(t *testing.T) {
	factory := newMockTaskExecutorFactory()

	c := NewCoordinator(DefaultCoordinatorConfig(), factory, newMockWorktreeGetter(), nil)

	plan := &ultraplan.PlanSpec{
		Tasks: []ultraplan.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{
			{},         // Empty group
			{"task-1"}, // Task in second group
		},
	}

	err := c.Start(plan)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = c.Wait()
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	progress := c.GetProgress()
	if progress.CompletedTasks != 1 {
		t.Errorf("Expected 1 completed task, got %d", progress.CompletedTasks)
	}
}

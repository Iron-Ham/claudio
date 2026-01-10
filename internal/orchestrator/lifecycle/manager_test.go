package lifecycle

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInstanceStatus_Constants(t *testing.T) {
	tests := []struct {
		status InstanceStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusWorking, "working"},
		{StatusWaitingInput, "waiting_input"},
		{StatusPaused, "paused"},
		{StatusCompleted, "completed"},
		{StatusError, "error"},
		{StatusCreatingPR, "creating_pr"},
		{StatusStuck, "stuck"},
		{StatusTimeout, "timeout"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.want {
			t.Errorf("InstanceStatus %q != %q", tc.status, tc.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TmuxSessionPrefix != "claudio" {
		t.Errorf("Expected TmuxSessionPrefix %q, got %q", "claudio", cfg.TmuxSessionPrefix)
	}
	if cfg.DefaultTermWidth != 200 {
		t.Errorf("Expected DefaultTermWidth 200, got %d", cfg.DefaultTermWidth)
	}
	if cfg.DefaultTermHeight != 30 {
		t.Errorf("Expected DefaultTermHeight 30, got %d", cfg.DefaultTermHeight)
	}
	if cfg.ActivityTimeout != 5*time.Minute {
		t.Errorf("Expected ActivityTimeout 5m, got %v", cfg.ActivityTimeout)
	}
	if cfg.CompletionTimeout != 30*time.Second {
		t.Errorf("Expected CompletionTimeout 30s, got %v", cfg.CompletionTimeout)
	}
}

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	callbacks := Callbacks{}

	m := NewManager(cfg, callbacks, nil)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.config.TmuxSessionPrefix != cfg.TmuxSessionPrefix {
		t.Errorf("Expected TmuxSessionPrefix %q, got %q", cfg.TmuxSessionPrefix, m.config.TmuxSessionPrefix)
	}

	if m.instances == nil {
		t.Error("Expected instances map to be initialized")
	}

	if m.stopChan == nil {
		t.Error("Expected stopChan to be initialized")
	}
}

func TestManager_CreateInstance(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	inst, err := m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if inst == nil {
		t.Fatal("CreateInstance returned nil instance")
	}

	if inst.ID != "test-1" {
		t.Errorf("Expected ID %q, got %q", "test-1", inst.ID)
	}
	if inst.WorktreePath != "/tmp/worktree" {
		t.Errorf("Expected WorktreePath %q, got %q", "/tmp/worktree", inst.WorktreePath)
	}
	if inst.Branch != "feature/test" {
		t.Errorf("Expected Branch %q, got %q", "feature/test", inst.Branch)
	}
	if inst.Task != "Test task" {
		t.Errorf("Expected Task %q, got %q", "Test task", inst.Task)
	}
	if inst.Status != StatusPending {
		t.Errorf("Expected Status %q, got %q", StatusPending, inst.Status)
	}
	if inst.TmuxSession == "" {
		t.Error("Expected TmuxSession to be set")
	}
}

func TestManager_CreateInstance_Duplicate(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, err := m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")
	if err != nil {
		t.Fatalf("First CreateInstance failed: %v", err)
	}

	_, err = m.CreateInstance("test-1", "/tmp/worktree2", "feature/test2", "Test task 2")
	if err == nil {
		t.Error("Expected error for duplicate instance ID")
	}
}

func TestManager_GetInstance(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	inst, exists := m.GetInstance("test-1")
	if !exists {
		t.Error("Expected instance to exist")
	}
	if inst == nil {
		t.Fatal("GetInstance returned nil")
	}
	if inst.ID != "test-1" {
		t.Errorf("Expected ID %q, got %q", "test-1", inst.ID)
	}

	// Test non-existent instance
	_, exists = m.GetInstance("nonexistent")
	if exists {
		t.Error("Expected instance not to exist")
	}
}

func TestManager_GetInstance_ReturnsCopy(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	inst1, _ := m.GetInstance("test-1")
	inst1.Status = StatusCompleted // Modify the copy

	inst2, _ := m.GetInstance("test-1")
	if inst2.Status == StatusCompleted {
		t.Error("GetInstance should return a copy, not a reference")
	}
}

func TestManager_ListInstances(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	// Initially empty
	list := m.ListInstances()
	if len(list) != 0 {
		t.Errorf("Expected 0 instances, got %d", len(list))
	}

	_, _ = m.CreateInstance("test-1", "/tmp/wt1", "branch1", "Task 1")
	_, _ = m.CreateInstance("test-2", "/tmp/wt2", "branch2", "Task 2")
	_, _ = m.CreateInstance("test-3", "/tmp/wt3", "branch3", "Task 3")

	list = m.ListInstances()
	if len(list) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(list))
	}
}

func TestManager_StartInstance(t *testing.T) {
	var statusChangeCalled bool
	var oldStatus, newStatus InstanceStatus
	var mu sync.Mutex

	callbacks := Callbacks{
		OnStatusChange: func(id string, old, new InstanceStatus) {
			mu.Lock()
			statusChangeCalled = true
			oldStatus = old
			newStatus = new
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	err := m.StartInstance(context.Background(), "test-1")
	if err != nil {
		t.Fatalf("StartInstance failed: %v", err)
	}

	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusWorking {
		t.Errorf("Expected status %q, got %q", StatusWorking, inst.Status)
	}
	if inst.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}

	mu.Lock()
	called := statusChangeCalled
	old := oldStatus
	newSt := newStatus
	mu.Unlock()

	if !called {
		t.Error("Expected OnStatusChange callback to be called")
	}
	if old != StatusPending {
		t.Errorf("Expected old status %q, got %q", StatusPending, old)
	}
	if newSt != StatusWorking {
		t.Errorf("Expected new status %q, got %q", StatusWorking, newSt)
	}
}

func TestManager_StartInstance_NotFound(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	err := m.StartInstance(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestManager_StartInstance_NotPending(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")
	_ = m.StartInstance(context.Background(), "test-1")

	// Try to start again
	err := m.StartInstance(context.Background(), "test-1")
	if err == nil {
		t.Error("Expected error when starting non-pending instance")
	}
}

func TestManager_StopInstance(t *testing.T) {
	var statusChangeCalled bool
	var mu sync.Mutex

	callbacks := Callbacks{
		OnStatusChange: func(id string, old, new InstanceStatus) {
			mu.Lock()
			statusChangeCalled = true
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")
	_ = m.StartInstance(context.Background(), "test-1")

	err := m.StopInstance("test-1")
	if err != nil {
		t.Fatalf("StopInstance failed: %v", err)
	}

	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusCompleted {
		t.Errorf("Expected status %q, got %q", StatusCompleted, inst.Status)
	}
	if inst.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	mu.Lock()
	called := statusChangeCalled
	mu.Unlock()

	if !called {
		t.Error("Expected OnStatusChange callback to be called")
	}
}

func TestManager_StopInstance_NotFound(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	err := m.StopInstance("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestManager_UpdateStatus(t *testing.T) {
	var receivedID string
	var oldStatus, newStatus InstanceStatus
	var mu sync.Mutex

	callbacks := Callbacks{
		OnStatusChange: func(id string, old, new InstanceStatus) {
			mu.Lock()
			receivedID = id
			oldStatus = old
			newStatus = new
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	err := m.UpdateStatus("test-1", StatusWorking)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusWorking {
		t.Errorf("Expected status %q, got %q", StatusWorking, inst.Status)
	}

	mu.Lock()
	gotID := receivedID
	gotOld := oldStatus
	gotNew := newStatus
	mu.Unlock()

	if gotID != "test-1" {
		t.Errorf("Expected ID %q, got %q", "test-1", gotID)
	}
	if gotOld != StatusPending {
		t.Errorf("Expected old status %q, got %q", StatusPending, gotOld)
	}
	if gotNew != StatusWorking {
		t.Errorf("Expected new status %q, got %q", StatusWorking, gotNew)
	}
}

func TestManager_UpdateStatus_SameStatus(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	callbacks := Callbacks{
		OnStatusChange: func(id string, old, new InstanceStatus) {
			mu.Lock()
			callCount++
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	// Update to same status
	_ = m.UpdateStatus("test-1", StatusPending)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Error("OnStatusChange should not be called when status doesn't change")
	}
}

func TestManager_UpdateStatus_SetsCompletedAt(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	// Test each terminal status
	terminalStatuses := []InstanceStatus{StatusCompleted, StatusError, StatusTimeout}

	for i, status := range terminalStatuses {
		id := "test-" + string(rune('1'+i))
		_, _ = m.CreateInstance(id, "/tmp/wt", "branch", "task")
		_ = m.UpdateStatus(id, status)

		inst, _ := m.GetInstance(id)
		if inst.CompletedAt == nil {
			t.Errorf("Expected CompletedAt to be set for status %q", status)
		}
	}
}

func TestManager_UpdateStatus_NotFound(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	err := m.UpdateStatus("nonexistent", StatusWorking)
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestManager_RemoveInstance(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")

	err := m.RemoveInstance("test-1")
	if err != nil {
		t.Fatalf("RemoveInstance failed: %v", err)
	}

	_, exists := m.GetInstance("test-1")
	if exists {
		t.Error("Expected instance to be removed")
	}
}

func TestManager_RemoveInstance_NotFound(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	err := m.RemoveInstance("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestManager_GetRunningCount(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	if m.GetRunningCount() != 0 {
		t.Error("Expected 0 running instances initially")
	}

	// Create instances in various states
	_, _ = m.CreateInstance("test-1", "/tmp/wt1", "branch1", "Task 1")
	_, _ = m.CreateInstance("test-2", "/tmp/wt2", "branch2", "Task 2")
	_, _ = m.CreateInstance("test-3", "/tmp/wt3", "branch3", "Task 3")
	_, _ = m.CreateInstance("test-4", "/tmp/wt4", "branch4", "Task 4")

	_ = m.UpdateStatus("test-1", StatusWorking)
	_ = m.UpdateStatus("test-2", StatusWaitingInput)
	_ = m.UpdateStatus("test-3", StatusCompleted)
	// test-4 remains Pending

	count := m.GetRunningCount()
	if count != 2 {
		t.Errorf("Expected 2 running instances (working + waiting_input), got %d", count)
	}
}

func TestManager_Stop(t *testing.T) {
	var stopCount int
	var mu sync.Mutex

	callbacks := Callbacks{
		OnStatusChange: func(id string, old, new InstanceStatus) {
			mu.Lock()
			if new == StatusCompleted {
				stopCount++
			}
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	// Create some running instances
	_, _ = m.CreateInstance("test-1", "/tmp/wt1", "branch1", "Task 1")
	_, _ = m.CreateInstance("test-2", "/tmp/wt2", "branch2", "Task 2")
	_, _ = m.CreateInstance("test-3", "/tmp/wt3", "branch3", "Task 3")

	_ = m.UpdateStatus("test-1", StatusWorking)
	_ = m.UpdateStatus("test-2", StatusWaitingInput)
	// test-3 remains Pending

	// Stop manager
	m.Stop()

	mu.Lock()
	count := stopCount
	mu.Unlock()

	// 2 instances (working + waiting_input) should have been stopped
	// The completion callback increments stopCount for each
	if count < 2 {
		t.Errorf("Expected at least 2 instances to be stopped, got %d", count)
	}

	// Verify stopped flag
	m.mu.RLock()
	stopped := m.stopped
	m.mu.RUnlock()

	if !stopped {
		t.Error("Expected manager to be marked as stopped")
	}
}

func TestManager_Stop_Idempotent(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	// Stop twice should not panic
	m.Stop()
	m.Stop()

	m.mu.RLock()
	stopped := m.stopped
	m.mu.RUnlock()

	if !stopped {
		t.Error("Expected manager to be stopped")
	}
}

func TestInstance_Fields(t *testing.T) {
	now := time.Now()
	inst := Instance{
		ID:            "test-id",
		WorktreePath:  "/tmp/worktree",
		Branch:        "feature/test",
		Task:          "Test task description",
		Status:        StatusWorking,
		PID:           12345,
		FilesModified: []string{"file1.go", "file2.go"},
		TmuxSession:   "claudio-test",
		StartedAt:     &now,
		CompletedAt:   nil,
	}

	if inst.ID != "test-id" {
		t.Errorf("Expected ID %q, got %q", "test-id", inst.ID)
	}
	if inst.WorktreePath != "/tmp/worktree" {
		t.Errorf("Expected WorktreePath %q, got %q", "/tmp/worktree", inst.WorktreePath)
	}
	if inst.Status != StatusWorking {
		t.Errorf("Expected Status %q, got %q", StatusWorking, inst.Status)
	}
	if inst.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", inst.PID)
	}
	if len(inst.FilesModified) != 2 {
		t.Errorf("Expected 2 files modified, got %d", len(inst.FilesModified))
	}
	if inst.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}
	if inst.CompletedAt != nil {
		t.Error("Expected CompletedAt to be nil")
	}
}

func TestCallbacks_AllNil(t *testing.T) {
	// Test that manager works correctly with all nil callbacks
	callbacks := Callbacks{
		OnStatusChange: nil,
		OnPRComplete:   nil,
		OnTimeout:      nil,
		OnBell:         nil,
		OnError:        nil,
	}

	m := NewManager(DefaultConfig(), callbacks, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/worktree", "feature/test", "Test task")
	_ = m.StartInstance(context.Background(), "test-1")
	_ = m.UpdateStatus("test-1", StatusCompleted)

	// Should not panic
	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusCompleted {
		t.Errorf("Expected status %q, got %q", StatusCompleted, inst.Status)
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		TmuxSessionPrefix: "test-prefix",
		DefaultTermWidth:  120,
		DefaultTermHeight: 40,
		ActivityTimeout:   3 * time.Minute,
		CompletionTimeout: 20 * time.Second,
	}

	if cfg.TmuxSessionPrefix != "test-prefix" {
		t.Errorf("Expected TmuxSessionPrefix %q, got %q", "test-prefix", cfg.TmuxSessionPrefix)
	}
	if cfg.DefaultTermWidth != 120 {
		t.Errorf("Expected DefaultTermWidth 120, got %d", cfg.DefaultTermWidth)
	}
	if cfg.DefaultTermHeight != 40 {
		t.Errorf("Expected DefaultTermHeight 40, got %d", cfg.DefaultTermHeight)
	}
	if cfg.ActivityTimeout != 3*time.Minute {
		t.Errorf("Expected ActivityTimeout 3m, got %v", cfg.ActivityTimeout)
	}
	if cfg.CompletionTimeout != 20*time.Second {
		t.Errorf("Expected CompletionTimeout 20s, got %v", cfg.CompletionTimeout)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	var wg sync.WaitGroup

	// Create instances concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = m.CreateInstance(
				"test-"+string(rune('a'+id)),
				"/tmp/wt",
				"branch",
				"task",
			)
		}(i)
	}

	wg.Wait()

	// Verify all instances were created
	list := m.ListInstances()
	if len(list) != 10 {
		t.Errorf("Expected 10 instances, got %d", len(list))
	}
}

func TestManager_ConcurrentStatusUpdates(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	var wg sync.WaitGroup

	// Update status concurrently
	statuses := []InstanceStatus{
		StatusWorking,
		StatusWaitingInput,
		StatusPaused,
		StatusWorking,
		StatusCompleted,
	}

	for _, status := range statuses {
		wg.Add(1)
		go func(s InstanceStatus) {
			defer wg.Done()
			_ = m.UpdateStatus("test-1", s)
		}(status)
	}

	wg.Wait()

	// Should not panic, final status is indeterminate due to race
	_, exists := m.GetInstance("test-1")
	if !exists {
		t.Error("Instance should still exist after concurrent updates")
	}
}

func TestManager_TmuxSessionName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmuxSessionPrefix = "custom-prefix"

	m := NewManager(cfg, Callbacks{}, nil)

	inst, _ := m.CreateInstance("test-id", "/tmp/wt", "branch", "task")

	expected := "custom-prefix-test-id"
	if inst.TmuxSession != expected {
		t.Errorf("Expected TmuxSession %q, got %q", expected, inst.TmuxSession)
	}
}

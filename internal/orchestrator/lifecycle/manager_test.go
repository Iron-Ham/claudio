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

// -----------------------------------------------------------------------------
// Group-Aware Lifecycle Tests
// -----------------------------------------------------------------------------

// mockGroupProvider implements GroupProvider for testing.
type mockGroupProvider struct {
	groups         map[string]*GroupInfo
	instanceGroups map[string]string // instanceID -> groupID
	mu             sync.RWMutex
}

func newMockGroupProvider() *mockGroupProvider {
	return &mockGroupProvider{
		groups:         make(map[string]*GroupInfo),
		instanceGroups: make(map[string]string),
	}
}

func (p *mockGroupProvider) AddGroup(group *GroupInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.groups[group.ID] = group
	for _, instID := range group.InstanceIDs {
		p.instanceGroups[instID] = group.ID
	}
}

func (p *mockGroupProvider) GetGroupForInstance(instanceID string) *GroupInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	groupID, exists := p.instanceGroups[instanceID]
	if !exists {
		return nil
	}
	return p.groups[groupID]
}

func (p *mockGroupProvider) GetGroup(groupID string) *GroupInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.groups[groupID]
}

func (p *mockGroupProvider) GetAllGroups() []*GroupInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*GroupInfo, 0, len(p.groups))
	for _, g := range p.groups {
		result = append(result, g)
	}
	// Sort by execution order
	for i := 1; i < len(result); i++ {
		key := result[i]
		j := i - 1
		for j >= 0 && result[j].ExecutionOrder > key.ExecutionOrder {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = key
	}
	return result
}

func (p *mockGroupProvider) AreGroupDependenciesMet(groupID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	group := p.groups[groupID]
	if group == nil {
		return false
	}
	for _, depID := range group.DependsOn {
		dep := p.groups[depID]
		if dep == nil || dep.Phase != GroupPhaseCompleted {
			return false
		}
	}
	return true
}

func (p *mockGroupProvider) AdvanceGroupPhase(groupID string, phase GroupPhase) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if group := p.groups[groupID]; group != nil {
		group.Phase = phase
	}
}

func TestGroupPhase_Constants(t *testing.T) {
	tests := []struct {
		phase GroupPhase
		want  string
	}{
		{GroupPhasePending, "pending"},
		{GroupPhaseExecuting, "executing"},
		{GroupPhaseCompleted, "completed"},
		{GroupPhaseFailed, "failed"},
	}

	for _, tc := range tests {
		if string(tc.phase) != tc.want {
			t.Errorf("GroupPhase %q != %q", tc.phase, tc.want)
		}
	}
}

func TestGroupCompletionState_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		state    GroupCompletionState
		expected bool
	}{
		{
			name:     "all completed",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 3},
			expected: true,
		},
		{
			name:     "all failed",
			state:    GroupCompletionState{TotalCount: 3, FailedCount: 3},
			expected: true,
		},
		{
			name:     "mixed success/failure",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 2, FailedCount: 1},
			expected: true,
		},
		{
			name:     "some pending",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 2, PendingCount: 1},
			expected: false,
		},
		{
			name:     "some running",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 1, RunningCount: 2},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.IsComplete(); got != tc.expected {
				t.Errorf("IsComplete() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestGroupCompletionState_AllSucceeded(t *testing.T) {
	tests := []struct {
		name     string
		state    GroupCompletionState
		expected bool
	}{
		{
			name:     "all succeeded",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 3},
			expected: true,
		},
		{
			name:     "mixed success/failure",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 2, FailedCount: 1},
			expected: false,
		},
		{
			name:     "all failed",
			state:    GroupCompletionState{TotalCount: 3, FailedCount: 3},
			expected: false,
		},
		{
			name:     "some still pending",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 2, PendingCount: 1},
			expected: false,
		},
		{
			name:     "empty group",
			state:    GroupCompletionState{TotalCount: 0},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.AllSucceeded(); got != tc.expected {
				t.Errorf("AllSucceeded() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestGroupCompletionState_HasFailures(t *testing.T) {
	tests := []struct {
		name     string
		state    GroupCompletionState
		expected bool
	}{
		{
			name:     "no failures",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 3},
			expected: false,
		},
		{
			name:     "one failure",
			state:    GroupCompletionState{TotalCount: 3, SuccessCount: 2, FailedCount: 1},
			expected: true,
		},
		{
			name:     "all failed",
			state:    GroupCompletionState{TotalCount: 3, FailedCount: 3},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.HasFailures(); got != tc.expected {
				t.Errorf("HasFailures() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestManager_SetGroupProvider(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	m.SetGroupProvider(provider)

	// Provider should be set (verify by checking CanStartInstance behavior)
	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	// Without a group, should be able to start
	canStart, _ := m.CanStartInstance("test-1")
	if !canStart {
		t.Error("Expected to be able to start instance without group")
	}
}

func TestManager_CanStartInstance_NoProvider(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	canStart, reason := m.CanStartInstance("test-1")
	if !canStart {
		t.Errorf("Expected to start without provider, got reason: %s", reason)
	}
}

func TestManager_CanStartInstance_NotFound(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	canStart, reason := m.CanStartInstance("nonexistent")
	if canStart {
		t.Error("Expected false for non-existent instance")
	}
	if reason != "instance not found" {
		t.Errorf("Expected 'instance not found', got %q", reason)
	}
}

func TestManager_CanStartInstance_NotPending(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")
	_ = m.StartInstance(context.Background(), "test-1")

	canStart, reason := m.CanStartInstance("test-1")
	if canStart {
		t.Error("Expected false for non-pending instance")
	}
	if reason == "" {
		t.Error("Expected a reason for non-pending instance")
	}
}

func TestManager_CanStartInstance_UngroupedInstance(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	canStart, reason := m.CanStartInstance("test-1")
	if !canStart {
		t.Errorf("Expected to start ungrouped instance, got reason: %s", reason)
	}
}

func TestManager_CanStartInstance_GroupDependenciesMet(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Create two groups, group 2 depends on group 1
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhaseCompleted, // Already completed
		ExecutionOrder: 0,
		InstanceIDs:    []string{},
	}
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"test-1"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	canStart, reason := m.CanStartInstance("test-1")
	if !canStart {
		t.Errorf("Expected to start when dependencies met, got reason: %s", reason)
	}
}

func TestManager_CanStartInstance_GroupDependenciesNotMet(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Create two groups, group 2 depends on group 1 which is still pending
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhasePending, // Not yet completed
		ExecutionOrder: 0,
		InstanceIDs:    []string{"group1-inst"},
	}
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"test-1"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	canStart, reason := m.CanStartInstance("test-1")
	if canStart {
		t.Error("Expected cannot start when dependencies not met")
	}
	if reason == "" {
		t.Error("Expected a reason when dependencies not met")
	}
}

func TestManager_StartInstanceIfReady(t *testing.T) {
	var phaseChangeCalled bool
	var mu sync.Mutex

	callbacks := Callbacks{
		OnGroupPhaseChange: func(groupID, groupName string, oldPhase, newPhase GroupPhase) {
			mu.Lock()
			phaseChangeCalled = true
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhasePending,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"test-1"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	err := m.StartInstanceIfReady(context.Background(), "test-1")
	if err != nil {
		t.Fatalf("StartInstanceIfReady failed: %v", err)
	}

	// Verify instance is now working
	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusWorking {
		t.Errorf("Expected status %q, got %q", StatusWorking, inst.Status)
	}

	// Verify group phase changed
	mu.Lock()
	called := phaseChangeCalled
	mu.Unlock()

	if !called {
		t.Error("Expected OnGroupPhaseChange callback to be called")
	}

	// Verify group phase in provider
	updatedGroup := provider.GetGroup("group1")
	if updatedGroup.Phase != GroupPhaseExecuting {
		t.Errorf("Expected group phase %q, got %q", GroupPhaseExecuting, updatedGroup.Phase)
	}
}

func TestManager_StartInstanceIfReady_DependenciesNotMet(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhaseExecuting, // Still executing
		ExecutionOrder: 0,
		InstanceIDs:    []string{},
	}
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"test-1"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("test-1", "/tmp/wt", "branch", "task")

	err := m.StartInstanceIfReady(context.Background(), "test-1")
	if err == nil {
		t.Error("Expected error when dependencies not met")
	}

	// Instance should still be pending
	inst, _ := m.GetInstance("test-1")
	if inst.Status != StatusPending {
		t.Errorf("Expected status %q, got %q", StatusPending, inst.Status)
	}
}

func TestManager_GetGroupCompletionState(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1", "inst-2", "inst-3", "inst-4"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	// Create instances in various states
	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")
	_, _ = m.CreateInstance("inst-3", "/tmp/wt3", "branch3", "task3")
	_, _ = m.CreateInstance("inst-4", "/tmp/wt4", "branch4", "task4")

	_ = m.UpdateStatus("inst-1", StatusWorking)
	_ = m.UpdateStatus("inst-2", StatusCompleted)
	_ = m.UpdateStatus("inst-3", StatusError)
	// inst-4 remains pending

	state := m.GetGroupCompletionState("group1")
	if state == nil {
		t.Fatal("Expected non-nil state")
	}

	if state.TotalCount != 4 {
		t.Errorf("Expected TotalCount 4, got %d", state.TotalCount)
	}
	if state.RunningCount != 1 {
		t.Errorf("Expected RunningCount 1, got %d", state.RunningCount)
	}
	if state.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", state.SuccessCount)
	}
	if state.FailedCount != 1 {
		t.Errorf("Expected FailedCount 1, got %d", state.FailedCount)
	}
	if state.PendingCount != 1 {
		t.Errorf("Expected PendingCount 1, got %d", state.PendingCount)
	}
}

func TestManager_GetGroupCompletionState_NoProvider(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	state := m.GetGroupCompletionState("group1")
	if state != nil {
		t.Error("Expected nil state when no provider")
	}
}

func TestManager_GetGroupCompletionState_NonexistentGroup(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()
	m.SetGroupProvider(provider)

	state := m.GetGroupCompletionState("nonexistent")
	if state != nil {
		t.Error("Expected nil state for nonexistent group")
	}
}

func TestManager_CheckGroupCompletion_Success(t *testing.T) {
	var completeCalled bool
	var receivedSuccess bool
	var mu sync.Mutex

	callbacks := Callbacks{
		OnGroupComplete: func(groupID, groupName string, success bool, failedCount, successCount int) {
			mu.Lock()
			completeCalled = true
			receivedSuccess = success
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1", "inst-2"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// Both complete successfully
	_ = m.UpdateStatus("inst-1", StatusCompleted)
	_ = m.UpdateStatus("inst-2", StatusCompleted)

	state := m.CheckGroupCompletion("inst-1")
	if state == nil {
		t.Fatal("Expected non-nil state")
	}

	if !state.AllSucceeded() {
		t.Error("Expected AllSucceeded to be true")
	}

	mu.Lock()
	called := completeCalled
	success := receivedSuccess
	mu.Unlock()

	if !called {
		t.Error("Expected OnGroupComplete callback to be called")
	}
	if !success {
		t.Error("Expected success=true in callback")
	}

	// Verify group phase
	updatedGroup := provider.GetGroup("group1")
	if updatedGroup.Phase != GroupPhaseCompleted {
		t.Errorf("Expected group phase %q, got %q", GroupPhaseCompleted, updatedGroup.Phase)
	}
}

func TestManager_CheckGroupCompletion_Failure(t *testing.T) {
	var receivedSuccess bool
	var mu sync.Mutex

	callbacks := Callbacks{
		OnGroupComplete: func(groupID, groupName string, success bool, failedCount, successCount int) {
			mu.Lock()
			receivedSuccess = success
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1", "inst-2"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// One succeeds, one fails
	_ = m.UpdateStatus("inst-1", StatusCompleted)
	_ = m.UpdateStatus("inst-2", StatusError)

	state := m.CheckGroupCompletion("inst-1")
	if state == nil {
		t.Fatal("Expected non-nil state")
	}

	if state.AllSucceeded() {
		t.Error("Expected AllSucceeded to be false")
	}

	mu.Lock()
	success := receivedSuccess
	mu.Unlock()

	if success {
		t.Error("Expected success=false in callback")
	}

	// Verify group phase
	updatedGroup := provider.GetGroup("group1")
	if updatedGroup.Phase != GroupPhaseFailed {
		t.Errorf("Expected group phase %q, got %q", GroupPhaseFailed, updatedGroup.Phase)
	}
}

func TestManager_CheckGroupCompletion_NotComplete(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1", "inst-2"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// Only one is completed
	_ = m.UpdateStatus("inst-1", StatusCompleted)
	// inst-2 is still pending

	state := m.CheckGroupCompletion("inst-1")
	if state != nil {
		t.Error("Expected nil state when group not complete")
	}
}

func TestManager_GetReadyGroups(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Group 1: completed
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhaseCompleted,
		ExecutionOrder: 0,
		InstanceIDs:    []string{},
	}
	// Group 2: pending, depends on group1 (should be ready)
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"inst-1"},
		DependsOn:      []string{"group1"},
	}
	// Group 3: pending, depends on group2 (should NOT be ready)
	group3 := &GroupInfo{
		ID:             "group3",
		Name:           "Group 3",
		Phase:          GroupPhasePending,
		ExecutionOrder: 2,
		InstanceIDs:    []string{"inst-2"},
		DependsOn:      []string{"group2"},
	}
	// Group 4: pending, no dependencies (should be ready)
	group4 := &GroupInfo{
		ID:             "group4",
		Name:           "Group 4",
		Phase:          GroupPhasePending,
		ExecutionOrder: 3,
		InstanceIDs:    []string{"inst-3"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	provider.AddGroup(group3)
	provider.AddGroup(group4)
	m.SetGroupProvider(provider)

	ready := m.GetReadyGroups()
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready groups, got %d", len(ready))
	}

	// Verify group2 and group4 are ready
	readyIDs := make(map[string]bool)
	for _, g := range ready {
		readyIDs[g.ID] = true
	}

	if !readyIDs["group2"] {
		t.Error("Expected group2 to be ready")
	}
	if !readyIDs["group4"] {
		t.Error("Expected group4 to be ready")
	}
}

func TestManager_GetReadyGroups_NoProvider(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)

	ready := m.GetReadyGroups()
	if ready != nil {
		t.Error("Expected nil when no provider")
	}
}

func TestManager_StartNextGroup(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhasePending,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1", "inst-2"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	started, err := m.StartNextGroup(context.Background())
	if err != nil {
		t.Fatalf("StartNextGroup failed: %v", err)
	}

	if started != 2 {
		t.Errorf("Expected 2 instances started, got %d", started)
	}

	// Verify both instances are working
	inst1, _ := m.GetInstance("inst-1")
	inst2, _ := m.GetInstance("inst-2")
	if inst1.Status != StatusWorking {
		t.Errorf("Expected inst-1 status %q, got %q", StatusWorking, inst1.Status)
	}
	if inst2.Status != StatusWorking {
		t.Errorf("Expected inst-2 status %q, got %q", StatusWorking, inst2.Status)
	}
}

func TestManager_StartNextGroup_NoReadyGroups(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// No pending groups
	group := &GroupInfo{
		ID:             "group1",
		Name:           "Test Group",
		Phase:          GroupPhaseExecuting, // Already executing
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1"},
	}
	provider.AddGroup(group)
	m.SetGroupProvider(provider)

	started, err := m.StartNextGroup(context.Background())
	if err != nil {
		t.Fatalf("StartNextGroup failed: %v", err)
	}

	if started != 0 {
		t.Errorf("Expected 0 instances started, got %d", started)
	}
}

func TestManager_StartAllReadyInstances(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Two groups, both pending with no dependencies
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhasePending,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1"},
	}
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"inst-2", "inst-3"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")
	_, _ = m.CreateInstance("inst-3", "/tmp/wt3", "branch3", "task3")

	started, err := m.StartAllReadyInstances(context.Background())
	if err != nil {
		t.Fatalf("StartAllReadyInstances failed: %v", err)
	}

	if started != 3 {
		t.Errorf("Expected 3 instances started, got %d", started)
	}
}

func TestManager_OnInstanceComplete_AutoStart(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Group 1: will complete
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1"},
	}
	// Group 2: pending, depends on group1
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"inst-2"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// Complete inst-1
	_ = m.UpdateStatus("inst-1", StatusCompleted)

	// Trigger OnInstanceComplete
	startedIDs := m.OnInstanceComplete(context.Background(), "inst-1")

	if len(startedIDs) != 1 {
		t.Errorf("Expected 1 auto-started instance, got %d", len(startedIDs))
	}
	if len(startedIDs) > 0 && startedIDs[0] != "inst-2" {
		t.Errorf("Expected inst-2 to be auto-started, got %s", startedIDs[0])
	}

	// Verify inst-2 is now working
	inst2, _ := m.GetInstance("inst-2")
	if inst2.Status != StatusWorking {
		t.Errorf("Expected inst-2 status %q, got %q", StatusWorking, inst2.Status)
	}
}

func TestManager_OnInstanceComplete_NoAutoStartOnFailure(t *testing.T) {
	m := NewManager(DefaultConfig(), Callbacks{}, nil)
	provider := newMockGroupProvider()

	// Group 1: one instance fails
	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhaseExecuting,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1"},
	}
	// Group 2: depends on group1
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"inst-2"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// inst-1 fails
	_ = m.UpdateStatus("inst-1", StatusError)

	// Trigger OnInstanceComplete
	startedIDs := m.OnInstanceComplete(context.Background(), "inst-1")

	// No instances should be auto-started because group1 failed
	if len(startedIDs) != 0 {
		t.Errorf("Expected 0 auto-started instances on failure, got %d", len(startedIDs))
	}

	// Verify inst-2 is still pending
	inst2, _ := m.GetInstance("inst-2")
	if inst2.Status != StatusPending {
		t.Errorf("Expected inst-2 status %q, got %q", StatusPending, inst2.Status)
	}
}

func TestManager_GroupLifecycle_FullWorkflow(t *testing.T) {
	// This test exercises a full group workflow:
	// 1. Group 1 executes and completes
	// 2. Group 2 auto-starts after group 1 completes
	// 3. Group 2 completes, verifying the full lifecycle

	var phaseChanges []struct {
		groupID  string
		oldPhase GroupPhase
		newPhase GroupPhase
	}
	var completions []struct {
		groupID string
		success bool
	}
	var mu sync.Mutex

	callbacks := Callbacks{
		OnGroupPhaseChange: func(groupID, groupName string, oldPhase, newPhase GroupPhase) {
			mu.Lock()
			phaseChanges = append(phaseChanges, struct {
				groupID  string
				oldPhase GroupPhase
				newPhase GroupPhase
			}{groupID, oldPhase, newPhase})
			mu.Unlock()
		},
		OnGroupComplete: func(groupID, groupName string, success bool, failedCount, successCount int) {
			mu.Lock()
			completions = append(completions, struct {
				groupID string
				success bool
			}{groupID, success})
			mu.Unlock()
		},
	}

	m := NewManager(DefaultConfig(), callbacks, nil)
	provider := newMockGroupProvider()

	group1 := &GroupInfo{
		ID:             "group1",
		Name:           "Group 1",
		Phase:          GroupPhasePending,
		ExecutionOrder: 0,
		InstanceIDs:    []string{"inst-1"},
	}
	group2 := &GroupInfo{
		ID:             "group2",
		Name:           "Group 2",
		Phase:          GroupPhasePending,
		ExecutionOrder: 1,
		InstanceIDs:    []string{"inst-2"},
		DependsOn:      []string{"group1"},
	}

	provider.AddGroup(group1)
	provider.AddGroup(group2)
	m.SetGroupProvider(provider)

	_, _ = m.CreateInstance("inst-1", "/tmp/wt1", "branch1", "task1")
	_, _ = m.CreateInstance("inst-2", "/tmp/wt2", "branch2", "task2")

	// Start group 1
	started, _ := m.StartNextGroup(context.Background())
	if started != 1 {
		t.Fatalf("Expected 1 instance started, got %d", started)
	}

	// Verify phase changed to executing
	mu.Lock()
	if len(phaseChanges) < 1 || phaseChanges[0].newPhase != GroupPhaseExecuting {
		t.Error("Expected phase change to executing")
	}
	mu.Unlock()

	// Complete inst-1
	_ = m.UpdateStatus("inst-1", StatusCompleted)
	autoStarted := m.OnInstanceComplete(context.Background(), "inst-1")

	// Verify group 2 auto-started
	if len(autoStarted) != 1 || autoStarted[0] != "inst-2" {
		t.Errorf("Expected inst-2 to auto-start, got %v", autoStarted)
	}

	// Verify phase changes
	mu.Lock()
	changeCount := len(phaseChanges)
	mu.Unlock()

	// Should have: group1 pending->executing, group1 executing->completed, group2 pending->executing
	if changeCount < 3 {
		t.Errorf("Expected at least 3 phase changes, got %d", changeCount)
	}

	// Complete inst-2
	_ = m.UpdateStatus("inst-2", StatusCompleted)
	m.CheckGroupCompletion("inst-2")

	// Verify completions
	mu.Lock()
	completionCount := len(completions)
	mu.Unlock()

	if completionCount < 2 {
		t.Errorf("Expected at least 2 completions, got %d", completionCount)
	}
}

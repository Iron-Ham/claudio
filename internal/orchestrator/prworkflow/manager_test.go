package prworkflow

import (
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
)

func TestNewManager(t *testing.T) {
	cfg := Config{
		UseAI:      true,
		Draft:      true,
		AutoRebase: false,
		TmuxWidth:  120,
		TmuxHeight: 40,
	}
	eventBus := event.NewBus()

	m := NewManager(cfg, "test-session", eventBus)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.config.UseAI != cfg.UseAI {
		t.Errorf("UseAI = %v, want %v", m.config.UseAI, cfg.UseAI)
	}
	if m.config.Draft != cfg.Draft {
		t.Errorf("Draft = %v, want %v", m.config.Draft, cfg.Draft)
	}
	if m.sessionID != "test-session" {
		t.Errorf("sessionID = %q, want %q", m.sessionID, "test-session")
	}
	if m.eventBus != eventBus {
		t.Error("eventBus not set correctly")
	}
	if m.workflows == nil {
		t.Error("workflows map not initialized")
	}
}

func TestNewConfigFromConfig(t *testing.T) {
	globalCfg := &config.Config{
		PR: config.PRConfig{
			UseAI:      true,
			Draft:      false,
			AutoRebase: true,
		},
		Instance: config.InstanceConfig{
			TmuxWidth:  160,
			TmuxHeight: 50,
		},
	}

	cfg := NewConfigFromConfig(globalCfg)

	if cfg.UseAI != globalCfg.PR.UseAI {
		t.Errorf("UseAI = %v, want %v", cfg.UseAI, globalCfg.PR.UseAI)
	}
	if cfg.Draft != globalCfg.PR.Draft {
		t.Errorf("Draft = %v, want %v", cfg.Draft, globalCfg.PR.Draft)
	}
	if cfg.AutoRebase != globalCfg.PR.AutoRebase {
		t.Errorf("AutoRebase = %v, want %v", cfg.AutoRebase, globalCfg.PR.AutoRebase)
	}
	if cfg.TmuxWidth != globalCfg.Instance.TmuxWidth {
		t.Errorf("TmuxWidth = %v, want %v", cfg.TmuxWidth, globalCfg.Instance.TmuxWidth)
	}
	if cfg.TmuxHeight != globalCfg.Instance.TmuxHeight {
		t.Errorf("TmuxHeight = %v, want %v", cfg.TmuxHeight, globalCfg.Instance.TmuxHeight)
	}
}

func TestSetDisplayDimensions(t *testing.T) {
	m := NewManager(Config{TmuxWidth: 80, TmuxHeight: 24}, "", nil)

	m.SetDisplayDimensions(200, 60)

	m.mu.RLock()
	width := m.displayWidth
	height := m.displayHeight
	m.mu.RUnlock()

	if width != 200 {
		t.Errorf("displayWidth = %d, want %d", width, 200)
	}
	if height != 60 {
		t.Errorf("displayHeight = %d, want %d", height, 60)
	}
}

func TestSetCompleteCallback(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	called := false
	cb := func(instanceID string, success bool) {
		called = true
	}

	m.SetCompleteCallback(cb)

	m.mu.RLock()
	hasCallback := m.completeCallback != nil
	m.mu.RUnlock()

	if !hasCallback {
		t.Error("completeCallback not set")
	}

	// Verify it can be called
	m.completeCallback("test", true)
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestSetOpenedCallback(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	called := false
	cb := func(instanceID string) {
		called = true
	}

	m.SetOpenedCallback(cb)

	m.mu.RLock()
	hasCallback := m.openedCallback != nil
	m.mu.RUnlock()

	if !hasCallback {
		t.Error("openedCallback not set")
	}

	// Verify it can be called
	m.openedCallback("test")
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestGet_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	workflow := m.Get("nonexistent")

	if workflow != nil {
		t.Error("Get() should return nil for nonexistent workflow")
	}
}

func TestRunning_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	running := m.Running("nonexistent")

	if running {
		t.Error("Running() should return false for nonexistent workflow")
	}
}

func TestCount_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	count := m.Count()

	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}
}

func TestIDs_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	ids := m.IDs()

	if len(ids) != 0 {
		t.Errorf("IDs() returned %d items, want 0", len(ids))
	}
}

func TestStop_NoWorkflow(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	err := m.Stop("nonexistent")

	if err != nil {
		t.Errorf("Stop() returned unexpected error: %v", err)
	}
}

func TestStopAll_Empty(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	// Should not panic
	m.StopAll()

	count := m.Count()
	if count != 0 {
		t.Errorf("Count() after StopAll = %d, want 0", count)
	}
}

func TestHandleComplete_WithCallback(t *testing.T) {
	eventBus := event.NewBus()
	m := NewManager(Config{}, "", eventBus)

	var capturedID string
	var capturedSuccess bool
	var callbackCalled bool

	m.SetCompleteCallback(func(instanceID string, success bool) {
		callbackCalled = true
		capturedID = instanceID
		capturedSuccess = success
	})

	// Simulate having a workflow (add it directly to test cleanup)
	m.mu.Lock()
	m.workflows["test-id"] = nil // We just need the key to test cleanup
	m.mu.Unlock()

	m.HandleComplete("test-id", true, "output text")

	if !callbackCalled {
		t.Error("callback was not called")
	}
	if capturedID != "test-id" {
		t.Errorf("capturedID = %q, want %q", capturedID, "test-id")
	}
	if !capturedSuccess {
		t.Error("capturedSuccess should be true")
	}

	// Verify workflow was cleaned up
	if m.Get("test-id") != nil {
		t.Error("workflow should have been removed")
	}
}

func TestHandleComplete_WithEventBus(t *testing.T) {
	eventBus := event.NewBus()
	m := NewManager(Config{}, "", eventBus)

	// Subscribe to events
	var receivedEvent event.Event
	var wg sync.WaitGroup
	wg.Add(1)
	eventBus.Subscribe("pr.completed", func(e event.Event) {
		receivedEvent = e
		wg.Done()
	})

	m.HandleComplete("test-id", true, "output text")

	// Wait for event with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Event received
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	if receivedEvent.EventType() != "pr.completed" {
		t.Errorf("event type = %v, want %v", receivedEvent.EventType(), "pr.completed")
	}

	// Type assert to get the specific event type
	prEvent, ok := receivedEvent.(event.PRCompleteEvent)
	if !ok {
		t.Fatal("event is not a PRCompleteEvent")
	}
	if prEvent.InstanceID != "test-id" {
		t.Errorf("event instanceID = %q, want %q", prEvent.InstanceID, "test-id")
	}
}

func TestHandleComplete_NoCallbackOrEventBus(t *testing.T) {
	m := NewManager(Config{}, "", nil) // nil eventBus

	// Should not panic
	m.HandleComplete("test-id", false, "")
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(Config{}, "session", nil)

	// Test concurrent reads and writes
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(3)
		id := string(rune('a' + i))

		go func(id string) {
			defer wg.Done()
			m.SetDisplayDimensions(100, 50)
		}(id)

		go func(id string) {
			defer wg.Done()
			_ = m.Get(id)
		}(id)

		go func(id string) {
			defer wg.Done()
			_ = m.Running(id)
		}(id)
	}

	wg.Wait()
}

func TestManagerWithNilEventBus(t *testing.T) {
	m := NewManager(Config{
		TmuxWidth:  120,
		TmuxHeight: 40,
	}, "", nil)

	// Ensure nil eventBus doesn't cause panic during HandleComplete
	m.HandleComplete("test-id", true, "output")

	// Should complete without panic
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestNewManager_EmptySessionID(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	if m.sessionID != "" {
		t.Errorf("sessionID = %q, want empty string", m.sessionID)
	}
}

func TestSetLogger(t *testing.T) {
	m := NewManager(Config{}, "", nil)

	// Just verify it doesn't panic with nil logger
	m.SetLogger(nil)

	m.mu.RLock()
	logger := m.logger
	m.mu.RUnlock()

	if logger != nil {
		t.Error("logger should be nil")
	}
}

// mockGroupInfo implements GroupInfo for testing
type mockGroupInfo struct {
	id             string
	name           string
	instanceIDs    []string
	executionOrder int
	dependsOn      []string
}

func (m *mockGroupInfo) GetID() string            { return m.id }
func (m *mockGroupInfo) GetName() string          { return m.name }
func (m *mockGroupInfo) GetInstanceIDs() []string { return m.instanceIDs }
func (m *mockGroupInfo) GetExecutionOrder() int   { return m.executionOrder }
func (m *mockGroupInfo) GetDependsOn() []string   { return m.dependsOn }

// mockInstanceInfo implements InstanceInfo for testing
type mockInstanceInfo struct {
	id           string
	worktreePath string
	branch       string
	task         string
}

func (m *mockInstanceInfo) GetID() string           { return m.id }
func (m *mockInstanceInfo) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstanceInfo) GetBranch() string       { return m.branch }
func (m *mockInstanceInfo) GetTask() string         { return m.task }

func TestGroupPRModeString(t *testing.T) {
	tests := []struct {
		mode     GroupPRMode
		expected string
	}{
		{GroupPRModeStacked, "stacked"},
		{GroupPRModeConsolidated, "consolidated"},
		{GroupPRModeSingle, "single"},
		{GroupPRMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.expected {
				t.Errorf("GroupPRMode.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGenerateGroupPRDescription(t *testing.T) {
	group1 := &mockGroupInfo{
		id:             "g1",
		name:           "Foundation",
		instanceIDs:    []string{"inst1", "inst2"},
		executionOrder: 0,
	}
	group2 := &mockGroupInfo{
		id:             "g2",
		name:           "Features",
		instanceIDs:    []string{"inst3"},
		executionOrder: 1,
		dependsOn:      []string{"g1"},
	}

	instances := map[string]InstanceInfo{
		"inst1": &mockInstanceInfo{id: "inst1", task: "Set up database models"},
		"inst2": &mockInstanceInfo{id: "inst2", task: "Add authentication"},
		"inst3": &mockInstanceInfo{id: "inst3", task: "Implement user dashboard"},
	}

	t.Run("basic description", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:      GroupPRModeSingle,
			Groups:    []GroupInfo{group1, group2},
			Instances: instances,
		}

		desc := GenerateGroupPRDescription(opts, group1, nil)

		if !containsString(desc, "Foundation") {
			t.Error("description should contain group name")
		}
		if !containsString(desc, "Set up database models") {
			t.Error("description should contain task from inst1")
		}
		if !containsString(desc, "Add authentication") {
			t.Error("description should contain task from inst2")
		}
	})

	t.Run("with session name", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:        GroupPRModeSingle,
			Groups:      []GroupInfo{group1},
			Instances:   instances,
			SessionName: "My Test Session",
		}

		desc := GenerateGroupPRDescription(opts, group1, nil)

		if !containsString(desc, "My Test Session") {
			t.Error("description should contain session name")
		}
	})

	t.Run("with group structure", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:                  GroupPRModeStacked,
			Groups:                []GroupInfo{group1, group2},
			Instances:             instances,
			IncludeGroupStructure: true,
		}

		desc := GenerateGroupPRDescription(opts, group2, nil)

		if !containsString(desc, "Group Structure") {
			t.Error("description should contain group structure section")
		}
		if !containsString(desc, "depends on") {
			t.Error("description should show dependency info")
		}
	})

	t.Run("with related PRs", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:               GroupPRModeStacked,
			Groups:             []GroupInfo{group1, group2},
			Instances:          instances,
			AutoLinkRelatedPRs: true,
		}

		relatedPRs := map[string]string{
			"g1": "https://github.com/org/repo/pull/123",
		}

		desc := GenerateGroupPRDescription(opts, group2, relatedPRs)

		if !containsString(desc, "Related PRs") {
			t.Error("description should contain related PRs section")
		}
		if !containsString(desc, "https://github.com/org/repo/pull/123") {
			t.Error("description should contain related PR URL")
		}
	})
}

func TestGenerateGroupPRTitle(t *testing.T) {
	group := &mockGroupInfo{
		id:             "g1",
		name:           "Foundation Layer",
		executionOrder: 0,
	}

	tests := []struct {
		name        string
		mode        GroupPRMode
		totalGroups int
		expected    string
	}{
		{
			name:        "single mode",
			mode:        GroupPRModeSingle,
			totalGroups: 1,
			expected:    "Foundation Layer",
		},
		{
			name:        "stacked mode",
			mode:        GroupPRModeStacked,
			totalGroups: 3,
			expected:    "[1/3] Foundation Layer",
		},
		{
			name:        "consolidated mode",
			mode:        GroupPRModeConsolidated,
			totalGroups: 3,
			expected:    "Foundation Layer (consolidated from 3 groups)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateGroupPRTitle(group, tt.mode, tt.totalGroups)
			if got != tt.expected {
				t.Errorf("GenerateGroupPRTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGenerateConsolidatedPRDescription(t *testing.T) {
	group1 := &mockGroupInfo{
		id:             "g1",
		name:           "Foundation",
		instanceIDs:    []string{"inst1"},
		executionOrder: 0,
	}
	group2 := &mockGroupInfo{
		id:             "g2",
		name:           "Features",
		instanceIDs:    []string{"inst2"},
		executionOrder: 1,
	}

	instances := map[string]InstanceInfo{
		"inst1": &mockInstanceInfo{id: "inst1", task: "Task 1"},
		"inst2": &mockInstanceInfo{id: "inst2", task: "Task 2"},
	}

	opts := GroupPROptions{
		Mode:        GroupPRModeConsolidated,
		Groups:      []GroupInfo{group1, group2},
		Instances:   instances,
		SessionName: "Test Session",
	}

	desc := GenerateConsolidatedPRDescription(opts)

	if !containsString(desc, "consolidates changes from 2 groups") {
		t.Error("description should mention number of groups")
	}
	if !containsString(desc, "Test Session") {
		t.Error("description should contain session name")
	}
	if !containsString(desc, "1. Foundation") {
		t.Error("description should list groups in order")
	}
	if !containsString(desc, "2. Features") {
		t.Error("description should list groups in order")
	}
}

func TestGenerateConsolidatedPRTitle(t *testing.T) {
	tests := []struct {
		name     string
		opts     GroupPROptions
		expected string
	}{
		{
			name:     "empty groups",
			opts:     GroupPROptions{Groups: []GroupInfo{}},
			expected: "Consolidated changes",
		},
		{
			name: "with session name",
			opts: GroupPROptions{
				SessionName: "My Session",
				Groups:      []GroupInfo{&mockGroupInfo{name: "Group 1"}},
			},
			expected: "My Session (consolidated)",
		},
		{
			name: "single group no session",
			opts: GroupPROptions{
				Groups: []GroupInfo{&mockGroupInfo{name: "Only Group"}},
			},
			expected: "Only Group",
		},
		{
			name: "multiple groups no session",
			opts: GroupPROptions{
				Groups: []GroupInfo{
					&mockGroupInfo{name: "First"},
					&mockGroupInfo{name: "Second"},
					&mockGroupInfo{name: "Third"},
				},
			},
			expected: "Consolidated: First + 2 more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateConsolidatedPRTitle(tt.opts)
			if got != tt.expected {
				t.Errorf("GenerateConsolidatedPRTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPrepareGroupPR(t *testing.T) {
	group1 := &mockGroupInfo{
		id:             "g1",
		name:           "Foundation",
		instanceIDs:    []string{"inst1"},
		executionOrder: 0,
	}
	group2 := &mockGroupInfo{
		id:             "g2",
		name:           "Features",
		instanceIDs:    []string{"inst2"},
		executionOrder: 1,
	}

	instances := map[string]InstanceInfo{
		"inst1": &mockInstanceInfo{id: "inst1", branch: "feature/foundation", task: "Task 1"},
		"inst2": &mockInstanceInfo{id: "inst2", branch: "feature/features", task: "Task 2"},
	}

	m := NewManager(Config{}, "", nil)

	t.Run("single mode", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:      GroupPRModeSingle,
			Groups:    []GroupInfo{group1},
			Instances: instances,
		}

		result := m.PrepareGroupPR(opts, group1, nil)

		if result.GroupID != "g1" {
			t.Errorf("GroupID = %q, want %q", result.GroupID, "g1")
		}
		if result.HeadBranch != "feature/foundation" {
			t.Errorf("HeadBranch = %q, want %q", result.HeadBranch, "feature/foundation")
		}
		if result.BaseBranch != "main" {
			t.Errorf("BaseBranch = %q, want %q", result.BaseBranch, "main")
		}
	})

	t.Run("stacked mode - first group", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:       GroupPRModeStacked,
			Groups:     []GroupInfo{group1, group2},
			Instances:  instances,
			BaseBranch: "main",
		}

		result := m.PrepareGroupPR(opts, group1, nil)

		if result.BaseBranch != "main" {
			t.Errorf("first group BaseBranch = %q, want %q", result.BaseBranch, "main")
		}
	})

	t.Run("stacked mode - second group", func(t *testing.T) {
		opts := GroupPROptions{
			Mode:       GroupPRModeStacked,
			Groups:     []GroupInfo{group1, group2},
			Instances:  instances,
			BaseBranch: "main",
		}

		result := m.PrepareGroupPR(opts, group2, nil)

		// Second group should base on first group's branch
		if result.BaseBranch != "feature/foundation" {
			t.Errorf("second group BaseBranch = %q, want %q", result.BaseBranch, "feature/foundation")
		}
	})
}

func TestPrepareConsolidatedPR(t *testing.T) {
	group1 := &mockGroupInfo{
		id:             "g1",
		name:           "Foundation",
		instanceIDs:    []string{"inst1"},
		executionOrder: 0,
	}
	group2 := &mockGroupInfo{
		id:             "g2",
		name:           "Features",
		instanceIDs:    []string{"inst2", "inst3"},
		executionOrder: 1,
	}

	instances := map[string]InstanceInfo{
		"inst1": &mockInstanceInfo{id: "inst1", branch: "feature/foundation"},
		"inst2": &mockInstanceInfo{id: "inst2", branch: "feature/features"},
		"inst3": &mockInstanceInfo{id: "inst3", branch: "feature/features"},
	}

	m := NewManager(Config{}, "", nil)

	opts := GroupPROptions{
		Mode:      GroupPRModeConsolidated,
		Groups:    []GroupInfo{group1, group2},
		Instances: instances,
	}

	result := m.PrepareConsolidatedPR(opts)

	if result.GroupID != "consolidated" {
		t.Errorf("GroupID = %q, want %q", result.GroupID, "consolidated")
	}
	if len(result.InstanceIDs) != 3 {
		t.Errorf("InstanceIDs count = %d, want 3", len(result.InstanceIDs))
	}
	// Head branch should be from the last group
	if result.HeadBranch != "feature/features" {
		t.Errorf("HeadBranch = %q, want %q", result.HeadBranch, "feature/features")
	}
	if result.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", result.BaseBranch, "main")
	}
}

func TestNewGroupPRSession(t *testing.T) {
	group1 := &mockGroupInfo{id: "g1", name: "Group 1"}
	group2 := &mockGroupInfo{id: "g2", name: "Group 2"}

	opts := GroupPROptions{
		Mode:   GroupPRModeStacked,
		Groups: []GroupInfo{group1, group2},
	}

	session := NewGroupPRSession("session-123", opts)

	if session.ID != "session-123" {
		t.Errorf("ID = %q, want %q", session.ID, "session-123")
	}
	if session.Mode != GroupPRModeStacked {
		t.Errorf("Mode = %v, want %v", session.Mode, GroupPRModeStacked)
	}
	if len(session.PendingGroups) != 2 {
		t.Errorf("PendingGroups = %d, want 2", len(session.PendingGroups))
	}
	if len(session.CreatedPRURLs) != 0 {
		t.Errorf("CreatedPRURLs should be empty initially")
	}
	if session.IsComplete() {
		t.Error("session should not be complete initially")
	}
}

func TestGroupPRSession_RecordPRCreated(t *testing.T) {
	group1 := &mockGroupInfo{id: "g1", name: "Group 1"}
	group2 := &mockGroupInfo{id: "g2", name: "Group 2"}

	opts := GroupPROptions{
		Mode:   GroupPRModeStacked,
		Groups: []GroupInfo{group1, group2},
	}

	session := NewGroupPRSession("test", opts)
	result := &GroupPRResult{GroupID: "g1", GroupName: "Group 1"}

	session.RecordPRCreated("g1", "https://github.com/org/repo/pull/123", result)

	if session.SuccessCount() != 1 {
		t.Errorf("SuccessCount = %d, want 1", session.SuccessCount())
	}
	if len(session.PendingGroups) != 1 {
		t.Errorf("PendingGroups = %d, want 1", len(session.PendingGroups))
	}
	if session.CreatedPRURLs["g1"] != "https://github.com/org/repo/pull/123" {
		t.Error("PR URL not recorded correctly")
	}
	if session.IsComplete() {
		t.Error("session should not be complete yet")
	}

	// Record second PR
	result2 := &GroupPRResult{GroupID: "g2", GroupName: "Group 2"}
	session.RecordPRCreated("g2", "https://github.com/org/repo/pull/124", result2)

	if session.SuccessCount() != 2 {
		t.Errorf("SuccessCount = %d, want 2", session.SuccessCount())
	}
	if !session.IsComplete() {
		t.Error("session should be complete")
	}
}

func TestGroupPRSession_RecordPRFailed(t *testing.T) {
	group1 := &mockGroupInfo{id: "g1", name: "Group 1"}
	group2 := &mockGroupInfo{id: "g2", name: "Group 2"}

	opts := GroupPROptions{
		Mode:   GroupPRModeStacked,
		Groups: []GroupInfo{group1, group2},
	}

	session := NewGroupPRSession("test", opts)

	session.RecordPRFailed("g1")

	if session.FailureCount() != 1 {
		t.Errorf("FailureCount = %d, want 1", session.FailureCount())
	}
	if len(session.PendingGroups) != 1 {
		t.Errorf("PendingGroups = %d, want 1", len(session.PendingGroups))
	}
	if !containsInSlice(session.FailedGroups, "g1") {
		t.Error("g1 should be in FailedGroups")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to check if a slice contains a string
func containsInSlice(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

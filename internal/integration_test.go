// Package internal contains integration tests that verify the refactored packages
// work together correctly. These tests ensure the orchestrator composition pattern
// and event bus communication work as expected.
package internal

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/orchestrator/lifecycle"
	orchsession "github.com/Iron-Ham/claudio/internal/orchestrator/session"
)

// TestEventBusIntegration tests that the event bus correctly routes events
// between components, simulating TUI-Orchestrator communication.
func TestEventBusIntegration(t *testing.T) {
	bus := event.NewBus()

	// Simulate TUI subscribing to various event types
	var receivedEvents []event.Event
	var mu sync.Mutex

	// Subscribe to instance lifecycle events
	bus.Subscribe("instance.started", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	bus.Subscribe("instance.stopped", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	// Subscribe to PR events
	bus.Subscribe("pr.completed", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	bus.Subscribe("pr.opened", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	// Subscribe to timeout events
	bus.Subscribe("instance.timeout", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	// Simulate orchestrator publishing events
	bus.Publish(event.NewInstanceStartedEvent("inst-1", "/tmp/wt", "feature/test", "Test task"))
	bus.Publish(event.NewPROpenedEvent("inst-1", ""))
	bus.Publish(event.NewTimeoutEvent("inst-2", event.TimeoutActivity, "5m"))
	bus.Publish(event.NewPRCompleteEvent("inst-1", true, "https://github.com/org/repo/pull/123", ""))
	bus.Publish(event.NewInstanceStoppedEvent("inst-1", true, "completed"))

	// Verify all events were received
	mu.Lock()
	eventCount := len(receivedEvents)
	mu.Unlock()

	if eventCount != 5 {
		t.Errorf("Expected 5 events, got %d", eventCount)
	}

	// Verify event order
	mu.Lock()
	defer mu.Unlock()

	expectedTypes := []string{
		"instance.started",
		"pr.opened",
		"instance.timeout",
		"pr.completed",
		"instance.stopped",
	}

	for i, expected := range expectedTypes {
		if i >= len(receivedEvents) {
			t.Errorf("Missing event at index %d", i)
			continue
		}
		if receivedEvents[i].EventType() != expected {
			t.Errorf("Event %d: expected type %q, got %q", i, expected, receivedEvents[i].EventType())
		}
	}
}

// TestEventBusWildcardSubscription tests that SubscribeAll receives all events,
// simulating a logging/metrics component.
func TestEventBusWildcardSubscription(t *testing.T) {
	bus := event.NewBus()

	var allEvents []string
	var mu sync.Mutex

	// Subscribe to all events (like a logging component would)
	bus.SubscribeAll(func(e event.Event) {
		mu.Lock()
		allEvents = append(allEvents, e.EventType())
		mu.Unlock()
	})

	// Publish various event types
	bus.Publish(event.NewInstanceStartedEvent("inst-1", "/tmp", "branch", "task"))
	bus.Publish(event.NewMetricsUpdateEvent("inst-1", 1000, 500, 100, 50, 0.05, 1))
	bus.Publish(event.NewBellEvent("inst-1"))
	bus.Publish(event.NewTaskCompletedEvent("task-1", "inst-1", true, ""))
	bus.Publish(event.NewPhaseChangeEvent("session-1", event.PhasePlanning, event.PhaseExecuting))
	bus.Publish(event.NewConflictDetectedEvent("inst-1", "branch", []string{"file.go"}))

	mu.Lock()
	count := len(allEvents)
	mu.Unlock()

	if count != 6 {
		t.Errorf("Expected wildcard subscriber to receive 6 events, got %d", count)
	}
}

// TestEventBusConcurrentPublish tests that the event bus handles concurrent
// publishing from multiple goroutines safely.
func TestEventBusConcurrentPublish(t *testing.T) {
	bus := event.NewBus()

	var receivedCount int
	var mu sync.Mutex

	bus.SubscribeAll(func(e event.Event) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	publishCount := 100

	// Simulate multiple instances publishing events concurrently
	for i := 0; i < publishCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bus.Publish(event.NewMetricsUpdateEvent(
				"inst-"+string(rune('a'+id%26)),
				int64(id*100),
				int64(id*50),
				0, 0, 0.01, 1,
			))
		}(i)
	}

	wg.Wait()

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	if count != publishCount {
		t.Errorf("Expected %d events, got %d", publishCount, count)
	}
}

// TestLifecycleManagerIntegration tests the lifecycle manager's ability to
// track instance state changes and trigger callbacks.
func TestLifecycleManagerIntegration(t *testing.T) {
	var statusChanges []struct {
		id        string
		oldStatus lifecycle.InstanceStatus
		newStatus lifecycle.InstanceStatus
	}
	var mu sync.Mutex

	callbacks := lifecycle.Callbacks{
		OnStatusChange: func(id string, old, new lifecycle.InstanceStatus) {
			mu.Lock()
			statusChanges = append(statusChanges, struct {
				id        string
				oldStatus lifecycle.InstanceStatus
				newStatus lifecycle.InstanceStatus
			}{id, old, new})
			mu.Unlock()
		},
	}

	mgr := lifecycle.NewManager(lifecycle.DefaultConfig(), callbacks, nil)

	// Create multiple instances
	_, err := mgr.CreateInstance("inst-1", "/tmp/wt1", "branch1", "Task 1")
	if err != nil {
		t.Fatalf("Failed to create instance 1: %v", err)
	}

	_, err = mgr.CreateInstance("inst-2", "/tmp/wt2", "branch2", "Task 2")
	if err != nil {
		t.Fatalf("Failed to create instance 2: %v", err)
	}

	// Start instances
	if err := mgr.StartInstance(context.Background(), "inst-1"); err != nil {
		t.Fatalf("Failed to start instance 1: %v", err)
	}
	if err := mgr.StartInstance(context.Background(), "inst-2"); err != nil {
		t.Fatalf("Failed to start instance 2: %v", err)
	}

	// Verify running count
	if count := mgr.GetRunningCount(); count != 2 {
		t.Errorf("Expected 2 running instances, got %d", count)
	}

	// Simulate one instance waiting for input
	if err := mgr.UpdateStatus("inst-1", lifecycle.StatusWaitingInput); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify still counts as running
	if count := mgr.GetRunningCount(); count != 2 {
		t.Errorf("Expected 2 running instances (working + waiting_input), got %d", count)
	}

	// Stop one instance
	if err := mgr.StopInstance("inst-2"); err != nil {
		t.Fatalf("Failed to stop instance: %v", err)
	}

	// Verify status change callbacks were triggered
	mu.Lock()
	changeCount := len(statusChanges)
	mu.Unlock()

	// Expected: 2 starts (pending->working) + 1 update (working->waiting_input) + 1 stop (working->completed)
	if changeCount != 4 {
		t.Errorf("Expected 4 status changes, got %d", changeCount)
	}

	// Stop the manager
	mgr.Stop()

	// Verify cleanup
	if count := mgr.GetRunningCount(); count != 0 {
		t.Errorf("Expected 0 running instances after stop, got %d", count)
	}
}

// TestLifecycleManagerWithEventBus tests the integration between the lifecycle
// manager callbacks and the event bus.
func TestLifecycleManagerWithEventBus(t *testing.T) {
	bus := event.NewBus()

	var receivedEvents []event.Event
	var mu sync.Mutex

	// Subscribe to instance events on the bus
	bus.Subscribe("instance.started", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})
	bus.Subscribe("instance.stopped", func(e event.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	// Create lifecycle manager that publishes to event bus
	callbacks := lifecycle.Callbacks{
		OnStatusChange: func(id string, old, new lifecycle.InstanceStatus) {
			if new == lifecycle.StatusWorking && old == lifecycle.StatusPending {
				bus.Publish(event.NewInstanceStartedEvent(id, "/tmp/wt", "branch", "task"))
			}
			if new == lifecycle.StatusCompleted {
				bus.Publish(event.NewInstanceStoppedEvent(id, true, "completed"))
			}
		},
	}

	mgr := lifecycle.NewManager(lifecycle.DefaultConfig(), callbacks, nil)

	// Create and start instance
	_, _ = mgr.CreateInstance("inst-1", "/tmp/wt", "branch", "task")
	_ = mgr.StartInstance(context.Background(), "inst-1")

	// Stop instance
	_ = mgr.StopInstance("inst-1")

	// Verify events were published
	mu.Lock()
	count := len(receivedEvents)
	mu.Unlock()

	if count != 2 {
		t.Errorf("Expected 2 events (started + stopped), got %d", count)
	}
}

// TestSessionManagerIntegration tests session persistence operations.
func TestSessionManagerIntegration(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	mgr := orchsession.NewManager(orchsession.Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Initialize directories
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize session manager: %v", err)
	}

	// Verify session doesn't exist yet
	if mgr.Exists() {
		t.Error("Session should not exist before creation")
	}

	// Create session
	sess, err := mgr.CreateSession("Test Session", tempDir)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if sess.Name != "Test Session" {
		t.Errorf("Expected session name %q, got %q", "Test Session", sess.Name)
	}

	// Verify session now exists
	if !mgr.Exists() {
		t.Error("Session should exist after creation")
	}

	// Add instance data to session
	inst := orchsession.NewInstanceData("Test task")
	inst.WorktreePath = "/tmp/wt"
	inst.Branch = "feature/test"
	sess.Instances = append(sess.Instances, inst)

	// Save session
	if err := mgr.SaveSession(sess); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Create new manager to load session
	mgr2 := orchsession.NewManager(orchsession.Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Load session
	loadedSess, err := mgr2.LoadSession()
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	if loadedSess.Name != "Test Session" {
		t.Errorf("Expected loaded session name %q, got %q", "Test Session", loadedSess.Name)
	}

	if len(loadedSess.Instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(loadedSess.Instances))
	}

	if loadedSess.Instances[0].Task != "Test task" {
		t.Errorf("Expected task %q, got %q", "Test task", loadedSess.Instances[0].Task)
	}

	// Delete session
	if err := mgr2.DeleteSession(); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	if mgr2.Exists() {
		t.Error("Session should not exist after deletion")
	}
}

// TestSessionManagerContextFile tests the context file writing functionality.
func TestSessionManagerContextFile(t *testing.T) {
	tempDir := t.TempDir()

	mgr := orchsession.NewManager(orchsession.Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	contextContent := `# Claudio Session Context

## Active Instances

### Instance 1
- Task: Test task
- Branch: feature/test
`

	if err := mgr.WriteContext(contextContent); err != nil {
		t.Fatalf("Failed to write context: %v", err)
	}

	// Verify context file path
	contextPath := mgr.ContextFilePath()
	if contextPath == "" {
		t.Error("Expected non-empty context file path")
	}
}

// TestSessionManagerErrorPaths tests error handling for session operations.
func TestSessionManagerErrorPaths(t *testing.T) {
	t.Run("LoadNonExistentSession", func(t *testing.T) {
		tempDir := t.TempDir()

		mgr := orchsession.NewManager(orchsession.Config{
			BaseDir:   tempDir,
			SessionID: "non-existent",
		})

		if err := mgr.Init(); err != nil {
			t.Fatalf("Failed to initialize: %v", err)
		}

		// Loading a session that doesn't exist should return an error
		_, err := mgr.LoadSession()
		if err == nil {
			t.Error("Expected error when loading non-existent session")
		}
	})

	t.Run("LoadCorruptedSession", func(t *testing.T) {
		tempDir := t.TempDir()

		mgr := orchsession.NewManager(orchsession.Config{
			BaseDir:   tempDir,
			SessionID: "corrupted",
		})

		if err := mgr.Init(); err != nil {
			t.Fatalf("Failed to initialize: %v", err)
		}

		// Write invalid JSON to the session file
		sessionFile := mgr.SessionFilePath()
		if err := os.WriteFile(sessionFile, []byte("{ invalid json content"), 0644); err != nil {
			t.Fatalf("Failed to write corrupted file: %v", err)
		}

		// Loading corrupted session should return an error
		_, err := mgr.LoadSession()
		if err == nil {
			t.Error("Expected error when loading corrupted session file")
		}
	})

	t.Run("LoadEmptySession", func(t *testing.T) {
		tempDir := t.TempDir()

		mgr := orchsession.NewManager(orchsession.Config{
			BaseDir:   tempDir,
			SessionID: "empty",
		})

		if err := mgr.Init(); err != nil {
			t.Fatalf("Failed to initialize: %v", err)
		}

		// Write empty file
		sessionFile := mgr.SessionFilePath()
		if err := os.WriteFile(sessionFile, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		// Loading empty session should return an error
		_, err := mgr.LoadSession()
		if err == nil {
			t.Error("Expected error when loading empty session file")
		}
	})
}

// TestEventTypesIntegration tests all event types can be created and
// have proper timestamps.
func TestEventTypesIntegration(t *testing.T) {
	before := time.Now()

	events := []event.Event{
		event.NewInstanceStartedEvent("id", "/path", "branch", "task"),
		event.NewInstanceStoppedEvent("id", true, "reason"),
		event.NewPRCompleteEvent("id", true, "url", ""),
		event.NewPROpenedEvent("id", "url"),
		event.NewConflictDetectedEvent("id", "branch", []string{"file"}),
		event.NewTimeoutEvent("id", event.TimeoutActivity, "5m"),
		event.NewTimeoutEvent("id", event.TimeoutCompletion, "30m"),
		event.NewTimeoutEvent("id", event.TimeoutStale, "5m"),
		event.NewTaskCompletedEvent("task", "inst", true, ""),
		event.NewPhaseChangeEvent("session", event.PhasePlanning, event.PhaseExecuting),
		event.NewMetricsUpdateEvent("id", 100, 50, 10, 5, 0.01, 1),
		event.NewBellEvent("id"),
	}

	after := time.Now()

	for i, e := range events {
		if e.EventType() == "" {
			t.Errorf("Event %d has empty type", i)
		}

		ts := e.Timestamp()
		if ts.Before(before) || ts.After(after) {
			t.Errorf("Event %d timestamp %v not in expected range [%v, %v]", i, ts, before, after)
		}
	}
}

// TestTimeoutTypeStrings tests the TimeoutType String method.
func TestTimeoutTypeStrings(t *testing.T) {
	tests := []struct {
		tt   event.TimeoutType
		want string
	}{
		{event.TimeoutActivity, "activity"},
		{event.TimeoutCompletion, "completion"},
		{event.TimeoutStale, "stale"},
		{event.TimeoutType(99), "unknown"},
	}

	for _, tc := range tests {
		if got := tc.tt.String(); got != tc.want {
			t.Errorf("TimeoutType(%d).String() = %q, want %q", tc.tt, got, tc.want)
		}
	}
}

// TestPhaseConstants tests that phase constants match expected values.
func TestPhaseConstants(t *testing.T) {
	tests := []struct {
		phase event.Phase
		want  string
	}{
		{event.PhasePlanning, "planning"},
		{event.PhasePlanSelection, "plan_selection"},
		{event.PhaseRefresh, "context_refresh"},
		{event.PhaseExecuting, "executing"},
		{event.PhaseSynthesis, "synthesis"},
		{event.PhaseRevision, "revision"},
		{event.PhaseConsolidating, "consolidating"},
		{event.PhaseComplete, "complete"},
		{event.PhaseFailed, "failed"},
	}

	for _, tc := range tests {
		if string(tc.phase) != tc.want {
			t.Errorf("Phase %v != %q", tc.phase, tc.want)
		}
	}
}

// TestMetricsEventTotalTokens tests the TotalTokens helper method.
func TestMetricsEventTotalTokens(t *testing.T) {
	e := event.NewMetricsUpdateEvent("id", 1000, 500, 100, 50, 0.05, 5)

	if total := e.TotalTokens(); total != 1500 {
		t.Errorf("Expected TotalTokens 1500, got %d", total)
	}
}

// TestSubcomponentComposition tests that all subcomponents can be created
// and work together in the composition pattern.
func TestSubcomponentComposition(t *testing.T) {
	tempDir := t.TempDir()

	// Create all subcomponents
	eventBus := event.NewBus()
	sessionMgr := orchsession.NewManager(orchsession.Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})
	lifecycleMgr := lifecycle.NewManager(
		lifecycle.DefaultConfig(),
		lifecycle.Callbacks{},
		nil,
	)

	// Verify they can be used together
	if eventBus == nil {
		t.Error("EventBus should not be nil")
	}
	if sessionMgr == nil {
		t.Error("SessionManager should not be nil")
	}
	if lifecycleMgr == nil {
		t.Error("LifecycleManager should not be nil")
	}

	// Initialize session manager
	if err := sessionMgr.Init(); err != nil {
		t.Fatalf("Failed to initialize session manager: %v", err)
	}

	// Create a session
	sess, err := sessionMgr.CreateSession("Test", tempDir)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Wire up lifecycle manager to publish events
	var eventCount int
	var mu sync.Mutex

	eventBus.SubscribeAll(func(e event.Event) {
		mu.Lock()
		eventCount++
		mu.Unlock()
	})

	// Create instance in lifecycle manager
	_, err = lifecycleMgr.CreateInstance("inst-1", "/tmp/wt", "branch", "task")
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Publish event manually (simulating orchestrator behavior)
	eventBus.Publish(event.NewInstanceStartedEvent("inst-1", "/tmp/wt", "branch", "task"))

	mu.Lock()
	count := eventCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}

	// Verify session has correct ID
	if sess.ID == "" {
		t.Error("Session should have an ID")
	}

	// Clean up
	lifecycleMgr.Stop()
	_ = sessionMgr.DeleteSession()
}

// TestEventBusUnsubscribe tests that unsubscribed handlers stop receiving events.
func TestEventBusUnsubscribe(t *testing.T) {
	bus := event.NewBus()

	var count1, count2 int
	var mu sync.Mutex

	id1 := bus.Subscribe("instance.bell", func(e event.Event) {
		mu.Lock()
		count1++
		mu.Unlock()
	})

	bus.Subscribe("instance.bell", func(e event.Event) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	// Publish first event - both handlers should receive it
	bus.Publish(event.NewBellEvent("test"))

	mu.Lock()
	if count1 != 1 {
		t.Errorf("Handler 1 should be called once before unsubscribe, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("Handler 2 should be called once, got %d", count2)
	}
	mu.Unlock()

	// Unsubscribe first handler
	if !bus.Unsubscribe(id1) {
		t.Error("Unsubscribe should return true")
	}

	// Publish second event - only handler 2 should receive it
	bus.Publish(event.NewBellEvent("test"))

	mu.Lock()
	defer mu.Unlock()

	// First handler should have been called exactly once (before unsubscribe)
	if count1 != 1 {
		t.Errorf("Handler 1 should be called exactly once, got %d", count1)
	}
	// Second handler should have been called twice
	if count2 != 2 {
		t.Errorf("Handler 2 should be called twice, got %d", count2)
	}
}

// TestEventBusSubscriptionOrder tests that handlers are called in registration order.
func TestEventBusSubscriptionOrder(t *testing.T) {
	bus := event.NewBus()

	var order []int
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		i := i
		bus.Subscribe("instance.bell", func(e event.Event) {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		})
	}

	bus.Publish(event.NewBellEvent("test"))

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 5 {
		t.Errorf("Expected 5 handlers called, got %d", len(order))
	}

	for i := 0; i < 5; i++ {
		if order[i] != i {
			t.Errorf("Expected handler %d at position %d, got %d", i, i, order[i])
		}
	}
}

// TestLifecycleInstanceDataIntegrity tests that instance data is preserved
// correctly through lifecycle operations.
func TestLifecycleInstanceDataIntegrity(t *testing.T) {
	mgr := lifecycle.NewManager(lifecycle.DefaultConfig(), lifecycle.Callbacks{}, nil)

	// Create instance with specific data
	original, err := mgr.CreateInstance(
		"test-123",
		"/home/user/project/.claudio/worktrees/test-123",
		"claudio/test-123-fix-bug",
		"Fix the authentication bug in the login handler",
	)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Start the instance
	if err := mgr.StartInstance(context.Background(), "test-123"); err != nil {
		t.Fatalf("Failed to start instance: %v", err)
	}

	// Retrieve instance
	retrieved, exists := mgr.GetInstance("test-123")
	if !exists {
		t.Fatal("Instance should exist")
	}

	// Verify all fields
	if retrieved.ID != original.ID {
		t.Errorf("ID mismatch: %q != %q", retrieved.ID, original.ID)
	}
	if retrieved.WorktreePath != original.WorktreePath {
		t.Errorf("WorktreePath mismatch: %q != %q", retrieved.WorktreePath, original.WorktreePath)
	}
	if retrieved.Branch != original.Branch {
		t.Errorf("Branch mismatch: %q != %q", retrieved.Branch, original.Branch)
	}
	if retrieved.Task != original.Task {
		t.Errorf("Task mismatch: %q != %q", retrieved.Task, original.Task)
	}
	if retrieved.Status != lifecycle.StatusWorking {
		t.Errorf("Status should be working, got %q", retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("StartedAt should be set after starting")
	}
}

// TestSessionDataInstanceLookup tests the GetInstance helper method.
func TestSessionDataInstanceLookup(t *testing.T) {
	sess := orchsession.NewSessionData("Test Session", "/tmp/repo")

	// Add some instances
	inst1 := orchsession.NewInstanceData("Task 1")
	inst2 := orchsession.NewInstanceData("Task 2")
	inst3 := orchsession.NewInstanceData("Task 3")

	sess.Instances = append(sess.Instances, inst1, inst2, inst3)

	// Look up existing instance
	found := sess.GetInstance(inst2.ID)
	if found == nil {
		t.Fatal("Should find instance by ID")
	}
	if found.Task != "Task 2" {
		t.Errorf("Expected task %q, got %q", "Task 2", found.Task)
	}

	// Look up non-existent instance
	notFound := sess.GetInstance("non-existent-id")
	if notFound != nil {
		t.Error("Should return nil for non-existent ID")
	}
}

// TestEventBusClear tests that Clear removes all subscriptions.
func TestEventBusClear(t *testing.T) {
	bus := event.NewBus()

	var called bool
	bus.Subscribe("test", func(e event.Event) {
		called = true
	})
	bus.SubscribeAll(func(e event.Event) {
		called = true
	})

	if bus.SubscriptionCount() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", bus.SubscriptionCount())
	}

	bus.Clear()

	if bus.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after clear, got %d", bus.SubscriptionCount())
	}

	// Publish should not call any handlers
	bus.Publish(event.NewBellEvent("test"))

	if called {
		t.Error("Handler should not be called after Clear")
	}
}

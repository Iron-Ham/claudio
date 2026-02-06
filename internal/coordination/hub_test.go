package coordination

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
	"github.com/Iron-Ham/claudio/internal/scaling"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func testPlan(tasks ...ultraplan.PlannedTask) *ultraplan.PlanSpec {
	plan := &ultraplan.PlanSpec{
		ID:        "test-plan",
		Objective: "test objective",
		Tasks:     tasks,
	}
	plan.DependencyGraph = make(map[string][]string)
	for _, t := range tasks {
		plan.DependencyGraph[t.ID] = t.DependsOn
	}
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	plan.ExecutionOrder = [][]string{ids}
	return plan
}

func TestNewHub(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{
		ID:    "task-1",
		Title: "Test Task",
	})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	})
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	if hub.Gate() == nil {
		t.Error("Gate() is nil")
	}
	if hub.EventQueue() == nil {
		t.Error("EventQueue() is nil")
	}
	if hub.TaskQueue() == nil {
		t.Error("TaskQueue() is nil")
	}
	if hub.Lead() == nil {
		t.Error("Lead() is nil")
	}
	if hub.ScalingMonitor() == nil {
		t.Error("ScalingMonitor() is nil")
	}
	if hub.Propagator() == nil {
		t.Error("Propagator() is nil")
	}
	if hub.FileLockRegistry() == nil {
		t.Error("FileLockRegistry() is nil")
	}
	if hub.Mailbox() == nil {
		t.Error("Mailbox() is nil")
	}
	if hub.Running() {
		t.Error("Running() = true before Start")
	}
}

func TestNewHub_Validation(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing Bus",
			cfg:  Config{Bus: nil, SessionDir: dir, Plan: plan},
		},
		{
			name: "missing Plan",
			cfg:  Config{Bus: bus, SessionDir: dir, Plan: nil},
		},
		{
			name: "missing SessionDir",
			cfg:  Config{Bus: bus, SessionDir: "", Plan: plan},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHub(tt.cfg)
			if err == nil {
				t.Fatalf("NewHub() expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestNewHub_DefaultTaskLookup(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{
		ID:               "task-1",
		Title:            "Approval Task",
		RequiresApproval: true,
	})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
		TaskLookup: nil, // default: no approval required
	})
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	// Claim a task and try to mark running through the gate.
	// The default lookup returns (false, true), so it should pass through.
	task, err := hub.Gate().ClaimNext("instance-1")
	if err != nil {
		t.Fatalf("ClaimNext() error = %v", err)
	}
	if task == nil {
		t.Fatal("ClaimNext() returned nil")
	}

	err = hub.Gate().MarkRunning(task.ID)
	if err != nil {
		t.Fatalf("MarkRunning() error = %v, want nil (default lookup skips approval)", err)
	}
}

func TestNewHub_Options(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	policy := scaling.NewPolicy(scaling.WithMaxInstances(20))

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	},
		WithScalingPolicy(policy),
		WithMaxTasksPerInstance(5),
		WithStaleClaimTimeout(10*time.Second),
		WithRebalanceInterval(5*time.Second),
		WithInitialInstances(3),
	)
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	// Verify the hub was created successfully with options applied.
	// We can't inspect internal config directly, but we verify the hub works.
	if hub == nil {
		t.Fatal("NewHub() returned nil hub")
	}
}

func TestHub_Start(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1)) // disable periodic rebalance for test speed
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	ctx := context.Background()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !hub.Running() {
		t.Error("Running() = false after Start")
	}

	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestHub_StartAlreadyStarted(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	ctx := context.Background()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = hub.Stop() }()

	err = hub.Start(ctx)
	if err == nil {
		t.Fatal("Start() expected error on double start, got nil")
	}
}

func TestHub_Stop(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	ctx := context.Background()
	if err := hub.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if hub.Running() {
		t.Error("Running() = true after Stop")
	}
}

func TestHub_StopIdempotent(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	if err := hub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := hub.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := hub.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	if hub.Running() {
		t.Error("Running() = true after double Stop")
	}
}

func TestHub_StopWithoutStart(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	})
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}

	// Should not panic or error.
	if err := hub.Stop(); err != nil {
		t.Fatalf("Stop() without Start error = %v", err)
	}
}

func TestHub_EndToEnd_TaskClaim(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(
		ultraplan.PlannedTask{ID: "task-1", Title: "First", Priority: 0},
	)

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}
	if err := hub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = hub.Stop() }()

	// Track events.
	var mu sync.Mutex
	var claimedEvents []event.TaskClaimedEvent
	var depthEvents []event.QueueDepthChangedEvent

	bus.Subscribe("queue.task_claimed", func(e event.Event) {
		if ce, ok := e.(event.TaskClaimedEvent); ok {
			mu.Lock()
			claimedEvents = append(claimedEvents, ce)
			mu.Unlock()
		}
	})
	bus.Subscribe("queue.depth_changed", func(e event.Event) {
		if de, ok := e.(event.QueueDepthChangedEvent); ok {
			mu.Lock()
			depthEvents = append(depthEvents, de)
			mu.Unlock()
		}
	})

	// Claim task through gate.
	task, err := hub.Gate().ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext() error = %v", err)
	}
	if task == nil {
		t.Fatal("ClaimNext() returned nil")
	}
	if task.ID != "task-1" {
		t.Errorf("ClaimNext() task.ID = %q, want %q", task.ID, "task-1")
	}

	// Mark running and complete.
	if err := hub.Gate().MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	unblocked, err := hub.Gate().Complete(task.ID)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	// No downstream tasks to unblock.
	if len(unblocked) != 0 {
		t.Errorf("Complete() unblocked = %v, want empty", unblocked)
	}

	// Verify events were published.
	mu.Lock()
	defer mu.Unlock()

	if len(claimedEvents) != 1 {
		t.Errorf("claimed events = %d, want 1", len(claimedEvents))
	}
	if len(claimedEvents) == 1 && claimedEvents[0].TaskID != "task-1" {
		t.Errorf("claimed event TaskID = %q, want %q", claimedEvents[0].TaskID, "task-1")
	}
	if len(depthEvents) == 0 {
		t.Error("depth changed events = 0, want > 0")
	}
}

func TestHub_EndToEnd_FileLock(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}
	if err := hub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = hub.Stop() }()

	// Track file claim events.
	claimCh := make(chan event.FileClaimEvent, 1)
	releaseCh := make(chan event.FileReleaseEvent, 1)
	bus.Subscribe("filelock.claimed", func(e event.Event) {
		if ce, ok := e.(event.FileClaimEvent); ok {
			claimCh <- ce
		}
	})
	bus.Subscribe("filelock.released", func(e event.Event) {
		if re, ok := e.(event.FileReleaseEvent); ok {
			releaseCh <- re
		}
	})

	// Claim a file.
	if err := hub.FileLockRegistry().Claim("inst-1", "main.go"); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	// Verify file is claimed.
	owner, ok := hub.FileLockRegistry().Owner("main.go")
	if !ok || owner != "inst-1" {
		t.Errorf("Owner(main.go) = %q, %v; want %q, true", owner, ok, "inst-1")
	}

	// Verify claim event.
	select {
	case ce := <-claimCh:
		if ce.FilePath != "main.go" || ce.InstanceID != "inst-1" {
			t.Errorf("FileClaimEvent = {%q, %q}, want {%q, %q}", ce.InstanceID, ce.FilePath, "inst-1", "main.go")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for FileClaimEvent")
	}

	// Release the file.
	if err := hub.FileLockRegistry().Release("inst-1", "main.go"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Verify file is released.
	if !hub.FileLockRegistry().IsAvailable("main.go") {
		t.Error("IsAvailable(main.go) = false after Release")
	}

	select {
	case re := <-releaseCh:
		if re.FilePath != "main.go" || re.InstanceID != "inst-1" {
			t.Errorf("FileReleaseEvent = {%q, %q}, want {%q, %q}", re.InstanceID, re.FilePath, "inst-1", "main.go")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for FileReleaseEvent")
	}
}

func TestHub_EndToEnd_ContextPropagation(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()
	plan := testPlan(ultraplan.PlannedTask{ID: "t1", Title: "T"})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	}, WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}
	if err := hub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = hub.Stop() }()

	// Track propagation events.
	propCh := make(chan event.ContextPropagatedEvent, 1)
	bus.Subscribe("context.propagated", func(e event.Event) {
		if pe, ok := e.(event.ContextPropagatedEvent); ok {
			propCh <- pe
		}
	})

	// Share a discovery.
	err = hub.Propagator().ShareDiscovery("inst-1", "found a pattern", map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("ShareDiscovery() error = %v", err)
	}

	select {
	case pe := <-propCh:
		if pe.From != "inst-1" {
			t.Errorf("ContextPropagatedEvent.From = %q, want %q", pe.From, "inst-1")
		}
		if pe.MessageType != "discovery" {
			t.Errorf("ContextPropagatedEvent.MessageType = %q, want %q", pe.MessageType, "discovery")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ContextPropagatedEvent")
	}

	// Verify propagated message is retrievable.
	got, err := hub.Propagator().GetContextForInstance("inst-2", mailbox.FilterOptions{})
	if err != nil {
		t.Fatalf("GetContextForInstance() error = %v", err)
	}
	if got == "" {
		t.Error("GetContextForInstance() returned empty string, want message content")
	}
}

func TestHub_EndToEnd_ScalingDecision(t *testing.T) {
	bus := event.NewBus()
	dir := t.TempDir()

	// Create a plan with enough pending tasks to trigger scale-up.
	plan := testPlan(
		ultraplan.PlannedTask{ID: "t1", Title: "T1"},
		ultraplan.PlannedTask{ID: "t2", Title: "T2"},
		ultraplan.PlannedTask{ID: "t3", Title: "T3"},
		ultraplan.PlannedTask{ID: "t4", Title: "T4"},
	)

	// Use a policy with zero cooldown so decisions fire immediately.
	policy := scaling.NewPolicy(
		scaling.WithCooldownPeriod(0),
		scaling.WithScaleUpThreshold(0),
	)

	// Track scaling decisions — subscribe BEFORE hub.Start so the handler
	// is in place before the monitor's subscription.
	decisionCh := make(chan event.ScalingDecisionEvent, 5)
	bus.Subscribe("scaling.decision", func(e event.Event) {
		if de, ok := e.(event.ScalingDecisionEvent); ok {
			decisionCh <- de
		}
	})

	hub, err := NewHub(Config{
		Bus:        bus,
		SessionDir: dir,
		Plan:       plan,
	},
		WithScalingPolicy(policy),
		WithInitialInstances(1),
		WithRebalanceInterval(-1),
	)
	if err != nil {
		t.Fatalf("NewHub() error = %v", err)
	}
	if err := hub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = hub.Stop() }()

	// The monitor goroutine must subscribe before we trigger events.
	// The lead subscribes synchronously in Start (4 subscriptions), but the
	// monitor runs in a goroutine and subscribes at the top of its Start.
	// Wait until the monitor's subscription appears on the bus.
	// Before hub.Start: 1 (our decisionCh handler)
	// After lead.Start: +4 (lead's handlers)
	// After monitor.Start goroutine subscribes: +1 = 6 total
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if bus.SubscriptionCount() >= 6 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Claim a task to change queue depth, which triggers the monitor.
	_, err = hub.Gate().ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext() error = %v", err)
	}

	// The QueueDepthChangedEvent from ClaimNext triggers the scaling monitor.
	// Wait for a scaling decision.
	select {
	case de := <-decisionCh:
		if de.Action != "scale_up" {
			t.Errorf("ScalingDecisionEvent.Action = %q, want %q", de.Action, "scale_up")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ScalingDecisionEvent")
	}
}

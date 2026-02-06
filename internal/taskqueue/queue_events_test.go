package taskqueue

import (
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func makeEventPlan() *ultraplan.PlanSpec {
	return &ultraplan.PlanSpec{
		ID: "event-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "t1",
				Title:         "Task 1",
				Description:   "First",
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
			{
				ID:            "t2",
				Title:         "Task 2",
				Description:   "Second",
				DependsOn:     []string{"t1"},
				EstComplexity: ultraplan.ComplexityMedium,
			},
		},
	}
}

type eventCollector struct {
	mu     sync.Mutex
	events []event.Event
}

func (c *eventCollector) handler(e event.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *eventCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func (c *eventCollector) findByType(eventType string) []event.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	var found []event.Event
	for _, e := range c.events {
		if e.EventType() == eventType {
			found = append(found, e)
		}
	}
	return found
}

func TestEventQueue_ClaimNext(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	task, err := eq.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task == nil {
		t.Fatal("expected a task")
	}

	claimed := col.findByType("queue.task_claimed")
	if len(claimed) != 1 {
		t.Fatalf("expected 1 TaskClaimedEvent, got %d", len(claimed))
	}
	ce := claimed[0].(event.TaskClaimedEvent)
	if ce.TaskID != task.ID {
		t.Errorf("TaskClaimedEvent.TaskID = %q, want %q", ce.TaskID, task.ID)
	}
	if ce.InstanceID != "inst-1" {
		t.Errorf("TaskClaimedEvent.InstanceID = %q, want inst-1", ce.InstanceID)
	}

	depth := col.findByType("queue.depth_changed")
	if len(depth) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depth))
	}
	de := depth[0].(event.QueueDepthChangedEvent)
	if de.Claimed != 1 {
		t.Errorf("QueueDepthChangedEvent.Claimed = %d, want 1", de.Claimed)
	}
}

func TestEventQueue_ClaimNext_NilReturnsNoEvents(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	plan := &ultraplan.PlanSpec{
		ID: "empty",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", DependsOn: []string{"t2"}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t2", DependsOn: []string{"t1"}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)
	eq := NewEventQueue(q, bus)

	// Circular deps means nothing claimable. ClaimNext should return nil
	// and no events should be published.
	task, err := eq.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil task, got %q", task.ID)
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events, got %d", col.count())
	}
}

func TestEventQueue_MarkRunning(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)
	task, _ := eq.ClaimNext("inst-1")

	// Clear collector
	*col = eventCollector{}

	if err := eq.MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}

	depth := col.findByType("queue.depth_changed")
	if len(depth) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depth))
	}
	de := depth[0].(event.QueueDepthChangedEvent)
	if de.Running != 1 {
		t.Errorf("Running = %d, want 1", de.Running)
	}
}

func TestEventQueue_MarkRunning_Error(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	err := eq.MarkRunning("t1") // still pending, not claimed
	if err == nil {
		t.Error("expected error for pending task")
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events on error, got %d", col.count())
	}
}

func TestEventQueue_Complete(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)
	task, _ := eq.ClaimNext("inst-1")
	_ = eq.MarkRunning(task.ID)

	*col = eventCollector{}

	unblocked, err := eq.Complete(task.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(unblocked) != 1 || unblocked[0] != "t2" {
		t.Errorf("unblocked = %v, want [t2]", unblocked)
	}

	depth := col.findByType("queue.depth_changed")
	if len(depth) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depth))
	}
	de := depth[0].(event.QueueDepthChangedEvent)
	if de.Completed != 1 {
		t.Errorf("Completed = %d, want 1", de.Completed)
	}
}

func TestEventQueue_Complete_Error(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	_, err := eq.Complete("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events on error, got %d", col.count())
	}
}

func TestEventQueue_Fail(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)
	task, _ := eq.ClaimNext("inst-1")
	_ = eq.MarkRunning(task.ID)

	*col = eventCollector{}

	if err := eq.Fail(task.ID, "crash"); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	depth := col.findByType("queue.depth_changed")
	if len(depth) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depth))
	}
}

func TestEventQueue_Fail_Error(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	err := eq.Fail("nonexistent", "err")
	if err == nil {
		t.Error("expected error")
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events on error, got %d", col.count())
	}
}

func TestEventQueue_Release(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)
	task, _ := eq.ClaimNext("inst-1")

	*col = eventCollector{}

	if err := eq.Release(task.ID, "instance_died"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	released := col.findByType("queue.task_released")
	if len(released) != 1 {
		t.Fatalf("expected 1 TaskReleasedEvent, got %d", len(released))
	}
	re := released[0].(event.TaskReleasedEvent)
	if re.TaskID != task.ID {
		t.Errorf("TaskReleasedEvent.TaskID = %q, want %q", re.TaskID, task.ID)
	}
	if re.Reason != "instance_died" {
		t.Errorf("TaskReleasedEvent.Reason = %q, want instance_died", re.Reason)
	}

	depth := col.findByType("queue.depth_changed")
	if len(depth) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depth))
	}
}

func TestEventQueue_Release_Error(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	err := eq.Release("nonexistent", "reason")
	if err == nil {
		t.Error("expected error")
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events on error, got %d", col.count())
	}
}

func TestEventQueue_Passthrough(t *testing.T) {
	bus := event.NewBus()
	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)

	// Status
	s := eq.Status()
	if s.Total != 2 {
		t.Errorf("Status().Total = %d, want 2", s.Total)
	}

	// IsComplete
	if eq.IsComplete() {
		t.Error("IsComplete should be false")
	}

	// GetTask
	task := eq.GetTask("t1")
	if task == nil {
		t.Error("GetTask returned nil")
	}

	// GetInstanceTasks
	_, _ = eq.ClaimNext("inst-1")
	tasks := eq.GetInstanceTasks("inst-1")
	if len(tasks) != 1 {
		t.Errorf("GetInstanceTasks = %d, want 1", len(tasks))
	}

	// SaveState
	dir := t.TempDir()
	if err := eq.SaveState(dir); err != nil {
		t.Errorf("SaveState: %v", err)
	}
}

func TestEventQueue_ClaimStaleBefore(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	plan := &ultraplan.PlanSpec{
		ID: "stale-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t2", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := NewFromPlan(plan)
	eq := NewEventQueue(q, bus)

	// Claim both tasks
	_, _ = eq.ClaimNext("inst-1")
	_, _ = eq.ClaimNext("inst-2")

	// Manually set task-1 claimed time to the past
	q.mu.Lock()
	past := time.Now().Add(-10 * time.Minute)
	q.tasks["t1"].ClaimedAt = &past
	q.mu.Unlock()

	*col = eventCollector{}

	// Release claims older than 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	released := eq.ClaimStaleBefore(cutoff)

	if len(released) != 1 || released[0] != "t1" {
		t.Errorf("released = %v, want [t1]", released)
	}

	// Check task is back to pending
	task := eq.GetTask("t1")
	if task.Status != TaskPending {
		t.Errorf("t1 status = %s, want pending", task.Status)
	}

	// Check events
	releasedEvents := col.findByType("queue.task_released")
	if len(releasedEvents) != 1 {
		t.Fatalf("expected 1 TaskReleasedEvent, got %d", len(releasedEvents))
	}
	re := releasedEvents[0].(event.TaskReleasedEvent)
	if re.Reason != "stale_claim" {
		t.Errorf("Reason = %q, want stale_claim", re.Reason)
	}

	depthEvents := col.findByType("queue.depth_changed")
	if len(depthEvents) != 1 {
		t.Fatalf("expected 1 QueueDepthChangedEvent, got %d", len(depthEvents))
	}
}

func TestEventQueue_ClaimStaleBefore_NoneStale(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	q := NewFromPlan(makeEventPlan())
	eq := NewEventQueue(q, bus)
	_, _ = eq.ClaimNext("inst-1")

	*col = eventCollector{}

	// All claims are fresh
	cutoff := time.Now().Add(-5 * time.Minute)
	released := eq.ClaimStaleBefore(cutoff)

	if len(released) != 0 {
		t.Errorf("released = %v, want empty", released)
	}
	if col.count() != 0 {
		t.Errorf("expected 0 events, got %d", col.count())
	}
}

func TestNewEventTypes_SatisfyInterface(t *testing.T) {
	// Verify the constructors produce valid events
	claimed := event.NewTaskClaimedEvent("task-1", "inst-1")
	if claimed.EventType() != "queue.task_claimed" {
		t.Errorf("TaskClaimedEvent.EventType() = %q", claimed.EventType())
	}
	if claimed.Timestamp().IsZero() {
		t.Error("TaskClaimedEvent timestamp should not be zero")
	}

	released := event.NewTaskReleasedEvent("task-1", "stale")
	if released.EventType() != "queue.task_released" {
		t.Errorf("TaskReleasedEvent.EventType() = %q", released.EventType())
	}

	depth := event.NewQueueDepthChangedEvent(1, 2, 3, 4, 5, 15)
	if depth.EventType() != "queue.depth_changed" {
		t.Errorf("QueueDepthChangedEvent.EventType() = %q", depth.EventType())
	}
	if depth.Pending != 1 || depth.Claimed != 2 || depth.Running != 3 ||
		depth.Completed != 4 || depth.Failed != 5 || depth.Total != 15 {
		t.Errorf("QueueDepthChangedEvent fields incorrect: %+v", depth)
	}
}

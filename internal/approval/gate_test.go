package approval

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// eventCollector gathers events from the bus for assertions.
type eventCollector struct {
	mu     sync.Mutex
	events []event.Event
}

func (c *eventCollector) handler(e event.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
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

func (c *eventCollector) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = nil
}

// makePlan creates a test plan with two tasks: t1 (requires approval) and t2 (no approval).
func makePlan() *ultraplan.PlanSpec {
	return &ultraplan.PlanSpec{
		ID: "approval-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:               "t1",
				Title:            "Task 1",
				Description:      "Requires approval",
				DependsOn:        []string{},
				EstComplexity:    ultraplan.ComplexityMedium,
				RequiresApproval: true,
			},
			{
				ID:            "t2",
				Title:         "Task 2",
				Description:   "No approval needed",
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
		},
	}
}

// makeLookup creates a TaskLookup from a plan.
func makeLookup(plan *ultraplan.PlanSpec) TaskLookup {
	tasks := make(map[string]bool)
	for _, t := range plan.Tasks {
		tasks[t.ID] = t.RequiresApproval
	}
	return func(taskID string) (bool, bool) {
		req, exists := tasks[taskID]
		return req, exists
	}
}

// setupGate creates a Gate with the default test plan and returns all components.
func setupGate(t *testing.T) (*Gate, *eventCollector) {
	t.Helper()
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	plan := makePlan()
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))
	return gate, col
}

func TestGate_MarkRunning_RequiresApproval(t *testing.T) {
	gate, col := setupGate(t)

	task, err := gate.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	// The claimed task could be t1 or t2 depending on priority order.
	// Claim both to find t1.
	task2, err := gate.ClaimNext("inst-2")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}

	var approvalTask *taskqueue.QueuedTask
	if task.RequiresApproval {
		approvalTask = task
	} else {
		approvalTask = task2
	}

	col.reset()

	// MarkRunning on a task requiring approval should NOT actually mark it running
	if err := gate.MarkRunning(approvalTask.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}

	// Should have published TaskAwaitingApprovalEvent
	awaitEvents := col.findByType("queue.task_awaiting_approval")
	if len(awaitEvents) != 1 {
		t.Fatalf("expected 1 TaskAwaitingApprovalEvent, got %d", len(awaitEvents))
	}
	ae := awaitEvents[0].(event.TaskAwaitingApprovalEvent)
	if ae.TaskID != approvalTask.ID {
		t.Errorf("TaskAwaitingApprovalEvent.TaskID = %q, want %q", ae.TaskID, approvalTask.ID)
	}

	// Task should be awaiting approval
	if !gate.IsAwaitingApproval(approvalTask.ID) {
		t.Error("expected task to be awaiting approval")
	}

	// GetTask should reflect awaiting_approval status
	got := gate.GetTask(approvalTask.ID)
	if got.Status != taskqueue.TaskAwaitingApproval {
		t.Errorf("GetTask status = %q, want %q", got.Status, taskqueue.TaskAwaitingApproval)
	}
}

func TestGate_MarkRunning_NoApproval(t *testing.T) {
	gate, col := setupGate(t)

	// Claim both tasks
	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var noApprovalTask *taskqueue.QueuedTask
	if !task1.RequiresApproval {
		noApprovalTask = task1
	} else {
		noApprovalTask = task2
	}

	col.reset()

	// MarkRunning on a task not requiring approval should pass through
	if err := gate.MarkRunning(noApprovalTask.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}

	// Should NOT have published TaskAwaitingApprovalEvent
	awaitEvents := col.findByType("queue.task_awaiting_approval")
	if len(awaitEvents) != 0 {
		t.Errorf("expected 0 TaskAwaitingApprovalEvent, got %d", len(awaitEvents))
	}

	// Should have published QueueDepthChangedEvent (from the underlying EventQueue)
	depthEvents := col.findByType("queue.depth_changed")
	if len(depthEvents) != 1 {
		t.Errorf("expected 1 QueueDepthChangedEvent, got %d", len(depthEvents))
	}

	if gate.IsAwaitingApproval(noApprovalTask.ID) {
		t.Error("task should NOT be awaiting approval")
	}
}

func TestGate_MarkRunning_TaskNotFound(t *testing.T) {
	gate, _ := setupGate(t)

	err := gate.MarkRunning("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("error = %v, want ErrTaskNotFound", err)
	}
}

func TestGate_MarkRunning_InvalidTransition(t *testing.T) {
	gate, _ := setupGate(t)

	// t1 is pending, not claimed — MarkRunning should fail
	err := gate.MarkRunning("t1")
	if err == nil {
		t.Fatal("expected error for pending task")
	}
	if !errors.Is(err, taskqueue.ErrInvalidTransition) {
		t.Errorf("error = %v, want ErrInvalidTransition", err)
	}
}

func TestGate_Approve(t *testing.T) {
	gate, col := setupGate(t)

	// Claim and gate t1 (requires approval)
	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var approvalTask *taskqueue.QueuedTask
	if task1.RequiresApproval {
		approvalTask = task1
	} else {
		approvalTask = task2
	}

	_ = gate.MarkRunning(approvalTask.ID)
	col.reset()

	// Approve
	if err := gate.Approve(approvalTask.ID); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Should no longer be awaiting approval
	if gate.IsAwaitingApproval(approvalTask.ID) {
		t.Error("task should no longer be awaiting approval")
	}

	// GetTask should now show running
	got := gate.GetTask(approvalTask.ID)
	if got.Status != taskqueue.TaskRunning {
		t.Errorf("status = %q, want running", got.Status)
	}

	// Should have published QueueDepthChangedEvent from the underlying MarkRunning
	depthEvents := col.findByType("queue.depth_changed")
	if len(depthEvents) != 1 {
		t.Errorf("expected 1 QueueDepthChangedEvent, got %d", len(depthEvents))
	}
}

func TestGate_Approve_NotAwaiting(t *testing.T) {
	gate, _ := setupGate(t)

	err := gate.Approve("t1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotAwaitingApproval) {
		t.Errorf("error = %v, want ErrNotAwaitingApproval", err)
	}
}

func TestGate_Reject(t *testing.T) {
	gate, col := setupGate(t)

	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var approvalTask *taskqueue.QueuedTask
	if task1.RequiresApproval {
		approvalTask = task1
	} else {
		approvalTask = task2
	}

	_ = gate.MarkRunning(approvalTask.ID)
	col.reset()

	// Reject
	if err := gate.Reject(approvalTask.ID, "too risky"); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	if gate.IsAwaitingApproval(approvalTask.ID) {
		t.Error("task should no longer be awaiting approval")
	}

	// Should have published QueueDepthChangedEvent from the underlying Fail
	depthEvents := col.findByType("queue.depth_changed")
	if len(depthEvents) != 1 {
		t.Errorf("expected 1 QueueDepthChangedEvent, got %d", len(depthEvents))
	}
}

func TestGate_Reject_NotAwaiting(t *testing.T) {
	gate, _ := setupGate(t)

	err := gate.Reject("t1", "reason")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotAwaitingApproval) {
		t.Errorf("error = %v, want ErrNotAwaitingApproval", err)
	}
}

func TestGate_PendingApprovals(t *testing.T) {
	gate, _ := setupGate(t)

	// Initially empty
	pending := gate.PendingApprovals()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}

	// Claim and gate t1
	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var approvalTask *taskqueue.QueuedTask
	if task1.RequiresApproval {
		approvalTask = task1
	} else {
		approvalTask = task2
	}

	_ = gate.MarkRunning(approvalTask.ID)

	pending = gate.PendingApprovals()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0] != approvalTask.ID {
		t.Errorf("pending[0] = %q, want %q", pending[0], approvalTask.ID)
	}

	// After approve, should be empty
	_ = gate.Approve(approvalTask.ID)
	pending = gate.PendingApprovals()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after approve, got %d", len(pending))
	}
}

func TestGate_Status_AdjustsCounts(t *testing.T) {
	gate, _ := setupGate(t)

	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var approvalTask *taskqueue.QueuedTask
	if task1.RequiresApproval {
		approvalTask = task1
	} else {
		approvalTask = task2
	}

	// Before gating: both claimed
	s := gate.Status()
	if s.Claimed != 2 {
		t.Errorf("Claimed = %d, want 2", s.Claimed)
	}
	if s.AwaitingApproval != 0 {
		t.Errorf("AwaitingApproval = %d, want 0", s.AwaitingApproval)
	}

	// Gate the approval task
	_ = gate.MarkRunning(approvalTask.ID)

	s = gate.Status()
	if s.Claimed != 1 {
		t.Errorf("Claimed = %d, want 1", s.Claimed)
	}
	if s.AwaitingApproval != 1 {
		t.Errorf("AwaitingApproval = %d, want 1", s.AwaitingApproval)
	}
}

func TestGate_Release_CleansUpPending(t *testing.T) {
	gate, _ := setupGate(t)

	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var approvalTask *taskqueue.QueuedTask
	if task1.RequiresApproval {
		approvalTask = task1
	} else {
		approvalTask = task2
	}

	_ = gate.MarkRunning(approvalTask.ID)

	if !gate.IsAwaitingApproval(approvalTask.ID) {
		t.Fatal("expected awaiting approval before release")
	}

	if err := gate.Release(approvalTask.ID, "instance_died"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if gate.IsAwaitingApproval(approvalTask.ID) {
		t.Error("pending approval should be cleaned up after release")
	}
}

func TestGate_ClaimStaleBefore_CleansUpPending(t *testing.T) {
	bus := event.NewBus()
	col := &eventCollector{}
	bus.SubscribeAll(col.handler)

	plan := &ultraplan.PlanSpec{
		ID: "stale-approval-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:               "t1",
				Title:            "Task 1",
				DependsOn:        []string{},
				EstComplexity:    ultraplan.ComplexityLow,
				RequiresApproval: true,
			},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))

	task, _ := gate.ClaimNext("inst-1")
	_ = gate.MarkRunning(task.ID)

	if !gate.IsAwaitingApproval("t1") {
		t.Fatal("expected awaiting approval")
	}

	// Release stale claims from the future (all claims are stale)
	released := gate.ClaimStaleBefore(time.Now().Add(time.Hour))
	if len(released) != 1 {
		t.Fatalf("released = %v, want [t1]", released)
	}

	if gate.IsAwaitingApproval("t1") {
		t.Error("pending approval should be cleaned up after stale release")
	}
}

func TestGate_Passthrough_Complete(t *testing.T) {
	gate, _ := setupGate(t)

	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var noApprovalTask *taskqueue.QueuedTask
	if !task1.RequiresApproval {
		noApprovalTask = task1
	} else {
		noApprovalTask = task2
	}

	_ = gate.MarkRunning(noApprovalTask.ID)

	unblocked, err := gate.Complete(noApprovalTask.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// No tasks depend on t2 in our plan, so unblocked should be empty
	_ = unblocked
}

func TestGate_Passthrough_Fail(t *testing.T) {
	gate, _ := setupGate(t)

	task1, _ := gate.ClaimNext("inst-1")
	task2, _ := gate.ClaimNext("inst-2")

	var noApprovalTask *taskqueue.QueuedTask
	if !task1.RequiresApproval {
		noApprovalTask = task1
	} else {
		noApprovalTask = task2
	}

	_ = gate.MarkRunning(noApprovalTask.ID)

	if err := gate.Fail(noApprovalTask.ID, "crash"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
}

func TestGate_Passthrough_IsComplete(t *testing.T) {
	gate, _ := setupGate(t)

	if gate.IsComplete() {
		t.Error("should not be complete")
	}
}

func TestGate_Passthrough_GetInstanceTasks(t *testing.T) {
	gate, _ := setupGate(t)

	_, _ = gate.ClaimNext("inst-1")

	tasks := gate.GetInstanceTasks("inst-1")
	if len(tasks) != 1 {
		t.Errorf("GetInstanceTasks = %d, want 1", len(tasks))
	}
}

func TestGate_Passthrough_SaveState(t *testing.T) {
	gate, _ := setupGate(t)
	dir := t.TempDir()
	if err := gate.SaveState(dir); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
}

func TestGate_GetTask_NotAwaiting(t *testing.T) {
	gate, _ := setupGate(t)

	// Not awaiting — just returns underlying task
	task := gate.GetTask("t2")
	if task == nil {
		t.Fatal("expected non-nil task")
	}
	if task.Status != taskqueue.TaskPending {
		t.Errorf("status = %q, want pending", task.Status)
	}
}

func TestGate_GetTask_NotFound(t *testing.T) {
	gate, _ := setupGate(t)

	task := gate.GetTask("nonexistent")
	if task != nil {
		t.Errorf("expected nil for nonexistent task, got %+v", task)
	}
}

func TestGate_MultiplePendingApprovals(t *testing.T) {
	bus := event.NewBus()
	plan := &ultraplan.PlanSpec{
		ID: "multi-approval",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
			{ID: "a2", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
			{ID: "a3", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))

	_, _ = gate.ClaimNext("inst-1")
	_, _ = gate.ClaimNext("inst-2")
	_, _ = gate.ClaimNext("inst-3")

	// Gate both approval tasks
	_ = gate.MarkRunning("a1")
	_ = gate.MarkRunning("a2")
	// Pass through non-approval task
	_ = gate.MarkRunning("a3")

	pending := gate.PendingApprovals()
	sort.Strings(pending)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
	if pending[0] != "a1" || pending[1] != "a2" {
		t.Errorf("pending = %v, want [a1 a2]", pending)
	}

	s := gate.Status()
	if s.AwaitingApproval != 2 {
		t.Errorf("AwaitingApproval = %d, want 2", s.AwaitingApproval)
	}
	if s.Running != 1 {
		t.Errorf("Running = %d, want 1", s.Running)
	}
}

func TestGate_ConcurrentOperations(t *testing.T) {
	bus := event.NewBus()
	plan := &ultraplan.PlanSpec{
		ID: "concurrent-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "c1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
			{ID: "c2", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
			{ID: "c3", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "c4", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))

	var wg sync.WaitGroup

	// Claim all tasks concurrently
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			instanceID := fmt.Sprintf("inst-%d", idx)
			_, _ = gate.ClaimNext(instanceID)
		}(i)
	}
	wg.Wait()

	// MarkRunning + Approve/Reject concurrently
	wg.Add(4)
	go func() {
		defer wg.Done()
		_ = gate.MarkRunning("c1")
	}()
	go func() {
		defer wg.Done()
		_ = gate.MarkRunning("c2")
	}()
	go func() {
		defer wg.Done()
		_ = gate.MarkRunning("c3")
	}()
	go func() {
		defer wg.Done()
		_ = gate.MarkRunning("c4")
	}()
	wg.Wait()

	// Approve/reject concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = gate.Approve("c1")
	}()
	go func() {
		defer wg.Done()
		_ = gate.Reject("c2", "not needed")
	}()
	wg.Wait()

	// Verify final state
	if gate.IsAwaitingApproval("c1") {
		t.Error("c1 should not be awaiting after approve")
	}
	if gate.IsAwaitingApproval("c2") {
		t.Error("c2 should not be awaiting after reject")
	}
}

func TestGate_Approve_UnderlyingError(t *testing.T) {
	// Test that Approve propagates errors from the underlying MarkRunning.
	// Force this by releasing the task underneath the gate, then trying to approve.
	bus := event.NewBus()
	plan := &ultraplan.PlanSpec{
		ID: "approve-error",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))

	_, _ = gate.ClaimNext("inst-1")
	_ = gate.MarkRunning("t1")

	// Release the task underneath via the EventQueue directly
	_ = eq.Release("t1", "forced")

	// Now approve should fail because the underlying task is pending, not claimed
	err := gate.Approve("t1")
	if err == nil {
		t.Fatal("expected error from Approve after underlying release")
	}
}

func TestGate_Reject_UnderlyingError(t *testing.T) {
	// Test that Reject propagates errors from the underlying Fail.
	// Force this by completing the task underneath the gate, then trying to reject.
	bus := event.NewBus()
	plan := &ultraplan.PlanSpec{
		ID: "reject-error",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow, RequiresApproval: true},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)
	gate := NewGate(eq, bus, makeLookup(plan))

	_, _ = gate.ClaimNext("inst-1")
	_ = gate.MarkRunning("t1")

	// Release and then complete underneath
	_ = eq.Release("t1", "forced")

	// Now reject should fail because the task is no longer claimed/running
	err := gate.Reject("t1", "reason")
	if err == nil {
		t.Fatal("expected error from Reject after underlying release")
	}
}

func TestGate_MarkRunning_GetTaskNil(t *testing.T) {
	// Test MarkRunning when lookup says the task exists but GetTask returns nil.
	// This is a defensive check for an inconsistent state.
	bus := event.NewBus()
	plan := &ultraplan.PlanSpec{
		ID: "nil-task",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
		},
	}
	q := taskqueue.NewFromPlan(plan)
	eq := taskqueue.NewEventQueue(q, bus)

	// Create a lookup that claims "phantom" exists and requires approval,
	// but it's not actually in the queue.
	lookup := func(taskID string) (bool, bool) {
		if taskID == "phantom" {
			return true, true
		}
		return false, taskID == "t1"
	}
	gate := NewGate(eq, bus, lookup)

	err := gate.MarkRunning("phantom")
	if err == nil {
		t.Fatal("expected error for phantom task")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("error = %v, want ErrTaskNotFound", err)
	}
}

// Compile-time interface checks.
var _ event.Event = event.TaskAwaitingApprovalEvent{}

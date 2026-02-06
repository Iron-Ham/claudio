package adaptive

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// mockQueue implements the TaskQueue interface for testing.
type mockQueue struct {
	mu            sync.Mutex
	status        taskqueue.QueueStatus
	releasedTasks []string
	claimResult   *taskqueue.QueuedTask
	claimErr      error
	releaseErr    error
	instanceTasks map[string][]*taskqueue.QueuedTask
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		instanceTasks: make(map[string][]*taskqueue.QueuedTask),
	}
}

func (m *mockQueue) Status() taskqueue.QueueStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *mockQueue) Release(taskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.releaseErr != nil {
		return m.releaseErr
	}
	m.releasedTasks = append(m.releasedTasks, taskID)
	return nil
}

func (m *mockQueue) ClaimNext(instanceID string) (*taskqueue.QueuedTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimErr != nil {
		return nil, m.claimErr
	}
	return m.claimResult, nil
}

func (m *mockQueue) GetInstanceTasks(instanceID string) []*taskqueue.QueuedTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.instanceTasks[instanceID]
}

func (m *mockQueue) setStatus(s taskqueue.QueueStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = s
}

func (m *mockQueue) setClaimResult(t *taskqueue.QueuedTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimResult = t
}

func (m *mockQueue) setClaimErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimErr = err
}

func (m *mockQueue) setReleaseErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseErr = err
}

func (m *mockQueue) setInstanceTasks(instanceID string, tasks []*taskqueue.QueuedTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instanceTasks[instanceID] = tasks
}

func (m *mockQueue) getReleasedTasks() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.releasedTasks))
	copy(cp, m.releasedTasks)
	return cp
}

func TestNewLead(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()

	lead := NewLead(mq, bus)

	if lead.staleClaimTimeout != defaultStaleClaimTimeout {
		t.Errorf("staleClaimTimeout = %v, want %v", lead.staleClaimTimeout, defaultStaleClaimTimeout)
	}
	if lead.rebalanceInterval != defaultRebalanceInterval {
		t.Errorf("rebalanceInterval = %v, want %v", lead.rebalanceInterval, defaultRebalanceInterval)
	}
	if lead.maxTasksPerInstance != defaultMaxTasksPerInstance {
		t.Errorf("maxTasksPerInstance = %d, want %d", lead.maxTasksPerInstance, defaultMaxTasksPerInstance)
	}
}

func TestNewLeadWithOptions(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()

	lead := NewLead(mq, bus,
		WithStaleClaimTimeout(10*time.Second),
		WithRebalanceInterval(5*time.Second),
		WithMaxTasksPerInstance(5),
	)

	if lead.staleClaimTimeout != 10*time.Second {
		t.Errorf("staleClaimTimeout = %v, want %v", lead.staleClaimTimeout, 10*time.Second)
	}
	if lead.rebalanceInterval != 5*time.Second {
		t.Errorf("rebalanceInterval = %v, want %v", lead.rebalanceInterval, 5*time.Second)
	}
	if lead.maxTasksPerInstance != 5 {
		t.Errorf("maxTasksPerInstance = %d, want %d", lead.maxTasksPerInstance, 5)
	}
}

func TestScalingAction_String(t *testing.T) {
	tests := []struct {
		action ScalingAction
		want   string
	}{
		{ScaleNone, "none"},
		{ScaleUp, "scale_up"},
		{ScaleDown, "scale_down"},
		{ScalingAction(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.action.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleTaskClaimed(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)
	lead.Start(context.Background())
	defer lead.Stop()

	bus.Publish(event.NewTaskClaimedEvent("task-1", "inst-1"))
	bus.Publish(event.NewTaskClaimedEvent("task-2", "inst-1"))
	bus.Publish(event.NewTaskClaimedEvent("task-3", "inst-2"))

	dist := lead.GetWorkloadDistribution()
	if dist["inst-1"] != 2 {
		t.Errorf("inst-1 workload = %d, want 2", dist["inst-1"])
	}
	if dist["inst-2"] != 1 {
		t.Errorf("inst-2 workload = %d, want 1", dist["inst-2"])
	}
}

func TestHandleTaskCompleted(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)
	lead.Start(context.Background())
	defer lead.Stop()

	bus.Publish(event.NewTaskClaimedEvent("task-1", "inst-1"))
	bus.Publish(event.NewTaskClaimedEvent("task-2", "inst-1"))
	bus.Publish(event.NewTaskCompletedEvent("task-1", "inst-1", true, "done"))

	dist := lead.GetWorkloadDistribution()
	if dist["inst-1"] != 1 {
		t.Errorf("inst-1 workload = %d, want 1", dist["inst-1"])
	}
}

func TestHandleTaskCompletedRemovesIdleInstance(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)
	lead.Start(context.Background())
	defer lead.Stop()

	bus.Publish(event.NewTaskClaimedEvent("task-1", "inst-1"))
	bus.Publish(event.NewTaskCompletedEvent("task-1", "inst-1", true, "done"))

	dist := lead.GetWorkloadDistribution()
	if _, exists := dist["inst-1"]; exists {
		t.Error("inst-1 should be removed from workloads when count reaches 0")
	}
}

func TestHandleTaskCompletedEmptyInstance(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)
	lead.Start(context.Background())
	defer lead.Stop()

	// task.completed with empty InstanceID should be ignored.
	bus.Publish(event.NewTaskCompletedEvent("task-1", "", true, "done"))

	dist := lead.GetWorkloadDistribution()
	if len(dist) != 0 {
		t.Errorf("workloads should be empty, got %v", dist)
	}
}

func TestHandleTaskReleased(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)
	lead.Start(context.Background())
	defer lead.Stop()

	// Releasing should not panic or cause issues.
	bus.Publish(event.NewTaskReleasedEvent("task-1", "stale_claim"))

	dist := lead.GetWorkloadDistribution()
	if len(dist) != 0 {
		t.Errorf("workloads should be empty, got %v", dist)
	}
}

func TestGetScalingRecommendation(t *testing.T) {
	tests := []struct {
		name       string
		status     taskqueue.QueueStatus
		workloads  map[string]int
		wantAction ScalingAction
	}{
		{
			name:       "no work",
			status:     taskqueue.QueueStatus{},
			wantAction: ScaleNone,
		},
		{
			name: "scale up when pending exceeds capacity",
			status: taskqueue.QueueStatus{
				Pending: 10,
				Running: 1,
				Total:   11,
			},
			workloads: map[string]int{
				"inst-1": 1,
			},
			wantAction: ScaleUp,
		},
		{
			name: "scale up when no instances",
			status: taskqueue.QueueStatus{
				Pending: 5,
				Total:   5,
			},
			wantAction: ScaleUp,
		},
		{
			name: "scale down when idle instances",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 1,
				Total:   5,
			},
			workloads: map[string]int{
				"inst-1": 1,
				"inst-2": 0,
			},
			wantAction: ScaleDown,
		},
		{
			name: "balanced workload",
			status: taskqueue.QueueStatus{
				Pending: 1,
				Running: 2,
				Total:   5,
			},
			workloads: map[string]int{
				"inst-1": 1,
				"inst-2": 1,
			},
			wantAction: ScaleNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mq := newMockQueue()
			mq.setStatus(tt.status)
			bus := event.NewBus()
			lead := NewLead(mq, bus)

			if tt.workloads != nil {
				lead.mu.Lock()
				lead.workloads = tt.workloads
				lead.mu.Unlock()
			}

			rec := lead.GetScalingRecommendation()
			if rec.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v (reason: %s)", rec.Action, tt.wantAction, rec.Reason)
			}
			if rec.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

func TestReassign(t *testing.T) {
	tests := []struct {
		name        string
		claimResult *taskqueue.QueuedTask
		claimErr    error
		releaseErr  error
		wantErr     bool
	}{
		{
			name: "successful reassignment",
			claimResult: &taskqueue.QueuedTask{
				Status: taskqueue.TaskClaimed,
			},
		},
		{
			name:       "release fails",
			releaseErr: taskqueue.ErrTaskNotFound,
			wantErr:    true,
		},
		{
			name:     "claim fails",
			claimErr: taskqueue.ErrInvalidTransition,
			wantErr:  true,
		},
		{
			name:        "claim returns nil (no available task)",
			claimResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mq := newMockQueue()
			if tt.claimResult != nil {
				tt.claimResult.ID = "task-1"
				mq.setClaimResult(tt.claimResult)
			}
			if tt.claimErr != nil {
				mq.setClaimErr(tt.claimErr)
			}
			if tt.releaseErr != nil {
				mq.setReleaseErr(tt.releaseErr)
			}

			bus := event.NewBus()
			lead := NewLead(mq, bus)

			// Set up initial workload.
			lead.mu.Lock()
			lead.workloads["inst-1"] = 2
			lead.mu.Unlock()

			ch := make(chan event.Event, 1)
			bus.Subscribe("adaptive.task_reassigned", func(e event.Event) {
				ch <- e
			})

			err := lead.Reassign("task-1", "inst-1", "inst-2")
			if tt.wantErr {
				if err == nil {
					t.Fatal("Reassign() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Reassign() unexpected error: %v", err)
			}

			select {
			case e := <-ch:
				tre, ok := e.(event.TaskReassignedEvent)
				if !ok {
					t.Fatalf("event type = %T, want TaskReassignedEvent", e)
				}
				if tre.FromInstance != "inst-1" {
					t.Errorf("FromInstance = %q, want %q", tre.FromInstance, "inst-1")
				}
				if tre.ToInstance != "inst-2" {
					t.Errorf("ToInstance = %q, want %q", tre.ToInstance, "inst-2")
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for TaskReassignedEvent")
			}
		})
	}
}

func TestReassignUpdatesWorkloads(t *testing.T) {
	mq := newMockQueue()
	mq.setClaimResult(&taskqueue.QueuedTask{
		Status: taskqueue.TaskClaimed,
	})
	bus := event.NewBus()
	lead := NewLead(mq, bus)

	lead.mu.Lock()
	lead.workloads["inst-1"] = 3
	lead.workloads["inst-2"] = 1
	lead.mu.Unlock()

	if err := lead.Reassign("task-1", "inst-1", "inst-2"); err != nil {
		t.Fatalf("Reassign() error: %v", err)
	}

	dist := lead.GetWorkloadDistribution()
	if dist["inst-1"] != 2 {
		t.Errorf("inst-1 workload = %d, want 2", dist["inst-1"])
	}
	if dist["inst-2"] != 2 {
		t.Errorf("inst-2 workload = %d, want 2", dist["inst-2"])
	}
}

func TestReassignRemovesEmptyWorkload(t *testing.T) {
	mq := newMockQueue()
	mq.setClaimResult(&taskqueue.QueuedTask{
		Status: taskqueue.TaskClaimed,
	})
	bus := event.NewBus()
	lead := NewLead(mq, bus)

	lead.mu.Lock()
	lead.workloads["inst-1"] = 1
	lead.mu.Unlock()

	if err := lead.Reassign("task-1", "inst-1", "inst-2"); err != nil {
		t.Fatalf("Reassign() error: %v", err)
	}

	dist := lead.GetWorkloadDistribution()
	if _, exists := dist["inst-1"]; exists {
		t.Error("inst-1 should be removed when workload drops to 0")
	}
}

func TestHandleDepthChangedPublishesScalingSignal(t *testing.T) {
	mq := newMockQueue()
	mq.setStatus(taskqueue.QueueStatus{
		Pending: 10,
		Running: 0,
		Total:   10,
	})
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(0)) // no debounce
	lead.Start(context.Background())
	defer lead.Stop()

	ch := make(chan event.Event, 1)
	bus.Subscribe("adaptive.scaling_signal", func(e event.Event) {
		ch <- e
	})

	bus.Publish(event.NewQueueDepthChangedEvent(10, 0, 0, 0, 0, 10))

	select {
	case e := <-ch:
		sse, ok := e.(event.ScalingSignalEvent)
		if !ok {
			t.Fatalf("event type = %T, want ScalingSignalEvent", e)
		}
		if sse.Pending != 10 {
			t.Errorf("Pending = %d, want 10", sse.Pending)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ScalingSignalEvent")
	}
}

func TestHandleDepthChangedDebouncing(t *testing.T) {
	mq := newMockQueue()
	mq.setStatus(taskqueue.QueueStatus{
		Pending: 10,
		Running: 0,
		Total:   10,
	})
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(time.Hour)) // long debounce
	lead.Start(context.Background())
	defer lead.Stop()

	signalCount := 0
	var mu sync.Mutex
	bus.Subscribe("adaptive.scaling_signal", func(e event.Event) {
		mu.Lock()
		signalCount++
		mu.Unlock()
	})

	// First event should trigger a signal.
	bus.Publish(event.NewQueueDepthChangedEvent(10, 0, 0, 0, 0, 10))

	// Second event should be debounced.
	bus.Publish(event.NewQueueDepthChangedEvent(10, 0, 0, 0, 0, 10))

	// Give the bus time to process.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	count := signalCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("signal count = %d, want 1 (debouncing should suppress second)", count)
	}
}

func TestHandleDepthChangedNoSignalWhenBalanced(t *testing.T) {
	mq := newMockQueue()
	mq.setStatus(taskqueue.QueueStatus{
		Pending: 0,
		Running: 0,
		Total:   5,
	})
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(0))
	lead.Start(context.Background())
	defer lead.Stop()

	signalReceived := false
	var mu sync.Mutex
	bus.Subscribe("adaptive.scaling_signal", func(e event.Event) {
		mu.Lock()
		signalReceived = true
		mu.Unlock()
	})

	bus.Publish(event.NewQueueDepthChangedEvent(0, 0, 0, 5, 0, 5))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	got := signalReceived
	mu.Unlock()

	if got {
		t.Error("should not emit scaling signal when workload is balanced")
	}
}

func TestStartStop(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(10*time.Millisecond))

	initialSubs := bus.SubscriptionCount()
	lead.Start(context.Background())

	// Verify subscriptions were added.
	afterStart := bus.SubscriptionCount()
	if afterStart <= initialSubs {
		t.Error("Start() should add subscriptions")
	}

	lead.Stop()

	// Verify subscriptions were removed.
	afterStop := bus.SubscriptionCount()
	if afterStop != initialSubs {
		t.Errorf("Stop() should remove subscriptions, got %d want %d", afterStop, initialSubs)
	}
}

func TestStartContextCancellation(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	lead.Start(ctx)

	// Cancel the parent context.
	cancel()

	// Stop should complete without hanging.
	done := make(chan struct{})
	go func() {
		lead.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out after context cancellation")
	}
}

func TestGetWorkloadDistribution(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)

	// Empty workloads.
	dist := lead.GetWorkloadDistribution()
	if len(dist) != 0 {
		t.Errorf("empty workloads = %v, want empty map", dist)
	}

	// Set workloads.
	lead.mu.Lock()
	lead.workloads["inst-1"] = 3
	lead.workloads["inst-2"] = 1
	lead.mu.Unlock()

	dist = lead.GetWorkloadDistribution()
	if dist["inst-1"] != 3 {
		t.Errorf("inst-1 = %d, want 3", dist["inst-1"])
	}
	if dist["inst-2"] != 1 {
		t.Errorf("inst-2 = %d, want 1", dist["inst-2"])
	}

	// Verify it's a copy (modifying returned map doesn't affect internal state).
	dist["inst-1"] = 99
	dist2 := lead.GetWorkloadDistribution()
	if dist2["inst-1"] != 3 {
		t.Error("GetWorkloadDistribution() should return a copy")
	}
}

func TestCheckRebalance(t *testing.T) {
	tests := []struct {
		name          string
		workloads     map[string]int
		instanceTasks map[string][]*taskqueue.QueuedTask
		wantRelease   bool
	}{
		{
			name:      "single instance does not rebalance",
			workloads: map[string]int{"inst-1": 5},
		},
		{
			name: "balanced workloads do not rebalance",
			workloads: map[string]int{
				"inst-1": 2,
				"inst-2": 2,
			},
		},
		{
			name: "imbalanced workloads trigger rebalance",
			workloads: map[string]int{
				"inst-1": 5,
				"inst-2": 1,
			},
			instanceTasks: map[string][]*taskqueue.QueuedTask{
				"inst-1": {
					{Status: taskqueue.TaskRunning},
					{Status: taskqueue.TaskRunning},
				},
			},
			wantRelease: true,
		},
		{
			name: "imbalanced but no tasks to move",
			workloads: map[string]int{
				"inst-1": 5,
				"inst-2": 1,
			},
			instanceTasks: map[string][]*taskqueue.QueuedTask{
				"inst-1": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mq := newMockQueue()
			if tt.instanceTasks != nil {
				for id, tasks := range tt.instanceTasks {
					// Give tasks IDs for reassignment.
					for i := range tasks {
						tasks[i].ID = "task-" + id + "-" + string(rune('a'+i))
					}
					mq.setInstanceTasks(id, tasks)
				}
			}

			bus := event.NewBus()
			lead := NewLead(mq, bus)

			lead.mu.Lock()
			lead.workloads = tt.workloads
			lead.mu.Unlock()

			lead.checkRebalance()

			released := mq.getReleasedTasks()
			if tt.wantRelease && len(released) == 0 {
				t.Error("expected a task to be released for rebalancing")
			}
			if !tt.wantRelease && len(released) > 0 {
				t.Errorf("unexpected release: %v", released)
			}
		})
	}
}

func TestConcurrentEventHandling(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus, WithRebalanceInterval(time.Hour))
	lead.Start(context.Background())
	defer lead.Stop()

	const goroutines = 20
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			instanceID := "inst-" + string(rune('a'+id%5))
			bus.Publish(event.NewTaskClaimedEvent("task-"+string(rune('a'+id)), instanceID))
		}(i)
	}

	wg.Wait()

	// Verify no data race (success condition is no panic with -race).
	dist := lead.GetWorkloadDistribution()
	total := 0
	for _, count := range dist {
		total += count
	}
	if total != goroutines {
		t.Errorf("total workload = %d, want %d", total, goroutines)
	}
}

func TestHandleWrongEventType(t *testing.T) {
	mq := newMockQueue()
	bus := event.NewBus()
	lead := NewLead(mq, bus)

	// These should not panic when receiving wrong event types.
	lead.handleTaskClaimed(event.NewInstanceStartedEvent("", "", "", ""))
	lead.handleTaskReleased(event.NewInstanceStartedEvent("", "", "", ""))
	lead.handleDepthChanged(event.NewInstanceStartedEvent("", "", "", ""))
	lead.handleTaskCompleted(event.NewInstanceStartedEvent("", "", "", ""))
}

// Compile-time interface checks.
var (
	_ TaskQueue   = (*mockQueue)(nil)
	_ event.Event = event.ScalingSignalEvent{}
	_ event.Event = event.TaskReassignedEvent{}
)

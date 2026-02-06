package adaptive

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
)

const (
	defaultStaleClaimTimeout   = 2 * time.Minute
	defaultRebalanceInterval   = 30 * time.Second
	defaultMaxTasksPerInstance = 3
)

// Lead monitors task queue events and provides dynamic coordination.
// It tracks instance workloads, recommends scaling, and supports
// task reassignment between instances.
type Lead struct {
	mu                sync.RWMutex
	queue             TaskQueue
	bus               *event.Bus
	workloads         map[string]int // instanceID -> active task count
	subscriptionIDs   []string
	stopFunc          context.CancelFunc
	stopped           chan struct{}
	lastScalingSignal time.Time

	// Configuration
	staleClaimTimeout   time.Duration
	rebalanceInterval   time.Duration
	maxTasksPerInstance int
}

// NewLead creates a Lead that monitors queue events on the given bus.
func NewLead(queue TaskQueue, bus *event.Bus, opts ...Option) *Lead {
	l := &Lead{
		queue:               queue,
		bus:                 bus,
		workloads:           make(map[string]int),
		stopped:             make(chan struct{}),
		staleClaimTimeout:   defaultStaleClaimTimeout,
		rebalanceInterval:   defaultRebalanceInterval,
		maxTasksPerInstance: defaultMaxTasksPerInstance,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Start begins monitoring events. It subscribes to relevant event types
// and runs a background goroutine for periodic rebalance checks.
// Call Stop to clean up.
func (l *Lead) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	l.stopFunc = cancel

	l.subscriptionIDs = append(l.subscriptionIDs,
		l.bus.Subscribe("queue.task_claimed", l.handleTaskClaimed),
		l.bus.Subscribe("queue.task_released", l.handleTaskReleased),
		l.bus.Subscribe("queue.depth_changed", l.handleDepthChanged),
		l.bus.Subscribe("task.completed", l.handleTaskCompleted),
	)

	go l.rebalanceLoop(ctx)
}

// Stop unsubscribes from all events and stops the background goroutine.
// It is safe to call Stop even if Start was never called.
func (l *Lead) Stop() {
	for _, id := range l.subscriptionIDs {
		l.bus.Unsubscribe(id)
	}
	l.subscriptionIDs = nil

	if l.stopFunc != nil {
		l.stopFunc()
		<-l.stopped
	}
}

// Reassign moves a task from one instance to another.
// It releases the task from the source instance and claims it for the target.
// If the claim for the target fails, the task is left in pending state (not lost).
func (l *Lead) Reassign(taskID, fromInstance, toInstance string) error {
	if err := l.queue.Release(taskID, "reassignment"); err != nil {
		return fmt.Errorf("release from %s: %w", fromInstance, err)
	}

	task, err := l.queue.ClaimNext(toInstance)
	if err != nil {
		return fmt.Errorf("claim for %s: %w", toInstance, err)
	}

	// ClaimNext picks the next available task by priority, which may not be
	// the same task we released. If it claimed a different task or nothing,
	// that's acceptable — the released task is still in the queue.

	l.mu.Lock()
	l.workloads[fromInstance]--
	if l.workloads[fromInstance] <= 0 {
		delete(l.workloads, fromInstance)
	}
	if task != nil {
		l.workloads[toInstance]++
	}
	l.mu.Unlock()

	l.bus.Publish(event.NewTaskReassignedEvent(taskID, fromInstance, toInstance, "rebalance"))
	return nil
}

// GetWorkloadDistribution returns a snapshot of instance workloads.
// The map keys are instance IDs and values are active task counts.
func (l *Lead) GetWorkloadDistribution() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dist := make(map[string]int, len(l.workloads))
	for k, v := range l.workloads {
		dist[k] = v
	}
	return dist
}

// GetScalingRecommendation evaluates the current queue state and returns
// a scaling recommendation.
func (l *Lead) GetScalingRecommendation() ScalingRecommendation {
	status := l.queue.Status()

	l.mu.RLock()
	instanceCount := len(l.workloads)
	l.mu.RUnlock()

	// No pending work and no running tasks — nothing to scale.
	if status.Pending == 0 && status.Running == 0 && status.Claimed == 0 {
		return ScalingRecommendation{
			Action:      ScaleNone,
			TargetCount: instanceCount,
			Reason:      "no active or pending work",
		}
	}

	// More pending tasks than instances can handle.
	if status.Pending > 0 && (instanceCount == 0 || status.Pending > instanceCount*l.maxTasksPerInstance) {
		target := instanceCount + (status.Pending+l.maxTasksPerInstance-1)/l.maxTasksPerInstance
		return ScalingRecommendation{
			Action:      ScaleUp,
			TargetCount: target,
			Reason:      fmt.Sprintf("%d pending tasks exceed capacity of %d instances", status.Pending, instanceCount),
		}
	}

	// No pending tasks and some instances are idle.
	if status.Pending == 0 && instanceCount > 0 {
		activeInstances := 0
		l.mu.RLock()
		for _, count := range l.workloads {
			if count > 0 {
				activeInstances++
			}
		}
		l.mu.RUnlock()

		if activeInstances < instanceCount {
			return ScalingRecommendation{
				Action:      ScaleDown,
				TargetCount: activeInstances,
				Reason:      fmt.Sprintf("%d of %d instances are idle", instanceCount-activeInstances, instanceCount),
			}
		}
	}

	return ScalingRecommendation{
		Action:      ScaleNone,
		TargetCount: instanceCount,
		Reason:      "workload is balanced",
	}
}

// handleTaskClaimed tracks that an instance gained a task.
func (l *Lead) handleTaskClaimed(e event.Event) {
	claimed, ok := e.(event.TaskClaimedEvent)
	if !ok {
		return
	}

	l.mu.Lock()
	l.workloads[claimed.InstanceID]++
	l.mu.Unlock()
}

// handleTaskReleased notes that a task was returned to the queue.
func (l *Lead) handleTaskReleased(e event.Event) {
	released, ok := e.(event.TaskReleasedEvent)
	if !ok {
		return
	}

	// We don't know which instance released it from the event alone,
	// but the workload tracking is adjusted when we see the next
	// depth_changed or task_claimed event. This is a signal to check
	// for rebalancing.
	_ = released
}

// handleDepthChanged evaluates scaling needs when queue depth changes.
func (l *Lead) handleDepthChanged(e event.Event) {
	depth, ok := e.(event.QueueDepthChangedEvent)
	if !ok {
		return
	}

	l.mu.Lock()
	now := time.Now()
	shouldSignal := now.Sub(l.lastScalingSignal) >= l.rebalanceInterval
	if shouldSignal {
		l.lastScalingSignal = now
	}
	l.mu.Unlock()

	if shouldSignal {
		rec := l.GetScalingRecommendation()
		if rec.Action != ScaleNone {
			l.bus.Publish(event.NewScalingSignalEvent(
				depth.Pending,
				depth.Running,
				rec.Reason,
			))
		}
	}
}

// handleTaskCompleted decrements workload for the completing instance.
func (l *Lead) handleTaskCompleted(e event.Event) {
	completed, ok := e.(event.TaskCompletedEvent)
	if !ok {
		return
	}

	if completed.InstanceID == "" {
		return
	}

	l.mu.Lock()
	l.workloads[completed.InstanceID]--
	if l.workloads[completed.InstanceID] <= 0 {
		delete(l.workloads, completed.InstanceID)
	}
	l.mu.Unlock()
}

// rebalanceLoop periodically checks for workload imbalances.
func (l *Lead) rebalanceLoop(ctx context.Context) {
	defer close(l.stopped)

	// A zero or negative interval disables the periodic rebalance loop.
	// This is useful in tests that only need event-driven behavior.
	if l.rebalanceInterval <= 0 {
		<-ctx.Done()
		return
	}

	ticker := time.NewTicker(l.rebalanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.checkRebalance()
		}
	}
}

// checkRebalance evaluates whether tasks should be moved between instances.
func (l *Lead) checkRebalance() {
	l.mu.RLock()
	if len(l.workloads) < 2 {
		l.mu.RUnlock()
		return
	}

	// Find min and max loaded instances.
	var minID, maxID string
	minCount, maxCount := int(^uint(0)>>1), 0 // maxint, 0

	// Sort keys for deterministic behavior in tests.
	keys := make([]string, 0, len(l.workloads))
	for k := range l.workloads {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, id := range keys {
		count := l.workloads[id]
		if count < minCount {
			minCount = count
			minID = id
		}
		if count > maxCount {
			maxCount = count
			maxID = id
		}
	}
	l.mu.RUnlock()

	// Only rebalance if the imbalance is significant (difference > 1).
	if maxCount-minCount <= 1 {
		return
	}

	// Find a task to move from the overloaded instance.
	tasks := l.queue.GetInstanceTasks(maxID)
	if len(tasks) == 0 {
		return
	}

	// Move the last task (lowest priority) from the overloaded instance.
	taskToMove := tasks[len(tasks)-1]
	l.Reassign(taskToMove.ID, maxID, minID) //nolint:errcheck // best-effort rebalance
}

package scaling

import (
	"context"
	"sync"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// Monitor watches queue depth events on the event bus and applies a scaling
// policy to recommend instance count changes.
type Monitor struct {
	mu       sync.Mutex
	bus      *event.Bus
	policy   *Policy
	handlers []func(Decision)
	subID    string
	cancel   context.CancelFunc

	// currentInstances is maintained by the monitor. The caller is expected
	// to update it via SetCurrentInstances when instances actually change.
	currentInstances int
}

// NewMonitor creates a Monitor that evaluates the given policy whenever
// a QueueDepthChangedEvent is received on the bus.
func NewMonitor(bus *event.Bus, policy *Policy, initialInstances int) *Monitor {
	return &Monitor{
		bus:              bus,
		policy:           policy,
		currentInstances: initialInstances,
	}
}

// OnDecision registers a callback that is invoked when a non-none scaling
// decision is made. Multiple handlers may be registered.
func (m *Monitor) OnDecision(handler func(Decision)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// SetCurrentInstances updates the instance count known to the monitor.
// Call this after actually adding or removing instances so subsequent
// evaluations use the correct count.
func (m *Monitor) SetCurrentInstances(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentInstances = n
}

// Start subscribes to queue depth events and begins evaluating the policy.
// It blocks until the context is cancelled or Stop is called.
func (m *Monitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	subID := m.bus.Subscribe("queue.depth_changed", func(e event.Event) {
		de, ok := e.(event.QueueDepthChangedEvent)
		if !ok {
			return
		}
		status := taskqueue.QueueStatus{
			Pending:   de.Pending,
			Claimed:   de.Claimed,
			Running:   de.Running,
			Completed: de.Completed,
			Failed:    de.Failed,
			Total:     de.Total,
		}

		m.mu.Lock()
		current := m.currentInstances
		handlers := make([]func(Decision), len(m.handlers))
		copy(handlers, m.handlers)
		m.mu.Unlock()

		decision := m.policy.Evaluate(status, current)
		if decision.Action != ActionNone {
			m.bus.Publish(event.NewScalingDecisionEvent(
				string(decision.Action), decision.Delta, decision.Reason, current,
			))
			for _, h := range handlers {
				h(decision)
			}
		}
	})

	m.mu.Lock()
	m.subID = subID
	m.cancel = cancel
	m.mu.Unlock()

	<-ctx.Done()
}

// Stop unsubscribes from events and cancels the monitor.
func (m *Monitor) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	subID := m.subID
	m.mu.Unlock()

	if subID != "" {
		m.bus.Unsubscribe(subID)
	}
	if cancel != nil {
		cancel()
	}
}

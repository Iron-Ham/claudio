package coordination

import (
	"context"
	"errors"
	"sync"

	"github.com/Iron-Ham/claudio/internal/adaptive"
	"github.com/Iron-Ham/claudio/internal/approval"
	"github.com/Iron-Ham/claudio/internal/contextprop"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/filelock"
	"github.com/Iron-Ham/claudio/internal/mailbox"
	"github.com/Iron-Ham/claudio/internal/scaling"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// Config holds required dependencies for creating a Hub.
type Config struct {
	Bus        *event.Bus
	SessionDir string
	Plan       *ultraplan.PlanSpec
	TaskLookup approval.TaskLookup
}

// Hub wires all Orchestration 2.0 components together for a single session.
// It owns the lifecycle of the adaptive lead and scaling monitor.
type Hub struct {
	mu      sync.RWMutex
	started bool
	cancel  context.CancelFunc

	// monitorDone is closed when the scaling monitor goroutine exits.
	monitorDone chan struct{}

	// Components
	mb             *mailbox.Mailbox
	queue          *taskqueue.TaskQueue
	eventQueue     *taskqueue.EventQueue
	gate           *approval.Gate
	lead           *adaptive.Lead
	scalingMonitor *scaling.Monitor
	propagator     *contextprop.Propagator
	fileLockReg    *filelock.Registry
}

// NewHub creates a Hub that wires all Orchestration 2.0 components together.
func NewHub(cfg Config, opts ...Option) (*Hub, error) {
	if cfg.Bus == nil {
		return nil, errors.New("coordination: Bus is required")
	}
	if cfg.Plan == nil {
		return nil, errors.New("coordination: Plan is required")
	}
	if cfg.SessionDir == "" {
		return nil, errors.New("coordination: SessionDir is required")
	}

	hc := &hubConfig{}
	for _, opt := range opts {
		opt(hc)
	}

	// Build adaptive lead options from hub config.
	var adaptiveOpts []adaptive.Option
	if hc.maxTasksPerInstance > 0 {
		adaptiveOpts = append(adaptiveOpts, adaptive.WithMaxTasksPerInstance(hc.maxTasksPerInstance))
	}
	if hc.staleClaimTimeout > 0 {
		adaptiveOpts = append(adaptiveOpts, adaptive.WithStaleClaimTimeout(hc.staleClaimTimeout))
	}
	if hc.rebalanceInterval > 0 {
		adaptiveOpts = append(adaptiveOpts, adaptive.WithRebalanceInterval(hc.rebalanceInterval))
	}

	// Default TaskLookup: no approvals required.
	lookup := cfg.TaskLookup
	if lookup == nil {
		lookup = func(string) (bool, bool) { return false, true }
	}

	// Scaling policy defaults — apply per-hub min/max overrides.
	policy := hc.scalingPolicy
	if policy == nil {
		var policyOpts []scaling.Option
		if hc.minInstances > 0 {
			policyOpts = append(policyOpts, scaling.WithMinInstances(hc.minInstances))
		}
		if hc.maxInstances > 0 {
			policyOpts = append(policyOpts, scaling.WithMaxInstances(hc.maxInstances))
		}
		policy = scaling.NewPolicy(policyOpts...)
	}

	mb := mailbox.NewMailbox(cfg.SessionDir, mailbox.WithBus(cfg.Bus))
	queue := taskqueue.NewFromPlan(cfg.Plan)
	eq := taskqueue.NewEventQueue(queue, cfg.Bus)
	gate := approval.NewGate(eq, cfg.Bus, lookup)
	lead := adaptive.NewLead(eq, cfg.Bus, adaptiveOpts...)
	monitor := scaling.NewMonitor(cfg.Bus, policy, hc.initialInstances)
	prop := contextprop.NewPropagator(mb, cfg.Bus)
	reg := filelock.NewRegistry(mb, cfg.Bus)

	return &Hub{
		mb:             mb,
		queue:          queue,
		eventQueue:     eq,
		gate:           gate,
		lead:           lead,
		scalingMonitor: monitor,
		propagator:     prop,
		fileLockReg:    reg,
	}, nil
}

// Gate returns the approval gate for task operations.
func (h *Hub) Gate() *approval.Gate { return h.gate }

// EventQueue returns the event-publishing task queue.
func (h *Hub) EventQueue() *taskqueue.EventQueue { return h.eventQueue }

// TaskQueue returns the underlying task queue.
func (h *Hub) TaskQueue() *taskqueue.TaskQueue { return h.queue }

// Lead returns the adaptive lead for workload monitoring.
func (h *Hub) Lead() *adaptive.Lead { return h.lead }

// ScalingMonitor returns the scaling monitor.
func (h *Hub) ScalingMonitor() *scaling.Monitor { return h.scalingMonitor }

// Propagator returns the context propagator for cross-instance knowledge sharing.
func (h *Hub) Propagator() *contextprop.Propagator { return h.propagator }

// FileLockRegistry returns the file lock registry for conflict prevention.
func (h *Hub) FileLockRegistry() *filelock.Registry { return h.fileLockReg }

// Mailbox returns the underlying mailbox for inter-instance messaging.
func (h *Hub) Mailbox() *mailbox.Mailbox { return h.mb }

// Start begins the adaptive lead and scaling monitor.
// Returns an error if the hub is already started.
func (h *Hub) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.started {
		return errors.New("coordination: hub already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.started = true
	h.monitorDone = make(chan struct{})

	h.lead.Start(ctx)

	go func() {
		defer close(h.monitorDone)
		h.scalingMonitor.Start(ctx)
	}()

	return nil
}

// Stop stops all components in reverse order. It is idempotent.
func (h *Hub) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.started {
		return nil
	}

	// Cancel context to unblock the scaling monitor and adaptive lead goroutines.
	h.cancel()

	// Stop scaling monitor first (reverse of start order).
	h.scalingMonitor.Stop()
	<-h.monitorDone

	// Stop adaptive lead.
	h.lead.Stop()

	h.started = false
	return nil
}

// Running returns whether the hub is currently started.
func (h *Hub) Running() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.started
}

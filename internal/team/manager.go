package team

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// ManagerConfig holds required dependencies for creating a Manager.
type ManagerConfig struct {
	Bus     *event.Bus // Shared event bus for all teams
	BaseDir string     // Base directory; per-team subdirs created under this
}

// Manager orchestrates multiple teams running in parallel.
// Teams are added with AddTeam before calling Start. The manager handles
// dependency ordering, per-team Hub creation, inter-team message routing,
// and lifecycle events.
type Manager struct {
	mu      sync.RWMutex
	bus     *event.Bus
	baseDir string
	teams   map[string]*Team
	order   []string // insertion order for deterministic iteration
	router  *Router
	started bool
	ctx     context.Context //nolint:containedctx // stored for dynamic team addition
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	hubOpts []coordination.Option

	// completionSubID tracks the event bus subscription for team completion monitoring.
	completionSubID string
}

// NewManager creates a Manager with the given configuration and options.
func NewManager(cfg ManagerConfig, opts ...ManagerOption) (*Manager, error) {
	if cfg.Bus == nil {
		return nil, errors.New("team: Bus is required")
	}
	if cfg.BaseDir == "" {
		return nil, errors.New("team: BaseDir is required")
	}

	mc := &managerConfig{}
	for _, opt := range opts {
		opt(mc)
	}

	m := &Manager{
		bus:     cfg.Bus,
		baseDir: cfg.BaseDir,
		teams:   make(map[string]*Team),
		hubOpts: mc.hubOpts,
	}

	m.router = newRouter(
		cfg.Bus,
		func(id string) *Team { return m.Team(id) },
		func() []string {
			m.mu.RLock()
			defer m.mu.RUnlock()
			out := make([]string, len(m.order))
			copy(out, m.order)
			return out
		},
	)

	return m, nil
}

// AddTeam registers a team specification. Must be called before Start.
func (m *Manager) AddTeam(spec Spec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return errors.New("team: cannot add teams after Start")
	}

	if err := spec.Validate(); err != nil {
		return err
	}

	if _, exists := m.teams[spec.ID]; exists {
		return fmt.Errorf("team: duplicate team ID %q", spec.ID)
	}

	// Dependency validation is deferred to Start.
	_, err := m.buildAndRegisterTeamLocked(spec)
	return err
}

// buildAndRegisterTeamLocked creates a Hub, BudgetTracker, and Team from the
// spec, registers them in the maps, and publishes a TeamCreatedEvent. Must be
// called with m.mu held.
func (m *Manager) buildAndRegisterTeamLocked(spec Spec) (*Team, error) {
	plan := &ultraplan.PlanSpec{
		ID:        fmt.Sprintf("team-%s-plan", spec.ID),
		Objective: fmt.Sprintf("Team %s: %s", spec.Name, spec.Role),
		Tasks:     spec.Tasks,
	}

	sessionDir := filepath.Join(m.baseDir, spec.ID)

	hub, err := coordination.NewHub(coordination.Config{
		Bus:        m.bus,
		SessionDir: sessionDir,
		Plan:       plan,
	}, m.hubOpts...)
	if err != nil {
		return nil, fmt.Errorf("team: creating hub for %q: %w", spec.ID, err)
	}

	bt := newBudgetTracker(spec.ID, spec.Budget, m.bus)
	t := newTeam(spec, hub, bt)

	m.teams[spec.ID] = t
	m.order = append(m.order, spec.ID)

	m.bus.Publish(event.NewTeamCreatedEvent(spec.ID, spec.Name, string(spec.Role)))

	return t, nil
}

// AddTeamDynamic adds a team to a running manager. Unlike AddTeam, this can
// be called after Start. The new team starts immediately if all its
// dependencies are satisfied, or enters Blocked phase otherwise.
//
// The context parameter is ignored — the manager uses its own context
// (from Start) so that Stop can cancel all teams. The parameter is kept
// for API consistency and future use.
func (m *Manager) AddTeamDynamic(_ context.Context, spec Spec) error {
	// Phase 1: Register the team under the write lock.
	t, shouldStart, mctx, err := m.registerDynamicTeamLocked(spec)
	if err != nil {
		return err
	}

	// Phase 2: Start or block the team outside the lock. This prevents
	// deadlock when startTeamLocked spawns monitorTeamCompletion, which
	// publishes team.completed → onTeamCompleted tries to acquire m.mu.
	if shouldStart {
		m.mu.Lock()
		m.startTeamLocked(mctx, t)
		m.mu.Unlock()
		m.bus.Publish(event.NewTeamDynamicAddedEvent(spec.ID, spec.Name, string(PhaseWorking)))
	} else {
		prev := t.setPhase(PhaseBlocked)
		m.bus.Publish(event.NewTeamPhaseChangedEvent(
			spec.ID, spec.Name, string(prev), string(PhaseBlocked),
		))
		m.bus.Publish(event.NewTeamDynamicAddedEvent(spec.ID, spec.Name, string(PhaseBlocked)))
	}

	return nil
}

// registerDynamicTeamLocked validates and registers a team, returning the team,
// whether it should start immediately, and the manager's context. Holds and
// releases m.mu internally.
func (m *Manager) registerDynamicTeamLocked(spec Spec) (*Team, bool, context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil, false, nil, errors.New("team: AddTeamDynamic requires a started manager")
	}

	if err := spec.Validate(); err != nil {
		return nil, false, nil, err
	}

	if _, exists := m.teams[spec.ID]; exists {
		return nil, false, nil, fmt.Errorf("team: duplicate team ID %q", spec.ID)
	}

	for _, dep := range spec.DependsOn {
		if _, exists := m.teams[dep]; !exists {
			return nil, false, nil, fmt.Errorf("team %q depends on unknown team %q", spec.ID, dep)
		}
	}

	t, err := m.buildAndRegisterTeamLocked(spec)
	if err != nil {
		return nil, false, nil, err
	}

	shouldStart := m.allDepsSatisfiedLocked(t)
	return t, shouldStart, m.ctx, nil
}

// Start begins multi-team execution. Teams with no dependencies start
// immediately; others wait until their dependencies complete.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return errors.New("team: manager already started")
	}

	if len(m.teams) == 0 {
		return errors.New("team: no teams registered")
	}

	// Validate all dependency references.
	for _, id := range m.order {
		t := m.teams[id]
		for _, dep := range t.spec.DependsOn {
			if _, exists := m.teams[dep]; !exists {
				return fmt.Errorf("team %q depends on unknown team %q", id, dep)
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	m.ctx = ctx
	m.cancel = cancel
	m.started = true

	// Subscribe to team completion events to cascade dependencies.
	m.completionSubID = m.bus.Subscribe("team.completed", func(e event.Event) {
		if tce, ok := e.(event.TeamCompletedEvent); ok {
			m.onTeamCompleted(ctx, tce.TeamID)
		}
	})

	// Start teams whose dependencies are satisfied (including those with none).
	for _, id := range m.order {
		t := m.teams[id]
		if m.allDepsSatisfiedLocked(t) {
			m.startTeamLocked(ctx, t)
		} else {
			prev := t.setPhase(PhaseBlocked)
			m.bus.Publish(event.NewTeamPhaseChangedEvent(
				t.spec.ID, t.spec.Name, string(prev), string(PhaseBlocked),
			))
		}
	}

	return nil
}

// Stop stops all teams and the manager. It is idempotent.
func (m *Manager) Stop() error {
	m.mu.Lock()

	if !m.started {
		m.mu.Unlock()
		return nil
	}

	// Unsubscribe from completion events so new publishes won't trigger
	// onTeamCompleted. (A publish already in-flight may still call the
	// handler from its copied snapshot — setting started=false below
	// ensures the handler bails out.)
	if m.completionSubID != "" {
		m.bus.Unsubscribe(m.completionSubID)
		m.completionSubID = ""
	}

	// Cancel context to signal all monitor goroutines.
	m.cancel()

	// Stop all hubs and budget trackers.
	for _, id := range m.order {
		t := m.teams[id]
		_ = t.hub.Stop()
		t.budget.Stop()
	}

	// Mark stopped before releasing the lock so any racing onTeamCompleted
	// handler sees started=false and returns immediately.
	m.started = false
	m.mu.Unlock()

	// Wait for monitor goroutines outside the lock. The old code held m.mu
	// through wg.Wait(), which deadlocked when monitorTeamCompletion
	// published TeamCompletedEvent (triggering onTeamCompleted inline,
	// which tried to acquire m.mu).
	m.wg.Wait()
	return nil
}

// Team returns the team with the given ID, or nil if not found.
func (m *Manager) Team(id string) *Team {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.teams[id]
}

// TeamStatus returns a snapshot of the given team's status.
func (m *Manager) TeamStatus(id string) (Status, bool) {
	m.mu.RLock()
	t, exists := m.teams[id]
	m.mu.RUnlock()

	if !exists {
		return Status{}, false
	}
	return t.Status(), true
}

// AllStatuses returns status snapshots for all teams in insertion order.
func (m *Manager) AllStatuses() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]Status, 0, len(m.order))
	for _, id := range m.order {
		statuses = append(statuses, m.teams[id].Status())
	}
	return statuses
}

// RouteMessage routes an inter-team message through the router.
func (m *Manager) RouteMessage(msg InterTeamMessage) error {
	return m.router.Route(msg)
}

// Running returns whether the manager is currently started.
func (m *Manager) Running() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// startTeamLocked starts a team's Hub and budget tracker.
// Must be called with m.mu held (write lock).
func (m *Manager) startTeamLocked(ctx context.Context, t *Team) {
	prev := t.setPhase(PhaseWorking)
	t.budget.Start()

	if err := t.hub.Start(ctx); err != nil {
		t.budget.Stop()
		t.setPhase(PhaseFailed)
		m.bus.Publish(event.NewTeamPhaseChangedEvent(
			t.spec.ID, t.spec.Name, string(PhaseWorking), string(PhaseFailed),
		))
		return
	}

	m.bus.Publish(event.NewTeamPhaseChangedEvent(
		t.spec.ID, t.spec.Name, string(prev), string(PhaseWorking),
	))

	// Monitor task queue completion in a goroutine.
	m.wg.Add(1)
	go func(team *Team) {
		defer m.wg.Done()
		m.monitorTeamCompletion(ctx, team)
	}(t)
}

// monitorTeamCompletion watches for the team's task queue to complete.
func (m *Manager) monitorTeamCompletion(ctx context.Context, t *Team) {
	// Subscribe to queue depth changes and check for completion.
	done := make(chan struct{}, 1)
	subID := m.bus.Subscribe("queue.depth_changed", func(e event.Event) {
		if t.hub.TaskQueue().IsComplete() {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer m.bus.Unsubscribe(subID)

	// Also check immediately in case the queue is already complete.
	if t.hub.TaskQueue().IsComplete() {
		select {
		case done <- struct{}{}:
		default:
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-done:
	}

	qs := t.hub.TaskQueue().Status()
	success := qs.Failed == 0

	if success {
		t.setPhase(PhaseDone)
	} else {
		t.setPhase(PhaseFailed)
	}

	m.bus.Publish(event.NewTeamCompletedEvent(
		t.spec.ID, t.spec.Name, success, qs.Completed, qs.Failed,
	))
}

// onTeamCompleted handles the completion of a team by checking if dependent
// teams can now start. The completed team's ID is unused because we scan all
// blocked teams rather than maintaining a reverse dependency index.
func (m *Manager) onTeamCompleted(ctx context.Context, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	for _, id := range m.order {
		t := m.teams[id]
		if t.Phase() != PhaseBlocked {
			continue
		}
		if m.allDepsSatisfiedLocked(t) {
			m.startTeamLocked(ctx, t)
		}
	}
}

// allDepsSatisfiedLocked returns true if all of the team's dependencies
// completed successfully. A failed dependency does NOT satisfy the condition —
// dependent teams remain blocked. Must be called with m.mu held.
func (m *Manager) allDepsSatisfiedLocked(t *Team) bool {
	for _, dep := range t.spec.DependsOn {
		dt, exists := m.teams[dep]
		if !exists {
			return false
		}
		if dt.Phase() != PhaseDone {
			return false
		}
	}
	return true
}

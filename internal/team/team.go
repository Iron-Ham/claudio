package team

import (
	"sync"

	"github.com/Iron-Ham/claudio/internal/coordination"
)

// Team wraps a coordination.Hub with team-specific metadata and budget tracking.
type Team struct {
	mu     sync.RWMutex
	spec   Spec
	phase  Phase
	hub    *coordination.Hub
	budget *BudgetTracker
}

// newTeam creates a Team in the Forming phase.
func newTeam(spec Spec, hub *coordination.Hub, budget *BudgetTracker) *Team {
	return &Team{
		spec:   spec,
		phase:  PhaseForming,
		hub:    hub,
		budget: budget,
	}
}

// Hub returns the team's coordination hub.
func (t *Team) Hub() *coordination.Hub {
	return t.hub
}

// Spec returns the team's configuration.
func (t *Team) Spec() Spec {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.spec
}

// Phase returns the team's current lifecycle phase.
func (t *Team) Phase() Phase {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.phase
}

// setPhase transitions the team to a new phase and returns the previous phase.
func (t *Team) setPhase(p Phase) Phase {
	t.mu.Lock()
	defer t.mu.Unlock()
	prev := t.phase
	t.phase = p
	return prev
}

// Status returns a read-only snapshot of the team's current state.
func (t *Team) Status() Status {
	t.mu.RLock()
	phase := t.phase
	spec := t.spec
	t.mu.RUnlock()

	var usage BudgetUsage
	if t.budget != nil {
		usage = t.budget.Usage()
	}

	var tasksDone, tasksFailed int
	if t.hub != nil {
		qs := t.hub.TaskQueue().Status()
		tasksDone = qs.Completed
		tasksFailed = qs.Failed
	}

	return Status{
		ID:          spec.ID,
		Name:        spec.Name,
		Role:        spec.Role,
		Phase:       phase,
		TasksTotal:  len(spec.Tasks),
		TasksDone:   tasksDone,
		TasksFailed: tasksFailed,
		BudgetUsed:  usage,
	}
}

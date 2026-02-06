package team

import (
	"sync"

	"github.com/Iron-Ham/claudio/internal/event"
)

// BudgetTracker monitors resource consumption for a team.
// The manager accumulates usage by calling Record after mapping instances to teams.
type BudgetTracker struct {
	mu     sync.RWMutex
	teamID string
	budget TokenBudget
	used   BudgetUsage
	bus    *event.Bus
	subID  string // event bus subscription ID
}

// newBudgetTracker creates a BudgetTracker for the given team and budget.
func newBudgetTracker(teamID string, budget TokenBudget, bus *event.Bus) *BudgetTracker {
	return &BudgetTracker{
		teamID: teamID,
		budget: budget,
		bus:    bus,
	}
}

// Start marks the tracker as active. Resource accumulation happens through
// the Record method, which the manager calls after mapping instance → team.
//
// The manager is responsible for routing MetricsUpdateEvent to the correct
// team's tracker externally; the tracker itself does not subscribe to the bus.
func (bt *BudgetTracker) Start() {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if bt.subID != "" {
		return // already started
	}

	// Mark as started with a sentinel value. No bus subscription is needed
	// because the manager handles instance→team routing externally.
	bt.subID = "active"
}

// Stop marks the tracker as inactive. It is idempotent.
func (bt *BudgetTracker) Stop() {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.subID = ""
}

// Usage returns the current resource consumption.
func (bt *BudgetTracker) Usage() BudgetUsage {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.used
}

// Exhausted returns true if any budget limit has been exceeded.
// Returns false if the budget is unlimited (all limits are zero).
func (bt *BudgetTracker) Exhausted() bool {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.exhaustedLocked()
}

// exhaustedLocked checks exhaustion without acquiring the lock.
func (bt *BudgetTracker) exhaustedLocked() bool {
	if bt.budget.IsUnlimited() {
		return false
	}
	if bt.budget.MaxInputTokens > 0 && bt.used.InputTokens >= bt.budget.MaxInputTokens {
		return true
	}
	if bt.budget.MaxOutputTokens > 0 && bt.used.OutputTokens >= bt.budget.MaxOutputTokens {
		return true
	}
	if bt.budget.MaxTotalCost > 0 && bt.used.TotalCost >= bt.budget.MaxTotalCost {
		return true
	}
	return false
}

// Record adds resource consumption to the tracker. If this causes the budget
// to become exhausted, it publishes a TeamBudgetExhaustedEvent.
func (bt *BudgetTracker) Record(input, output int64, cost float64) {
	bt.mu.Lock()

	wasBelowBudget := !bt.exhaustedLocked()
	bt.used.InputTokens += input
	bt.used.OutputTokens += output
	bt.used.TotalCost += cost
	nowExhausted := bt.exhaustedLocked()
	budget := bt.budget
	used := bt.used

	bt.mu.Unlock()

	if wasBelowBudget && nowExhausted {
		bt.bus.Publish(event.NewTeamBudgetExhaustedEvent(
			bt.teamID,
			budget.MaxInputTokens, budget.MaxOutputTokens,
			used.InputTokens, used.OutputTokens,
			budget.MaxTotalCost, used.TotalCost,
		))
	}
}

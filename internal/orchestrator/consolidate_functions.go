package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/group/consolidate"
)

// ConsolidateGroupWithVerification consolidates a group and verifies commits exist.
// This is a package-level function that replaces the former Coordinator method,
// delegating to the consolidate package.
func ConsolidateGroupWithVerification(c *Coordinator, groupIndex int) error {
	adapter := newCoordinatorConsolidateAdapter(c)
	consolidator := consolidate.NewConsolidator(adapter)
	return consolidator.ConsolidateWithVerification(groupIndex)
}

// GetBaseBranchForGroup returns the base branch that new tasks in a group should use.
// For group 0, this is empty (use default/main). For other groups, it's the consolidated
// branch from the previous group.
// This is a package-level function that replaces the former Coordinator method,
// delegating to the consolidate package.
func GetBaseBranchForGroup(c *Coordinator, groupIndex int) string {
	adapter := newCoordinatorConsolidateAdapter(c)
	consolidator := consolidate.NewConsolidator(adapter)
	return consolidator.GetBaseBranchForGroup(groupIndex)
}

// StartGroupConsolidatorSession creates and starts a backend session for consolidating a group.
// This is a package-level function that replaces the former Coordinator method,
// delegating to the consolidate package.
func StartGroupConsolidatorSession(c *Coordinator, groupIndex int) error {
	adapter := newCoordinatorConsolidateAdapter(c)
	consolidator := consolidate.NewConsolidator(adapter)
	return consolidator.StartSession(groupIndex)
}

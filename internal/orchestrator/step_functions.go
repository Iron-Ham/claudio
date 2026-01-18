package orchestrator

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator/phase/step"
)

// GetStepInfo returns information about a step given its instance ID.
// This is used by the TUI to determine what kind of step is selected for restart/input operations.
// It queries both session state and phase orchestrators to ensure consistency.
//
// This is a package-level function that replaces the former Coordinator.GetStepInfo method,
// delegating to step.Resolver for the actual introspection logic.
func GetStepInfo(c *Coordinator, instanceID string) *StepInfo {
	adapter := newCoordinatorStepAdapter(c)
	resolver := step.NewResolver(adapter)
	stepInfo := resolver.GetStepInfo(instanceID)
	if stepInfo == nil {
		return nil
	}
	// Convert step.StepInfo to orchestrator.StepInfo
	return &StepInfo{
		Type:       StepType(stepInfo.Type),
		InstanceID: stepInfo.InstanceID,
		TaskID:     stepInfo.TaskID,
		GroupIndex: stepInfo.GroupIndex,
		Label:      stepInfo.Label,
	}
}

// RestartStep restarts the specified step. This stops any existing instance for that step
// and starts a fresh one. Returns the new instance ID or an error.
//
// This is a package-level function that replaces the former Coordinator.RestartStep method,
// delegating to step.Restarter for the actual restart logic.
func RestartStep(c *Coordinator, stepInfo *StepInfo) (string, error) {
	if stepInfo == nil {
		return "", fmt.Errorf("step info is nil")
	}
	// Convert orchestrator.StepInfo to step.StepInfo
	si := &step.StepInfo{
		Type:       step.StepType(stepInfo.Type),
		InstanceID: stepInfo.InstanceID,
		TaskID:     stepInfo.TaskID,
		GroupIndex: stepInfo.GroupIndex,
		Label:      stepInfo.Label,
	}
	adapter := newCoordinatorStepAdapter(c)
	restarter := step.NewRestarter(adapter)
	return restarter.RestartStep(si)
}

package step

import "fmt"

// Resolver provides step introspection for ultra-plan workflows.
// It resolves instance IDs to step information by checking all possible
// step types and their associated orchestrator states.
type Resolver struct {
	coordinator StepCoordinatorInterface
}

// NewResolver creates a new step resolver with the given coordinator interface.
func NewResolver(coord StepCoordinatorInterface) *Resolver {
	return &Resolver{coordinator: coord}
}

// GetStepInfo resolves an instance ID to its step information.
// Returns nil if the instance ID doesn't match any known step.
func (r *Resolver) GetStepInfo(instanceID string) *StepInfo {
	session := r.coordinator.Session()
	if session == nil || instanceID == "" {
		return nil
	}

	// Check if it's the planning coordinator (session state)
	if session.GetCoordinatorID() == instanceID {
		return &StepInfo{
			Type:       StepTypePlanning,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Planning Coordinator",
		}
	}

	// Check planning orchestrator state as fallback
	if planOrch := r.coordinator.PlanningOrchestrator(); planOrch != nil {
		if planOrch.GetInstanceID() == instanceID {
			return &StepInfo{
				Type:       StepTypePlanning,
				InstanceID: instanceID,
				GroupIndex: -1,
				Label:      "Planning Coordinator",
			}
		}
		// Check multi-pass coordinators from orchestrator state
		for i, coordID := range planOrch.GetPlanCoordinatorIDs() {
			if coordID == instanceID {
				strategies := r.coordinator.GetMultiPassStrategyNames()
				label := fmt.Sprintf("Plan Coordinator %d", i+1)
				if i < len(strategies) {
					label = fmt.Sprintf("Plan Coordinator (%s)", strategies[i])
				}
				return &StepInfo{
					Type:       StepTypePlanning,
					InstanceID: instanceID,
					GroupIndex: i,
					Label:      label,
				}
			}
		}
	}

	// Check multi-pass plan coordinators (session state)
	for i, coordID := range session.GetPlanCoordinatorIDs() {
		if coordID == instanceID {
			strategies := r.coordinator.GetMultiPassStrategyNames()
			label := fmt.Sprintf("Plan Coordinator %d", i+1)
			if i < len(strategies) {
				label = fmt.Sprintf("Plan Coordinator (%s)", strategies[i])
			}
			return &StepInfo{
				Type:       StepTypePlanning,
				InstanceID: instanceID,
				GroupIndex: i,
				Label:      label,
			}
		}
	}

	// Check if it's the plan manager
	if session.GetPlanManagerID() == instanceID {
		return &StepInfo{
			Type:       StepTypePlanManager,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Plan Manager",
		}
	}

	// Check if it's a task instance (session state)
	for taskID, instID := range session.GetTaskToInstance() {
		if instID == instanceID {
			task := session.GetTask(taskID)
			label := taskID
			if task != nil {
				label = task.GetTitle()
			}
			groupIdx := r.coordinator.GetTaskGroupIndex(taskID)
			return &StepInfo{
				Type:       StepTypeTask,
				InstanceID: instanceID,
				TaskID:     taskID,
				GroupIndex: groupIdx,
				Label:      label,
			}
		}
	}

	// Check execution orchestrator for running tasks as fallback
	if execOrch := r.coordinator.ExecutionOrchestrator(); execOrch != nil {
		state := execOrch.State()
		for taskID, instID := range state.GetRunningTasks() {
			if instID == instanceID {
				task := session.GetTask(taskID)
				label := taskID
				if task != nil {
					label = task.GetTitle()
				}
				groupIdx := r.coordinator.GetTaskGroupIndex(taskID)
				return &StepInfo{
					Type:       StepTypeTask,
					InstanceID: instanceID,
					TaskID:     taskID,
					GroupIndex: groupIdx,
					Label:      label,
				}
			}
		}
	}

	// Check group consolidators (session state)
	for i, consolidatorID := range session.GetGroupConsolidatorIDs() {
		if consolidatorID == instanceID {
			return &StepInfo{
				Type:       StepTypeGroupConsolidator,
				InstanceID: instanceID,
				GroupIndex: i,
				Label:      fmt.Sprintf("Group %d Consolidator", i+1),
			}
		}
	}

	// Check if it's the synthesis instance (session state)
	if session.GetSynthesisID() == instanceID {
		return &StepInfo{
			Type:       StepTypeSynthesis,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Synthesis",
		}
	}

	// Check synthesis orchestrator as fallback
	if synthOrch := r.coordinator.SynthesisOrchestrator(); synthOrch != nil {
		if synthOrch.GetInstanceID() == instanceID {
			return &StepInfo{
				Type:       StepTypeSynthesis,
				InstanceID: instanceID,
				GroupIndex: -1,
				Label:      "Synthesis",
			}
		}
	}

	// Check if it's the revision instance (session state)
	if session.GetRevisionID() == instanceID {
		return &StepInfo{
			Type:       StepTypeRevision,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Revision",
		}
	}

	// Check synthesis orchestrator for revision running tasks as fallback
	if synthOrch := r.coordinator.SynthesisOrchestrator(); synthOrch != nil {
		synthState := synthOrch.State()
		for _, instID := range synthState.GetRunningRevisionTasks() {
			if instID == instanceID {
				return &StepInfo{
					Type:       StepTypeRevision,
					InstanceID: instanceID,
					GroupIndex: -1,
					Label:      "Revision",
				}
			}
		}
	}

	// Check if it's the consolidation instance (session state)
	if session.GetConsolidationID() == instanceID {
		return &StepInfo{
			Type:       StepTypeConsolidation,
			InstanceID: instanceID,
			GroupIndex: -1,
			Label:      "Consolidation",
		}
	}

	// Check consolidation orchestrator as fallback
	if consolOrch := r.coordinator.ConsolidationOrchestrator(); consolOrch != nil {
		if consolOrch.GetInstanceID() == instanceID {
			return &StepInfo{
				Type:       StepTypeConsolidation,
				InstanceID: instanceID,
				GroupIndex: -1,
				Label:      "Consolidation",
			}
		}
	}

	return nil
}

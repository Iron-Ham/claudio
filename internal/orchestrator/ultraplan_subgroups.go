package orchestrator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator/grouptypes"
)

// UltraPlan subgroup name constants.
const (
	SubgroupPlanning           = "Planning"
	SubgroupPlanSelection      = "Plan Selection"
	SubgroupSynthesis          = "Synthesis"
	SubgroupRevision           = "Revision"
	SubgroupConsolidation      = "Consolidation"
	SubgroupExecutionPrefix    = "Group"        // e.g., "Group 1", "Group 2"
	SubgroupConsolidatorSuffix = "Consolidator" // e.g., "Group 1 Consolidator"
)

// SubgroupType represents the type of ultraplan subgroup.
type SubgroupType int

const (
	SubgroupTypeUnknown SubgroupType = iota
	SubgroupTypePlanning
	SubgroupTypePlanSelection
	SubgroupTypeExecution    // For task instances
	SubgroupTypeConsolidator // For group consolidators
	SubgroupTypeSynthesis
	SubgroupTypeRevision
	SubgroupTypeFinalConsolidation
)

// determineSubgroupType determines which subgroup type an instance belongs to
// based on the UltraPlanSession state.
func determineSubgroupType(session *UltraPlanSession, instanceID string) SubgroupType {
	if session == nil || instanceID == "" {
		return SubgroupTypeUnknown
	}

	// Check planning coordinators
	if session.CoordinatorID == instanceID {
		return SubgroupTypePlanning
	}
	if slices.Contains(session.PlanCoordinatorIDs, instanceID) {
		return SubgroupTypePlanning
	}

	// Check plan manager (plan selection)
	if session.PlanManagerID == instanceID {
		return SubgroupTypePlanSelection
	}

	// Check task instances
	for _, instID := range session.TaskToInstance {
		if instID == instanceID {
			return SubgroupTypeExecution
		}
	}

	// Check group consolidators
	if slices.Contains(session.GroupConsolidatorIDs, instanceID) {
		return SubgroupTypeConsolidator
	}

	// Check synthesis
	if session.SynthesisID == instanceID {
		return SubgroupTypeSynthesis
	}

	// Check revision
	if session.RevisionID == instanceID {
		return SubgroupTypeRevision
	}

	// Check final consolidation
	if session.ConsolidationID == instanceID {
		return SubgroupTypeFinalConsolidation
	}

	return SubgroupTypeUnknown
}

// getTaskGroupIndex returns the execution group index for a task instance.
// Returns -1 if the instance is not a task instance or group cannot be determined.
func getTaskGroupIndex(session *UltraPlanSession, instanceID string) int {
	if session == nil || session.Plan == nil {
		return -1
	}

	// Find the task ID for this instance
	var taskID string
	for tID, instID := range session.TaskToInstance {
		if instID == instanceID {
			taskID = tID
			break
		}
	}
	if taskID == "" {
		return -1
	}

	// Find which execution group this task belongs to
	for groupIdx, taskIDs := range session.Plan.ExecutionOrder {
		if slices.Contains(taskIDs, taskID) {
			return groupIdx
		}
	}

	return -1
}

// getConsolidatorGroupIndex returns the execution group index for a group consolidator instance.
// Returns -1 if the instance is not a consolidator or group cannot be determined.
func getConsolidatorGroupIndex(session *UltraPlanSession, instanceID string) int {
	if session == nil {
		return -1
	}
	return slices.Index(session.GroupConsolidatorIDs, instanceID)
}

// executionGroupSubgroupName returns the subgroup name for an execution group.
func executionGroupSubgroupName(groupIndex int) string {
	return fmt.Sprintf("%s %d", SubgroupExecutionPrefix, groupIndex+1)
}

// getOrCreateSubgroup finds or creates a subgroup within the ultraplan group.
// The subgroup is created with the appropriate name and added to the parent's SubGroups.
func getOrCreateSubgroup(parentGroup *grouptypes.InstanceGroup, subgroupName string) *grouptypes.InstanceGroup {
	if parentGroup == nil {
		return nil
	}

	// First, try to find existing subgroup
	for _, sg := range parentGroup.SubGroups {
		if sg.Name == subgroupName {
			return sg
		}
	}

	// Create new subgroup
	subgroupID := fmt.Sprintf("%s-%s", parentGroup.ID, sanitizeSubgroupID(subgroupName))
	subgroup := grouptypes.NewInstanceGroup(subgroupID, subgroupName)
	parentGroup.AddSubGroup(subgroup)

	return subgroup
}

// sanitizeSubgroupID converts a subgroup name to a valid ID component.
// Lowercase letters and digits are preserved, spaces become dashes, other characters are removed.
func sanitizeSubgroupID(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == ' ':
			return '-'
		default:
			return -1 // Remove character
		}
	}, name)
}

// addInstanceToSubgroup determines the correct subgroup for an instance and adds it there.
// Returns true if the instance was added to a subgroup, false otherwise.
func addInstanceToSubgroup(parentGroup *grouptypes.InstanceGroup, session *UltraPlanSession, instanceID string) bool {
	if parentGroup == nil || session == nil || instanceID == "" {
		return false
	}

	subgroupType := determineSubgroupType(session, instanceID)
	var subgroupName string

	switch subgroupType {
	case SubgroupTypePlanning:
		subgroupName = SubgroupPlanning
	case SubgroupTypePlanSelection:
		subgroupName = SubgroupPlanSelection
	case SubgroupTypeExecution:
		groupIdx := getTaskGroupIndex(session, instanceID)
		if groupIdx < 0 {
			return false
		}
		subgroupName = executionGroupSubgroupName(groupIdx)
	case SubgroupTypeConsolidator:
		groupIdx := getConsolidatorGroupIndex(session, instanceID)
		if groupIdx < 0 {
			return false
		}
		// Add consolidator to the same subgroup as its execution group
		subgroupName = executionGroupSubgroupName(groupIdx)
	case SubgroupTypeSynthesis:
		subgroupName = SubgroupSynthesis
	case SubgroupTypeRevision:
		subgroupName = SubgroupRevision
	case SubgroupTypeFinalConsolidation:
		subgroupName = SubgroupConsolidation
	default:
		// Unknown type - add to parent group directly
		parentGroup.AddInstance(instanceID)
		return true
	}

	// Get or create the subgroup and add the instance
	subgroup := getOrCreateSubgroup(parentGroup, subgroupName)
	if subgroup == nil {
		return false
	}

	subgroup.AddInstance(instanceID)
	return true
}

// SubgroupRouter is a wrapper that routes AddInstance calls to the appropriate subgroup.
// This implements the interface expected by phase.addInstanceToGroup.
type SubgroupRouter struct {
	parentGroup *grouptypes.InstanceGroup
	session     *UltraPlanSession
}

// NewSubgroupRouter creates a new SubgroupRouter for routing instances to ultraplan subgroups.
func NewSubgroupRouter(parentGroup *grouptypes.InstanceGroup, session *UltraPlanSession) *SubgroupRouter {
	return &SubgroupRouter{
		parentGroup: parentGroup,
		session:     session,
	}
}

// AddInstance routes the instance to the appropriate subgroup based on session state.
// This method is called by phase.addInstanceToGroup via interface.
func (r *SubgroupRouter) AddInstance(instanceID string) {
	if r.parentGroup == nil || instanceID == "" {
		return
	}

	// Try to route to a subgroup
	if r.session != nil && addInstanceToSubgroup(r.parentGroup, r.session, instanceID) {
		return
	}

	// Fallback: add to main group
	r.parentGroup.AddInstance(instanceID)
}

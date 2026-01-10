package orchestrator

import (
	"time"
)

// =============================================================================
// Coordinator Events
// =============================================================================

// CoordinatorEventType represents the type of coordinator event
type CoordinatorEventType string

const (
	EventTaskStarted   CoordinatorEventType = "task_started"
	EventTaskComplete  CoordinatorEventType = "task_complete"
	EventTaskFailed    CoordinatorEventType = "task_failed"
	EventTaskBlocked   CoordinatorEventType = "task_blocked"
	EventGroupComplete CoordinatorEventType = "group_complete"
	EventPhaseChange   CoordinatorEventType = "phase_change"
	EventConflict      CoordinatorEventType = "conflict"
	EventPlanReady     CoordinatorEventType = "plan_ready"

	// Multi-pass planning events
	EventMultiPassPlanGenerated CoordinatorEventType = "multipass_plan_generated" // One coordinator finished planning
	EventAllPlansGenerated      CoordinatorEventType = "all_plans_generated"      // All coordinators finished
	EventPlanSelectionStarted   CoordinatorEventType = "plan_selection_started"   // Manager started evaluating
	EventPlanSelected           CoordinatorEventType = "plan_selected"            // Final plan chosen
)

// CoordinatorEvent represents an event from the coordinator during execution
type CoordinatorEvent struct {
	Type       CoordinatorEventType `json:"type"`
	TaskID     string               `json:"task_id,omitempty"`
	InstanceID string               `json:"instance_id,omitempty"`
	Message    string               `json:"message,omitempty"`
	Timestamp  time.Time            `json:"timestamp"`
	// Multi-pass planning fields
	PlanIndex int    `json:"plan_index,omitempty"` // Which plan was generated/selected (0-indexed)
	Strategy  string `json:"strategy,omitempty"`   // Planning strategy name (e.g., "maximize-parallelism", "minimize-complexity", "balanced-approach")
}

// =============================================================================
// Consolidation Events
// =============================================================================

// ConsolidationEventType represents events during consolidation
type ConsolidationEventType string

const (
	EventConsolidationStarted       ConsolidationEventType = "consolidation_started"
	EventConsolidationGroupStarted  ConsolidationEventType = "consolidation_group_started"
	EventConsolidationTaskMerging   ConsolidationEventType = "consolidation_task_merging"
	EventConsolidationTaskMerged    ConsolidationEventType = "consolidation_task_merged"
	EventConsolidationGroupComplete ConsolidationEventType = "consolidation_group_complete"
	EventConsolidationPRCreating    ConsolidationEventType = "consolidation_pr_creating"
	EventConsolidationPRCreated     ConsolidationEventType = "consolidation_pr_created"
	EventConsolidationConflict      ConsolidationEventType = "consolidation_conflict"
	EventConsolidationComplete      ConsolidationEventType = "consolidation_complete"
	EventConsolidationFailed        ConsolidationEventType = "consolidation_failed"
)

// ConsolidationEvent represents an event during consolidation
type ConsolidationEvent struct {
	Type      ConsolidationEventType `json:"type"`
	GroupIdx  int                    `json:"group_idx,omitempty"`
	TaskID    string                 `json:"task_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

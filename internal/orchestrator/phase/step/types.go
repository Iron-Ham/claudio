package step

// StepType represents the type of step in an ultra-plan workflow.
type StepType string

const (
	StepTypePlanning          StepType = "planning"
	StepTypePlanManager       StepType = "plan_manager"
	StepTypeTask              StepType = "task"
	StepTypeSynthesis         StepType = "synthesis"
	StepTypeRevision          StepType = "revision"
	StepTypeConsolidation     StepType = "consolidation"
	StepTypeGroupConsolidator StepType = "group_consolidator"
)

// StepInfo contains information about a specific step in the workflow.
type StepInfo struct {
	Type       StepType
	InstanceID string
	TaskID     string // Only set for task and group_consolidator steps
	GroupIndex int    // Only set for task and group_consolidator steps (-1 otherwise)
	Label      string // Human-readable description
}

package orchestrator

import (
	"time"
)

// TaskComplexity represents the estimated complexity of a planned task
type TaskComplexity string

const (
	ComplexityLow    TaskComplexity = "low"
	ComplexityMedium TaskComplexity = "medium"
	ComplexityHigh   TaskComplexity = "high"
)

// PlannedTask represents a single decomposed task from the planning phase
type PlannedTask struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`     // Detailed task prompt for child session
	Files         []string       `json:"files,omitempty"` // Expected files to be modified
	DependsOn     []string       `json:"depends_on"`      // Task IDs this depends on
	Priority      int            `json:"priority"`        // Execution priority (lower = earlier)
	EstComplexity TaskComplexity `json:"est_complexity"`
}

// PlanSpec represents the output of the planning phase
type PlanSpec struct {
	ID              string              `json:"id"`
	Objective       string              `json:"objective"`        // Original user request
	Summary         string              `json:"summary"`          // Executive summary of the plan
	Tasks           []PlannedTask       `json:"tasks"`
	DependencyGraph map[string][]string `json:"dependency_graph"` // task_id -> depends_on[]
	ExecutionOrder  [][]string          `json:"execution_order"`  // Groups of parallelizable tasks
	Insights        []string            `json:"insights"`         // Key findings from exploration
	Constraints     []string            `json:"constraints"`      // Identified constraints/risks
	CreatedAt       time.Time           `json:"created_at"`
}

// PlanScore represents the evaluation of a single candidate plan
type PlanScore struct {
	Strategy   string `json:"strategy"`
	Score      int    `json:"score"`
	Strengths  string `json:"strengths"`
	Weaknesses string `json:"weaknesses"`
}

// PlanDecision captures the coordinator-manager's decision when evaluating multiple plans
type PlanDecision struct {
	Action        string      `json:"action"`         // "select" or "merge"
	SelectedIndex int         `json:"selected_index"` // 0-2 or -1 for merge
	Reasoning     string      `json:"reasoning"`
	PlanScores    []PlanScore `json:"plan_scores"`
}

// ValidationSeverity represents the severity level of a validation message
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
	SeverityInfo    ValidationSeverity = "info"
)

// ValidationMessage represents a single validation issue with structured information
type ValidationMessage struct {
	Severity   ValidationSeverity `json:"severity"`
	Message    string             `json:"message"`
	TaskID     string             `json:"task_id,omitempty"`     // The task this message relates to (empty for plan-level issues)
	Field      string             `json:"field,omitempty"`       // The field causing the issue (e.g., "depends_on", "description")
	Suggestion string             `json:"suggestion,omitempty"`  // A suggested fix
	RelatedIDs []string           `json:"related_ids,omitempty"` // Related task IDs (for cycles, conflicts, etc.)
}

// ValidationResult contains the complete validation results for a plan
type ValidationResult struct {
	IsValid      bool                `json:"is_valid"` // True if there are no errors (warnings allowed)
	Messages     []ValidationMessage `json:"messages"`
	ErrorCount   int                 `json:"error_count"`
	WarningCount int                 `json:"warning_count"`
	InfoCount    int                 `json:"info_count"`
}

// HasErrors returns true if there are any error-level messages
func (v *ValidationResult) HasErrors() bool {
	return v.ErrorCount > 0
}

// HasWarnings returns true if there are any warning-level messages
func (v *ValidationResult) HasWarnings() bool {
	return v.WarningCount > 0
}

// GetMessagesForTask returns all validation messages for a specific task
func (v *ValidationResult) GetMessagesForTask(taskID string) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.TaskID == taskID {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetMessagesBySeverity returns all messages of a specific severity
func (v *ValidationResult) GetMessagesBySeverity(severity ValidationSeverity) []ValidationMessage {
	var messages []ValidationMessage
	for _, msg := range v.Messages {
		if msg.Severity == severity {
			messages = append(messages, msg)
		}
	}
	return messages
}

// Package validation provides a composable validation framework for plan editing.
// It defines interfaces and types for building modular validation rules that can
// be combined to validate plans and tasks.
package validation

// Severity represents the severity level of a validation issue.
// It determines how the issue should be treated by the validation consumer.
type Severity int

const (
	// SeverityInfo indicates an informational message that doesn't affect validity.
	// Use for suggestions or best practices that are not required.
	SeverityInfo Severity = iota

	// SeverityWarning indicates a potential issue that should be reviewed.
	// Warnings don't make a plan invalid but may indicate suboptimal configuration.
	SeverityWarning

	// SeverityError indicates a critical issue that makes the plan invalid.
	// Errors must be resolved before the plan can be executed.
	SeverityError
)

// String returns the string representation of a Severity level.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseSeverity converts a string to a Severity level.
// Returns SeverityInfo for unrecognized strings.
func ParseSeverity(s string) Severity {
	switch s {
	case "error":
		return SeverityError
	case "warning":
		return SeverityWarning
	case "info":
		return SeverityInfo
	default:
		return SeverityInfo
	}
}

// ValidationError represents a single validation issue found during plan validation.
// It provides structured information about the issue including its location,
// severity, and suggested remediation.
type ValidationError struct {
	// RuleName identifies which validation rule generated this error.
	RuleName string `json:"rule_name"`

	// Message is a human-readable description of the validation issue.
	Message string `json:"message"`

	// TaskID identifies the specific task this error relates to.
	// Empty string indicates a plan-level issue not tied to a specific task.
	TaskID string `json:"task_id,omitempty"`

	// Severity indicates the importance of this validation issue.
	Severity Severity `json:"severity"`

	// Field identifies the specific field causing the issue (e.g., "depends_on", "title").
	// Empty string if not applicable to a specific field.
	Field string `json:"field,omitempty"`

	// Suggestion provides actionable guidance for resolving the issue.
	Suggestion string `json:"suggestion,omitempty"`

	// RelatedTaskIDs lists other task IDs related to this issue.
	// Useful for issues like dependency cycles or file conflicts between tasks.
	RelatedTaskIDs []string `json:"related_task_ids,omitempty"`
}

// IsError returns true if this validation error has error severity.
func (e ValidationError) IsError() bool {
	return e.Severity == SeverityError
}

// IsWarning returns true if this validation error has warning severity.
func (e ValidationError) IsWarning() bool {
	return e.Severity == SeverityWarning
}

// IsInfo returns true if this validation error has info severity.
func (e ValidationError) IsInfo() bool {
	return e.Severity == SeverityInfo
}

// Task represents the minimal interface for a task being validated.
// This abstraction allows validation rules to work with different task implementations.
type Task interface {
	// GetID returns the unique identifier for this task.
	GetID() string

	// GetTitle returns the task's title/name.
	GetTitle() string

	// GetDescription returns the detailed task description.
	GetDescription() string

	// GetFiles returns the list of files this task expects to modify.
	GetFiles() []string

	// GetDependencies returns the IDs of tasks this task depends on.
	GetDependencies() []string

	// GetPriority returns the task's execution priority.
	GetPriority() int

	// GetComplexity returns the estimated complexity level.
	GetComplexity() string
}

// Plan represents the minimal interface for a plan being validated.
// This abstraction allows validation rules to work with different plan implementations.
type Plan interface {
	// GetID returns the unique identifier for this plan.
	GetID() string

	// GetObjective returns the plan's primary objective.
	GetObjective() string

	// GetTasks returns all tasks in the plan.
	GetTasks() []Task

	// GetTaskByID returns a specific task by ID, or nil if not found.
	GetTaskByID(id string) Task

	// GetExecutionOrder returns the planned execution order as groups of task IDs.
	// Tasks within the same group can be executed in parallel.
	GetExecutionOrder() [][]string

	// GetDependencyGraph returns the dependency relationships.
	// Maps task ID to the IDs of tasks it depends on.
	GetDependencyGraph() map[string][]string
}

// ValidationRule defines the interface for a single validation check.
// Each rule encapsulates a specific validation concern, enabling
// modular and composable validation logic.
type ValidationRule interface {
	// Name returns a unique identifier for this validation rule.
	// Used in error reporting to identify which rule generated an error.
	Name() string

	// Validate checks the given plan and returns any validation errors found.
	// The returned slice may be empty if no issues are found.
	// Rules should not panic; any internal errors should be returned as ValidationErrors.
	Validate(plan Plan) []ValidationError

	// Severity returns the default severity level for issues found by this rule.
	// Individual ValidationErrors may override this if needed.
	Severity() Severity
}

// TaskValidationRule defines the interface for rules that validate individual tasks.
// This is a specialized form of ValidationRule for task-scoped validation.
type TaskValidationRule interface {
	// Name returns a unique identifier for this validation rule.
	Name() string

	// ValidateTask checks a single task within the context of its plan.
	// The plan parameter provides context for cross-task validation.
	ValidateTask(task Task, plan Plan) []ValidationError

	// Severity returns the default severity level for issues found by this rule.
	Severity() Severity
}

// ValidationResult aggregates the results of running multiple validation rules.
// It provides convenient methods for querying and filtering validation issues.
type ValidationResult struct {
	// Errors contains all validation issues found, regardless of severity.
	Errors []ValidationError `json:"errors"`

	// ErrorCount is the number of error-severity issues.
	ErrorCount int `json:"error_count"`

	// WarningCount is the number of warning-severity issues.
	WarningCount int `json:"warning_count"`

	// InfoCount is the number of info-severity issues.
	InfoCount int `json:"info_count"`
}

// NewValidationResult creates a new empty ValidationResult.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Errors: make([]ValidationError, 0),
	}
}

// IsValid returns true if there are no error-severity issues.
// A plan with only warnings and info messages is considered valid.
func (r *ValidationResult) IsValid() bool {
	return r.ErrorCount == 0
}

// HasErrors returns true if there are any error-severity issues.
func (r *ValidationResult) HasErrors() bool {
	return r.ErrorCount > 0
}

// HasWarnings returns true if there are any warning-severity issues.
func (r *ValidationResult) HasWarnings() bool {
	return r.WarningCount > 0
}

// HasIssues returns true if there are any issues of any severity.
func (r *ValidationResult) HasIssues() bool {
	return len(r.Errors) > 0
}

// AddError adds a validation error to the result and updates counts.
func (r *ValidationResult) AddError(err ValidationError) {
	r.Errors = append(r.Errors, err)
	switch err.Severity {
	case SeverityError:
		r.ErrorCount++
	case SeverityWarning:
		r.WarningCount++
	case SeverityInfo:
		r.InfoCount++
	}
}

// AddErrors adds multiple validation errors to the result.
func (r *ValidationResult) AddErrors(errs []ValidationError) {
	for _, err := range errs {
		r.AddError(err)
	}
}

// Merge combines another ValidationResult into this one.
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}
	r.AddErrors(other.Errors)
}

// GetErrorsOnly returns only error-severity issues.
func (r *ValidationResult) GetErrorsOnly() []ValidationError {
	return r.getBySeverity(SeverityError)
}

// GetWarningsOnly returns only warning-severity issues.
func (r *ValidationResult) GetWarningsOnly() []ValidationError {
	return r.getBySeverity(SeverityWarning)
}

// GetInfoOnly returns only info-severity issues.
func (r *ValidationResult) GetInfoOnly() []ValidationError {
	return r.getBySeverity(SeverityInfo)
}

// GetByTaskID returns all issues related to a specific task.
func (r *ValidationResult) GetByTaskID(taskID string) []ValidationError {
	var result []ValidationError
	for _, err := range r.Errors {
		if err.TaskID == taskID {
			result = append(result, err)
		}
	}
	return result
}

// GetByRule returns all issues generated by a specific rule.
func (r *ValidationResult) GetByRule(ruleName string) []ValidationError {
	var result []ValidationError
	for _, err := range r.Errors {
		if err.RuleName == ruleName {
			result = append(result, err)
		}
	}
	return result
}

// getBySeverity returns all issues with the specified severity.
func (r *ValidationResult) getBySeverity(severity Severity) []ValidationError {
	var result []ValidationError
	for _, err := range r.Errors {
		if err.Severity == severity {
			result = append(result, err)
		}
	}
	return result
}

// Validator defines the interface for a validation orchestrator.
// It manages a collection of validation rules and coordinates their execution.
type Validator interface {
	// AddRule adds a validation rule to this validator.
	// Rules are executed in the order they are added.
	AddRule(rule ValidationRule)

	// AddTaskRule adds a task-specific validation rule.
	// Task rules are applied to each task in the plan.
	AddTaskRule(rule TaskValidationRule)

	// Validate runs all registered rules against the given plan.
	// Returns a ValidationResult containing all issues found.
	Validate(plan Plan) *ValidationResult

	// ValidateTask runs only task-specific rules against a single task.
	// Useful for incremental validation during editing.
	ValidateTask(task Task, plan Plan) *ValidationResult
}

// RuleOption is a function that configures a validation rule.
// Used for optional configuration when creating rules.
type RuleOption func(any)

// WithSeverity creates a RuleOption that sets the severity level.
func WithSeverity(severity Severity) RuleOption {
	return func(rule any) {
		if s, ok := rule.(interface{ SetSeverity(Severity) }); ok {
			s.SetSeverity(severity)
		}
	}
}

// ErrorBuilder provides a fluent API for constructing ValidationErrors.
type ErrorBuilder struct {
	err ValidationError
}

// NewError creates a new ErrorBuilder with the given rule name and message.
func NewError(ruleName, message string) *ErrorBuilder {
	return &ErrorBuilder{
		err: ValidationError{
			RuleName: ruleName,
			Message:  message,
			Severity: SeverityError,
		},
	}
}

// WithSeverity sets the severity level.
func (b *ErrorBuilder) WithSeverity(s Severity) *ErrorBuilder {
	b.err.Severity = s
	return b
}

// ForTask sets the task ID this error relates to.
func (b *ErrorBuilder) ForTask(taskID string) *ErrorBuilder {
	b.err.TaskID = taskID
	return b
}

// OnField sets the specific field causing the issue.
func (b *ErrorBuilder) OnField(field string) *ErrorBuilder {
	b.err.Field = field
	return b
}

// WithSuggestion adds a suggested fix.
func (b *ErrorBuilder) WithSuggestion(suggestion string) *ErrorBuilder {
	b.err.Suggestion = suggestion
	return b
}

// WithRelatedTasks adds related task IDs.
func (b *ErrorBuilder) WithRelatedTasks(taskIDs ...string) *ErrorBuilder {
	b.err.RelatedTaskIDs = taskIDs
	return b
}

// Build returns the constructed ValidationError.
func (b *ErrorBuilder) Build() ValidationError {
	return b.err
}

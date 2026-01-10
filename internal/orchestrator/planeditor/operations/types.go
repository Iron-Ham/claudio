// Package operations defines the Command Pattern interfaces for undoable plan editing.
// It provides the foundation for implementing undo/redo functionality in the plan editor.
package operations

import (
	"errors"
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// Common errors for edit operations
var (
	// ErrNilPlan is returned when an operation is attempted on a nil plan.
	ErrNilPlan = errors.New("plan is nil")

	// ErrNothingToUndo is returned when Undo is called with no operations to undo.
	ErrNothingToUndo = errors.New("nothing to undo")

	// ErrNothingToRedo is returned when Redo is called with no operations to redo.
	ErrNothingToRedo = errors.New("nothing to redo")

	// ErrOperationFailed is returned when an operation fails to execute.
	ErrOperationFailed = errors.New("operation failed")

	// ErrUndoFailed is returned when an undo operation fails.
	ErrUndoFailed = errors.New("undo failed")

	// ErrInvalidState is returned when the editor is in an invalid state for the requested operation.
	ErrInvalidState = errors.New("invalid editor state")
)

// OperationError wraps an error with additional context about the failed operation.
type OperationError struct {
	Op      string // The operation name (e.g., "UpdateTaskTitle", "DeleteTask")
	TaskID  string // The task ID involved, if any
	Cause   error  // The underlying error
	Message string // Human-readable description
}

// Error implements the error interface.
func (e *OperationError) Error() string {
	if e.TaskID != "" {
		return fmt.Sprintf("%s (task %s): %s", e.Op, e.TaskID, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *OperationError) Unwrap() error {
	return e.Cause
}

// NewOperationError creates a new OperationError.
func NewOperationError(op, taskID, message string, cause error) *OperationError {
	return &OperationError{
		Op:      op,
		TaskID:  taskID,
		Cause:   cause,
		Message: message,
	}
}

// EditOperation represents a single undoable operation on a plan.
// Operations follow the Command Pattern, encapsulating both the action
// and the information needed to reverse it.
//
// Implementations must capture enough state during Execute to enable
// a complete Undo. For example, a DeleteTask operation must store
// the deleted task's data, position, and dependency relationships.
type EditOperation interface {
	// Execute applies the operation to the plan.
	// It should capture any state needed for Undo before making changes.
	// Returns an error if the operation cannot be applied.
	Execute(plan *orchestrator.PlanSpec) error

	// Undo reverses the operation, restoring the plan to its previous state.
	// Returns an error if the operation cannot be undone.
	Undo(plan *orchestrator.PlanSpec) error

	// Description returns a human-readable description of the operation.
	// This is used for UI display (e.g., "Undo: Delete task 'Implement auth'").
	Description() string
}

// OperationHistory manages the undo/redo stack for edit operations.
// It maintains two stacks: one for undoable operations and one for
// operations that have been undone (available for redo).
//
// The redo stack is cleared whenever a new operation is pushed,
// as the operation timeline has diverged from the previous redo history.
type OperationHistory interface {
	// Push adds a new operation to the history.
	// This clears the redo stack as the timeline has diverged.
	Push(op EditOperation)

	// Undo reverses the most recent operation.
	// The undone operation is moved to the redo stack.
	// Returns ErrNothingToUndo if the undo stack is empty.
	Undo() error

	// Redo reapplies the most recently undone operation.
	// The redone operation is moved back to the undo stack.
	// Returns ErrNothingToRedo if the redo stack is empty.
	Redo() error

	// CanUndo returns true if there are operations available to undo.
	CanUndo() bool

	// CanRedo returns true if there are operations available to redo.
	CanRedo() bool

	// UndoDescription returns the description of the next operation to be undone.
	// Returns an empty string if there is nothing to undo.
	UndoDescription() string

	// RedoDescription returns the description of the next operation to be redone.
	// Returns an empty string if there is nothing to redo.
	RedoDescription() string

	// Clear removes all operations from both undo and redo stacks.
	Clear()

	// UndoCount returns the number of operations available to undo.
	UndoCount() int

	// RedoCount returns the number of operations available to redo.
	RedoCount() int
}

// Editor provides a high-level interface for editing plans with undo support.
// It wraps the operation history and provides a convenient API for applying
// operations while automatically managing the history.
type Editor interface {
	// Apply executes an operation on the plan and adds it to the history.
	// If the operation fails, it is not added to the history.
	// Returns an error if the operation cannot be applied.
	Apply(op EditOperation) error

	// Undo reverses the most recent operation.
	// Returns ErrNothingToUndo if there are no operations to undo.
	Undo() error

	// Redo reapplies the most recently undone operation.
	// Returns ErrNothingToRedo if there are no operations to redo.
	Redo() error

	// GetHistory returns the operation history for inspection.
	GetHistory() OperationHistory

	// GetPlan returns the current plan being edited.
	GetPlan() *orchestrator.PlanSpec

	// CanUndo returns true if there are operations available to undo.
	CanUndo() bool

	// CanRedo returns true if there are operations available to redo.
	CanRedo() bool

	// IsDirty returns true if the plan has been modified since last save/mark.
	IsDirty() bool

	// MarkClean marks the current state as clean (e.g., after saving).
	MarkClean()
}

// OperationType categorizes operations for grouping and filtering.
type OperationType int

const (
	// OpTypeUnknown is the default/unset operation type.
	OpTypeUnknown OperationType = iota

	// OpTypeFieldUpdate represents simple field updates (title, description, etc.).
	OpTypeFieldUpdate

	// OpTypeDependencyChange represents changes to task dependencies.
	OpTypeDependencyChange

	// OpTypeStructural represents structural changes (add, delete, move tasks).
	OpTypeStructural

	// OpTypeReorder represents task reordering within the list.
	OpTypeReorder

	// OpTypeComplex represents complex operations like split/merge.
	OpTypeComplex
)

// String returns the string representation of the operation type.
func (t OperationType) String() string {
	switch t {
	case OpTypeFieldUpdate:
		return "field_update"
	case OpTypeDependencyChange:
		return "dependency_change"
	case OpTypeStructural:
		return "structural"
	case OpTypeReorder:
		return "reorder"
	case OpTypeComplex:
		return "complex"
	default:
		return "unknown"
	}
}

// TypedOperation extends EditOperation with type information.
// This is useful for operations that need to be categorized or filtered.
type TypedOperation interface {
	EditOperation

	// Type returns the category of this operation.
	Type() OperationType
}

// CompositeOperation represents a group of operations that should be
// executed and undone as a single unit. This is useful for complex
// operations that internally perform multiple steps.
type CompositeOperation interface {
	EditOperation

	// Operations returns the individual operations in this composite.
	Operations() []EditOperation

	// Count returns the number of operations in this composite.
	Count() int
}

// ValidatedOperation extends EditOperation with pre-execution validation.
// This allows operations to check preconditions before modifying the plan.
type ValidatedOperation interface {
	EditOperation

	// Validate checks if the operation can be executed on the given plan.
	// Returns nil if valid, or an error describing why it cannot be executed.
	Validate(plan *orchestrator.PlanSpec) error
}

// BatchOperation represents an operation that affects multiple tasks.
// This interface is useful for operations like "delete all completed tasks"
// or "update priority for selected tasks".
type BatchOperation interface {
	EditOperation

	// AffectedTaskIDs returns the IDs of all tasks affected by this operation.
	AffectedTaskIDs() []string
}

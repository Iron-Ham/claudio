// Package errors provides centralized error definitions and error handling utilities
// for the Claudio codebase. It defines domain-specific errors, semantic error types,
// error constructors with context wrapping, and error classification helpers.
//
// # Error Types
//
// The package provides two categories of errors:
//
// Domain-specific errors represent errors from specific subsystems:
//   - SessionError: errors related to session management
//   - InstanceError: errors related to Claude instance management
//   - CoordinatorError: errors related to task coordination/orchestration
//   - GitError: errors related to git operations (worktrees, branches, commits)
//
// Semantic errors represent common error conditions:
//   - NotFoundError: resource not found
//   - AlreadyExistsError: resource already exists
//   - ValidationError: invalid input or state
//   - TimeoutError: operation timed out
//
// # Usage
//
// Creating errors:
//
//	// Domain-specific error
//	err := errors.NewSessionError("failed to load session", errors.ErrSessionNotFound)
//
//	// Semantic error
//	err := errors.NewNotFoundError("session", "abc123")
//
//	// With context wrapping
//	err := errors.NewGitError("checkout failed", baseErr).WithBranch("feature-x")
//
// Checking errors:
//
//	// Check for specific sentinel errors
//	if errors.Is(err, errors.ErrSessionNotFound) { ... }
//
//	// Check for error types
//	var sessionErr *errors.SessionError
//	if errors.As(err, &sessionErr) { ... }
//
//	// Use classification helpers
//	if errors.IsRetryable(err) { ... }
//	if errors.IsUserFacing(err) { ... }
//
// # Error Classification
//
// Errors can be classified by severity and behavior:
//   - Retryable: transient errors that may succeed on retry
//   - UserFacing: errors safe to display to users (vs internal errors)
//   - Severity: Debug, Info, Warning, Error, Critical
package errors

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Re-export standard library functions for convenience.
// This allows callers to import only this package for all error handling.
var (
	Is     = errors.Is
	As     = errors.As
	Unwrap = errors.Unwrap
	New    = errors.New
	Join   = errors.Join
)

// Severity represents the severity level of an error.
type Severity int

const (
	// SeverityDebug is for errors that are useful for debugging but not critical.
	SeverityDebug Severity = iota
	// SeverityInfo is for informational errors that don't indicate a problem.
	SeverityInfo
	// SeverityWarning is for errors that might indicate a problem but aren't critical.
	SeverityWarning
	// SeverityError is for errors that indicate a real problem.
	SeverityError
	// SeverityCritical is for errors that require immediate attention.
	SeverityCritical
)

// String returns the string representation of the severity level.
func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "debug"
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// -----------------------------------------------------------------------------
// Sentinel Errors
// -----------------------------------------------------------------------------

// Session-related sentinel errors
var (
	// ErrSessionNotFound indicates that a session could not be found.
	ErrSessionNotFound = New("session not found")
	// ErrSessionLocked indicates that a session is locked by another process.
	ErrSessionLocked = New("session is locked")
	// ErrSessionCorrupted indicates that session data is corrupted.
	ErrSessionCorrupted = New("session data corrupted")
	// ErrSessionInactive indicates that a session is not currently active.
	ErrSessionInactive = New("session is not active")
)

// Instance-related sentinel errors
var (
	// ErrInstanceNotFound indicates that an instance could not be found.
	ErrInstanceNotFound = New("instance not found")
	// ErrInstanceAlreadyRunning indicates that an instance is already running.
	ErrInstanceAlreadyRunning = New("instance already running")
	// ErrInstanceNotRunning indicates that an instance is not running.
	ErrInstanceNotRunning = New("instance not running")
	// ErrInstanceStartFailed indicates that an instance failed to start.
	ErrInstanceStartFailed = New("instance failed to start")
	// ErrInstanceCommunication indicates a communication failure with an instance.
	ErrInstanceCommunication = New("instance communication failed")
)

// Coordinator-related sentinel errors
var (
	// ErrPlanNotFound indicates that a plan could not be found.
	ErrPlanNotFound = New("plan not found")
	// ErrPlanInvalid indicates that a plan is invalid.
	ErrPlanInvalid = New("plan is invalid")
	// ErrTaskNotFound indicates that a task could not be found.
	ErrTaskNotFound = New("task not found")
	// ErrTaskFailed indicates that a task execution failed.
	ErrTaskFailed = New("task failed")
	// ErrDependencyCycle indicates a circular dependency in tasks.
	ErrDependencyCycle = New("dependency cycle detected")
	// ErrCoordinatorCanceled indicates that coordination was canceled.
	ErrCoordinatorCanceled = New("coordinator canceled")
)

// Git-related sentinel errors
var (
	// ErrNotGitRepository indicates that the directory is not a git repository.
	ErrNotGitRepository = New("not a git repository")
	// ErrWorktreeNotFound indicates that a worktree could not be found.
	ErrWorktreeNotFound = New("worktree not found")
	// ErrWorktreeExists indicates that a worktree already exists.
	ErrWorktreeExists = New("worktree already exists")
	// ErrBranchNotFound indicates that a branch could not be found.
	ErrBranchNotFound = New("branch not found")
	// ErrBranchExists indicates that a branch already exists.
	ErrBranchExists = New("branch already exists")
	// ErrMergeConflict indicates that a merge conflict occurred.
	ErrMergeConflict = New("merge conflict")
	// ErrDirtyWorktree indicates that the worktree has uncommitted changes.
	ErrDirtyWorktree = New("worktree has uncommitted changes")
)

// General sentinel errors
var (
	// ErrTimeout indicates that an operation timed out.
	ErrTimeout = New("operation timed out")
	// ErrCanceled indicates that an operation was canceled.
	ErrCanceled = New("operation canceled")
	// ErrInvalidInput indicates that input validation failed.
	ErrInvalidInput = New("invalid input")
	// ErrOperationFailed indicates a general operation failure.
	ErrOperationFailed = New("operation failed")
)

// -----------------------------------------------------------------------------
// Base Error Interface
// -----------------------------------------------------------------------------

// ClaudioError is the base interface for all Claudio errors.
// It extends the standard error interface with additional methods for
// error handling and classification.
type ClaudioError interface {
	error

	// Unwrap returns the underlying error, if any.
	Unwrap() error

	// Is reports whether this error matches the target error.
	// This is used by errors.Is() for error comparison.
	Is(target error) bool

	// Severity returns the severity level of this error.
	Severity() Severity

	// IsRetryable returns true if the error is transient and the operation
	// may succeed on retry.
	IsRetryable() bool

	// IsUserFacing returns true if the error message is safe to display
	// to end users.
	IsUserFacing() bool
}

// -----------------------------------------------------------------------------
// Base Error Implementation
// -----------------------------------------------------------------------------

// baseError provides common functionality for all error types.
type baseError struct {
	message    string
	cause      error
	severity   Severity
	retryable  bool
	userFacing bool
}

// Error returns the error message.
func (e *baseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Unwrap returns the underlying error.
func (e *baseError) Unwrap() error {
	return e.cause
}

// Is checks if this error matches the target.
func (e *baseError) Is(target error) bool {
	if e.cause != nil {
		return errors.Is(e.cause, target)
	}
	return false
}

// Severity returns the error severity.
func (e *baseError) Severity() Severity {
	return e.severity
}

// IsRetryable returns whether the error is retryable.
func (e *baseError) IsRetryable() bool {
	return e.retryable
}

// IsUserFacing returns whether the error is safe to show users.
func (e *baseError) IsUserFacing() bool {
	return e.userFacing
}

// -----------------------------------------------------------------------------
// Domain-Specific Errors
// -----------------------------------------------------------------------------

// SessionError represents errors related to session management.
//
// Example:
//
//	err := errors.NewSessionError("failed to load session", errors.ErrSessionNotFound)
//	err = err.WithSessionID("abc123")
//	fmt.Println(err) // "session error [session=abc123]: failed to load session: session not found"
type SessionError struct {
	baseError
	SessionID string
}

// NewSessionError creates a new SessionError.
func NewSessionError(message string, cause error) *SessionError {
	return &SessionError{
		baseError: baseError{
			message:    message,
			cause:      cause,
			severity:   SeverityError,
			retryable:  false,
			userFacing: true,
		},
	}
}

// WithSessionID adds a session ID to the error context.
func (e *SessionError) WithSessionID(id string) *SessionError {
	e.SessionID = id
	return e
}

// WithSeverity sets the error severity.
func (e *SessionError) WithSeverity(s Severity) *SessionError {
	e.severity = s
	return e
}

// WithRetryable sets whether the error is retryable.
func (e *SessionError) WithRetryable(r bool) *SessionError {
	e.retryable = r
	return e
}

// Error returns the formatted error message.
func (e *SessionError) Error() string {
	var parts []string
	if e.SessionID != "" {
		parts = append(parts, fmt.Sprintf("session=%s", e.SessionID))
	}

	prefix := "session error"
	if len(parts) > 0 {
		prefix = fmt.Sprintf("session error [%s]", strings.Join(parts, ", "))
	}

	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", prefix, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", prefix, e.message)
}

// Is checks if this error matches the target.
func (e *SessionError) Is(target error) bool {
	if _, ok := target.(*SessionError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// InstanceError represents errors related to Claude instance management.
//
// Example:
//
//	err := errors.NewInstanceError("instance crashed", errors.ErrInstanceCommunication)
//	err = err.WithInstanceID("inst-1").WithTmuxSession("claudio-abc")
type InstanceError struct {
	baseError
	InstanceID  string
	TmuxSession string
}

// NewInstanceError creates a new InstanceError.
func NewInstanceError(message string, cause error) *InstanceError {
	return &InstanceError{
		baseError: baseError{
			message:    message,
			cause:      cause,
			severity:   SeverityError,
			retryable:  false,
			userFacing: true,
		},
	}
}

// WithInstanceID adds an instance ID to the error context.
func (e *InstanceError) WithInstanceID(id string) *InstanceError {
	e.InstanceID = id
	return e
}

// WithTmuxSession adds a tmux session name to the error context.
func (e *InstanceError) WithTmuxSession(session string) *InstanceError {
	e.TmuxSession = session
	return e
}

// WithSeverity sets the error severity.
func (e *InstanceError) WithSeverity(s Severity) *InstanceError {
	e.severity = s
	return e
}

// WithRetryable sets whether the error is retryable.
func (e *InstanceError) WithRetryable(r bool) *InstanceError {
	e.retryable = r
	return e
}

// Error returns the formatted error message.
func (e *InstanceError) Error() string {
	var parts []string
	if e.InstanceID != "" {
		parts = append(parts, fmt.Sprintf("instance=%s", e.InstanceID))
	}
	if e.TmuxSession != "" {
		parts = append(parts, fmt.Sprintf("tmux=%s", e.TmuxSession))
	}

	prefix := "instance error"
	if len(parts) > 0 {
		prefix = fmt.Sprintf("instance error [%s]", strings.Join(parts, ", "))
	}

	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", prefix, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", prefix, e.message)
}

// Is checks if this error matches the target.
func (e *InstanceError) Is(target error) bool {
	if _, ok := target.(*InstanceError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// CoordinatorError represents errors related to task coordination/orchestration.
//
// Example:
//
//	err := errors.NewCoordinatorError("task execution failed", errors.ErrTaskFailed)
//	err = err.WithTaskID("task-1").WithGroupIndex(2)
type CoordinatorError struct {
	baseError
	TaskID     string
	GroupIndex int
	Phase      string
}

// NewCoordinatorError creates a new CoordinatorError.
func NewCoordinatorError(message string, cause error) *CoordinatorError {
	return &CoordinatorError{
		baseError: baseError{
			message:    message,
			cause:      cause,
			severity:   SeverityError,
			retryable:  false,
			userFacing: true,
		},
		GroupIndex: -1, // -1 indicates not set
	}
}

// WithTaskID adds a task ID to the error context.
func (e *CoordinatorError) WithTaskID(id string) *CoordinatorError {
	e.TaskID = id
	return e
}

// WithGroupIndex adds an execution group index to the error context.
func (e *CoordinatorError) WithGroupIndex(idx int) *CoordinatorError {
	e.GroupIndex = idx
	return e
}

// WithPhase adds a phase name to the error context.
func (e *CoordinatorError) WithPhase(phase string) *CoordinatorError {
	e.Phase = phase
	return e
}

// WithSeverity sets the error severity.
func (e *CoordinatorError) WithSeverity(s Severity) *CoordinatorError {
	e.severity = s
	return e
}

// WithRetryable sets whether the error is retryable.
func (e *CoordinatorError) WithRetryable(r bool) *CoordinatorError {
	e.retryable = r
	return e
}

// Error returns the formatted error message.
func (e *CoordinatorError) Error() string {
	var parts []string
	if e.TaskID != "" {
		parts = append(parts, fmt.Sprintf("task=%s", e.TaskID))
	}
	if e.GroupIndex >= 0 {
		parts = append(parts, fmt.Sprintf("group=%d", e.GroupIndex))
	}
	if e.Phase != "" {
		parts = append(parts, fmt.Sprintf("phase=%s", e.Phase))
	}

	prefix := "coordinator error"
	if len(parts) > 0 {
		prefix = fmt.Sprintf("coordinator error [%s]", strings.Join(parts, ", "))
	}

	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", prefix, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", prefix, e.message)
}

// Is checks if this error matches the target.
func (e *CoordinatorError) Is(target error) bool {
	if _, ok := target.(*CoordinatorError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// GitError represents errors related to git operations.
//
// Example:
//
//	err := errors.NewGitError("failed to create worktree", errors.ErrWorktreeExists)
//	err = err.WithBranch("feature-x").WithWorktree("/path/to/worktree")
type GitError struct {
	baseError
	Branch      string
	Worktree    string
	Repository  string
	GitOutput   string // Captured git command output
}

// NewGitError creates a new GitError.
func NewGitError(message string, cause error) *GitError {
	return &GitError{
		baseError: baseError{
			message:    message,
			cause:      cause,
			severity:   SeverityError,
			retryable:  false,
			userFacing: true,
		},
	}
}

// WithBranch adds a branch name to the error context.
func (e *GitError) WithBranch(branch string) *GitError {
	e.Branch = branch
	return e
}

// WithWorktree adds a worktree path to the error context.
func (e *GitError) WithWorktree(path string) *GitError {
	e.Worktree = path
	return e
}

// WithRepository adds a repository path to the error context.
func (e *GitError) WithRepository(path string) *GitError {
	e.Repository = path
	return e
}

// WithGitOutput adds git command output to the error context.
func (e *GitError) WithGitOutput(output string) *GitError {
	e.GitOutput = output
	return e
}

// WithSeverity sets the error severity.
func (e *GitError) WithSeverity(s Severity) *GitError {
	e.severity = s
	return e
}

// WithRetryable sets whether the error is retryable.
func (e *GitError) WithRetryable(r bool) *GitError {
	e.retryable = r
	return e
}

// Error returns the formatted error message.
func (e *GitError) Error() string {
	var parts []string
	if e.Branch != "" {
		parts = append(parts, fmt.Sprintf("branch=%s", e.Branch))
	}
	if e.Worktree != "" {
		parts = append(parts, fmt.Sprintf("worktree=%s", e.Worktree))
	}
	if e.Repository != "" {
		parts = append(parts, fmt.Sprintf("repo=%s", e.Repository))
	}

	prefix := "git error"
	if len(parts) > 0 {
		prefix = fmt.Sprintf("git error [%s]", strings.Join(parts, ", "))
	}

	msg := e.message
	if e.cause != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.cause)
	}
	if e.GitOutput != "" {
		msg = fmt.Sprintf("%s\ngit output: %s", msg, e.GitOutput)
	}

	return fmt.Sprintf("%s: %s", prefix, msg)
}

// Is checks if this error matches the target.
func (e *GitError) Is(target error) bool {
	if _, ok := target.(*GitError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// -----------------------------------------------------------------------------
// Semantic Errors
// -----------------------------------------------------------------------------

// NotFoundError represents a resource that could not be found.
//
// Example:
//
//	err := errors.NewNotFoundError("session", "abc123")
//	fmt.Println(err) // "session 'abc123' not found"
type NotFoundError struct {
	baseError
	ResourceType string
	ResourceID   string
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resourceType, resourceID string) *NotFoundError {
	return &NotFoundError{
		baseError: baseError{
			message:    fmt.Sprintf("%s '%s' not found", resourceType, resourceID),
			severity:   SeverityWarning,
			retryable:  false,
			userFacing: true,
		},
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// WithCause adds a cause to the error.
func (e *NotFoundError) WithCause(cause error) *NotFoundError {
	e.cause = cause
	return e
}

// Error returns the formatted error message.
func (e *NotFoundError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s '%s' not found: %v", e.ResourceType, e.ResourceID, e.cause)
	}
	return fmt.Sprintf("%s '%s' not found", e.ResourceType, e.ResourceID)
}

// Is checks if this error matches the target.
func (e *NotFoundError) Is(target error) bool {
	if _, ok := target.(*NotFoundError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// AlreadyExistsError represents a resource that already exists.
//
// Example:
//
//	err := errors.NewAlreadyExistsError("branch", "feature-x")
//	fmt.Println(err) // "branch 'feature-x' already exists"
type AlreadyExistsError struct {
	baseError
	ResourceType string
	ResourceID   string
}

// NewAlreadyExistsError creates a new AlreadyExistsError.
func NewAlreadyExistsError(resourceType, resourceID string) *AlreadyExistsError {
	return &AlreadyExistsError{
		baseError: baseError{
			message:    fmt.Sprintf("%s '%s' already exists", resourceType, resourceID),
			severity:   SeverityWarning,
			retryable:  false,
			userFacing: true,
		},
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// WithCause adds a cause to the error.
func (e *AlreadyExistsError) WithCause(cause error) *AlreadyExistsError {
	e.cause = cause
	return e
}

// Error returns the formatted error message.
func (e *AlreadyExistsError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s '%s' already exists: %v", e.ResourceType, e.ResourceID, e.cause)
	}
	return fmt.Sprintf("%s '%s' already exists", e.ResourceType, e.ResourceID)
}

// Is checks if this error matches the target.
func (e *AlreadyExistsError) Is(target error) bool {
	if _, ok := target.(*AlreadyExistsError); ok {
		return true
	}
	return e.baseError.Is(target)
}

// ValidationError represents invalid input or state.
//
// Example:
//
//	err := errors.NewValidationError("session ID cannot be empty")
//	err = err.WithField("sessionID").WithValue("")
type ValidationError struct {
	baseError
	Field string
	Value any
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		baseError: baseError{
			message:    message,
			severity:   SeverityWarning,
			retryable:  false,
			userFacing: true,
		},
	}
}

// WithField adds a field name to the error context.
func (e *ValidationError) WithField(field string) *ValidationError {
	e.Field = field
	return e
}

// WithValue adds the invalid value to the error context.
func (e *ValidationError) WithValue(value any) *ValidationError {
	e.Value = value
	return e
}

// WithCause adds a cause to the error.
func (e *ValidationError) WithCause(cause error) *ValidationError {
	e.cause = cause
	return e
}

// Error returns the formatted error message.
func (e *ValidationError) Error() string {
	var parts []string
	if e.Field != "" {
		parts = append(parts, fmt.Sprintf("field=%s", e.Field))
	}
	if e.Value != nil {
		parts = append(parts, fmt.Sprintf("value=%v", e.Value))
	}

	prefix := "validation error"
	if len(parts) > 0 {
		prefix = fmt.Sprintf("validation error [%s]", strings.Join(parts, ", "))
	}

	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", prefix, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", prefix, e.message)
}

// Is checks if this error matches the target.
func (e *ValidationError) Is(target error) bool {
	if _, ok := target.(*ValidationError); ok {
		return true
	}
	if errors.Is(target, ErrInvalidInput) {
		return true
	}
	return e.baseError.Is(target)
}

// TimeoutError represents an operation that timed out.
//
// Example:
//
//	err := errors.NewTimeoutError("waiting for instance to start", 30*time.Second)
//	fmt.Println(err) // "timeout error: waiting for instance to start (timeout: 30s)"
type TimeoutError struct {
	baseError
	Operation string
	Duration  time.Duration
}

// NewTimeoutError creates a new TimeoutError.
func NewTimeoutError(operation string, duration time.Duration) *TimeoutError {
	return &TimeoutError{
		baseError: baseError{
			message:    operation,
			severity:   SeverityWarning,
			retryable:  true, // Timeouts are generally retryable
			userFacing: true,
		},
		Operation: operation,
		Duration:  duration,
	}
}

// WithCause adds a cause to the error.
func (e *TimeoutError) WithCause(cause error) *TimeoutError {
	e.cause = cause
	return e
}

// WithRetryable sets whether the error is retryable (default true for timeouts).
func (e *TimeoutError) WithRetryable(r bool) *TimeoutError {
	e.retryable = r
	return e
}

// Error returns the formatted error message.
func (e *TimeoutError) Error() string {
	base := fmt.Sprintf("timeout error: %s (timeout: %s)", e.Operation, e.Duration)
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", base, e.cause)
	}
	return base
}

// Is checks if this error matches the target.
func (e *TimeoutError) Is(target error) bool {
	if _, ok := target.(*TimeoutError); ok {
		return true
	}
	if errors.Is(target, ErrTimeout) {
		return true
	}
	return e.baseError.Is(target)
}

// -----------------------------------------------------------------------------
// Error Classification Helpers
// -----------------------------------------------------------------------------

// IsRetryable returns true if the error represents a transient condition
// that may succeed on retry. This checks for:
//   - Errors implementing ClaudioError with IsRetryable() returning true
//   - TimeoutError instances
//   - Errors wrapping ErrTimeout
//
// Example:
//
//	if errors.IsRetryable(err) {
//	    time.Sleep(backoff)
//	    return retry(operation)
//	}
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements ClaudioError
	var claudioErr ClaudioError
	if As(err, &claudioErr) {
		return claudioErr.IsRetryable()
	}

	// Check for known retryable sentinel errors
	if Is(err, ErrTimeout) {
		return true
	}

	return false
}

// IsUserFacing returns true if the error message is safe to display to end users.
// This checks for:
//   - Errors implementing ClaudioError with IsUserFacing() returning true
//   - Semantic errors (NotFoundError, AlreadyExistsError, ValidationError, TimeoutError)
//
// Example:
//
//	if errors.IsUserFacing(err) {
//	    displayToUser(err.Error())
//	} else {
//	    displayToUser("An internal error occurred")
//	    log.Error("internal error", "err", err)
//	}
func IsUserFacing(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements ClaudioError
	var claudioErr ClaudioError
	if As(err, &claudioErr) {
		return claudioErr.IsUserFacing()
	}

	// Semantic errors are always user-facing
	var notFound *NotFoundError
	var alreadyExists *AlreadyExistsError
	var validation *ValidationError
	var timeout *TimeoutError

	if As(err, &notFound) || As(err, &alreadyExists) ||
		As(err, &validation) || As(err, &timeout) {
		return true
	}

	return false
}

// GetSeverity returns the severity level of the error.
// Returns SeverityError for errors that don't implement ClaudioError.
//
// Example:
//
//	switch errors.GetSeverity(err) {
//	case errors.SeverityCritical:
//	    alertOnCall(err)
//	case errors.SeverityError:
//	    log.Error("error occurred", "err", err)
//	case errors.SeverityWarning:
//	    log.Warn("warning", "err", err)
//	}
func GetSeverity(err error) Severity {
	if err == nil {
		return SeverityDebug
	}

	// Check if error implements ClaudioError
	var claudioErr ClaudioError
	if As(err, &claudioErr) {
		return claudioErr.Severity()
	}

	// Default to Error severity for unknown errors
	return SeverityError
}

// IsDomainError returns true if the error is a domain-specific error
// (SessionError, InstanceError, CoordinatorError, or GitError).
func IsDomainError(err error) bool {
	if err == nil {
		return false
	}

	var sessionErr *SessionError
	var instanceErr *InstanceError
	var coordinatorErr *CoordinatorError
	var gitErr *GitError

	return As(err, &sessionErr) || As(err, &instanceErr) ||
		As(err, &coordinatorErr) || As(err, &gitErr)
}

// IsSemanticError returns true if the error is a semantic error
// (NotFoundError, AlreadyExistsError, ValidationError, or TimeoutError).
func IsSemanticError(err error) bool {
	if err == nil {
		return false
	}

	var notFound *NotFoundError
	var alreadyExists *AlreadyExistsError
	var validation *ValidationError
	var timeout *TimeoutError

	return As(err, &notFound) || As(err, &alreadyExists) ||
		As(err, &validation) || As(err, &timeout)
}

// -----------------------------------------------------------------------------
// Convenience Constructors
// -----------------------------------------------------------------------------

// Wrap wraps an error with additional context message.
// Unlike fmt.Errorf with %w, this preserves the ClaudioError interface.
//
// Example:
//
//	err := errors.Wrap(baseErr, "failed to process request")
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted context message.
//
// Example:
//
//	err := errors.Wrapf(baseErr, "failed to process session %s", sessionID)
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

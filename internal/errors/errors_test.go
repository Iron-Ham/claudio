package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Severity Tests
// -----------------------------------------------------------------------------

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityDebug, "debug"},
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityError, "error"},
		{SeverityCritical, "critical"},
		{Severity(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("Severity.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// SessionError Tests
// -----------------------------------------------------------------------------

func TestNewSessionError(t *testing.T) {
	cause := ErrSessionNotFound
	err := NewSessionError("failed to load session", cause)

	if err.message != "failed to load session" {
		t.Errorf("message = %q, want %q", err.message, "failed to load session")
	}
	if err.cause != cause {
		t.Errorf("cause = %v, want %v", err.cause, cause)
	}
	if err.Severity() != SeverityError {
		t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityError)
	}
	if err.IsRetryable() {
		t.Error("IsRetryable() = true, want false")
	}
	if !err.IsUserFacing() {
		t.Error("IsUserFacing() = false, want true")
	}
}

func TestSessionError_WithMethods(t *testing.T) {
	err := NewSessionError("test", nil).
		WithSessionID("sess-123").
		WithSeverity(SeverityCritical).
		WithRetryable(true)

	if err.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", err.SessionID, "sess-123")
	}
	if err.Severity() != SeverityCritical {
		t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityCritical)
	}
	if !err.IsRetryable() {
		t.Error("IsRetryable() = false, want true")
	}
}

func TestSessionError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *SessionError
		want string
	}{
		{
			name: "basic error",
			err:  NewSessionError("test error", nil),
			want: "session error: test error",
		},
		{
			name: "with cause",
			err:  NewSessionError("test error", ErrSessionNotFound),
			want: "session error: test error: session not found",
		},
		{
			name: "with session ID",
			err:  NewSessionError("test error", nil).WithSessionID("abc123"),
			want: "session error [session=abc123]: test error",
		},
		{
			name: "with session ID and cause",
			err:  NewSessionError("test error", ErrSessionLocked).WithSessionID("xyz"),
			want: "session error [session=xyz]: test error: session is locked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionError_Is(t *testing.T) {
	err := NewSessionError("test", ErrSessionNotFound).WithSessionID("abc")

	// Should match SessionError type
	if !Is(err, &SessionError{}) {
		t.Error("Is(SessionError{}) = false, want true")
	}

	// Should match wrapped sentinel error
	if !Is(err, ErrSessionNotFound) {
		t.Error("Is(ErrSessionNotFound) = false, want true")
	}

	// Should not match unrelated errors
	if Is(err, ErrInstanceNotFound) {
		t.Error("Is(ErrInstanceNotFound) = true, want false")
	}
}

func TestSessionError_Unwrap(t *testing.T) {
	cause := ErrSessionNotFound
	err := NewSessionError("test", cause)

	if unwrapped := Unwrap(err); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

// -----------------------------------------------------------------------------
// InstanceError Tests
// -----------------------------------------------------------------------------

func TestNewInstanceError(t *testing.T) {
	cause := ErrInstanceAlreadyRunning
	err := NewInstanceError("instance failed", cause)

	if err.message != "instance failed" {
		t.Errorf("message = %q, want %q", err.message, "instance failed")
	}
	if err.cause != cause {
		t.Errorf("cause = %v, want %v", err.cause, cause)
	}
}

func TestInstanceError_WithMethods(t *testing.T) {
	err := NewInstanceError("test", nil).
		WithInstanceID("inst-456").
		WithTmuxSession("claudio-abc").
		WithSeverity(SeverityWarning).
		WithRetryable(true)

	if err.InstanceID != "inst-456" {
		t.Errorf("InstanceID = %q, want %q", err.InstanceID, "inst-456")
	}
	if err.TmuxSession != "claudio-abc" {
		t.Errorf("TmuxSession = %q, want %q", err.TmuxSession, "claudio-abc")
	}
	if err.Severity() != SeverityWarning {
		t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
	}
}

func TestInstanceError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *InstanceError
		want string
	}{
		{
			name: "basic error",
			err:  NewInstanceError("test error", nil),
			want: "instance error: test error",
		},
		{
			name: "with instance ID",
			err:  NewInstanceError("test error", nil).WithInstanceID("inst-1"),
			want: "instance error [instance=inst-1]: test error",
		},
		{
			name: "with all fields",
			err:  NewInstanceError("crashed", ErrInstanceCommunication).WithInstanceID("inst-1").WithTmuxSession("tmux-sess"),
			want: "instance error [instance=inst-1, tmux=tmux-sess]: crashed: instance communication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstanceError_Is(t *testing.T) {
	err := NewInstanceError("test", ErrInstanceAlreadyRunning)

	if !Is(err, &InstanceError{}) {
		t.Error("Is(InstanceError{}) = false, want true")
	}
	if !Is(err, ErrInstanceAlreadyRunning) {
		t.Error("Is(ErrInstanceAlreadyRunning) = false, want true")
	}
	if Is(err, &SessionError{}) {
		t.Error("Is(SessionError{}) = true, want false")
	}
}

// -----------------------------------------------------------------------------
// CoordinatorError Tests
// -----------------------------------------------------------------------------

func TestNewCoordinatorError(t *testing.T) {
	cause := ErrTaskFailed
	err := NewCoordinatorError("task execution failed", cause)

	if err.message != "task execution failed" {
		t.Errorf("message = %q, want %q", err.message, "task execution failed")
	}
	if err.GroupIndex != -1 {
		t.Errorf("GroupIndex = %d, want -1", err.GroupIndex)
	}
}

func TestCoordinatorError_WithMethods(t *testing.T) {
	err := NewCoordinatorError("test", nil).
		WithTaskID("task-789").
		WithGroupIndex(2).
		WithPhase("execution").
		WithSeverity(SeverityCritical).
		WithRetryable(true)

	if err.TaskID != "task-789" {
		t.Errorf("TaskID = %q, want %q", err.TaskID, "task-789")
	}
	if err.GroupIndex != 2 {
		t.Errorf("GroupIndex = %d, want 2", err.GroupIndex)
	}
	if err.Phase != "execution" {
		t.Errorf("Phase = %q, want %q", err.Phase, "execution")
	}
}

func TestCoordinatorError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *CoordinatorError
		want string
	}{
		{
			name: "basic error",
			err:  NewCoordinatorError("test error", nil),
			want: "coordinator error: test error",
		},
		{
			name: "with task ID",
			err:  NewCoordinatorError("test error", nil).WithTaskID("task-1"),
			want: "coordinator error [task=task-1]: test error",
		},
		{
			name: "with all fields",
			err:  NewCoordinatorError("failed", ErrDependencyCycle).WithTaskID("task-1").WithGroupIndex(3).WithPhase("planning"),
			want: "coordinator error [task=task-1, group=3, phase=planning]: failed: dependency cycle detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoordinatorError_Is(t *testing.T) {
	err := NewCoordinatorError("test", ErrDependencyCycle)

	if !Is(err, &CoordinatorError{}) {
		t.Error("Is(CoordinatorError{}) = false, want true")
	}
	if !Is(err, ErrDependencyCycle) {
		t.Error("Is(ErrDependencyCycle) = false, want true")
	}
}

// -----------------------------------------------------------------------------
// GitError Tests
// -----------------------------------------------------------------------------

func TestNewGitError(t *testing.T) {
	cause := ErrMergeConflict
	err := NewGitError("merge failed", cause)

	if err.message != "merge failed" {
		t.Errorf("message = %q, want %q", err.message, "merge failed")
	}
}

func TestGitError_WithMethods(t *testing.T) {
	err := NewGitError("test", nil).
		WithBranch("feature-x").
		WithWorktree("/path/to/wt").
		WithRepository("/path/to/repo").
		WithGitOutput("fatal: error message").
		WithSeverity(SeverityWarning).
		WithRetryable(true)

	if err.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", err.Branch, "feature-x")
	}
	if err.Worktree != "/path/to/wt" {
		t.Errorf("Worktree = %q, want %q", err.Worktree, "/path/to/wt")
	}
	if err.Repository != "/path/to/repo" {
		t.Errorf("Repository = %q, want %q", err.Repository, "/path/to/repo")
	}
	if err.GitOutput != "fatal: error message" {
		t.Errorf("GitOutput = %q, want %q", err.GitOutput, "fatal: error message")
	}
}

func TestGitError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *GitError
		want string
	}{
		{
			name: "basic error",
			err:  NewGitError("test error", nil),
			want: "git error: test error",
		},
		{
			name: "with branch",
			err:  NewGitError("checkout failed", nil).WithBranch("main"),
			want: "git error [branch=main]: checkout failed",
		},
		{
			name: "with git output",
			err:  NewGitError("failed", ErrMergeConflict).WithBranch("dev").WithGitOutput("CONFLICT"),
			want: "git error [branch=dev]: failed: merge conflict\ngit output: CONFLICT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitError_Is(t *testing.T) {
	err := NewGitError("test", ErrWorktreeExists)

	if !Is(err, &GitError{}) {
		t.Error("Is(GitError{}) = false, want true")
	}
	if !Is(err, ErrWorktreeExists) {
		t.Error("Is(ErrWorktreeExists) = false, want true")
	}
}

// -----------------------------------------------------------------------------
// NotFoundError Tests
// -----------------------------------------------------------------------------

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("session", "abc123")

	if err.ResourceType != "session" {
		t.Errorf("ResourceType = %q, want %q", err.ResourceType, "session")
	}
	if err.ResourceID != "abc123" {
		t.Errorf("ResourceID = %q, want %q", err.ResourceID, "abc123")
	}
	if err.Severity() != SeverityWarning {
		t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
	}
}

func TestNotFoundError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *NotFoundError
		want string
	}{
		{
			name: "basic error",
			err:  NewNotFoundError("session", "abc"),
			want: "session 'abc' not found",
		},
		{
			name: "with cause",
			err:  NewNotFoundError("worktree", "/path").WithCause(fmt.Errorf("IO error")),
			want: "worktree '/path' not found: IO error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotFoundError_Is(t *testing.T) {
	err := NewNotFoundError("session", "abc")

	if !Is(err, &NotFoundError{}) {
		t.Error("Is(NotFoundError{}) = false, want true")
	}
	// NotFoundError does not wrap sentinel errors by default
	if Is(err, ErrSessionNotFound) {
		t.Error("Is(ErrSessionNotFound) = true, want false (not wrapped)")
	}
}

// -----------------------------------------------------------------------------
// AlreadyExistsError Tests
// -----------------------------------------------------------------------------

func TestNewAlreadyExistsError(t *testing.T) {
	err := NewAlreadyExistsError("branch", "feature-x")

	if err.ResourceType != "branch" {
		t.Errorf("ResourceType = %q, want %q", err.ResourceType, "branch")
	}
	if err.ResourceID != "feature-x" {
		t.Errorf("ResourceID = %q, want %q", err.ResourceID, "feature-x")
	}
}

func TestAlreadyExistsError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *AlreadyExistsError
		want string
	}{
		{
			name: "basic error",
			err:  NewAlreadyExistsError("branch", "main"),
			want: "branch 'main' already exists",
		},
		{
			name: "with cause",
			err:  NewAlreadyExistsError("file", "test.txt").WithCause(fmt.Errorf("disk error")),
			want: "file 'test.txt' already exists: disk error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAlreadyExistsError_Is(t *testing.T) {
	err := NewAlreadyExistsError("branch", "main")

	if !Is(err, &AlreadyExistsError{}) {
		t.Error("Is(AlreadyExistsError{}) = false, want true")
	}
}

// -----------------------------------------------------------------------------
// ValidationError Tests
// -----------------------------------------------------------------------------

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("session ID cannot be empty")

	if err.message != "session ID cannot be empty" {
		t.Errorf("message = %q, want %q", err.message, "session ID cannot be empty")
	}
	if err.Severity() != SeverityWarning {
		t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
	}
}

func TestValidationError_WithMethods(t *testing.T) {
	err := NewValidationError("invalid value").
		WithField("sessionID").
		WithValue("").
		WithCause(fmt.Errorf("must not be empty"))

	if err.Field != "sessionID" {
		t.Errorf("Field = %q, want %q", err.Field, "sessionID")
	}
	if err.Value != "" {
		t.Errorf("Value = %v, want empty string", err.Value)
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{
			name: "basic error",
			err:  NewValidationError("invalid input"),
			want: "validation error: invalid input",
		},
		{
			name: "with field",
			err:  NewValidationError("cannot be empty").WithField("name"),
			want: "validation error [field=name]: cannot be empty",
		},
		{
			name: "with field and value",
			err:  NewValidationError("must be positive").WithField("count").WithValue(-1),
			want: "validation error [field=count, value=-1]: must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationError_Is(t *testing.T) {
	err := NewValidationError("test")

	if !Is(err, &ValidationError{}) {
		t.Error("Is(ValidationError{}) = false, want true")
	}
	// ValidationError should match ErrInvalidInput
	if !Is(err, ErrInvalidInput) {
		t.Error("Is(ErrInvalidInput) = false, want true")
	}
}

// -----------------------------------------------------------------------------
// TimeoutError Tests
// -----------------------------------------------------------------------------

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("waiting for instance", 30*time.Second)

	if err.Operation != "waiting for instance" {
		t.Errorf("Operation = %q, want %q", err.Operation, "waiting for instance")
	}
	if err.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want %v", err.Duration, 30*time.Second)
	}
	// Timeouts are retryable by default
	if !err.IsRetryable() {
		t.Error("IsRetryable() = false, want true")
	}
}

func TestTimeoutError_WithMethods(t *testing.T) {
	err := NewTimeoutError("test", time.Second).
		WithCause(fmt.Errorf("context deadline exceeded")).
		WithRetryable(false)

	if err.IsRetryable() {
		t.Error("IsRetryable() = true, want false")
	}
}

func TestTimeoutError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *TimeoutError
		want string
	}{
		{
			name: "basic error",
			err:  NewTimeoutError("waiting for response", 5*time.Second),
			want: "timeout error: waiting for response (timeout: 5s)",
		},
		{
			name: "with cause",
			err:  NewTimeoutError("connecting", time.Minute).WithCause(fmt.Errorf("network unreachable")),
			want: "timeout error: connecting (timeout: 1m0s): network unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeoutError_Is(t *testing.T) {
	err := NewTimeoutError("test", time.Second)

	if !Is(err, &TimeoutError{}) {
		t.Error("Is(TimeoutError{}) = false, want true")
	}
	// TimeoutError should match ErrTimeout
	if !Is(err, ErrTimeout) {
		t.Error("Is(ErrTimeout) = false, want true")
	}
}

// -----------------------------------------------------------------------------
// Classification Helper Tests
// -----------------------------------------------------------------------------

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "timeout error",
			err:  NewTimeoutError("test", time.Second),
			want: true,
		},
		{
			name: "session error not retryable",
			err:  NewSessionError("test", nil),
			want: false,
		},
		{
			name: "session error set retryable",
			err:  NewSessionError("test", nil).WithRetryable(true),
			want: true,
		},
		{
			name: "wrapped timeout sentinel",
			err:  fmt.Errorf("operation failed: %w", ErrTimeout),
			want: true,
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUserFacing(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "session error",
			err:  NewSessionError("test", nil),
			want: true,
		},
		{
			name: "not found error",
			err:  NewNotFoundError("session", "abc"),
			want: true,
		},
		{
			name: "validation error",
			err:  NewValidationError("invalid input"),
			want: true,
		},
		{
			name: "timeout error",
			err:  NewTimeoutError("waiting", time.Second),
			want: true,
		},
		{
			name: "standard error",
			err:  errors.New("internal error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUserFacing(tt.err); got != tt.want {
				t.Errorf("IsUserFacing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSeverity(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Severity
	}{
		{
			name: "nil error",
			err:  nil,
			want: SeverityDebug,
		},
		{
			name: "session error default",
			err:  NewSessionError("test", nil),
			want: SeverityError,
		},
		{
			name: "session error critical",
			err:  NewSessionError("test", nil).WithSeverity(SeverityCritical),
			want: SeverityCritical,
		},
		{
			name: "not found error",
			err:  NewNotFoundError("session", "abc"),
			want: SeverityWarning,
		},
		{
			name: "standard error",
			err:  errors.New("standard"),
			want: SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSeverity(tt.err); got != tt.want {
				t.Errorf("GetSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDomainError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "session error",
			err:  NewSessionError("test", nil),
			want: true,
		},
		{
			name: "instance error",
			err:  NewInstanceError("test", nil),
			want: true,
		},
		{
			name: "coordinator error",
			err:  NewCoordinatorError("test", nil),
			want: true,
		},
		{
			name: "git error",
			err:  NewGitError("test", nil),
			want: true,
		},
		{
			name: "not found error (semantic)",
			err:  NewNotFoundError("session", "abc"),
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("test"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDomainError(tt.err); got != tt.want {
				t.Errorf("IsDomainError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSemanticError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "not found error",
			err:  NewNotFoundError("session", "abc"),
			want: true,
		},
		{
			name: "already exists error",
			err:  NewAlreadyExistsError("branch", "main"),
			want: true,
		},
		{
			name: "validation error",
			err:  NewValidationError("invalid"),
			want: true,
		},
		{
			name: "timeout error",
			err:  NewTimeoutError("waiting", time.Second),
			want: true,
		},
		{
			name: "session error (domain)",
			err:  NewSessionError("test", nil),
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("test"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSemanticError(tt.err); got != tt.want {
				t.Errorf("IsSemanticError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Wrap/Wrapf Tests
// -----------------------------------------------------------------------------

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
		want    string
	}{
		{
			name:    "nil error",
			err:     nil,
			message: "context",
			want:    "",
		},
		{
			name:    "wrap standard error",
			err:     errors.New("base error"),
			message: "failed to process",
			want:    "failed to process: base error",
		},
		{
			name:    "wrap session error",
			err:     NewSessionError("session failed", nil),
			message: "operation failed",
			want:    "operation failed: session error: session failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.message)
			if tt.err == nil {
				if got != nil {
					t.Errorf("Wrap(nil) = %v, want nil", got)
				}
				return
			}
			if got.Error() != tt.want {
				t.Errorf("Wrap().Error() = %q, want %q", got.Error(), tt.want)
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	baseErr := errors.New("base error")
	err := Wrapf(baseErr, "failed to process %s", "request")

	want := "failed to process request: base error"
	if err.Error() != want {
		t.Errorf("Wrapf().Error() = %q, want %q", err.Error(), want)
	}

	// Wrapf with nil should return nil
	if got := Wrapf(nil, "test"); got != nil {
		t.Errorf("Wrapf(nil) = %v, want nil", got)
	}
}

// -----------------------------------------------------------------------------
// Re-exported Functions Tests
// -----------------------------------------------------------------------------

func TestReexportedFunctions(t *testing.T) {
	// Test that re-exported functions work correctly
	baseErr := New("base error")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)

	// Test Is
	if !Is(wrappedErr, baseErr) {
		t.Error("Is() should return true for wrapped error")
	}

	// Test Unwrap
	if Unwrap(wrappedErr) == nil {
		t.Error("Unwrap() should return the base error")
	}

	// Test As
	var sessionErr *SessionError
	testErr := NewSessionError("test", nil)
	if !As(testErr, &sessionErr) {
		t.Error("As() should extract SessionError")
	}

	// Test Join
	err1 := New("error 1")
	err2 := New("error 2")
	joined := Join(err1, err2)
	if !Is(joined, err1) || !Is(joined, err2) {
		t.Error("Join() should combine errors")
	}
}

// -----------------------------------------------------------------------------
// Error Chain Tests
// -----------------------------------------------------------------------------

func TestErrorChain(t *testing.T) {
	// Create a chain of errors
	baseErr := ErrSessionNotFound
	sessionErr := NewSessionError("failed to load", baseErr).WithSessionID("abc123")
	wrappedErr := Wrap(sessionErr, "operation failed")

	// Should be able to find all errors in the chain
	if !Is(wrappedErr, ErrSessionNotFound) {
		t.Error("Should find ErrSessionNotFound in chain")
	}

	var extracted *SessionError
	if !As(wrappedErr, &extracted) {
		t.Error("Should extract SessionError from chain")
	}
	if extracted.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", extracted.SessionID, "abc123")
	}
}

// -----------------------------------------------------------------------------
// Sentinel Error Tests
// -----------------------------------------------------------------------------

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors are distinct
	sentinels := []error{
		ErrSessionNotFound,
		ErrSessionLocked,
		ErrSessionCorrupted,
		ErrSessionInactive,
		ErrInstanceNotFound,
		ErrInstanceAlreadyRunning,
		ErrInstanceNotRunning,
		ErrInstanceStartFailed,
		ErrInstanceCommunication,
		ErrPlanNotFound,
		ErrPlanInvalid,
		ErrTaskNotFound,
		ErrTaskFailed,
		ErrDependencyCycle,
		ErrCoordinatorCanceled,
		ErrNotGitRepository,
		ErrWorktreeNotFound,
		ErrWorktreeExists,
		ErrBranchNotFound,
		ErrBranchExists,
		ErrMergeConflict,
		ErrDirtyWorktree,
		ErrTimeout,
		ErrCanceled,
		ErrInvalidInput,
		ErrOperationFailed,
	}

	// Check that each sentinel is distinct from all others
	for i, err1 := range sentinels {
		for j, err2 := range sentinels {
			if i != j && Is(err1, err2) {
				t.Errorf("Sentinel error %v should not match %v", err1, err2)
			}
		}
	}
}

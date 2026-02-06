package bridge

// InstanceFactory creates and starts Claude Code instances.
type InstanceFactory interface {
	// CreateInstance creates a new instance (worktree + branch) for the given task prompt.
	// Returns a handle that can be started and queried.
	CreateInstance(taskPrompt string) (Instance, error)

	// StartInstance launches the Claude Code backend for an instance.
	StartInstance(inst Instance) error
}

// Instance represents a running (or created) Claude Code backend.
type Instance interface {
	// ID returns the unique instance identifier.
	ID() string

	// WorktreePath returns the path to the instance's git worktree.
	WorktreePath() string

	// Branch returns the git branch name for this instance's work.
	Branch() string
}

// CompletionChecker detects whether an instance has finished its task.
type CompletionChecker interface {
	// CheckCompletion returns true if the instance has written its sentinel file.
	CheckCompletion(worktreePath string) (done bool, err error)

	// VerifyWork validates task output (commit presence and verification result).
	VerifyWork(taskID, instanceID, worktreePath, baseBranch string) (success bool, commitCount int, err error)
}

// SessionRecorder keeps session state in sync with bridge operations.
type SessionRecorder interface {
	// AssignTask records that a task has been assigned to an instance.
	AssignTask(taskID, instanceID string)

	// RecordCompletion records successful task completion.
	RecordCompletion(taskID string, commitCount int)

	// RecordFailure records a task failure with the given reason.
	RecordFailure(taskID, reason string)
}

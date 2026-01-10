package orchestrator

// taskCompletion represents a task completion notification
type taskCompletion struct {
	taskID      string
	instanceID  string
	success     bool
	error       string
	needsRetry  bool // Indicates task should be retried (no commits produced)
	commitCount int  // Number of commits produced by this task
}

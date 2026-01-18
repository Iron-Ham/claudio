package orchestrator

import "fmt"

// coordinatorRetryTracker adapts the Coordinator's RetryManager to the verify.RetryTracker interface.
type coordinatorRetryTracker struct {
	c *Coordinator
}

func (rt *coordinatorRetryTracker) GetRetryCount(taskID string) int {
	state := rt.c.retryManager.GetState(taskID)
	if state == nil {
		return 0
	}
	return state.RetryCount
}

func (rt *coordinatorRetryTracker) IncrementRetry(taskID string) int {
	session := rt.c.Session()
	config := session.Config
	maxRetries := config.MaxTaskRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	state := rt.c.retryManager.GetOrCreateState(taskID, maxRetries)
	rt.c.retryManager.RecordAttempt(taskID, false) // false = failure
	rt.c.retryManager.SetLastError(taskID, "task produced no commits")
	return state.RetryCount + 1
}

func (rt *coordinatorRetryTracker) RecordCommitCount(taskID string, count int) {
	session := rt.c.Session()
	config := session.Config
	maxRetries := config.MaxTaskRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	rt.c.retryManager.GetOrCreateState(taskID, maxRetries)
	rt.c.retryManager.RecordCommitCount(taskID, count)
}

func (rt *coordinatorRetryTracker) GetMaxRetries(taskID string) int {
	state := rt.c.retryManager.GetState(taskID)
	if state == nil {
		return 0
	}
	return state.MaxRetries
}

// coordinatorEventEmitter adapts the Coordinator's event emission to the verify.EventEmitter interface.
type coordinatorEventEmitter struct {
	c *Coordinator
}

func (e *coordinatorEventEmitter) EmitWarning(taskID, message string) {
	e.c.manager.emitEvent(CoordinatorEvent{
		Type:    EventConflict,
		TaskID:  taskID,
		Message: message,
	})
}

func (e *coordinatorEventEmitter) EmitRetry(taskID string, attempt, maxRetries int, reason string) {
	e.c.manager.emitEvent(CoordinatorEvent{
		Type:    EventTaskStarted, // Reuse for retry notification
		TaskID:  taskID,
		Message: fmt.Sprintf("Task %s produced no commits, scheduling retry %d/%d", taskID, attempt, maxRetries),
	})
}

func (e *coordinatorEventEmitter) EmitFailure(taskID, reason string) {
	e.c.manager.emitEvent(CoordinatorEvent{
		Type:    EventTaskFailed,
		TaskID:  taskID,
		Message: reason,
	})
}

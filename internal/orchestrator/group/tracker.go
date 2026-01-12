// Package group provides execution group tracking for ultra-plan orchestration.
package group

import "slices"

// SessionData provides the data needed by GroupTracker to track execution groups.
// This interface allows the tracker to be decoupled from the full UltraPlanSession.
type SessionData interface {
	// GetPlan returns the execution order and tasks for the plan.
	// Returns nil if no plan is available.
	GetPlan() PlanData

	// GetCompletedTasks returns the list of completed task IDs.
	GetCompletedTasks() []string

	// GetFailedTasks returns the list of failed task IDs.
	GetFailedTasks() []string

	// GetTaskCommitCounts returns the map of task ID to commit count.
	GetTaskCommitCounts() map[string]int

	// GetCurrentGroup returns the current group index.
	GetCurrentGroup() int
}

// PlanData provides access to plan information needed for group tracking.
type PlanData interface {
	// GetExecutionOrder returns the groups of task IDs in execution order.
	GetExecutionOrder() [][]string

	// GetTask returns the task with the given ID, or nil if not found.
	GetTask(taskID string) *Task
}

// Task represents a planned task with minimal information needed for group tracking.
type Task struct {
	ID          string
	Title       string
	Description string
	Files       []string
	DependsOn   []string
}

// Tracker manages execution group state for ultra-plan orchestration.
// It tracks which group is currently executing, which tasks belong to each group,
// and whether groups have completed with partial or full success/failure.
type Tracker struct {
	session SessionData
}

// NewTracker creates a new group tracker.
// The session must be non-nil.
func NewTracker(session SessionData) *Tracker {
	if session == nil {
		panic("group.NewTracker: session must not be nil")
	}
	return &Tracker{
		session: session,
	}
}

// GetTaskGroupIndex returns the execution group index for the given task ID.
// Returns -1 if the task is not found in any group.
func (t *Tracker) GetTaskGroupIndex(taskID string) int {
	plan := t.session.GetPlan()
	if plan == nil {
		return -1
	}

	for groupIdx, group := range plan.GetExecutionOrder() {
		if slices.Contains(group, taskID) {
			return groupIdx
		}
	}
	return -1
}

// IsGroupComplete returns true if all tasks in the given group have completed
// (either successfully or with failure).
func (t *Tracker) IsGroupComplete(groupIndex int) bool {
	plan := t.session.GetPlan()
	if plan == nil {
		return false
	}

	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return false
	}

	// Build set of completed/failed tasks
	doneSet := make(map[string]bool)
	for _, taskID := range t.session.GetCompletedTasks() {
		doneSet[taskID] = true
	}
	for _, taskID := range t.session.GetFailedTasks() {
		doneSet[taskID] = true
	}

	// Check if all tasks in the group are done
	for _, taskID := range executionOrder[groupIndex] {
		if !doneSet[taskID] {
			return false
		}
	}
	return true
}

// GetGroupTasks returns the tasks in the given group.
// Returns nil if the group index is out of bounds or no plan exists.
func (t *Tracker) GetGroupTasks(groupIndex int) []*Task {
	plan := t.session.GetPlan()
	if plan == nil {
		return nil
	}

	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return nil
	}

	taskIDs := executionOrder[groupIndex]
	tasks := make([]*Task, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		task := plan.GetTask(taskID)
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// AdvanceGroup advances from the given group to the next group.
// Returns the next group index and whether we're done (no more groups).
// This method does NOT modify session state - it only computes what the next state would be.
func (t *Tracker) AdvanceGroup(groupIndex int) (nextGroup int, done bool) {
	plan := t.session.GetPlan()
	if plan == nil {
		return groupIndex, true
	}

	executionOrder := plan.GetExecutionOrder()
	nextGroup = groupIndex + 1
	done = nextGroup >= len(executionOrder)
	return nextGroup, done
}

// HasPartialFailure returns true if the group has at least one successful task
// AND at least one failed task. This indicates a partial group failure that
// requires user decision on how to proceed.
//
// A task is considered successful if it:
// - Is in the completed tasks list AND
// - Has at least one verified commit (commit count > 0)
//
// A task is considered failed if it:
// - Is in the failed tasks list OR
// - Is in the completed tasks list but has no verified commits (commit count == 0)
func (t *Tracker) HasPartialFailure(groupIndex int) bool {
	plan := t.session.GetPlan()
	if plan == nil {
		return false
	}

	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return false
	}

	taskIDs := executionOrder[groupIndex]
	completedTasks := t.session.GetCompletedTasks()
	failedTasks := t.session.GetFailedTasks()
	commitCounts := t.session.GetTaskCommitCounts()

	successCount := 0
	failureCount := 0

	for _, taskID := range taskIDs {
		// Check if in completed tasks
		if slices.Contains(completedTasks, taskID) {
			// Verify it has commits
			if count, ok := commitCounts[taskID]; ok && count > 0 {
				successCount++
			} else {
				failureCount++
			}
			continue
		}

		// Check if in failed tasks
		if slices.Contains(failedTasks, taskID) {
			failureCount++
		}
	}

	// Partial failure = at least one success AND at least one failure
	return successCount > 0 && failureCount > 0
}

// GetGroupStatus returns detailed status information about a group.
type GroupStatus struct {
	GroupIndex     int
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	PendingTasks   int
	SuccessfulIDs  []string // Tasks completed with verified commits
	FailedIDs      []string // Tasks that failed or completed without commits
	PendingIDs     []string // Tasks not yet done
}

// GetGroupStatus returns detailed status information about the given group.
func (t *Tracker) GetGroupStatus(groupIndex int) *GroupStatus {
	plan := t.session.GetPlan()
	if plan == nil {
		return nil
	}

	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return nil
	}

	taskIDs := executionOrder[groupIndex]
	completedTasks := t.session.GetCompletedTasks()
	failedTasks := t.session.GetFailedTasks()
	commitCounts := t.session.GetTaskCommitCounts()

	status := &GroupStatus{
		GroupIndex:    groupIndex,
		TotalTasks:    len(taskIDs),
		SuccessfulIDs: make([]string, 0),
		FailedIDs:     make([]string, 0),
		PendingIDs:    make([]string, 0),
	}

	completedSet := make(map[string]bool)
	for _, ct := range completedTasks {
		completedSet[ct] = true
	}

	failedSet := make(map[string]bool)
	for _, ft := range failedTasks {
		failedSet[ft] = true
	}

	for _, taskID := range taskIDs {
		if completedSet[taskID] {
			// Check if it has verified commits
			if count, ok := commitCounts[taskID]; ok && count > 0 {
				status.SuccessfulIDs = append(status.SuccessfulIDs, taskID)
				status.CompletedTasks++
			} else {
				status.FailedIDs = append(status.FailedIDs, taskID)
				status.FailedTasks++
			}
		} else if failedSet[taskID] {
			status.FailedIDs = append(status.FailedIDs, taskID)
			status.FailedTasks++
		} else {
			status.PendingIDs = append(status.PendingIDs, taskID)
			status.PendingTasks++
		}
	}

	return status
}

// TotalGroups returns the total number of execution groups in the plan.
func (t *Tracker) TotalGroups() int {
	plan := t.session.GetPlan()
	if plan == nil {
		return 0
	}
	return len(plan.GetExecutionOrder())
}

// HasMoreGroups returns true if there are more groups after the given index.
func (t *Tracker) HasMoreGroups(groupIndex int) bool {
	return groupIndex+1 < t.TotalGroups()
}

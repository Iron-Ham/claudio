package group

// SessionAdapter adapts an UltraPlanSession-like object to the SessionData interface.
// This allows the group tracker to work with the orchestrator's session types
// without introducing a circular dependency.
type SessionAdapter struct {
	getPlan             func() PlanData
	getCompletedTasks   func() []string
	getFailedTasks      func() []string
	getTaskCommitCounts func() map[string]int
	getCurrentGroup     func() int
}

// NewSessionAdapter creates a new session adapter with the provided accessor functions.
func NewSessionAdapter(
	getPlan func() PlanData,
	getCompletedTasks func() []string,
	getFailedTasks func() []string,
	getTaskCommitCounts func() map[string]int,
	getCurrentGroup func() int,
) *SessionAdapter {
	return &SessionAdapter{
		getPlan:             getPlan,
		getCompletedTasks:   getCompletedTasks,
		getFailedTasks:      getFailedTasks,
		getTaskCommitCounts: getTaskCommitCounts,
		getCurrentGroup:     getCurrentGroup,
	}
}

func (a *SessionAdapter) GetPlan() PlanData {
	if a.getPlan == nil {
		return nil
	}
	return a.getPlan()
}

func (a *SessionAdapter) GetCompletedTasks() []string {
	if a.getCompletedTasks == nil {
		return nil
	}
	return a.getCompletedTasks()
}

func (a *SessionAdapter) GetFailedTasks() []string {
	if a.getFailedTasks == nil {
		return nil
	}
	return a.getFailedTasks()
}

func (a *SessionAdapter) GetTaskCommitCounts() map[string]int {
	if a.getTaskCommitCounts == nil {
		return nil
	}
	return a.getTaskCommitCounts()
}

func (a *SessionAdapter) GetCurrentGroup() int {
	if a.getCurrentGroup == nil {
		return 0
	}
	return a.getCurrentGroup()
}

// PlanAdapter adapts a PlanSpec-like object to the PlanData interface.
type PlanAdapter struct {
	getExecutionOrder func() [][]string
	getTask           func(taskID string) *Task
}

// NewPlanAdapter creates a new plan adapter with the provided accessor functions.
func NewPlanAdapter(
	getExecutionOrder func() [][]string,
	getTask func(taskID string) *Task,
) *PlanAdapter {
	return &PlanAdapter{
		getExecutionOrder: getExecutionOrder,
		getTask:           getTask,
	}
}

func (a *PlanAdapter) GetExecutionOrder() [][]string {
	if a.getExecutionOrder == nil {
		return nil
	}
	return a.getExecutionOrder()
}

func (a *PlanAdapter) GetTask(taskID string) *Task {
	if a.getTask == nil {
		return nil
	}
	return a.getTask(taskID)
}

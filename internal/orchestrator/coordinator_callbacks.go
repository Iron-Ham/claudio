package orchestrator

// CoordinatorCallbacks holds callbacks for coordinator events
type CoordinatorCallbacks struct {
	// OnPhaseChange is called when the ultra-plan phase changes
	OnPhaseChange func(phase UltraPlanPhase)

	// OnTaskStart is called when a task begins execution
	OnTaskStart func(taskID, instanceID string)

	// OnTaskComplete is called when a task completes successfully
	OnTaskComplete func(taskID string)

	// OnTaskFailed is called when a task fails
	OnTaskFailed func(taskID, reason string)

	// OnGroupComplete is called when an execution group completes
	OnGroupComplete func(groupIndex int)

	// OnPlanReady is called when the plan is ready (after planning phase)
	OnPlanReady func(plan *PlanSpec)

	// OnProgress is called periodically with progress updates
	OnProgress func(completed, total int, phase UltraPlanPhase)

	// OnComplete is called when the entire ultra-plan completes
	OnComplete func(success bool, summary string)
}

// notifyPhaseChange notifies callbacks of phase change
func (c *Coordinator) notifyPhaseChange(phase UltraPlanPhase) {
	// Get the previous phase for logging
	session := c.Session()
	fromPhase := PhasePlanning
	if session != nil {
		fromPhase = session.Phase
	}

	c.manager.SetPhase(phase)

	// Log the phase transition
	c.logger.Info("phase changed",
		"from_phase", string(fromPhase),
		"to_phase", string(phase),
		"session_id", session.ID,
	)

	// Persist the phase change
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(phase)
	}
}

// notifyTaskStart notifies callbacks of task start
func (c *Coordinator) notifyTaskStart(taskID, instanceID string) {
	c.manager.AssignTaskToInstance(taskID, instanceID)

	// Get task title for logging
	session := c.Session()
	taskTitle := ""
	if session != nil {
		if task := session.GetTask(taskID); task != nil {
			taskTitle = task.Title
		}
	}

	// Log task started
	c.logger.Info("task started",
		"task_id", taskID,
		"instance_id", instanceID,
		"task_title", taskTitle,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskStart != nil {
		cb.OnTaskStart(taskID, instanceID)
	}
}

// notifyTaskComplete notifies callbacks of task completion
func (c *Coordinator) notifyTaskComplete(taskID string) {
	c.manager.MarkTaskComplete(taskID)

	// Log task completed
	// Note: duration tracking requires instance start time, which could be added in the future
	c.logger.Info("task completed",
		"task_id", taskID,
	)

	// Persist the task completion
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskComplete != nil {
		cb.OnTaskComplete(taskID)
	}
}

// notifyTaskFailed notifies callbacks of task failure
func (c *Coordinator) notifyTaskFailed(taskID, reason string) {
	c.manager.MarkTaskFailed(taskID, reason)

	// Log task failed
	c.logger.Info("task failed",
		"task_id", taskID,
		"reason", reason,
	)

	// Persist the task failure
	_ = c.orch.SaveSession()

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnTaskFailed != nil {
		cb.OnTaskFailed(taskID, reason)
	}
}

// notifyPlanReady notifies callbacks that planning is complete
func (c *Coordinator) notifyPlanReady(plan *PlanSpec) {
	// Log plan ready
	taskCount := 0
	groupCount := 0
	if plan != nil {
		taskCount = len(plan.Tasks)
		groupCount = len(plan.ExecutionOrder)
	}
	c.logger.Info("plan ready",
		"task_count", taskCount,
		"group_count", groupCount,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPlanReady != nil {
		cb.OnPlanReady(plan)
	}
}

// notifyProgress notifies callbacks of progress
func (c *Coordinator) notifyProgress() {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return
	}

	completed := len(session.CompletedTasks)
	total := len(session.Plan.Tasks)

	// Log progress update at DEBUG level
	c.logger.Debug("progress update",
		"completed", completed,
		"total", total,
		"phase", string(session.Phase),
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnProgress != nil {
		cb.OnProgress(completed, total, session.Phase)
	}
}

// notifyComplete notifies callbacks of completion
func (c *Coordinator) notifyComplete(success bool, summary string) {
	// Log coordinator complete
	c.logger.Info("coordinator complete",
		"success", success,
		"summary", summary,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(success, summary)
	}
}

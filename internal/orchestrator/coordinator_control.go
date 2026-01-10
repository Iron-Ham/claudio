package orchestrator

import "fmt"

// Cancel cancels the ultra-plan execution
func (c *Coordinator) Cancel() {
	c.cancelFunc()

	// Stop all running task instances
	c.mu.RLock()
	runningTasks := make(map[string]string, len(c.runningTasks))
	for k, v := range c.runningTasks {
		runningTasks[k] = v
	}
	c.mu.RUnlock()

	for _, instanceID := range runningTasks {
		inst := c.orch.GetInstance(instanceID)
		if inst != nil {
			_ = c.orch.StopInstance(inst)
		}
	}

	c.manager.Stop()
	c.wg.Wait()

	c.mu.Lock()
	session := c.Session()
	session.Phase = PhaseFailed
	session.Error = "cancelled by user"
	c.mu.Unlock()

	// Persist the cancellation state
	_ = c.orch.SaveSession()
}

// Wait waits for the ultra-plan to complete
func (c *Coordinator) Wait() {
	c.wg.Wait()
}

// GetProgress returns the current progress
func (c *Coordinator) GetProgress() (completed, total int, phase UltraPlanPhase) {
	session := c.Session()
	if session == nil {
		return 0, 0, PhasePlanning
	}

	if session.Plan == nil {
		return 0, 0, session.Phase
	}

	return len(session.CompletedTasks), len(session.Plan.Tasks), session.Phase
}

// GetRunningTasks returns the currently running tasks and their instance IDs
func (c *Coordinator) GetRunningTasks() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.runningTasks))
	for k, v := range c.runningTasks {
		result[k] = v
	}
	return result
}

// hasPartialGroupFailure checks if a group has a mix of successful and failed tasks
func (c *Coordinator) hasPartialGroupFailure(groupIndex int) bool {
	session := c.Session()
	if session == nil || session.Plan == nil {
		return false
	}

	if groupIndex >= len(session.Plan.ExecutionOrder) {
		return false
	}

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	successCount := 0
	failureCount := 0

	for _, taskID := range taskIDs {
		// Check if in completed with verified commits
		isCompleted := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				isCompleted = true
				break
			}
		}

		if isCompleted {
			// Verify it has commits
			if count, ok := session.TaskCommitCounts[taskID]; ok && count > 0 {
				successCount++
			} else {
				failureCount++
			}
			continue
		}

		// Check if in failed
		for _, ft := range session.FailedTasks {
			if ft == taskID {
				failureCount++
				break
			}
		}
	}

	// Partial failure = at least one success AND at least one failure
	return successCount > 0 && failureCount > 0
}

// handlePartialGroupFailure pauses execution and waits for user decision
func (c *Coordinator) handlePartialGroupFailure(groupIndex int) {
	session := c.Session()

	taskIDs := session.Plan.ExecutionOrder[groupIndex]
	var succeeded, failed []string

	for _, taskID := range taskIDs {
		isCompleted := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				isCompleted = true
				break
			}
		}

		if isCompleted {
			if count, ok := session.TaskCommitCounts[taskID]; ok && count > 0 {
				succeeded = append(succeeded, taskID)
			} else {
				failed = append(failed, taskID)
			}
		} else {
			// Check if failed
			for _, ft := range session.FailedTasks {
				if ft == taskID {
					failed = append(failed, taskID)
					break
				}
			}
		}
	}

	c.mu.Lock()
	session.GroupDecision = &GroupDecisionState{
		GroupIndex:       groupIndex,
		SucceededTasks:   succeeded,
		FailedTasks:      failed,
		AwaitingDecision: true,
	}
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Group %d has partial success (%d/%d tasks succeeded). Awaiting user decision.", groupIndex+1, len(succeeded), len(taskIDs)),
	})

	_ = c.orch.SaveSession()
}

// ResumeWithPartialWork continues execution with only the successful tasks
func (c *Coordinator) ResumeWithPartialWork() error {
	session := c.Session()
	if session.GroupDecision == nil || !session.GroupDecision.AwaitingDecision {
		return fmt.Errorf("no pending group decision")
	}

	groupIdx := session.GroupDecision.GroupIndex

	c.mu.Lock()
	session.GroupDecision.AwaitingDecision = false
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Continuing group %d with partial work (%d tasks)", groupIdx+1, len(session.GroupDecision.SucceededTasks)),
	})

	// Consolidate only the successful tasks
	if err := c.consolidateGroupWithVerification(groupIdx); err != nil {
		return fmt.Errorf("failed to consolidate partial group: %w", err)
	}

	// Advance to the next group AFTER consolidation succeeds
	// This is critical - without this, checkAndAdvanceGroup() would detect
	// the partial failure again and re-prompt the user
	c.mu.Lock()
	session.CurrentGroup++
	session.GroupDecision = nil
	c.mu.Unlock()

	// Continue execution
	_ = c.orch.SaveSession()
	return nil
}

// RetryFailedTasks retries the failed tasks in the current group
func (c *Coordinator) RetryFailedTasks() error {
	session := c.Session()
	if session.GroupDecision == nil || !session.GroupDecision.AwaitingDecision {
		return fmt.Errorf("no pending group decision")
	}

	failedTasks := session.GroupDecision.FailedTasks
	groupIdx := session.GroupDecision.GroupIndex

	// Reset retry states and remove from failed list
	c.mu.Lock()
	for _, taskID := range failedTasks {
		// Reset retry counter
		if state, ok := session.TaskRetries[taskID]; ok {
			state.RetryCount = 0
		}
		// Remove from failed list
		newFailed := make([]string, 0)
		for _, ft := range session.FailedTasks {
			if ft != taskID {
				newFailed = append(newFailed, ft)
			}
		}
		session.FailedTasks = newFailed
		// Remove from completed list (in case it was there with 0 commits)
		newCompleted := make([]string, 0)
		for _, ct := range session.CompletedTasks {
			if ct != taskID {
				newCompleted = append(newCompleted, ct)
			}
		}
		session.CompletedTasks = newCompleted
		// Remove from TaskToInstance so they become ready again
		delete(session.TaskToInstance, taskID)
	}

	// Ensure we stay at the current group (should already be at groupIdx)
	// and clear the decision state so tasks can be retried
	session.CurrentGroup = groupIdx
	session.GroupDecision = nil
	c.mu.Unlock()

	c.manager.emitEvent(CoordinatorEvent{
		Type:    EventGroupComplete,
		Message: fmt.Sprintf("Retrying %d failed tasks in group %d", len(failedTasks), groupIdx+1),
	})

	_ = c.orch.SaveSession()
	return nil
}

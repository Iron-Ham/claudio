package taskqueue

// isClaimable returns true if the task can be claimed: it must be pending
// and all of its dependencies must be in the completed state.
func (q *TaskQueue) isClaimable(task *QueuedTask) bool {
	if task.Status != TaskPending {
		return false
	}
	for _, depID := range task.DependsOn {
		dep, ok := q.tasks[depID]
		if !ok || dep.Status != TaskCompleted {
			return false
		}
	}
	return true
}

// unblockedBy returns the IDs of tasks that become claimable after the
// given task completes. A task is newly claimable if all of its dependencies
// are now completed and it is still in the pending state.
func (q *TaskQueue) unblockedBy(taskID string) []string {
	var unblocked []string
	for _, id := range q.order {
		task := q.tasks[id]
		if task.Status != TaskPending {
			continue
		}
		dependsOnCompleted := false
		allDepsCompleted := true
		for _, depID := range task.DependsOn {
			if depID == taskID {
				dependsOnCompleted = true
			}
			dep, ok := q.tasks[depID]
			if !ok || dep.Status != TaskCompleted {
				allDepsCompleted = false
			}
		}
		if dependsOnCompleted && allDepsCompleted {
			unblocked = append(unblocked, id)
		}
	}
	return unblocked
}

// buildPriorityOrder computes the task ordering used for claim selection.
// Tasks are ordered by execution group (topological level), then by priority
// within each group. This preserves the natural dependency ordering while
// respecting priority for tasks at the same level.
func buildPriorityOrder(tasks map[string]*QueuedTask) []string {
	if len(tasks) == 0 {
		return nil
	}

	// Compute in-degree for topological sort
	inDegree := make(map[string]int, len(tasks))
	dependents := make(map[string][]string, len(tasks))
	for id, task := range tasks {
		inDegree[id] = 0
		_ = task // initialize all
	}
	for id, task := range tasks {
		for _, depID := range task.DependsOn {
			if _, ok := tasks[depID]; ok {
				inDegree[id]++
				dependents[depID] = append(dependents[depID], id)
			}
		}
	}

	// BFS-based topological sort, collecting tasks level by level
	var order []string
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		// Sort current level by priority (lower = higher priority)
		sortByPriority(queue, tasks)
		order = append(order, queue...)

		var next []string
		for _, id := range queue {
			for _, depID := range dependents[id] {
				inDegree[depID]--
				if inDegree[depID] == 0 {
					next = append(next, depID)
				}
			}
		}
		queue = next
	}

	return order
}

// sortByPriority sorts task IDs by their priority value (lower first),
// using insertion sort since the slices are typically small.
func sortByPriority(ids []string, tasks map[string]*QueuedTask) {
	for i := 1; i < len(ids); i++ {
		key := ids[i]
		j := i - 1
		for j >= 0 && tasks[ids[j]].Priority > tasks[key].Priority {
			ids[j+1] = ids[j]
			j--
		}
		ids[j+1] = key
	}
}

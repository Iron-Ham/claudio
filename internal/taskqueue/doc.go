// Package taskqueue provides a dynamic task queue with dependency-aware
// claiming and work-stealing for Ultra-Plan execution.
//
// Instead of static execution order batches where all tasks in group N must
// complete before group N+1 starts, taskqueue allows instances to claim the
// next available task as soon as its dependencies are satisfied. This keeps
// instances busy and reduces overall execution time.
//
// The core type is [TaskQueue], which holds all tasks from a [ultraplan.PlanSpec]
// and provides thread-safe operations for claiming, completing, and failing tasks.
// Dependencies are tracked internally so that completing a task automatically
// unblocks downstream tasks for claiming.
//
// Queue state can be persisted to disk and restored, enabling crash recovery
// during long-running plan executions.
//
// Usage:
//
//	queue := taskqueue.NewFromPlan(planSpec)
//
//	// Instance claims next available task
//	task, err := queue.ClaimNext("instance-1")
//	if task != nil {
//	    queue.MarkRunning(task.ID)
//	    // ... execute task ...
//	    unblocked, err := queue.Complete(task.ID)
//	}
package taskqueue

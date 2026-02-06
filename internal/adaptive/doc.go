// Package adaptive provides event-driven dynamic task coordination for Claudio.
//
// The [Lead] monitors task queue events and makes dynamic decisions about
// workload distribution and scaling. It subscribes to queue events via the
// event bus and tracks which instances are running which tasks.
//
// # Architecture
//
// The Lead sits between the task queue and the orchestrator. It does not own
// the queue or directly manage instances — instead it observes events and
// publishes recommendations:
//
//   - On queue.task_claimed: tracks instance workload
//   - On queue.task_released: triggers rebalance check
//   - On queue.depth_changed: evaluates scaling needs
//   - On task.completed: updates progress tracking
//
// # Scaling Recommendations
//
// The Lead provides scaling recommendations based on queue state:
//   - [ScaleUp]: More pending tasks than running instances
//   - [ScaleDown]: No pending tasks and idle instances
//   - [ScaleNone]: System is balanced
//
// # Task Reassignment
//
// The Lead can reassign tasks between instances via [Lead.Reassign].
// This releases the task from one instance and claims it for another,
// publishing a [event.TaskReassignedEvent].
//
// # Basic Usage
//
//	lead := adaptive.NewLead(queue, bus,
//	    adaptive.WithStaleClaimTimeout(30*time.Second),
//	    adaptive.WithRebalanceInterval(10*time.Second),
//	)
//	lead.Start(ctx)
//	defer lead.Stop()
//
//	dist := lead.GetWorkloadDistribution()
//	rec := lead.GetScalingRecommendation()
//
// # Thread Safety
//
// All [Lead] methods are safe for concurrent use via an internal sync.RWMutex.
package adaptive

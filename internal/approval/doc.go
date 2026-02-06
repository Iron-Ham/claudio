// Package approval provides per-task plan approval gates for Ultra-Plan execution.
//
// When tasks in a plan require human approval before execution, the approval
// gate intercepts the claimed-to-running transition and holds the task in an
// "awaiting_approval" state until explicitly approved or rejected.
//
// The core type is [Gate], which wraps a [taskqueue.EventQueue] using the same
// decorator pattern as EventQueue wraps TaskQueue. The gate is transparent for
// tasks that do not require approval — they pass through to the underlying
// EventQueue unchanged.
//
// # Usage
//
//	gate := approval.NewGate(eventQueue, bus, lookupFunc)
//
//	// MarkRunning is intercepted for tasks requiring approval
//	err := gate.MarkRunning(taskID)
//	// If task requires approval, it enters "awaiting_approval" state
//
//	// Human approves
//	err = gate.Approve(taskID)
//
//	// Or rejects
//	err = gate.Reject(taskID, "plan looks risky")
//
// # Thread Safety
//
// All methods on [Gate] are safe for concurrent use via an internal mutex.
package approval

// Package lifecycle provides instance lifecycle management for the orchestrator.
//
// This package encapsulates the creation, starting, stopping, and monitoring
// of AI backend instances within an orchestration session. It provides a
// clean abstraction over instance state management with callback support
// for status change notifications.
//
// # Main Types
//
//   - [Manager]: Manages instance lifecycle with callback support
//   - [Instance]: Represents a managed backend instance with full state
//   - [InstanceStatus]: Enum for instance states (pending, working, completed, etc.)
//   - [Config]: Configuration for timeout, dimensions, and naming
//   - [Callbacks]: Event handlers for status changes, PR completion, timeouts, etc.
//
// # Instance Status
//
// Instances progress through these states:
//
//   - [StatusPending]: Created but not started
//   - [StatusWorking]: Actively executing task
//   - [StatusWaitingInput]: Waiting for user input
//   - [StatusPaused]: Execution paused
//   - [StatusCompleted]: Task finished successfully
//   - [StatusError]: Encountered an error
//   - [StatusCreatingPR]: Creating a pull request
//   - [StatusStuck]: No activity detected (potential timeout)
//   - [StatusTimeout]: Exceeded time limit
//
// # Thread Safety
//
// [Manager] is safe for concurrent use. All methods use appropriate
// synchronization. Callbacks are invoked synchronously and should
// complete quickly to avoid blocking other operations.
//
// # Basic Usage
//
//	callbacks := lifecycle.Callbacks{
//	    OnStatusChange: func(id string, old, new lifecycle.InstanceStatus) {
//	        log.Printf("Instance %s: %s -> %s", id, old, new)
//	    },
//	}
//
//	mgr := lifecycle.NewManager(lifecycle.DefaultConfig(), callbacks, nil)
//
//	// Create an instance
//	inst, err := mgr.CreateInstance("inst-1", "/path/to/worktree", "branch", "task")
//	if err != nil {
//	    return err
//	}
//
//	// Start the instance
//	if err := mgr.StartInstance(ctx, "inst-1"); err != nil {
//	    return err
//	}
//
//	// Update status as needed
//	mgr.UpdateStatus("inst-1", lifecycle.StatusWaitingInput)
//
//	// Stop the instance
//	mgr.StopInstance("inst-1")
//
//	// Clean shutdown
//	mgr.Stop()
//
// # Callback Integration
//
// The manager triggers callbacks for lifecycle events, enabling integration
// with the event bus for decoupled communication:
//
//	callbacks := lifecycle.Callbacks{
//	    OnStatusChange: func(id string, old, new lifecycle.InstanceStatus) {
//	        if new == lifecycle.StatusWorking {
//	            eventBus.Publish(event.NewInstanceStartedEvent(id, ...))
//	        }
//	    },
//	    OnTimeout: func(id string) {
//	        eventBus.Publish(event.NewTimeoutEvent(id, event.TimeoutActivity, ""))
//	    },
//	}
package lifecycle

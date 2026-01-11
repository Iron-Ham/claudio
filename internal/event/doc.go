// Package event provides a pub-sub event bus for decoupled inter-component
// communication in Claudio.
//
// This package enables loose coupling between the TUI, Orchestrator, and other
// components by allowing them to communicate through events rather than direct
// method calls. Components can publish events without knowing who will receive
// them, and subscribe to events without knowing who will produce them.
//
// # Main Types
//
//   - [Event]: Interface that all events must implement, providing EventType() and Timestamp()
//   - [Bus]: Synchronous pub-sub event dispatcher with thread-safe operations
//   - [Handler]: Function type for event handlers (func(Event))
//
// # Event Categories
//
// The package defines several categories of events:
//
// Instance Lifecycle:
//   - [InstanceStartedEvent]: Emitted when a Claude instance begins execution
//   - [InstanceStoppedEvent]: Emitted when a Claude instance stops
//
// Pull Request Events:
//   - [PRCompleteEvent]: Emitted when a PR operation completes
//   - [PROpenedEvent]: Emitted when an inline PR is detected
//
// Status Events:
//   - [TimeoutEvent]: Emitted when an instance times out
//   - [ConflictDetectedEvent]: Emitted when a git conflict is detected
//   - [BellEvent]: Emitted when a terminal bell is detected
//
// Ultra-Plan Events:
//   - [TaskCompletedEvent]: Emitted when an ultra-plan task completes
//   - [PhaseChangeEvent]: Emitted when the ultra-plan phase changes
//   - [MetricsUpdateEvent]: Emitted when instance metrics are updated
//
// # Thread Safety
//
// The [Bus] type is safe for concurrent use. Multiple goroutines can publish
// and subscribe concurrently. Handlers are called synchronously and protected
// against panics - a panicking handler will not prevent other handlers from
// being called.
//
// # Basic Usage
//
//	bus := event.NewBus()
//
//	// Subscribe to specific event types
//	bus.Subscribe("instance.started", func(e event.Event) {
//	    started := e.(event.InstanceStartedEvent)
//	    log.Printf("Instance %s started", started.InstanceID)
//	})
//
//	// Subscribe to all events (useful for logging)
//	bus.SubscribeAll(func(e event.Event) {
//	    log.Printf("Event: %s at %v", e.EventType(), e.Timestamp())
//	})
//
//	// Publish events
//	bus.Publish(event.NewInstanceStartedEvent("inst-1", "/path", "branch", "task"))
//
//	// Unsubscribe when done
//	id := bus.Subscribe("pr.completed", handler)
//	bus.Unsubscribe(id)
//
// # Event Type Naming Convention
//
// Event types follow the pattern "category.action":
//   - instance.started, instance.stopped, instance.timeout, instance.bell
//   - pr.completed, pr.opened
//   - conflict.detected
//   - task.completed
//   - phase.changed
//   - metrics.updated
package event
